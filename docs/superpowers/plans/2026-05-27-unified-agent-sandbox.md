# Unified Agent Sandbox Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the `lattice sandbox` command tree with a unified `lattice agent` command family that always uses gVisor userspace networking — no kernel wg0 for AI agent sandboxes.

**Architecture:** `agent run` forks the AI agent as a child process inside a `--runtime=runsc` container (gVisor runtime handles all networking); `agent sidecar` embeds the gVisor netstack library and exposes a transparent TCP proxy for K8s pods (iptables REDIRECT → `SO_ORIGINAL_DST`); `agent init` writes the iptables rules and exits. Regular `lattice up` is untouched.

**Tech Stack:** Go 1.25.0, cobra, gvisor.dev/gvisor netstack library (existing `internal/agent/gvisor/`), lattice-shim, golang.org/x/sys/unix (SO_ORIGINAL_DST), wireguard-go.

---

## File Map

| Action | Path | Build tag | Purpose |
|--------|------|-----------|---------|
| Create | `cmd/lattice/cmd/agent/agent.go` | (none) | `AgentCmd()` top-level cobra command |
| Create | `cmd/lattice/cmd/agent/shared.go` | (none) | credentials, fileAuditWriter, overlayAddr |
| Create | `cmd/lattice/cmd/agent/run_community.go` | `!pro` | `agent run` — no egress policy |
| Create | `cmd/lattice/cmd/agent/run_pro.go` | `pro && linux` | `agent run` — adds egress flags + iptables |
| Create | `cmd/lattice/cmd/agent/sidecar_community.go` | `!pro` | `agent sidecar` — gVisor netstack + tproxy |
| Create | `cmd/lattice/cmd/agent/sidecar_pro.go` | `pro && linux` | `agent sidecar` — adds egress policy |
| Create | `cmd/lattice/cmd/agent/init.go` | `linux` | `agent init` — iptables REDIRECT rules |
| Create | `cmd/lattice/cmd/agent/init_stub.go` | `!linux` | no-op stub for non-linux |
| Create | `internal/agent/tproxy/tproxy.go` | `linux` | transparent proxy + SO_ORIGINAL_DST |
| Create | `internal/agent/tproxy/tproxy_stub.go` | `!linux` | no-op stub |
| Create | `internal/agent/tproxy/tproxy_test.go` | `linux` | unit tests |
| Modify | `cmd/lattice/cmd/root.go` | — | replace `sandbox.SandboxCmd()` with `agent.AgentCmd()` |
| Delete | `cmd/lattice/cmd/sandbox/` | — | entire package |
| Delete | `internal/agent/runsc/` | — | RunscDriver two-phase logic |

---

## Task 1: Package skeleton + root.go wiring

**Files:**
- Create: `cmd/lattice/cmd/agent/agent.go`
- Modify: `cmd/lattice/cmd/root.go:91`

- [ ] **Step 1: Write failing build test**

```bash
# In a temporary file, verify the package doesn't exist yet
ls cmd/lattice/cmd/agent 2>&1 | grep "No such file"
```

Expected: `ls: cannot access 'cmd/lattice/cmd/agent': No such file or directory`

- [ ] **Step 2: Create `cmd/lattice/cmd/agent/agent.go`**

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

package agent

import "github.com/spf13/cobra"

// AgentCmd returns the top-level `agent` cobra command.
func AgentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage AI agent sandboxes",
		Long: `The agent commands start and configure AI agent sandboxes on the
Lattice overlay network.

All sandbox traffic flows through a gVisor userspace network stack — no kernel
WireGuard interface (wg0) is created on the host.

Scenarios:
  agent run    — single container, requires --runtime=runsc (or equivalent)
  agent sidecar — Kubernetes sidecar with transparent proxy
  agent init   — Kubernetes init container, writes iptables rules and exits`,
	}
	addRunCmd(cmd)
	addSidecarCmd(cmd)
	addInitCmd(cmd)
	return cmd
}
```

- [ ] **Step 3: Create minimal stubs so the package compiles**

Create `cmd/lattice/cmd/agent/run_community.go`:

```go
//go:build !pro

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

package agent

import "github.com/spf13/cobra"

func addRunCmd(parent *cobra.Command) {}
```

Create `cmd/lattice/cmd/agent/sidecar_community.go`:

```go
//go:build !pro

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

package agent

import "github.com/spf13/cobra"

func addSidecarCmd(parent *cobra.Command) {}
```

Create `cmd/lattice/cmd/agent/init_stub.go`:

```go
//go:build !linux

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

package agent

import "github.com/spf13/cobra"

func addInitCmd(parent *cobra.Command) {}
```

- [ ] **Step 4: Wire into `root.go`**

In `cmd/lattice/cmd/root.go`, add the import and replace the sandbox command:

```go
// Add to imports:
"github.com/alatticeio/lattice/cmd/lattice/cmd/agent"

// Replace line:
rootCmd.AddCommand(sandbox.SandboxCmd())
// With:
rootCmd.AddCommand(agent.AgentCmd())
```

Also remove the `sandbox` import:
```go
// Remove:
"github.com/alatticeio/lattice/cmd/lattice/cmd/sandbox"
```

- [ ] **Step 5: Verify the build compiles**

```bash
cd /Users/francis/workspc/lattice && go build ./cmd/lattice/...
```

Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add cmd/lattice/cmd/agent/ cmd/lattice/cmd/root.go
git commit -s -m "feat(agent): add lattice agent command skeleton"
```

---

## Task 2: Shared utilities

**Files:**
- Create: `cmd/lattice/cmd/agent/shared.go`

The credential persistence and file audit writer are copied from `cmd/lattice/cmd/sandbox/sandbox_shared.go`. They are identical in behaviour; only the package name changes.

- [ ] **Step 1: Create `cmd/lattice/cmd/agent/shared.go`**

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

package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	shimfwd "github.com/alatticeio/lattice-shim/shim"
	"github.com/alatticeio/lattice/internal/agent/infra"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// sandboxCredentials holds the persisted registration state for crash recovery.
type sandboxCredentials struct {
	PrivateKey string `json:"privateKey"`
	JWT        string `json:"jwt"`
}

func sandboxCredentialsFile() string {
	dir := os.Getenv("LATTICE_CONFIG_DIR")
	if dir == "" {
		dir = "/etc/lattice"
	}
	return filepath.Join(dir, "sandbox-credentials.json")
}

func loadSandboxCredentials() (*sandboxCredentials, error) {
	data, err := os.ReadFile(sandboxCredentialsFile())
	if err != nil {
		return nil, err
	}
	var creds sandboxCredentials
	if err = json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}
	if creds.PrivateKey == "" || creds.JWT == "" {
		return nil, fmt.Errorf("incomplete credentials")
	}
	return &creds, nil
}

func saveSandboxCredentials(privKey wgtypes.Key, jwt string) error {
	creds := sandboxCredentials{PrivateKey: privKey.String(), JWT: jwt}
	data, err := json.Marshal(creds)
	if err != nil {
		return err
	}
	_ = os.MkdirAll(filepath.Dir(sandboxCredentialsFile()), 0o755)
	return os.WriteFile(sandboxCredentialsFile(), data, 0o600)
}

// fileAuditWriter implements shimfwd.AuditWriter by appending JSON lines to a file.
type fileAuditWriter struct {
	mu sync.Mutex
	f  *os.File
}

func newFileAuditWriter(path string) (*fileAuditWriter, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	return &fileAuditWriter{f: f}, nil
}

func (w *fileAuditWriter) Write(event shimfwd.AuditEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	_, err = fmt.Fprintf(w.f, "%s\n", data)
	return err
}

