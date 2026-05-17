# Agent Sandbox shim 扩展实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `lattice-shim` 独立库中实现 `EgressFilter`（出站访问控制）和 `ForwardListener`（overlay 入站转发），并在 `lattice` 主 repo 的 sandbox 二进制中接入。

**Architecture:** `EgressFilter` 实现 `shim.PolicyChecker` 接口，以 CIDR/域名白名单控制出站流量，通过 `atomic.Pointer` 支持热更新。`ForwardListener` 接受一个 `TCPListenFunc` 工厂函数，在 overlay IP 上监听并将连接双向 relay 到本地目标地址，不依赖任何 Lattice 类型。lattice 主 repo 通过 `go.mod replace` 指向本地 shim 开发。

**Tech Stack:** Go 1.25, `net`, `sync/atomic`, `context`, `io` (stdlib only in shim)；lattice-shim repo：`/Users/francis/workspc/lattice-shim`；lattice repo：`/Users/francis/workspc/lattice`

---

## 文件结构

### lattice-shim repo

| 文件 | 职责 |
|------|------|
| `shim/egress.go` | 新建：`EgressPolicy`, `EgressFilter`, 实现 `PolicyChecker` |
| `shim/egress_test.go` | 新建：EgressFilter 单元测试 |
| `shim/forward.go` | 新建：`ForwardRule`, `ForwardListener`, TCP relay |
| `shim/forward_test.go` | 新建：ForwardListener 单元测试（mock ListenFunc） |

### lattice repo

| 文件 | 职责 |
|------|------|
| `go.mod` | 修改：添加 `replace` 指向本地 lattice-shim |
| `cmd/lattice-agent-sandbox/cmd/start.go` | 修改：添加 `--forward`, `--egress-allow`, `--egress-default-deny` 标志 |
| `cmd/lattice-agent-sandbox/cmd/start_sandbox_pro.go` | 修改：接入 EgressFilter 作为 PolicyChecker；接入 ForwardListener |
| `cmd/lattice-agent-sandbox/cmd/start_sandbox_community.go` | 修改：添加 `StartForward` no-op stub |

---

## Task 1: shim/egress.go — EgressFilter

**Repo:** `/Users/francis/workspc/lattice-shim`

**Files:**
- Create: `shim/egress.go`
- Create: `shim/egress_test.go`

- [ ] **Step 1: 写失败测试**

```go
// shim/egress_test.go
package shim_test

import (
	"net"
	"testing"

	"github.com/alatticeio/lattice-shim/shim"
)

func TestEgressFilter_DefaultAllow(t *testing.T) {
	f := shim.NewEgressFilter(shim.EgressPolicy{DefaultDeny: false})
	if !f.Allow("agent", net.ParseIP("1.2.3.4"), 80) {
		t.Fatal("expected allow when DefaultDeny=false")
	}
}

func TestEgressFilter_DefaultDeny_BlocksUnknown(t *testing.T) {
	f := shim.NewEgressFilter(shim.EgressPolicy{DefaultDeny: true})
	if f.Allow("agent", net.ParseIP("1.2.3.4"), 80) {
		t.Fatal("expected deny when DefaultDeny=true and no CIDRs")
	}
}

func TestEgressFilter_AllowsCIDR(t *testing.T) {
	_, cidr, _ := net.ParseCIDR("10.0.0.0/24")
	f := shim.NewEgressFilter(shim.EgressPolicy{
		DefaultDeny:  true,
		AllowedCIDRs: []*net.IPNet{cidr},
	})
	if !f.Allow("agent", net.ParseIP("10.0.0.5"), 443) {
		t.Fatal("expected allow for IP in allowed CIDR")
	}
	if f.Allow("agent", net.ParseIP("192.168.1.1"), 443) {
		t.Fatal("expected deny for IP outside allowed CIDR")
	}
}

func TestEgressFilter_AllowsDomain(t *testing.T) {
	f := shim.NewEgressFilter(shim.EgressPolicy{
		DefaultDeny:    true,
		AllowedDomains: []string{"127.0.0.1"}, // use loopback to avoid external DNS
	})
	// 127.0.0.1 resolves to itself
	if !f.Allow("agent", net.ParseIP("127.0.0.1"), 443) {
		t.Fatal("expected allow for IP resolved from allowed domain")
	}
}

func TestEgressFilter_Update(t *testing.T) {
	f := shim.NewEgressFilter(shim.EgressPolicy{DefaultDeny: true})
	ip := net.ParseIP("10.0.0.1")
	if f.Allow("agent", ip, 80) {
		t.Fatal("expected deny before update")
	}

	_, cidr, _ := net.ParseCIDR("10.0.0.0/24")
	f.Update(shim.EgressPolicy{
		DefaultDeny:  true,
		AllowedCIDRs: []*net.IPNet{cidr},
	})
	if !f.Allow("agent", ip, 80) {
		t.Fatal("expected allow after update with matching CIDR")
	}
}
```