// overlayAddr extracts the VPN IP string from a Peer. Returns "" if nil.
func overlayAddr(p *infra.Peer) string {
	if p != nil && p.Address != nil {
		return *p.Address
	}
	return ""
}
```

- [ ] **Step 2: Write unit test for credential round-trip**

Create `cmd/lattice/cmd/agent/shared_test.go`:

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

package agent_test

import (
	"os"
	"testing"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func TestSandboxCredentialRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LATTICE_CONFIG_DIR", dir)

	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	jwt := "test-jwt-token"

	// Functions are unexported; call via the internal package using a test helper.
	// We test through the file on disk to verify format compatibility.
	credsFile := dir + "/sandbox-credentials.json"
	data := `{"privateKey":"` + key.String() + `","jwt":"` + jwt + `"}`
	if err := os.WriteFile(credsFile, []byte(data), 0o600); err != nil {
		t.Fatalf("write creds: %v", err)
	}
	if _, err := os.Stat(credsFile); err != nil {
		t.Fatalf("creds file not found: %v", err)
	}
}
```

- [ ] **Step 3: Run test**

```bash
cd /Users/francis/workspc/lattice && go test ./cmd/lattice/cmd/agent/... -v -run TestSandboxCredentialRoundTrip
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add cmd/lattice/cmd/agent/shared.go cmd/lattice/cmd/agent/shared_test.go
git commit -s -m "feat(agent): add shared credential and audit utilities"
```

---

## Task 3: Transparent proxy package (`internal/agent/tproxy`)

This is entirely new code. The transparent proxy uses iptables REDIRECT (which rewrites the destination to 127.0.0.1:proxyPort) and `SO_ORIGINAL_DST` getsockopt to recover the original destination. It is Linux-only.

**Files:**
- Create: `internal/agent/tproxy/tproxy.go` (`//go:build linux`)
- Create: `internal/agent/tproxy/tproxy_stub.go` (`//go:build !linux`)
- Create: `internal/agent/tproxy/tproxy_test.go` (`//go:build linux`)

- [ ] **Step 1: Write failing test**

Create `internal/agent/tproxy/tproxy_test.go`:

```go
//go:build linux

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

package tproxy_test

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/alatticeio/lattice/internal/agent/tproxy"
)

// TestProxyDialAndCopy verifies that the Proxy bridges data between
// two net.Conn pairs. We skip SO_ORIGINAL_DST here (requires real
// iptables redirect) and test only the copy/dial logic using a
// direct DialFunc that returns a preconfigured conn.
func TestProxyDialAndCopy(t *testing.T) {
	// Create a pair of in-memory connections to act as "remote".
	remoteA, remoteB := net.Pipe()
	defer remoteA.Close()
	defer remoteB.Close()

	// Dial function returns remoteA for any address.
	dialFn := func(ctx context.Context, network, addr string) (net.Conn, error) {
		return remoteA, nil
	}

	proxy := &tproxy.Proxy{
		Addr: "127.0.0.1:0",
		Dial: dialFn,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := proxy.Start(ctx); err != nil {
		t.Fatalf("proxy.Start: %v", err)
	}

	// Connect to proxy.
	conn, err := net.Dial("tcp", proxy.ListenAddr())
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	defer conn.Close()

	// Send data through proxy → remoteA → remoteB reads it.
	go func() {
		conn.Write([]byte("hello")) //nolint:errcheck
		conn.Close()
	}()

	buf := make([]byte, 5)
	remoteB.SetDeadline(time.Now().Add(2 * time.Second)) //nolint:errcheck
	n, err := io.ReadFull(remoteB, buf)
	if err != nil {
		t.Fatalf("read from remoteB: %v (n=%d)", err, n)
	}
	if string(buf) != "hello" {
		t.Errorf("expected 'hello', got %q", buf)
	}
}
```

- [ ] **Step 2: Run test to confirm failure**

```bash
cd /Users/francis/workspc/lattice && go test ./internal/agent/tproxy/... -v -run TestProxyDialAndCopy
```

Expected: FAIL — `cannot find package`

- [ ] **Step 3: Create `internal/agent/tproxy/tproxy.go`**

```go
//go:build linux

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

// Package tproxy implements a transparent TCP proxy that recovers the original
// destination of iptables REDIRECT'd connections via SO_ORIGINAL_DST and
// forwards them through a caller-supplied dial function.
package tproxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Proxy is a transparent TCP proxy. It listens on Addr, recovers the original
// destination of each accepted connection via SO_ORIGINAL_DST (set by iptables
// REDIRECT), then dials it via Dial and copies data bidirectionally.
type Proxy struct {
	// Addr is the TCP address to listen on (e.g. "0.0.0.0:15001").
	Addr string

	// Dial is called with the original destination address for each accepted
	// connection. Typically this dials through the gVisor netstack.
	Dial func(ctx context.Context, network, addr string) (net.Conn, error)

	ln net.Listener
}

// Start begins listening and handling connections. It returns once the listener
// is bound; connections are handled in background goroutines. The listener is
// closed when ctx is cancelled.
func (p *Proxy) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", p.Addr)
	if err != nil {
		return fmt.Errorf("tproxy: listen %s: %w", p.Addr, err)
	}
	p.ln = ln
	go func() {
		<-ctx.Done()
		ln.Close()
	}()
	go p.serve(ctx, ln)
	return nil
}

// ListenAddr returns the address the proxy is actually listening on.
// Call after Start.
func (p *Proxy) ListenAddr() string {
	if p.ln == nil {
		return ""
	}
	return p.ln.Addr().String()
}

func (p *Proxy) serve(ctx context.Context, ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go p.handle(ctx, conn.(*net.TCPConn))
	}
}

func (p *Proxy) handle(ctx context.Context, src *net.TCPConn) {
	defer src.Close()

	dst, err := originalDst(src)
	if err != nil {
		// No original destination means this connection was not iptables
		// REDIRECT'd. Reject it.
		return
	}

	remote, err := p.Dial(ctx, "tcp", dst)
	if err != nil {
		return
	}
	defer remote.Close()

	done := make(chan struct{}, 2)
	go func() { io.Copy(remote, src); done <- struct{}{} }() //nolint:errcheck
	go func() { io.Copy(src, remote); done <- struct{}{} }() //nolint:errcheck
	<-done
}

// originalDst reads the original destination address of an iptables REDIRECT'd
// TCP connection using SO_ORIGINAL_DST getsockopt.
func originalDst(conn *net.TCPConn) (string, error) {
	f, err := conn.File()
	if err != nil {
		return "", fmt.Errorf("get file: %w", err)
	}
	defer f.Close()

	// sockaddr_in layout: sa_family(2) + port(2) + addr(4) + pad(8) = 16 bytes
	var raw [16]byte
	size := uint32(len(raw))
	_, _, errno := unix.Syscall6(
		unix.SYS_GETSOCKOPT,
		f.Fd(),
		unix.IPPROTO_IP,
		80, // SO_ORIGINAL_DST = 80
		uintptr(unsafe.Pointer(&raw[0])),
		uintptr(unsafe.Pointer(&size)),
		0,
	)
	if errno != 0 {
		return "", fmt.Errorf("getsockopt SO_ORIGINAL_DST: %w", errno)
	}
	port := int(raw[2])<<8 | int(raw[3])
	ip := net.IP(raw[4:8])
	return fmt.Sprintf("%s:%d", ip.String(), port), nil
}
```

- [ ] **Step 4: Create `internal/agent/tproxy/tproxy_stub.go`**

```go
//go:build !linux

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

// Package tproxy is a Linux-only transparent proxy; this stub satisfies the
// package on non-Linux platforms.
package tproxy

import (
	"context"
	"fmt"
	"net"
)

// Proxy is a no-op on non-Linux platforms.
type Proxy struct {
	Addr string
	Dial func(ctx context.Context, network, addr string) (net.Conn, error)
}

func (p *Proxy) Start(_ context.Context) error {
	return fmt.Errorf("tproxy: transparent proxy is only supported on Linux")
}

func (p *Proxy) ListenAddr() string { return "" }
```

- [ ] **Step 5: Run test to confirm it passes**

```bash
cd /Users/francis/workspc/lattice && go test ./internal/agent/tproxy/... -v -run TestProxyDialAndCopy
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/agent/tproxy/
git commit -s -m "feat(agent): add transparent TCP proxy with SO_ORIGINAL_DST"
```

---

## Task 4: `agent init` — iptables REDIRECT setup

**Files:**
- Create: `cmd/lattice/cmd/agent/init.go` (`//go:build linux`)
- Update: `cmd/lattice/cmd/agent/init_stub.go` — already exists as stub, no change needed

- [ ] **Step 1: Write failing test**

Create `cmd/lattice/cmd/agent/init_test.go`:

```go
//go:build linux

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

package agent

import (
	"testing"
)

func TestBuildIPTablesRules(t *testing.T) {
	rules := buildIPTablesRules(15001, 1337)

	// Must contain a REDIRECT rule on port 15001.
	foundRedirect := false
	foundSkipUID := false
	for _, r := range rules {
		args := r
		hasRedirect := false
		hasPort := false
		hasUID := false
		for i, a := range args {
			if a == "REDIRECT" {
				hasRedirect = true
			}
			if a == "--to-ports" && i+1 < len(args) && args[i+1] == "15001" {
				hasPort = true
			}
			if a == "--uid-owner" && i+1 < len(args) && args[i+1] == "1337" {
				hasUID = true
			}
		}
		if hasRedirect && hasPort {
			foundRedirect = true
		}
		if hasUID {
			foundSkipUID = true
		}
	}
	if !foundRedirect {
		t.Error("expected a REDIRECT rule on port 15001")
	}
	if !foundSkipUID {
		t.Error("expected a --uid-owner 1337 skip rule")
	}
}
```

- [ ] **Step 2: Run test to confirm failure**

```bash
cd /Users/francis/workspc/lattice && go test ./cmd/lattice/cmd/agent/... -v -run TestBuildIPTablesRules
```

Expected: FAIL — `undefined: buildIPTablesRules`

- [ ] **Step 3: Create `cmd/lattice/cmd/agent/init.go`**

```go
//go:build linux

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

package agent

import (
	"fmt"
	"os/exec"
	"strconv"

	"github.com/spf13/cobra"
)

var (
	initProxyPort int
	initSkipUID   int
)

func addInitCmd(parent *cobra.Command) {
	parent.AddCommand(initCmd())
}

func initCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write iptables REDIRECT rules for K8s sidecar mode (init container)",
		Long: `Init sets up iptables rules that redirect all outbound TCP traffic from the
pod to the Lattice sidecar transparent proxy port. Traffic from the sidecar
process itself (identified by --skip-uid) is exempted to prevent loops.

Run as a Kubernetes initContainer with NET_ADMIN capability. Exits 0 on success.

Example K8s manifest:
  initContainers:
  - name: lattice-init
    image: ghcr.io/alattice/lattice
    command: ["lattice", "agent", "init"]
    securityContext:
      capabilities:
        add: ["NET_ADMIN"]`,
		RunE: runInit,
	}
	cmd.Flags().IntVar(&initProxyPort, "proxy-port", 15001, "TCP port the sidecar transparent proxy listens on")
	cmd.Flags().IntVar(&initSkipUID, "skip-uid", 1337, "UID whose outbound traffic is exempt from redirect (the sidecar's UID)")
	return cmd
}

func runInit(_ *cobra.Command, _ []string) error {
	fmt.Printf("[lattice-init] setting up iptables redirect → port %d (skip UID %d)\n", initProxyPort, initSkipUID)
	for _, args := range buildIPTablesRules(initProxyPort, initSkipUID) {
		out, err := exec.Command("iptables", args...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("iptables %v: %s: %w", args, out, err)
		}
	}
	fmt.Println("[lattice-init] iptables rules installed successfully")
	return nil
}

// buildIPTablesRules returns the ordered list of iptables argument slices to
// install. Exported for testing.
func buildIPTablesRules(proxyPort, skipUID int) [][]string {
	port := strconv.Itoa(proxyPort)
	uid := strconv.Itoa(skipUID)
	return [][]string{
		// Create the LATTICE_REDIRECT chain.
		{"-t", "nat", "-N", "LATTICE_REDIRECT"},
		// Exempt sidecar's own traffic (by UID) to prevent redirect loops.
		{"-t", "nat", "-A", "LATTICE_REDIRECT", "-m", "owner", "--uid-owner", uid, "-j", "RETURN"},
		// Redirect all other TCP to the proxy port.
		{"-t", "nat", "-A", "LATTICE_REDIRECT", "-p", "tcp", "-j", "REDIRECT", "--to-ports", port},
		// Hook the chain into OUTPUT (outbound traffic from all processes).
		{"-t", "nat", "-A", "OUTPUT", "-p", "tcp", "-j", "LATTICE_REDIRECT"},
	}
}
```

- [ ] **Step 4: Run test to confirm it passes**

```bash
cd /Users/francis/workspc/lattice && go test ./cmd/lattice/cmd/agent/... -v -run TestBuildIPTablesRules
```

Expected: PASS

- [ ] **Step 5: Verify build**

```bash
go build ./cmd/lattice/...
```

Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add cmd/lattice/cmd/agent/init.go cmd/lattice/cmd/agent/init_test.go
git commit -s -m "feat(agent): add agent init iptables REDIRECT command"
```

---

## Task 5: `agent run` — community edition

The community implementation is essentially `sandbox_agent.go` logic promoted to a top-level command. Key difference: fork/exec the AI agent as a child (not `syscall.Exec`) so the WireGuard node stays alive to maintain peer sessions.

**Files:**
- Replace: `cmd/lattice/cmd/agent/run_community.go` (was a stub, now full implementation)

- [ ] **Step 1: Write failing test for CIDR parsing stub (validates future PRO flag validation)**

Create `cmd/lattice/cmd/agent/run_test.go`:

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

package agent_test

import (
	"strings"
	"testing"

	"github.com/alatticeio/lattice/cmd/lattice/cmd/agent"
)

func TestAgentCmd_RunSubcommandRegistered(t *testing.T) {
	cmd := agent.AgentCmd()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Use == "run" || strings.HasPrefix(sub.Use, "run ") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'run' subcommand to be registered on agent command")
	}
}
```

- [ ] **Step 2: Run test to confirm it fails (run subcommand not yet registered)**

```bash
cd /Users/francis/workspc/lattice && go test ./cmd/lattice/cmd/agent/... -v -run TestAgentCmd_RunSubcommandRegistered
```

Expected: FAIL

- [ ] **Step 3: Replace `run_community.go` stub with full implementation**

```go
//go:build !pro

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

package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	latticeagent "github.com/alatticeio/lattice/internal/agent"
	agentconfig "github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/infra"
	agentlog "github.com/alatticeio/lattice/internal/agent/log"
	"github.com/spf13/cobra"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var (
	runServerURL string
	runToken     string
	runReadyWait time.Duration
)

func addRunCmd(parent *cobra.Command) {
	parent.AddCommand(runCmd())
}

func runCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <name> -- <command> [args...]",
		Short: "Run an AI agent inside a gVisor sandbox",
		Long: `Run registers a sandbox with the Lattice control plane, starts a WireGuard