- [ ] **Step 2: 运行测试，确认失败**

```bash
cd /Users/francis/workspc/lattice-shim
go test ./shim/ -run TestEgressFilter -v
```

预期：`package shim_test: shim.EgressFilter undefined`

- [ ] **Step 3: 实现 shim/egress.go**

```go
// Copyright 2026 The Lattice Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package shim

import (
	"net"
	"sync/atomic"
)

// EgressPolicy defines which outbound connections are allowed.
// When DefaultDeny is false, all traffic is allowed regardless of other fields.
// When DefaultDeny is true, only IPs matching AllowedCIDRs or resolved from
// AllowedDomains are permitted.
type EgressPolicy struct {
	// AllowedCIDRs lists IP ranges that are always permitted.
	AllowedCIDRs []*net.IPNet

	// AllowedDomains lists hostnames whose resolved IPs are permitted.
	// Resolved at Update() time via net.LookupHost; DNS changes require
	// a subsequent Update() call.
	AllowedDomains []string

	// DefaultDeny enables whitelist mode. When false (default), all
	// connections are allowed.
	DefaultDeny bool

	// resolvedIPs is populated by NewEgressFilter and Update from AllowedDomains.
	resolvedIPs []net.IP
}

// EgressFilter implements PolicyChecker with CIDR and domain-based whitelisting.
// It is safe for concurrent use. Policies can be hot-swapped via Update().
type EgressFilter struct {
	policy atomic.Pointer[EgressPolicy]
}

// NewEgressFilter returns a new EgressFilter with the given initial policy.
// Domain names in p.AllowedDomains are resolved immediately.
func NewEgressFilter(p EgressPolicy) *EgressFilter {
	resolved := resolveDomains(p.AllowedDomains)
	p.resolvedIPs = resolved
	f := &EgressFilter{}
	f.policy.Store(&p)
	return f
}

// Update atomically replaces the current policy. Domain names are resolved
// before the swap so concurrent callers always see a consistent policy.
func (f *EgressFilter) Update(p EgressPolicy) {
	resolved := resolveDomains(p.AllowedDomains)
	p.resolvedIPs = resolved
	f.policy.Store(&p)
}

// Allow implements PolicyChecker. Returns true if the connection from identity
// to dstIP:dstPort is permitted by the current policy.
func (f *EgressFilter) Allow(_ string, dstIP net.IP, _ uint16) bool {
	p := f.policy.Load()
	if p == nil || !p.DefaultDeny {
		return true
	}
	for _, cidr := range p.AllowedCIDRs {
		if cidr.Contains(dstIP) {
			return true
		}
	}
	for _, ip := range p.resolvedIPs {
		if ip.Equal(dstIP) {
			return true
		}
	}
	return false
}

// resolveDomains resolves each domain to its IPv4/IPv6 addresses.
// Unresolvable domains are silently skipped.
func resolveDomains(domains []string) []net.IP {
	var out []net.IP
	for _, d := range domains {
		// If it's already an IP literal, parse directly.
		if ip := net.ParseIP(d); ip != nil {
			out = append(out, ip)
			continue
		}
		addrs, err := net.LookupHost(d)
		if err != nil {
			continue
		}
		for _, a := range addrs {
			if ip := net.ParseIP(a); ip != nil {
				out = append(out, ip)
			}
		}
	}
	return out
}
```

- [ ] **Step 4: 运行测试，确认通过**

```bash
cd /Users/francis/workspc/lattice-shim
go test ./shim/ -run TestEgressFilter -v
```

预期：所有 `TestEgressFilter_*` PASS

- [ ] **Step 5: 提交**

```bash
cd /Users/francis/workspc/lattice-shim
git add shim/egress.go shim/egress_test.go
git commit -s -m "feat(shim): add EgressFilter for outbound access control"
```

---

## Task 2: shim/forward.go — ForwardListener