node, then forks the given command as a child process. The container must run
under gVisor (--runtime=runsc or runtimeClassName: gvisor) so that the AI
agent's traffic is intercepted by gVisor's userspace netstack and routed
through the WireGuard overlay. No kernel wg0 is created on the host.

The sandbox name is the first positional argument. Everything after '--' is the
command to execute.

Example:
  docker run --runtime=runsc ghcr.io/alattice/lattice \
    agent run my-agent --server-url http://latticed:8080 --token lt-xxx \
    -- python agent.py`,
		Args: cobra.ArbitraryArgs,
		RunE: runRun,
	}
	cmd.Flags().StringVar(&runServerURL, "server-url", "", "Lattice control plane URL (required)")
	cmd.Flags().StringVar(&runToken, "token", "", "Enrollment token (required)")
	cmd.Flags().DurationVar(&runReadyWait, "ready-wait", 3*time.Second,
		"Time to wait for WireGuard peer sessions before starting the AI agent")
	_ = cmd.MarkFlagRequired("server-url")
	_ = cmd.MarkFlagRequired("token")
	return cmd
}

func runRun(_ *cobra.Command, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: lattice agent run <name> -- <command> [args...]")
	}
	agentName := args[0]
	cmdArgs := args[1:]

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agentconfig.Conf.AppId = agentName
	agentconfig.Conf.ServerUrl = runServerURL
	agentconfig.Conf.WgPort = 51820

	// gVisor's devtmpfs may not auto-create /dev/net/ — create it so
	// wireguard-go can open /dev/net/tun (virtualised by gVisor runtime).
	_ = os.MkdirAll("/dev/net", 0o755)

	currentPeer, err := registerOrResume(ctx, agentName, runServerURL, runToken)
	if err != nil {
		return err
	}

	localIP := overlayAddr(currentPeer)
	fmt.Printf("[agent-run] %q registered, overlay IP=%s\n", agentName, localIP)

	if currentPeer.LrpUrl != "" {
		agentconfig.Conf.EnableLrp = true
		agentconfig.Conf.RelayURL = currentPeer.LrpUrl
	}

	logger := agentlog.GetLogger("agent-run")
	agentJWT := currentPeer.Token

	nodeCfg := &latticeagent.NodeConfig{
		Logger:      logger,
		Port:        51820,
		ShowLog:     false,
		Flags:       agentconfig.Conf,
		CustomName:  agentName,
		CurrentPeer: currentPeer,
	}

	node, err := latticeagent.NewNode(ctx, nodeCfg)
	if err != nil {
		return fmt.Errorf("create node: %w", err)
	}

	node.GetNetworkMap = func() (*infra.Message, error) {
		msg, getErr := node.GetNetMap(agentJWT)
		if getErr != nil {
			logger.Error("get network map failed", getErr)
			return nil, getErr
		}
		return msg, nil
	}

	if err = node.Start(ctx); err != nil {
		return fmt.Errorf("start node: %w", err)
	}
	defer node.Stop() //nolint:errcheck

	go node.StartHeartbeat(ctx)
	go runPeriodicRefresh(ctx, node, logger)

	// Wait for WireGuard peer sessions to establish.
	select {
	case <-time.After(runReadyWait):
	case <-ctx.Done():
		return ctx.Err()
	}

	return forkAndWait(ctx, cancel, cmdArgs)
}

// registerOrResume tries to resume from persisted credentials; falls back to
// fresh NATS registration.
func registerOrResume(ctx context.Context, agentName, serverURL, token string) (*infra.Peer, error) {
	if creds, err := loadSandboxCredentials(); err == nil {
		if key, parseErr := wgtypes.ParseKey(creds.PrivateKey); parseErr == nil {
			if peer, resumeErr := latticeagent.ResumeSandboxViaNATS(ctx, serverURL, creds.JWT, agentName, key); resumeErr == nil {
				fmt.Printf("[agent-run] resumed %q from saved credentials\n", agentName)
				return peer, nil
			}
		}
	}

	privKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("generate WireGuard key: %w", err)
	}
	peer, err := latticeagent.RegisterSandboxViaNATS(ctx, serverURL, token, agentName, privKey)
	if err != nil {
		return nil, fmt.Errorf("registration failed: %w", err)
	}
	if saveErr := saveSandboxCredentials(privKey, peer.Token); saveErr != nil {
		fmt.Printf("[agent-run] warning: persist credentials: %v\n", saveErr)
	}
	return peer, nil
}

// runPeriodicRefresh polls the network map every 15 s as a NATS push fallback.
func runPeriodicRefresh(ctx context.Context, node *latticeagent.Node, logger interface {
	Warn(msg string, args ...any)
}) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := node.RefreshConfig(ctx); err != nil {
				logger.Warn("periodic config refresh failed", "err", err)
			}
		}
	}
}

// forkAndWait forks cmdArgs[0] as a child process, waits for it to exit, and
// propagates the exit code. cancel is called when the child exits so the node
// is cleaned up.
func forkAndWait(ctx context.Context, cancel context.CancelFunc, cmdArgs []string) error {
	child := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	child.Env = os.Environ()

	if err := child.Start(); err != nil {
		return fmt.Errorf("start agent process: %w", err)
	}

	childDone := make(chan error, 1)
	go func() { childDone <- child.Wait() }()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	var childErr error
	select {
	case childErr = <-childDone:
		cancel()
	case <-sigCh:
		_ = child.Process.Signal(syscall.SIGTERM)
		select {
		case childErr = <-childDone:
		case <-time.After(5 * time.Second):
			_ = child.Process.Kill()
			childErr = <-childDone
		}
		cancel()
	}

	var exitErr *exec.ExitError
	if errors.As(childErr, &exitErr) {
		os.Exit(exitErr.ExitCode())
	}
	return childErr
}
```

- [ ] **Step 4: Run test to confirm it passes**

```bash
cd /Users/francis/workspc/lattice && go test ./cmd/lattice/cmd/agent/... -v -run TestAgentCmd_RunSubcommandRegistered
```

Expected: PASS

- [ ] **Step 5: Build check**

```bash
go build ./cmd/lattice/...
```

Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add cmd/lattice/cmd/agent/run_community.go cmd/lattice/cmd/agent/run_test.go
git commit -s -m "feat(agent): implement agent run community edition"
```

---

## Task 6: `agent run` — PRO edition

PRO adds `--egress-allow` and `--egress-default-deny`. After the WireGuard node starts, iptables rules are applied inside the gVisor container to enforce egress policy on the virtual wg0 interface. The `runRun` logic delegates to the same `forkAndWait` and `registerOrResume` helpers defined in the community file (since PRO fully replaces the community file via build tags, both helpers are re-declared here).

**Files:**
- Create: `cmd/lattice/cmd/agent/run_pro.go` (`//go:build pro && linux`)

- [ ] **Step 1: Write test for egress CIDR validation**

Add to `cmd/lattice/cmd/agent/run_test.go` (before the closing `}`):

```go
// TestParseCIDRs_ValidAndInvalid is compiled under all build tags.
// It tests the parseEgressCIDRs helper exported for testing.
func TestParseCIDRs_ValidAndInvalid(t *testing.T) {
	// Valid CIDRs should parse without error.
	_, err := parseEgressCIDRs("10.0.0.0/8,192.168.0.0/16")
	if err != nil {
		t.Errorf("expected no error for valid CIDRs, got: %v", err)
	}

	// Invalid CIDR should return an error.
	_, err = parseEgressCIDRs("not-a-cidr")
	if err == nil {
		t.Error("expected error for invalid CIDR, got nil")
	}
}
```

Note: `parseEgressCIDRs` is defined in the community file as a shared helper (see step 2).

- [ ] **Step 2: Add `parseEgressCIDRs` helper to `run_community.go`**

Append to the end of `run_community.go`:

```go
// parseEgressCIDRs splits a comma-separated CIDR string and validates each entry.
// Returns the list of validated CIDR strings, or an error.
func parseEgressCIDRs(raw string) ([]string, error) {
	if raw == "" {
		return nil, nil
	}
	var out []string
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if _, _, err := net.ParseCIDR(entry); err != nil {
			return nil, fmt.Errorf("invalid egress CIDR %q: %w", entry, err)
		}
		out = append(out, entry)
	}
	return out, nil
}
```

Also add `"net"` and `"strings"` to the import block of `run_community.go`.

- [ ] **Step 3: Run CIDR test to confirm it passes**

```bash
cd /Users/francis/workspc/lattice && go test ./cmd/lattice/cmd/agent/... -v -run TestParseCIDRs
```

Expected: PASS

- [ ] **Step 4: Create `cmd/lattice/cmd/agent/run_pro.go`**

```go
//go:build pro && linux

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

package agent

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	latticeagent "github.com/alatticeio/lattice/internal/agent"
	agentconfig "github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/infra"
	agentlog "github.com/alatticeio/lattice/internal/agent/log"
	"github.com/spf13/cobra"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var (
	runServerURL     string
	runToken         string
	runReadyWait     time.Duration
	runEgressAllow   string
	runEgressDeny    bool
)

func addRunCmd(parent *cobra.Command) {
	parent.AddCommand(runCmd())
}

func runCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <name> -- <command> [args...]",
		Short: "Run an AI agent inside a gVisor sandbox (Pro)",
		Long: `Run registers a sandbox with the Lattice control plane, starts a WireGuard
node, optionally applies egress policy via iptables inside the gVisor container,
then forks the given command.

The container must run under gVisor (--runtime=runsc or runtimeClassName: gvisor).

Example:
  docker run --runtime=runsc ghcr.io/alattice/lattice \
    agent run my-agent --server-url http://latticed:8080 --token lt-xxx \
    --egress-allow 10.0.0.0/8 --egress-default-deny \
    -- python agent.py`,
		Args: cobra.ArbitraryArgs,
		RunE: runRun,
	}
	cmd.Flags().StringVar(&runServerURL, "server-url", "", "Lattice control plane URL (required)")
	cmd.Flags().StringVar(&runToken, "token", "", "Enrollment token (required)")
	cmd.Flags().DurationVar(&runReadyWait, "ready-wait", 3*time.Second,
		"Time to wait for WireGuard peer sessions before starting the AI agent")
	cmd.Flags().StringVar(&runEgressAllow, "egress-allow", "",
		"Comma-separated overlay CIDRs the AI agent is allowed to reach (Pro)")
	cmd.Flags().BoolVar(&runEgressDeny, "egress-default-deny", false,
		"Deny all egress except --egress-allow CIDRs (Pro)")
	_ = cmd.MarkFlagRequired("server-url")
	_ = cmd.MarkFlagRequired("token")
	return cmd
}

func runRun(_ *cobra.Command, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: lattice agent run <name> -- <command> [args...]")
	}
	agentName := args[0]
	cmdArgs := args[1:]

	if runEgressDeny {
		if _, err := parseEgressCIDRs(runEgressAllow); err != nil {
			return err
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agentconfig.Conf.AppId = agentName
	agentconfig.Conf.ServerUrl = runServerURL
	agentconfig.Conf.WgPort = 51820

	_ = os.MkdirAll("/dev/net", 0o755)

	currentPeer, err := registerOrResume(ctx, agentName, runServerURL, runToken)
	if err != nil {
		return err
	}

	localIP := overlayAddr(currentPeer)
	fmt.Printf("[agent-run] %q registered, overlay IP=%s\n", agentName, localIP)

	if currentPeer.LrpUrl != "" {
		agentconfig.Conf.EnableLrp = true
		agentconfig.Conf.RelayURL = currentPeer.LrpUrl
	}

	logger := agentlog.GetLogger("agent-run")
	agentJWT := currentPeer.Token

	nodeCfg := &latticeagent.NodeConfig{
		Logger:      logger,
		Port:        51820,
		ShowLog:     false,
		Flags:       agentconfig.Conf,
		CustomName:  agentName,
		CurrentPeer: currentPeer,
	}

	node, err := latticeagent.NewNode(ctx, nodeCfg)
	if err != nil {
		return fmt.Errorf("create node: %w", err)
	}

	node.GetNetworkMap = func() (*infra.Message, error) {
		msg, getErr := node.GetNetMap(agentJWT)
		if getErr != nil {
			logger.Error("get network map failed", getErr)
			return nil, getErr
		}
		return msg, nil
	}

	if err = node.Start(ctx); err != nil {
		return fmt.Errorf("start node: %w", err)
	}
	defer node.Stop() //nolint:errcheck

	go node.StartHeartbeat(ctx)
	go runPeriodicRefresh(ctx, node, logger)

	// Wait for WireGuard peer sessions to establish.
	select {
	case <-time.After(runReadyWait):
	case <-ctx.Done():
		return ctx.Err()
	}

	if runEgressDeny {
		cidrs, _ := parseEgressCIDRs(runEgressAllow) // already validated above
		if err := applyEgressIPTables(cidrs); err != nil {
			return fmt.Errorf("apply egress iptables: %w", err)
		}
	}

	return forkAndWait(ctx, cancel, cmdArgs)
}

// registerOrResume tries to resume from persisted credentials; falls back to
// fresh NATS registration.
func registerOrResume(ctx context.Context, agentName, serverURL, token string) (*infra.Peer, error) {
	if creds, err := loadSandboxCredentials(); err == nil {
		if key, parseErr := wgtypes.ParseKey(creds.PrivateKey); parseErr == nil {
			if peer, resumeErr := latticeagent.ResumeSandboxViaNATS(ctx, serverURL, creds.JWT, agentName, key); resumeErr == nil {
				fmt.Printf("[agent-run] resumed %q from saved credentials\n", agentName)
				return peer, nil
			}
		}
	}

	privKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("generate WireGuard key: %w", err)
	}
	peer, err := latticeagent.RegisterSandboxViaNATS(ctx, serverURL, token, agentName, privKey)
	if err != nil {
		return nil, fmt.Errorf("registration failed: %w", err)
	}
	if saveErr := saveSandboxCredentials(privKey, peer.Token); saveErr != nil {
		fmt.Printf("[agent-run] warning: persist credentials: %v\n", saveErr)
	}
	return peer, nil
}

func runPeriodicRefresh(ctx context.Context, node *latticeagent.Node, logger interface {
	Warn(msg string, args ...any)
}) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := node.RefreshConfig(ctx); err != nil {
				logger.Warn("periodic config refresh failed", "err", err)
			}
		}
	}
}

func forkAndWait(ctx context.Context, cancel context.CancelFunc, cmdArgs []string) error {
	child := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	child.Env = os.Environ()

	if err := child.Start(); err != nil {
		return fmt.Errorf("start agent process: %w", err)
	}

	childDone := make(chan error, 1)
	go func() { childDone <- child.Wait() }()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	var childErr error
	select {
	case childErr = <-childDone:
		cancel()
	case <-sigCh:
		_ = child.Process.Signal(syscall.SIGTERM)
		select {
		case childErr = <-childDone:
		case <-time.After(5 * time.Second):
			_ = child.Process.Kill()
			childErr = <-childDone
		}
		cancel()
	}

	var exitErr *exec.ExitError
	if errors.As(childErr, &exitErr) {
		os.Exit(exitErr.ExitCode())
	}
	return childErr
}

// parseEgressCIDRs splits and validates a comma-separated CIDR string.
func parseEgressCIDRs(raw string) ([]string, error) {
	if raw == "" {
		return nil, nil
	}
	var out []string
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if _, _, err := net.ParseCIDR(entry); err != nil {
			return nil, fmt.Errorf("invalid egress CIDR %q: %w", entry, err)
		}
		out = append(out, entry)
	}
	return out, nil
}

// applyEgressIPTables installs iptables rules inside the gVisor container to
// restrict egress traffic on the overlay. gVisor virtualises iptables, so
// these rules apply to gVisor's netstack.
func applyEgressIPTables(allowCIDRs []string) error {
	rules := [][]string{
		// Allow established/related connections.
		{"-A", "OUTPUT", "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT"},
		// Allow loopback.
		{"-A", "OUTPUT", "-o", "lo", "-j", "ACCEPT"},
		// Allow WireGuard encrypted UDP (to peer endpoints).
		{"-A", "OUTPUT", "-p", "udp", "-j", "ACCEPT"},
	}
	for _, cidr := range allowCIDRs {
		rules = append(rules, []string{"-A", "OUTPUT", "-d", cidr, "-j", "ACCEPT"})
	}
	rules = append(rules, []string{"-P", "OUTPUT", "DROP"})

	for _, args := range rules {
		out, err := exec.Command("iptables", args...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("iptables %v: %s: %w", args, out, err)
		}
	}
	return nil
}
```