**Repo:** `/Users/francis/workspc/lattice-shim`

**Files:**
- Create: `shim/forward.go`
- Create: `shim/forward_test.go`

- [ ] **Step 1: 写失败测试**

```go
// shim/forward_test.go
package shim_test

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/alatticeio/lattice-shim/shim"
)

// TestForwardListener verifies that connections to the overlay port are relayed
// to the target address. Uses net.Listen as a stand-in for gVisor ListenTCP.
func TestForwardListener_RelaysData(t *testing.T) {
	// 1. Start a real TCP server on a random port (simulates the workload service).
	backend, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("backend listen: %v", err)
	}
	defer backend.Close()
	backendAddr := backend.Addr().String()

	go func() {
		conn, err := backend.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		// Echo server.
		io.Copy(conn, conn) //nolint:errcheck
	}()

	// 2. Create a fake ListenFunc that uses net.Listen (stand-in for shim.Netstack).
	overlayListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("overlay listen: %v", err)
	}
	overlayAddr := overlayListener.Addr().String()
	_, overlayPort, _ := net.SplitHostPort(overlayAddr)

	listenFunc := func(addr string) (net.Listener, error) {
		// Return the pre-created listener when called with overlay addr.
		return overlayListener, nil
	}

	portUint16 := uint16(mustParsePort(t, overlayPort))
	fl := shim.NewForwardListener(listenFunc, "127.0.0.1", []shim.ForwardRule{
		{OverlayPort: portUint16, TargetAddr: backendAddr},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go fl.Start(ctx) //nolint:errcheck

	// 3. Connect to the overlay port, expect relay to backend echo server.
	time.Sleep(50 * time.Millisecond) // allow Start to call listenFunc

	conn, err := net.DialTimeout("tcp", overlayAddr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial overlay: %v", err)
	}
	defer conn.Close()

	msg := []byte("ping")
	if _, err := conn.Write(msg); err != nil {
		t.Fatalf("write: %v", err)
	}
	conn.(*net.TCPConn).CloseWrite() //nolint:errcheck

	got, err := io.ReadAll(conn)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != string(msg) {
		t.Fatalf("expected %q, got %q", msg, got)
	}
}

func mustParsePort(t *testing.T, s string) int {
	t.Helper()
	p, err := net.LookupPort("tcp", s)
	if err != nil {
		t.Fatalf("parse port %q: %v", s, err)
	}
	return p
}
```

- [ ] **Step 2: 运行测试，确认失败**

```bash
cd /Users/francis/workspc/lattice-shim
go test ./shim/ -run TestForwardListener -v
```

预期：`shim.ForwardListener undefined`

- [ ] **Step 3: 实现 shim/forward.go**

```go
// Copyright 2026 The Lattice Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package shim

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
)

// ForwardRule maps an overlay port to a local target address.
type ForwardRule struct {
	// OverlayPort is the TCP port to listen on at the overlay IP.
	OverlayPort uint16

	// TargetAddr is the local address to forward connections to,
	// e.g. "127.0.0.1:8080".
	TargetAddr string
}

// TCPListenFunc creates a net.Listener for the given address.
// Pass shim.Netstack.ListenTCP or any compatible implementation.
type TCPListenFunc func(addr string) (net.Listener, error)

// ForwardListener accepts connections on overlay ports and relays them to
// local target addresses. It is safe for concurrent use.
type ForwardListener struct {
	listen    TCPListenFunc
	overlayIP string
	rules     []ForwardRule
}

// NewForwardListener creates a ForwardListener.
//
// listen is called once per ForwardRule to create the overlay listener.
// Pass ns.ListenTCP (shim.Netstack) or sb.ListenTCP (gvisor.Sandbox).
//
// overlayIP is the local IP address to listen on (e.g. "10.0.0.3").
//
// rules maps overlay ports to local target addresses.
func NewForwardListener(listen TCPListenFunc, overlayIP string, rules []ForwardRule) *ForwardListener {
	return &ForwardListener{
		listen:    listen,
		overlayIP: overlayIP,
		rules:     rules,
	}
}

// Start begins accepting on each rule's overlay port and relaying to the
// corresponding target address. It blocks until ctx is cancelled, then closes
// all listeners.
//
// Returns an error only if a listener cannot be created. Connection errors
// are logged and do not stop the listener.
func (f *ForwardListener) Start(ctx context.Context) error {
	for _, rule := range f.rules {
		addr := fmt.Sprintf("%s:%d", f.overlayIP, rule.OverlayPort)
		ln, err := f.listen(addr)
		if err != nil {
			return fmt.Errorf("forward listen %s: %w", addr, err)
		}
		log.Printf("[forward] listening on %s → %s", addr, rule.TargetAddr)
		go f.accept(ctx, ln, rule.TargetAddr)
	}
	<-ctx.Done()
	return nil
}

func (f *ForwardListener) accept(ctx context.Context, ln net.Listener, target string) {
	// Close the listener when context is done to unblock Accept.
	go func() {
		<-ctx.Done()
		ln.Close()
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go f.relay(conn, target)
	}
}

func (f *ForwardListener) relay(src net.Conn, target string) {
	defer src.Close()
	dst, err := net.Dial("tcp", target)
	if err != nil {
		log.Printf("[forward] dial %s: %v", target, err)
		return
	}
	defer dst.Close()

	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(dst, src); done <- struct{}{} }()
	go func() { _, _ = io.Copy(src, dst); done <- struct{}{} }()
	<-done
}
```