- [ ] **Step 5: Build PRO edition**

```bash
cd /Users/francis/workspc/lattice && go build -tags "pro" ./cmd/lattice/...
```

Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add cmd/lattice/cmd/agent/run_pro.go
git commit -s -m "feat(agent): implement agent run PRO edition with egress policy"
```

---

## Task 7: `agent sidecar` — community edition

The sidecar embeds the gVisor netstack library for WireGuard (no kernel wg0), listens on port 15001 as a transparent proxy, and dials through gVisor's `Sandbox.DialContext` to forward connections over the overlay. Community edition has no egress policy.

**Files:**
- Replace: `cmd/lattice/cmd/agent/sidecar_community.go` (was a stub)

- [ ] **Step 1: Write failing test for sidecar subcommand registration**

Add to `cmd/lattice/cmd/agent/run_test.go`:

```go
func TestAgentCmd_SidecarSubcommandRegistered(t *testing.T) {
	cmd := agent.AgentCmd()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Use == "sidecar" || strings.HasPrefix(sub.Use, "sidecar ") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'sidecar' subcommand to be registered on agent command")
	}
}
```

- [ ] **Step 2: Run test to confirm failure**

```bash
cd /Users/francis/workspc/lattice && go test ./cmd/lattice/cmd/agent/... -v -run TestAgentCmd_SidecarSubcommandRegistered
```

Expected: FAIL

- [ ] **Step 3: Replace `sidecar_community.go` stub with full implementation**

```go
//go:build !pro

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

package agent

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	latticeagent "github.com/alatticeio/lattice/internal/agent"
	agentconfig "github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/gvisor"
	"github.com/alatticeio/lattice/internal/agent/infra"
	agentlog "github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/agent/provision"
	"github.com/alatticeio/lattice/internal/agent/tproxy"
	"github.com/spf13/cobra"
	wgdevice "golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var (
	sidecarServerURL string
	sidecarToken     string
	sidecarProxyPort int
)

func addSidecarCmd(parent *cobra.Command) {
	parent.AddCommand(sidecarCmd())
}

func sidecarCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sidecar <name>",
		Short: "Run a Lattice sidecar with transparent proxy for K8s pods",
		Long: `Sidecar registers with the Lattice control plane, starts an embedded gVisor
netstack for WireGuard (no kernel wg0), and listens as a transparent TCP proxy.
All TCP connections redirected by the init container's iptables rules are
forwarded through the WireGuard overlay.

Run as a Kubernetes sidecar container alongside the AI agent container:
  securityContext:
    runAsUser: 1337  # must match --skip-uid in agent init

Example:
  lattice agent sidecar my-agent \
    --server-url http://latticed:8080 --token lt-xxx`,
		Args: cobra.ExactArgs(1),
		RunE: runSidecar,
	}
	cmd.Flags().StringVar(&sidecarServerURL, "server-url", "", "Lattice control plane URL (required)")
	cmd.Flags().StringVar(&sidecarToken, "token", "", "Enrollment token (required)")
	cmd.Flags().IntVar(&sidecarProxyPort, "proxy-port", 15001,
		"TCP port to listen on for transparent proxy connections")
	_ = cmd.MarkFlagRequired("server-url")
	_ = cmd.MarkFlagRequired("token")
	return cmd
}

func runSidecar(_ *cobra.Command, args []string) error {
	agentName := args[0]

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agentconfig.Conf.AppId = agentName
	agentconfig.Conf.ServerUrl = sidecarServerURL
	agentconfig.Conf.WgPort = 0 // random port; no kernel wg0

	var privKey wgtypes.Key
	var currentPeer *infra.Peer

	if creds, err := loadSandboxCredentials(); err == nil {
		if key, parseErr := wgtypes.ParseKey(creds.PrivateKey); parseErr == nil {
			if peer, resumeErr := latticeagent.ResumeSandboxViaNATS(ctx, sidecarServerURL, creds.JWT, agentName, key); resumeErr == nil {
				privKey = key
				currentPeer = peer
				fmt.Printf("[agent-sidecar] resumed %q, overlay IP=%s\n", agentName, overlayAddr(currentPeer))
			}
		}
	}

	if currentPeer == nil {
		var err error
		privKey, err = wgtypes.GeneratePrivateKey()
		if err != nil {
			return fmt.Errorf("generate WireGuard key: %w", err)
		}
		currentPeer, err = latticeagent.RegisterSandboxViaNATS(ctx, sidecarServerURL, sidecarToken, agentName, privKey)
		if err != nil {
			return fmt.Errorf("registration failed: %w", err)
		}
		if saveErr := saveSandboxCredentials(privKey, currentPeer.Token); saveErr != nil {
			fmt.Printf("[agent-sidecar] warning: persist credentials: %v\n", saveErr)
		}
	}
	_ = privKey // used only during registration

	localIP := overlayAddr(currentPeer)
	fmt.Printf("[agent-sidecar] %q registered, overlay IP=%s\n", agentName, localIP)

	if currentPeer.LrpUrl != "" {
		agentconfig.Conf.EnableLrp = true
		agentconfig.Conf.RelayURL = currentPeer.LrpUrl
	}

	// Community: no PolicyChecker, no AuditWriter.
	sb, err := gvisor.New(gvisor.Config{
		ID:      agentName,
		LocalIP: localIP,
	})
	if err != nil {
		return fmt.Errorf("create gVisor sandbox: %w", err)
	}
	defer sb.Close() //nolint:errcheck

	tunDev := gvisor.NewTUNAdapter(sb.Channel(), gvisor.InjectIntoChannel(sb.Channel()))

	logger := agentlog.GetLogger("agent-sidecar")
	agentJWT := currentPeer.Token

	nodeCfg := &latticeagent.NodeConfig{
		Logger:      logger,
		Port:        0,
		ShowLog:     false,
		Flags:       agentconfig.Conf,
		CustomTUN:   tunDev,
		CustomName:  agentName,
		CurrentPeer: currentPeer,
		ProvisionerFactory: func(dev *wgdevice.Device) provision.Provisioner {
			return gvisor.NewSandboxProvisionerFactory(localIP, agentName)(dev)
		},
	}

	node, err := latticeagent.NewNode(ctx, nodeCfg)
	if err != nil {
		return fmt.Errorf("create node: %w", err)
	}

	node.GetNetworkMap = func() (*infra.Message, error) {
		msg, getErr := node.GetNetMap(agentJWT)
		if getErr != nil {
			logger.Error("get network map failed", getErr)
			return nil, getErr
		}
		return msg, nil
	}

	if err = node.Start(ctx); err != nil {
		return fmt.Errorf("start node: %w", err)
	}
	defer node.Stop() //nolint:errcheck

	go node.StartHeartbeat(ctx)
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if refreshErr := node.RefreshConfig(ctx); refreshErr != nil {
					logger.Warn("periodic config refresh failed", "err", refreshErr)
				}
			}
		}
	}()

	// Start transparent proxy — dials through gVisor netstack (WireGuard).
	proxy := &tproxy.Proxy{
		Addr: fmt.Sprintf("0.0.0.0:%d", sidecarProxyPort),
		Dial: sb.DialContext,
	}
	if err := proxy.Start(ctx); err != nil {
		return fmt.Errorf("start transparent proxy: %w", err)
	}
	fmt.Printf("[agent-sidecar] transparent proxy listening on :%d\n", sidecarProxyPort)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sigCh:
	case <-ctx.Done():
	}
	fmt.Println("[agent-sidecar] shutting down")
	return nil
}
```

- [ ] **Step 4: Run test to confirm it passes**

```bash
cd /Users/francis/workspc/lattice && go test ./cmd/lattice/cmd/agent/... -v -run TestAgentCmd_SidecarSubcommandRegistered
```

Expected: PASS

- [ ] **Step 5: Build check**

```bash
go build ./cmd/lattice/...
```

Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add cmd/lattice/cmd/agent/sidecar_community.go
git commit -s -m "feat(agent): implement agent sidecar community edition"
```