- [ ] **Step 4: 运行测试，确认通过**

```bash
cd /Users/francis/workspc/lattice-shim
go test ./shim/ -run TestForwardListener -v
```

预期：`TestForwardListener_RelaysData PASS`

- [ ] **Step 5: 运行全部测试，确保无回归**

```bash
cd /Users/francis/workspc/lattice-shim
go test ./...
```

预期：全部 PASS

- [ ] **Step 6: 提交**

```bash
cd /Users/francis/workspc/lattice-shim
git add shim/forward.go shim/forward_test.go
git commit -s -m "feat(shim): add ForwardListener for overlay inbound TCP relay"
```

---

## Task 3: lattice go.mod — 添加本地 replace 指令

**Repo:** `/Users/francis/workspc/lattice`

**Files:**
- Modify: `go.mod`

- [ ] **Step 1: 添加 replace 指令**

在 `/Users/francis/workspc/lattice/go.mod` 末尾追加（在现有 `replace` 块中添加，若无 `replace` 块则新建）：

```
replace github.com/alatticeio/lattice-shim => ../lattice-shim
```

- [ ] **Step 2: 同步依赖**

```bash
cd /Users/francis/workspc/lattice
go mod tidy
```

预期：无错误。若报 `go.sum` 变更属正常，一并提交。

- [ ] **Step 3: 确认 shim 新类型可解析**

```bash
cd /Users/francis/workspc/lattice
go build ./cmd/lattice-agent-sandbox/...
```

预期：编译成功（shim 新文件已可见）。

---

## Task 4: start.go + community stub — CLI 标志与 no-op

**Repo:** `/Users/francis/workspc/lattice`

**Files:**
- Modify: `cmd/lattice-agent-sandbox/cmd/start.go`
- Modify: `cmd/lattice-agent-sandbox/cmd/start_sandbox_community.go`

- [ ] **Step 1: 在 start.go 添加新变量和标志**

在 `cmd/lattice-agent-sandbox/cmd/start.go` 的 `var (...)` 块中添加：

```go
sandboxForwardRules  []string
sandboxEgressAllow   string
sandboxEgressDeny    bool
```

在 `func init()` 中添加：

```go
startCmd.Flags().StringArrayVar(&sandboxForwardRules, "forward", nil,
    "Inbound forward rule: overlayPort:targetAddr (e.g. 8080:127.0.0.1:8080), repeatable")
startCmd.Flags().StringVar(&sandboxEgressAllow, "egress-allow", "",
    "Comma-separated CIDRs or domains allowed for egress (e.g. 10.0.0.0/24,api.openai.com)")
startCmd.Flags().BoolVar(&sandboxEgressDeny, "egress-default-deny", false,
    "Enable whitelist egress mode; only --egress-allow entries are permitted")
```

在 `runStart` 函数中，`sb.StartProxy(...)` 调用之后、`<-sigCh` 之前，添加：

```go
// Parse --forward rules.
var fwdRules []shim.ForwardRule
for _, r := range sandboxForwardRules {
    rule, err := parseForwardRule(r)
    if err != nil {
        return fmt.Errorf("parse --forward %q: %w", r, err)
    }
    fwdRules = append(fwdRules, rule)
}

ctx, cancel := context.WithCancel(context.Background())
defer cancel()

if len(fwdRules) > 0 {
    if err := sb.StartForward(ctx, fwdRules); err != nil {
        return fmt.Errorf("start forward listener: %w", err)
    }
}
```

在 `runStart` 中 `<-sigCh` 收到信号后调用 `cancel()`（在 `fmt.Println("\\nShutting down...")` 之前）：

```go
<-sigCh
cancel()
fmt.Println("\nShutting down...")
```

在 `start.go` 末尾（`parsePeerConfig` 之后）添加：

```go
// parseForwardRule parses "overlayPort:targetAddr" into a ForwardRule.
// overlayPort is a decimal port number; targetAddr is "host:port".
// Example: "8080:127.0.0.1:8080"
func parseForwardRule(s string) (shim.ForwardRule, error) {
    idx := strings.IndexByte(s, ':')
    if idx < 0 {
        return shim.ForwardRule{}, fmt.Errorf("expected overlayPort:targetAddr, got %q", s)
    }
    portStr := s[:idx]
    target := s[idx+1:]
    if target == "" {
        return shim.ForwardRule{}, fmt.Errorf("empty targetAddr in %q", s)
    }
    port, err := net.LookupPort("tcp", portStr)
    if err != nil || port < 1 || port > 65535 {
        return shim.ForwardRule{}, fmt.Errorf("invalid overlay port %q", portStr)
    }
    return shim.ForwardRule{
        OverlayPort: uint16(port),
        TargetAddr:  target,
    }, nil
}
```

在文件顶部 `import` 中补充（若缺少）：

```go
"github.com/alatticeio/lattice-shim/shim"
```

- [ ] **Step 2: 在 community stub 添加 StartForward no-op**

在 `/Users/francis/workspc/lattice/cmd/lattice-agent-sandbox/cmd/start_sandbox_community.go` 的 `sandboxCloser` 方法中添加：

```go
func (c *sandboxCloser) StartForward(_ context.Context, _ []shim.ForwardRule) error {
    return nil
}
```

同时在 community stub 文件顶部 `import` 中添加：

```go
"context"
"github.com/alatticeio/lattice-shim/shim"
```

- [ ] **Step 3: 编译确认**

```bash
cd /Users/francis/workspc/lattice
go build ./cmd/lattice-agent-sandbox/...
go build -tags pro ./cmd/lattice-agent-sandbox/...
```

预期：两者均编译成功（PRO 版会缺少 `StartForward` 实现，Step 在 Task 5 补全）。

---

## Task 5: start_sandbox_pro.go — 接入 EgressFilter 和 ForwardListener

**Repo:** `/Users/francis/workspc/lattice`

**Files:**
- Modify: `cmd/lattice-agent-sandbox/cmd/start_sandbox_pro.go`

- [ ] **Step 1: 在 start.go 的 runStart 中构建 EgressPolicy 并传入 createSandbox**

修改 `start.go` 的 `runStart` 函数，在调用 `createSandbox` 之前构建 `EgressPolicy`：

```go
// Build egress policy from flags.
egressPolicy := shim.EgressPolicy{DefaultDeny: sandboxEgressDeny}
if sandboxEgressAllow != "" {
    for _, entry := range strings.Split(sandboxEgressAllow, ",") {
        entry = strings.TrimSpace(entry)
        if entry == "" {
            continue
        }
        if _, cidr, err := net.ParseCIDR(entry); err == nil {
            egressPolicy.AllowedCIDRs = append(egressPolicy.AllowedCIDRs, cidr)
        } else {
            egressPolicy.AllowedDomains = append(egressPolicy.AllowedDomains, entry)
        }
    }
}

sb, err := createSandbox(sandboxName, sandboxLocalIP, agentJWT, sandboxWGEnabled, privateKey, peers, egressPolicy)
```

修改 `createSandbox` 签名（在两个 build-tag 文件中同步修改）：

```go
// start_sandbox_pro.go 和 start_sandbox_community.go 的签名改为：
func createSandbox(sandboxName, localIP, agentJWT string, wgEnabled bool,
    privateKey wgtypes.Key, peers []wgtypes.PeerConfig,
    egressPolicy shim.EgressPolicy) (*sandboxCloser, error)
```

- [ ] **Step 2: 在 start_sandbox_pro.go 中接入 EgressFilter**

在 `createSandbox` 函数中，替换：

```go
var policyChecker shim.PolicyChecker
```

改为：

```go
policyChecker := shim.NewEgressFilter(egressPolicy)
```