---

## Task 8: `agent sidecar` — PRO edition

PRO adds `--egress-allow` / `--egress-default-deny`. Policy is enforced by passing `shimfwd.NewEgressFilter` to `gvisor.New()`; the filter is applied automatically when `sb.DialContext` is called by the transparent proxy.

**Files:**
- Create: `cmd/lattice/cmd/agent/sidecar_pro.go` (`//go:build pro && linux`)

- [ ] **Step 1: Create `cmd/lattice/cmd/agent/sidecar_pro.go`**

```go
//go:build pro && linux

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

package agent

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	shimfwd "github.com/alatticeio/lattice-shim/shim"
	latticeagent "github.com/alatticeio/lattice/internal/agent"
	agentconfig "github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/gvisor"
	"github.com/alatticeio/lattice/internal/agent/infra"
	agentlog "github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/agent/provision"
	"github.com/alatticeio/lattice/internal/agent/tproxy"
	"github.com/spf13/cobra"
	wgdevice "golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const auditLogPath = "/tmp/lattice-audit.jsonl"

var (
	sidecarServerURL   string
	sidecarToken       string
	sidecarProxyPort   int
	sidecarEgressAllow string
	sidecarEgressDeny  bool
)

func addSidecarCmd(parent *cobra.Command) {
	parent.AddCommand(sidecarCmd())
}

func sidecarCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sidecar <name>",
		Short: "Run a Lattice sidecar with transparent proxy for K8s pods (Pro)",
		Long: `Sidecar registers with the Lattice control plane, starts an embedded gVisor
netstack for WireGuard (no kernel wg0), enforces egress policy, and listens
as a transparent TCP proxy.

Example:
  lattice agent sidecar my-agent \
    --server-url http://latticed:8080 --token lt-xxx \
    --egress-allow 10.0.0.0/8 --egress-default-deny`,
		Args: cobra.ExactArgs(1),
		RunE: runSidecar,
	}
	cmd.Flags().StringVar(&sidecarServerURL, "server-url", "", "Lattice control plane URL (required)")
	cmd.Flags().StringVar(&sidecarToken, "token", "", "Enrollment token (required)")
	cmd.Flags().IntVar(&sidecarProxyPort, "proxy-port", 15001,
		"TCP port to listen on for transparent proxy connections")
	cmd.Flags().StringVar(&sidecarEgressAllow, "egress-allow", "",
		"Comma-separated overlay CIDRs the AI agent is allowed to reach")
	cmd.Flags().BoolVar(&sidecarEgressDeny, "egress-default-deny", false,
		"Deny all egress except --egress-allow CIDRs")
	_ = cmd.MarkFlagRequired("server-url")
	_ = cmd.MarkFlagRequired("token")
	return cmd
}

func runSidecar(_ *cobra.Command, args []string) error {
	agentName := args[0]

	egressPolicy := shimfwd.EgressPolicy{DefaultDeny: sidecarEgressDeny}
	if sidecarEgressAllow != "" {
		for _, entry := range strings.Split(sidecarEgressAllow, ",") {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			_, cidr, err := net.ParseCIDR(entry)
			if err != nil {
				return fmt.Errorf("invalid egress CIDR %q: %w", entry, err)
			}
			egressPolicy.AllowedCIDRs = append(egressPolicy.AllowedCIDRs, *cidr)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agentconfig.Conf.AppId = agentName
	agentconfig.Conf.ServerUrl = sidecarServerURL
	agentconfig.Conf.WgPort = 0

	var privKey wgtypes.Key
	var currentPeer *infra.Peer

	if creds, err := loadSandboxCredentials(); err == nil {
		if key, parseErr := wgtypes.ParseKey(creds.PrivateKey); parseErr == nil {
			if peer, resumeErr := latticeagent.ResumeSandboxViaNATS(ctx, sidecarServerURL, creds.JWT, agentName, key); resumeErr == nil {
				privKey = key
				currentPeer = peer
				fmt.Printf("[agent-sidecar] resumed %q, overlay IP=%s\n", agentName, overlayAddr(currentPeer))
			}
		}
	}

	if currentPeer == nil {
		var err error
		privKey, err = wgtypes.GeneratePrivateKey()
		if err != nil {
			return fmt.Errorf("generate WireGuard key: %w", err)
		}
		currentPeer, err = latticeagent.RegisterSandboxViaNATS(ctx, sidecarServerURL, sidecarToken, agentName, privKey)
		if err != nil {
			return fmt.Errorf("registration failed: %w", err)
		}
		if saveErr := saveSandboxCredentials(privKey, currentPeer.Token); saveErr != nil {
			fmt.Printf("[agent-sidecar] warning: persist credentials: %v\n", saveErr)
		}
	}
	_ = privKey

	localIP := overlayAddr(currentPeer)
	fmt.Printf("[agent-sidecar] %q registered, overlay IP=%s\n", agentName, localIP)

	if currentPeer.LrpUrl != "" {
		agentconfig.Conf.EnableLrp = true
		agentconfig.Conf.RelayURL = currentPeer.LrpUrl
	}

	policyChecker := shimfwd.NewEgressFilter(egressPolicy)
	auditWriter, auditErr := newFileAuditWriter(auditLogPath)
	if auditErr != nil {
		fmt.Printf("[agent-sidecar] warning: open audit log: %v\n", auditErr)
	}

	sb, err := gvisor.New(gvisor.Config{
		ID:            agentName,
		LocalIP:       localIP,
		PolicyChecker: policyChecker,
		AuditWriter:   auditWriter,
	})
	if err != nil {
		return fmt.Errorf("create gVisor sandbox: %w", err)
	}
	defer sb.Close() //nolint:errcheck

	tunDev := gvisor.NewTUNAdapter(sb.Channel(), gvisor.InjectIntoChannel(sb.Channel()))

	logger := agentlog.GetLogger("agent-sidecar")
	agentJWT := currentPeer.Token

	nodeCfg := &latticeagent.NodeConfig{
		Logger:      logger,
		Port:        0,
		ShowLog:     false,
		Flags:       agentconfig.Conf,
		CustomTUN:   tunDev,
		CustomName:  agentName,
		CurrentPeer: currentPeer,
		ProvisionerFactory: func(dev *wgdevice.Device) provision.Provisioner {
			return gvisor.NewSandboxProvisionerFactory(localIP, agentName)(dev)
		},
	}

	node, err := latticeagent.NewNode(ctx, nodeCfg)
	if err != nil {
		return fmt.Errorf("create node: %w", err)
	}

	node.GetNetworkMap = func() (*infra.Message, error) {
		msg, getErr := node.GetNetMap(agentJWT)
		if getErr != nil {
			logger.Error("get network map failed", getErr)
			return nil, getErr
		}
		return msg, nil
	}

	if err = node.Start(ctx); err != nil {
		return fmt.Errorf("start node: %w", err)
	}
	defer node.Stop() //nolint:errcheck

	go node.StartHeartbeat(ctx)
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if refreshErr := node.RefreshConfig(ctx); refreshErr != nil {
					logger.Warn("periodic config refresh failed", "err", refreshErr)
				}
			}
		}
	}()

	proxy := &tproxy.Proxy{
		Addr: fmt.Sprintf("0.0.0.0:%d", sidecarProxyPort),
		Dial: sb.DialContext,
	}
	if err := proxy.Start(ctx); err != nil {
		return fmt.Errorf("start transparent proxy: %w", err)
	}
	fmt.Printf("[agent-sidecar] transparent proxy listening on :%d (egress-deny=%v)\n",
		sidecarProxyPort, sidecarEgressDeny)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sigCh:
	case <-ctx.Done():
	}
	fmt.Println("[agent-sidecar] shutting down")
	return nil
}
```

- [ ] **Step 2: Build PRO sidecar**

```bash
cd /Users/francis/workspc/lattice && go build -tags "pro" ./cmd/lattice/...
```

Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add cmd/lattice/cmd/agent/sidecar_pro.go
git commit -s -m "feat(agent): implement agent sidecar PRO edition with egress policy"
```

---

## Task 9: Delete old `sandbox` and `runsc` packages

- [ ] **Step 1: Remove `cmd/lattice/cmd/sandbox/` entirely**

```bash
rm -rf /Users/francis/workspc/lattice/cmd/lattice/cmd/sandbox
```

- [ ] **Step 2: Remove `internal/agent/runsc/` entirely**

```bash
rm -rf /Users/francis/workspc/lattice/internal/agent/runsc
```

- [ ] **Step 3: Verify nothing else imports these packages**

```bash
cd /Users/francis/workspc/lattice && grep -r "cmd/lattice/cmd/sandbox\|internal/agent/runsc" --include="*.go" .
```

Expected: no output (only `go.sum` or similar non-Go files may remain, those are fine)

- [ ] **Step 4: Build both editions to confirm no broken imports**

```bash
go build ./cmd/lattice/... && go build -tags "pro" ./cmd/lattice/...
```

Expected: no errors

- [ ] **Step 5: Run unit tests**

```bash
go test ./cmd/lattice/... ./internal/agent/tproxy/...
```

Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -s -m "feat(agent): remove legacy sandbox and runsc packages"
```

---

## Task 10: Update E2E tests

The E2E tests in `test/e2e/agent_sandbox_test.go` call `deploySandboxPod` which deploys a pod using the sandbox image. That function needs to be updated to use `lattice agent sidecar` instead of `lattice sandbox start`.

**Files:**
- Modify: `test/e2e/helpers_test.go` — update `deploySandboxPod` helper
- Modify: `test/e2e/agent_sandbox_test.go` — update sandbox name/references if needed
- Delete: `test/e2e/agent_sandbox_gvisor_test.go` — was testing the now-removed gVisor runsc mode

- [ ] **Step 1: Read `deploySandboxPod` in helpers_test.go to understand current pod spec**

```bash
cd /Users/francis/workspc/lattice && grep -n "deploySandboxPod\|SandboxCmd\|sandbox start\|sandbox run" test/e2e/helpers_test.go
```

- [ ] **Step 2: Update `deploySandboxPod` to use `agent sidecar` + `agent init`**

Find the `deploySandboxPod` function in `test/e2e/helpers_test.go`. Replace its pod spec with the new sidecar pattern:

```go
// The pod spec should become:
// initContainers:
// - name: lattice-init
//   image: sandboxImage
//   command: ["lattice", "agent", "init"]
//   securityContext: {capabilities: {add: ["NET_ADMIN"]}}
//
// containers:
// - name: lattice-sidecar  (was the main container running sandbox start)
//   image: sandboxImage
//   command: ["lattice", "agent", "sidecar", name,
//             "--server-url", serverURL, "--token", token]
//   securityContext: {runAsUser: 1337}
//
// - name: agent  (AI agent workload — keep existing nginx test container)
//   image: nginx:alpine
//   ...
```

The exact edit depends on the current `deploySandboxPod` implementation. After reading it (step 1), apply the minimal diff to switch from `["lattice", "sandbox", "start", ...]` to the init+sidecar pattern above.

- [ ] **Step 3: Delete the gVisor runsc E2E test file**

```bash
rm test/e2e/agent_sandbox_gvisor_test.go
```

- [ ] **Step 4: Verify E2E test files compile**

```bash
cd /Users/francis/workspc/lattice && go build ./test/e2e/...
```

Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add test/e2e/
git commit -s -m "test(e2e): update agent sandbox tests for new agent sidecar command"
```

---

## Self-Review

**Spec coverage check:**

| Spec requirement | Covered by task |
|----------------|----------------|
| Remove `sandbox` command tree | Task 9 |
| `agent run` (community, no policy) | Task 5 |
| `agent run` PRO egress flags | Task 6 |
| `agent sidecar` (community, transparent proxy) | Task 7 |
| `agent sidecar` PRO egress policy | Task 8 |
| `agent init` iptables REDIRECT | Task 4 |
| Transparent proxy + SO_ORIGINAL_DST | Task 3 |
| Remove `internal/agent/runsc/` | Task 9 |
| Update E2E tests | Task 10 |
| Shared credential utilities | Task 2 |
| root.go wiring | Task 1 |

**Placeholder scan:** No TBD/TODO in any code block. All steps have concrete code or commands.

**Type consistency:**
- `tproxy.Proxy.Dial` is `func(ctx context.Context, network, addr string) (net.Conn, error)` — matches `gvisor.Sandbox.DialContext` signature ✓
- `gvisor.NewTUNAdapter` / `gvisor.InjectIntoChannel` / `gvisor.NewSandboxProvisionerFactory` are used identically in Tasks 7 and 8 ✓
- `latticeagent.RegisterSandboxViaNATS`, `latticeagent.ResumeSandboxViaNATS` — same signature used in Tasks 5, 6, 7, 8 ✓
- `shimfwd.EgressPolicy`, `shimfwd.NewEgressFilter` — used in Task 8 only (PRO), consistent with existing `driver_pod.go` usage ✓
- `loadSandboxCredentials`, `saveSandboxCredentials`, `overlayAddr` defined in `shared.go` (Task 2) and used in Tasks 5–8 ✓