（`var policyChecker` 声明删除，直接赋值）

- [ ] **Step 3: 在 sandboxCloser 中添加 fwd 字段，实现 StartForward**

在 `start_sandbox_pro.go` 的 `sandboxCloser` struct 中添加字段：

```go
type sandboxCloser struct {
    sb       *gvisor.Sandbox
    auditF   *os.File
    wgBind   shim.WireGuardBind
    wgDevice *device.Device
    fwd      *shim.ForwardListener // nil if no rules configured
}
```

在 `createSandbox` return 前，不创建 `ForwardListener`（ForwardListener 需要 overlay IP，在 `StartForward` 调用时才知道规则，由 `runStart` 通过 `StartForward` 设置）。

添加 `StartForward` 方法到 `start_sandbox_pro.go`：

```go
// StartForward creates and starts a ForwardListener using the sandbox overlay IP.
// It is non-blocking: relay goroutines run in the background until ctx is cancelled.
func (c *sandboxCloser) StartForward(ctx context.Context, rules []shim.ForwardRule) error {
    fl := shim.NewForwardListener(c.sb.ListenTCP, c.sb.LocalIP(), rules)
    c.fwd = fl
    go func() {
        if err := fl.Start(ctx); err != nil {
            log.Printf("[sandbox] forward listener error: %v", err)
        }
    }()
    return nil
}
```

在 `import` 中确认有 `"context"` 和 `"log"`。

- [ ] **Step 4: 更新 community stub 的 createSandbox 签名**

在 `start_sandbox_community.go` 中，`createSandbox` 签名改为：

```go
func createSandbox(sandboxName, localIP, agentJWT string, wgEnabled bool,
    privateKey wgtypes.Key, peers []wgtypes.PeerConfig,
    egressPolicy shim.EgressPolicy) (*sandboxCloser, error) {
    return nil, errors.New("gVisor agent sandbox is a Lattice Pro feature")
}
```

- [ ] **Step 5: 编译全部**

```bash
cd /Users/francis/workspc/lattice
go build ./cmd/lattice-agent-sandbox/...
go build -tags pro ./cmd/lattice-agent-sandbox/...
make lint
```

预期：编译 + lint 均无错误。

- [ ] **Step 6: 快速功能验证**

```bash
cd /Users/francis/workspc/lattice
# Community 版报 Pro feature 错误（预期）
./bin/lattice-agent-sandbox start --name test --local-ip 10.0.0.1 2>&1 | grep "Pro feature"
```

预期输出包含 `Lattice Pro feature`。

- [ ] **Step 7: 提交**

```bash
cd /Users/francis/workspc/lattice
git add go.mod go.sum \
    cmd/lattice-agent-sandbox/cmd/start.go \
    cmd/lattice-agent-sandbox/cmd/start_sandbox_pro.go \
    cmd/lattice-agent-sandbox/cmd/start_sandbox_community.go
git commit -s -m "feat(agent-sandbox): add EgressFilter and ForwardListener via lattice-shim"
```

---

## 自检

**Spec 覆盖：**
- [x] EgressFilter (CIDR + domain whitelist, 热更新) → Task 1
- [x] ForwardListener (overlay 入站 relay) → Task 2
- [x] `--forward` / `--egress-allow` / `--egress-default-deny` CLI flags → Task 4
- [x] Community stub no-op → Task 4
- [x] PRO 版接入 → Task 5
- [ ] WireGuardManager (NATS 动态 peer) → **不在本 Plan 范围，单独 Plan**
- [ ] eBPF 透明代理 → **不在本 Plan 范围，PRO P4 单独 Plan**

**类型一致性：**
- `shim.ForwardRule` 定义于 Task 2，在 Task 4 (`parseForwardRule`) 和 Task 5 (`StartForward`) 中使用 ✓
- `shim.EgressPolicy` 定义于 Task 1，在 Task 4 (`runStart`) 和 Task 5 (`createSandbox`) 中使用 ✓
- `shim.TCPListenFunc` 定义于 Task 2，在 Task 5 (`shim.NewForwardListener(c.sb.ListenTCP, ...)`) 中使用 ✓ — `gvisor.Sandbox.ListenTCP` 签名为 `func(addr string) (net.Listener, error)`，与 `TCPListenFunc` 一致 ✓
- `createSandbox` 新签名在 Task 5 Step 1 定义，Task 5 Step 4 同步到 community stub ✓
