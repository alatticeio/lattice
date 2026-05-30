# AI Agent Secure Mesh — Plan A: Architecture Cleanup

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove gVisor from the `sandbox run` data path, replace iptables+tproxy with kernel TUN + SOCKS5, mark sidecar deprecated, and verify existing e2e tests pass.

**Architecture:** `lattice-run` creates a standard kernel wf0 (same as regular lattice agent), starts a `shim.Socks5Server` backed by a `policyDialer` that wraps `net.Dial` with optional egress policy checks, then forks the AI agent with `ALL_PROXY` set. The AI agent's HTTP traffic flows: AI agent → SOCKS5 → kernel wf0 → WireGuard → overlay peer. Policy and audit happen in `policyDialer` before dialing.

**Tech Stack:** Go 1.25, `github.com/alatticeio/lattice-shim/shim` (Socks5Server, PolicyChecker, AuditWriter, EgressFilter), `golang.zx2c4.com/wireguard`, `os/exec`, Ginkgo v2 e2e.

---

## File Map

| File | Action | Purpose |
|---|---|---|
| `cmd/lattice/cmd/sandbox/shared_linux.go` | Modify | Remove gVisor/tproxy/iptables; add policyDialer, Socks5Server, forkWithProxy |
| `cmd/lattice/cmd/sandbox/shared_linux_test.go` | Create | Unit tests for policyDialer |
| `cmd/lattice/cmd/sandbox/sidecar.go` | Modify | Add deprecated notice |
| `cmd/lattice/cmd/sandbox/run.go` | Modify | Remove gVisor/tproxy/provision imports |
| `internal/agent/node.go` | Already fixed | GetHandshake uses node.Name (done in prior session) |
| `test/e2e/helpers_test.go` | Already fixed | --forward flag added (done in prior session) |
| `test/e2e/agent_sandbox_test.go` | Already fixed | ForwardListener fix applied (done in prior session) |

---

### Task 1: Add `policyDialer` to shared_linux.go (TDD)

**Files:**
- Create: `cmd/lattice/cmd/sandbox/shared_linux_test.go`
- Modify: `cmd/lattice/cmd/sandbox/shared_linux.go`

- [ ] **Step 1: Write failing tests for policyDialer**

Create `cmd/lattice/cmd/sandbox/shared_linux_test.go`:

```go
//go:build linux

package sandbox

import (
	"context"
	"net"
	"testing"

	shim "github.com/alatticeio/lattice-shim/shim"
)

// stubListener listens on a random local TCP port and accepts one connection.
func stubListener(t *testing.T) (addr string, accepted <-chan struct{}) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	ch := make(chan struct{}, 1)
	go func() {
		conn, _ := ln.Accept()
		if conn != nil {
			conn.Close()
		}
		ch <- struct{}{}
	}()
	return ln.Addr().String(), ch
}

func TestPolicyDialer_NilChecker_Allows(t *testing.T) {
	addr, accepted := stubListener(t)
	d := &policyDialer{identity: "agent-1", checker: nil, auditor: nil}
	conn, err := d.DialContext(context.Background(), "tcp", addr)
	if err != nil {
		t.Fatalf("expected allow, got err: %v", err)
	}
	conn.Close()
	<-accepted
}

func TestPolicyDialer_PolicyAllow_Connects(t *testing.T) {
	addr, accepted := stubListener(t)
	policy := shim.EgressPolicy{
		DefaultDeny:  true,
		AllowedCIDRs: []net.IPNet{mustParseCIDR("127.0.0.0/8")},
	}
	d := &policyDialer{
		identity: "agent-1",
		checker:  shim.NewEgressFilter(policy),
		auditor:  nil,
	}
	conn, err := d.DialContext(context.Background(), "tcp", addr)
	if err != nil {
		t.Fatalf("expected allow for 127.x.x.x, got err: %v", err)
	}
	conn.Close()
	<-accepted
}

func TestPolicyDialer_PolicyDeny_Blocks(t *testing.T) {
	addr, _ := stubListener(t)
	policy := shim.EgressPolicy{
		DefaultDeny:  true,
		AllowedCIDRs: []net.IPNet{mustParseCIDR("10.0.0.0/8")}, // only 10.x allowed
	}
	d := &policyDialer{
		identity: "agent-1",
		checker:  shim.NewEgressFilter(policy),
		auditor:  nil,
	}
	_, err := d.DialContext(context.Background(), "tcp", addr) // addr is 127.x.x.x
	if err == nil {
		t.Fatal("expected deny for 127.x.x.x when only 10.x allowed")
	}
}

func TestPolicyDialer_Audit_WritesEvent(t *testing.T) {
	addr, accepted := stubListener(t)

	var written []shim.AuditEvent
	auditor := &testAuditor{onWrite: func(e shim.AuditEvent) { written = append(written, e) }}

	d := &policyDialer{identity: "agent-1", checker: nil, auditor: auditor}
	conn, err := d.DialContext(context.Background(), "tcp", addr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	conn.Close()
	<-accepted

	if len(written) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(written))
	}
	if written[0].Verdict != shim.VerdictAllow {
		t.Errorf("expected allow verdict, got %s", written[0].Verdict)
	}
	if written[0].Identity != "agent-1" {
		t.Errorf("expected identity agent-1, got %s", written[0].Identity)
	}
}

func TestPolicyDialer_Audit_WritesDenyEvent(t *testing.T) {
	addr, _ := stubListener(t)

	var written []shim.AuditEvent
	auditor := &testAuditor{onWrite: func(e shim.AuditEvent) { written = append(written, e) }}
	policy := shim.EgressPolicy{DefaultDeny: true} // deny all

	d := &policyDialer{
		identity: "agent-1",
		checker:  shim.NewEgressFilter(policy),
		auditor:  auditor,
	}
	_, _ = d.DialContext(context.Background(), "tcp", addr)

	if len(written) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(written))
	}
	if written[0].Verdict != shim.VerdictDrop {
		t.Errorf("expected drop verdict, got %s", written[0].Verdict)
	}
}

// helpers

type testAuditor struct {
	onWrite func(shim.AuditEvent)
}

func (a *testAuditor) Write(e shim.AuditEvent) error {
	a.onWrite(e)
	return nil
}

func mustParseCIDR(s string) net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return *n
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /Users/francis/workspc/lattice
go test ./cmd/lattice/cmd/sandbox/ -run TestPolicyDialer -v 2>&1 | head -20
```

Expected: compile error — `policyDialer` undefined.

- [ ] **Step 3: Add policyDialer to shared_linux.go**

In `cmd/lattice/cmd/sandbox/shared_linux.go`, add after the existing imports and constants (before `fileAuditWriter`):

```go
// policyDialer wraps net.Dial with optional egress policy checking and audit.
// Traffic goes through the kernel (wf0) to the WireGuard overlay.
type policyDialer struct {
	identity string
	checker  shim.PolicyChecker // nil = no policy enforcement
	auditor  shim.AuditWriter   // nil = no audit
}

func (d *policyDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, portStr, _ := net.SplitHostPort(addr)
	ip := net.ParseIP(host)
	var port uint16
	if p, parseErr := strconv.ParseUint(portStr, 10, 16); parseErr == nil {
		port = uint16(p)
	}

	if d.checker != nil && ip != nil && !d.checker.Allow(d.identity, ip, port) {
		if d.auditor != nil {
			_ = d.auditor.Write(shim.AuditEvent{
				Identity: d.identity,
				DstIP:    host,
				DstPort:  port,
				Protocol: network,
				Verdict:  shim.VerdictDrop,
			})
		}
		return nil, fmt.Errorf("egress policy denied: %s", addr)
	}

	conn, err := (&net.Dialer{}).DialContext(ctx, network, addr)
	if err == nil && d.auditor != nil {
		_ = d.auditor.Write(shim.AuditEvent{
			Identity: d.identity,
			DstIP:    host,
			DstPort:  port,
			Protocol: network,
			Verdict:  shim.VerdictAllow,
		})
	}
	return conn, err
}
```

Add `"strconv"` and `"net"` to imports in `shared_linux.go` if not already present.

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./cmd/lattice/cmd/sandbox/ -run TestPolicyDialer -v
```

Expected: all 5 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/lattice/cmd/sandbox/shared_linux.go cmd/lattice/cmd/sandbox/shared_linux_test.go
git commit -s -m "feat(sandbox): add policyDialer wrapping kernel net.Dial with egress policy"
```

---

### Task 2: Add `forkWithProxy` to shared_linux.go (TDD)

**Files:**
- Modify: `cmd/lattice/cmd/sandbox/shared_linux_test.go`
- Modify: `cmd/lattice/cmd/sandbox/shared_linux.go`

- [ ] **Step 1: Write failing test for forkWithProxy**

Append to `cmd/lattice/cmd/sandbox/shared_linux_test.go`:

```go
func TestForkWithProxy_SetsAllProxy(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Fork a child that prints ALL_PROXY and exits 0
	err := forkWithProxy(ctx, cancel, []string{"sh", "-c", "echo $ALL_PROXY"}, "socks5h://127.0.0.1:9999")
	if err != nil {
		t.Fatalf("forkWithProxy returned err: %v", err)
	}
}

func TestForkWithProxy_ChildExitCode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Fork a child that exits 0
	err := forkWithProxy(ctx, cancel, []string{"true"}, "socks5h://127.0.0.1:9999")
	if err != nil {
		t.Fatalf("expected nil error for exit 0, got: %v", err)
	}
}
```

Add `"time"` to test imports.

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./cmd/lattice/cmd/sandbox/ -run TestForkWithProxy -v 2>&1 | head -10
```

Expected: compile error — `forkWithProxy` undefined.

- [ ] **Step 3: Add forkWithProxy to shared_linux.go**

In `cmd/lattice/cmd/sandbox/shared_linux.go`, add after `forkAndWait`:

```go
// forkWithProxy forks the AI agent and sets ALL_PROXY so its HTTP traffic
// flows through the SOCKS5 proxy to the WireGuard overlay.
// Unlike forkAndWait, this does NOT set UID 999 or install iptables rules.
func forkWithProxy(ctx context.Context, cancel context.CancelFunc, cmdArgs []string, proxyAddr string) error {
	child := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	child.Env = append(os.Environ(),
		"ALL_PROXY="+proxyAddr,
		"all_proxy="+proxyAddr,
		"HTTPS_PROXY="+proxyAddr,
		"https_proxy="+proxyAddr,
	)

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

Add `"errors"` to imports if not present.

- [ ] **Step 4: Run tests**

```bash
go test ./cmd/lattice/cmd/sandbox/ -run "TestPolicyDialer|TestForkWithProxy" -v
```

Expected: all 7 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/lattice/cmd/sandbox/shared_linux.go cmd/lattice/cmd/sandbox/shared_linux_test.go
git commit -s -m "feat(sandbox): add forkWithProxy replacing UID-999 forkAndWait"
```

---

### Task 3: Rewrite runSandbox — remove gVisor/tproxy, use kernel TUN + SOCKS5

**Files:**
- Modify: `cmd/lattice/cmd/sandbox/shared_linux.go`

- [ ] **Step 1: Replace runSandbox body**

Replace the entire `runSandbox` function in `cmd/lattice/cmd/sandbox/shared_linux.go` with:

```go
// runSandbox is the shared sandbox engine for both community and PRO editions.
// It uses a standard kernel wf0 (same as regular lattice agent) and a SOCKS5
// proxy for AI agent egress — no gVisor CustomTUN, no iptables, no UID tricks.
func runSandbox(
	ctx context.Context,
	cancel context.CancelFunc,
	agentName string,
	currentPeer *infra.Peer,
	checker shim.PolicyChecker,
	auditor shim.AuditWriter,
	cmdArgs []string,
) error {
	agentconfig.Conf.AppId = agentName

	localIP := overlayAddr(currentPeer)
	fmt.Printf("[sandbox-run] %q registered, overlay IP=%s\n", agentName, localIP)

	if currentPeer.LrpUrl != "" {
		agentconfig.Conf.EnableLrp = true
		agentconfig.Conf.RelayURL = currentPeer.LrpUrl
	}

	agentJWT := currentPeer.Token
	logger := agentlog.GetLogger("sandbox-run")

	nodeCfg := &latticeagent.NodeConfig{
		Logger:      logger,
		Port:        0,
		ShowLog:     false,
		Flags:       agentconfig.Conf,
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

	dialer := &policyDialer{identity: agentName, checker: checker, auditor: auditor}
	socks5Srv, err := shim.NewSocks5Server(dialer, "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("start socks5 server: %w", err)
	}
	go func() { _ = socks5Srv.Serve() }()
	go func() { <-ctx.Done(); _ = socks5Srv.Close() }()

	proxyAddr := "socks5h://" + socks5Srv.Addr().String()
	fmt.Printf("[sandbox-run] SOCKS5 proxy on %s (egress policy: %v)\n", socks5Srv.Addr(), checker != nil)

	select {
	case <-time.After(runReadyWait):
	case <-ctx.Done():
		return ctx.Err()
	}

	return forkWithProxy(ctx, cancel, cmdArgs, proxyAddr)
}
```

- [ ] **Step 2: Remove dead constants and functions**

`runSandbox` no longer calls `forkAndWait`, `installRunIPTables`, or uses `sandboxAgentUID`/`runProxyPort`. Sidecar does not use any of these either. Delete them all from `shared_linux.go`:

```go
// DELETE these — no longer referenced anywhere:
const sandboxAgentUID = 999   // DELETE
const runProxyPort = 15001    // DELETE
func installRunIPTables(...) error { ... }  // DELETE
func forkAndWait(...) error { ... }         // DELETE (replaced by forkWithProxy)
```

Verify nothing else references them:
```bash
grep -rn "sandboxAgentUID\|runProxyPort\|installRunIPTables\|forkAndWait" cmd/lattice/
```
Expected: zero results after deletion.

- [ ] **Step 3: Clean up imports**

Update the import block in `shared_linux.go`. Remove unused imports:

```go
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	shim "github.com/alatticeio/lattice-shim/shim"
	latticeagent "github.com/alatticeio/lattice/internal/agent"
	agentconfig "github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/infra"
	agentlog "github.com/alatticeio/lattice/internal/agent/log"
)
```

Remove these imports (no longer used by `runSandbox`):
- `"github.com/alatticeio/lattice/internal/agent/gvisor"`
- `"github.com/alatticeio/lattice/internal/agent/provision"`
- `"github.com/alatticeio/lattice/internal/agent/tproxy"`
- `wgdevice "golang.zx2c4.com/wireguard/device"`

- [ ] **Step 4: Build and lint**

```bash
make lint
```

Expected: 0 issues. Fix any unused import or variable errors before continuing.

- [ ] **Step 5: Commit**

```bash
git add cmd/lattice/cmd/sandbox/shared_linux.go
git commit -s -m "refactor(sandbox): replace gVisor/tproxy with kernel TUN + SOCKS5 proxy"
```

---

### Task 4: Update run.go — remove gVisor-specific imports

**Files:**
- Modify: `cmd/lattice/cmd/sandbox/run.go`

- [ ] **Step 1: Check current imports**

```bash
head -50 cmd/lattice/cmd/sandbox/run.go
```

- [ ] **Step 2: Remove unused imports from run.go**

`run.go` imports `gvisor` and `tproxy` indirectly via `runSandbox` in `shared_linux.go`. Now that `runSandbox` no longer references them, check if `run.go` has its own direct references:

```bash
grep -n "gvisor\|tproxy\|provision\|wgdevice" cmd/lattice/cmd/sandbox/run.go
```

Remove any found references. The `run.go` file should only build policy/audit objects and call `runSandbox`.

- [ ] **Step 3: Build**

```bash
make build SERVICE=lattice
```

Expected: successful build with no errors.

- [ ] **Step 4: Run unit tests**

```bash
go test ./cmd/lattice/cmd/sandbox/... -v
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add cmd/lattice/cmd/sandbox/run.go
git commit -s -m "chore(sandbox): remove stale gVisor imports from run.go"
```

---

### Task 5: Mark sandbox sidecar as deprecated

**Files:**
- Modify: `cmd/lattice/cmd/sandbox/sidecar.go`

- [ ] **Step 1: Add deprecation notice**

At the top of `cmd/lattice/cmd/sandbox/sidecar.go`, after the build tag and license header, add:

```go
// Deprecated: sandbox sidecar requires an init container, iptables REDIRECT, and
// a ForwardListener to bridge between kernel and gVisor network stacks. Use
// lattice sandbox run instead — it provides the same overlay connectivity with
// a simpler architecture (kernel wf0 + SOCKS5, no init container required).
// This command will be removed in v0.6.
```

- [ ] **Step 2: Add runtime warning to runSidecar**

At the start of `runSidecar` function body, add:

```go
fmt.Fprintln(os.Stderr, "[DEPRECATED] sandbox sidecar is deprecated and will be removed in v0.6. Use 'lattice sandbox run' instead.")
```

- [ ] **Step 3: Build and lint**

```bash
make lint && make build SERVICE=lattice
```

Expected: 0 issues, successful build.

- [ ] **Step 4: Commit**

```bash
git add cmd/lattice/cmd/sandbox/sidecar.go
git commit -s -m "deprecate(sandbox): mark sidecar command as deprecated, use sandbox run instead"
```

---

### Task 6: Verify e2e tests pass

Prior session already fixed the two known failures:
1. `node.go` GetHandshake uses `node.Name` (not empty `config.Conf.InterfaceName`)
2. `sidecar.go` has `--forward 8080:127.0.0.1:8080` flag added + ForwardListener started
3. `helpers_test.go` passes `--forward 8080:127.0.0.1:8080` in `deploySandboxPod`

This task verifies those fixes are consistent with the current codebase and runs lint.

- [ ] **Step 1: Verify the three fixes are in place**

```bash
grep -n "node\.Name" internal/agent/node.go | grep -i handshake
grep -n "ForwardListener\|sidecarForward\|forward" cmd/lattice/cmd/sandbox/sidecar.go | head -10
grep -n "forward" test/e2e/helpers_test.go | head -5
```

Expected:
- `node.go`: `return wireguard.PeerHandshake(node.Name, pubKey)`
- `sidecar.go`: `sidecarForward []string` var and ForwardListener block present
- `helpers_test.go`: `"--forward", "8080:127.0.0.1:8080"` in sidecarArgs

- [ ] **Step 2: Run lint**

```bash
make lint
```

Expected: 0 issues.

- [ ] **Step 3: Run unit tests**

```bash
make test
```

Expected: all unit tests pass.

- [ ] **Step 4: Commit any remaining fixes**

```bash
git status
# commit if there are uncommitted changes from prior session
```

---

### Task 7: Final build verification and summary commit

- [ ] **Step 1: Full build**

```bash
make build SERVICE=lattice
make build SERVICE=latticed
make build SERVICE=manager
```

Expected: all three build successfully.

- [ ] **Step 2: Full unit test run**

```bash
make test
```

Expected: all tests pass.

- [ ] **Step 3: Lint**

```bash
make lint
```

Expected: 0 issues.

- [ ] **Step 4: Summary of what changed**

Verify the following:
```bash
git log --oneline -8
```

Should see commits for:
- policyDialer added
- forkWithProxy added
- runSandbox rewritten (kernel TUN + SOCKS5)
- run.go cleaned up
- sidecar deprecated
- node.go GetHandshake fix (from prior session)

---

## Plan B: New Features (MCPServer + AgentPolicy + Audit)

> **Prerequisite:** Plan A fully merged.

Plan B covers Blocks 2-4 from the spec. It will be written as a separate detailed plan once Plan A is merged. Below is the task outline.

### Block 2: MCPServer CRD

| Task | Description |
|---|---|
| B-1 | Add `api/v1alpha1/mcp_server_types.go` with MCPServer, MCPServerSpec, MCPServerStatus, MCPTool types |
| B-2 | Register MCPServer in `groupversion_info.go`, run `controller-gen` to generate deepcopy + CRD YAML |
| B-3 | Add MCPServer controller in `internal/manager/controller/mcp_server_controller.go` |
| B-4 | Add MCPServer service in `internal/server/service/mcp_server.go` |
| B-5 | Add MCPServer API router in `internal/server/server/mcp_server_router.go`, wire into server.go |
| B-6 | Unit + e2e tests for MCPServer CRUD and status reconciliation |

### Block 3: AgentPolicy CRD + MCP Proxy

| Task | Description |
|---|---|
| B-7 | Add `api/v1alpha1/agent_policy_types.go`, generate deepcopy + CRD YAML |
| B-8 | Add AgentPolicy service + API router |
| B-9 | Add `internal/agent/mcpproxy/proxy.go` — HTTP proxy with MCP JSON-RPC inspection |
| B-10 | Add `internal/agent/mcpproxy/policy_cache.go` — 15s refresh + NATS invalidation |
| B-11 | Wire MCP proxy into `lattice-run` (alongside SOCKS5 server) |
| B-12 | Unit tests for MCP JSON-RPC parsing and policy enforcement |

### Block 4: MCP Audit

| Task | Description |
|---|---|
| B-13 | Add `internal/agent/mcpproxy/audit.go` — MCP audit event type, param truncation/redaction |
| B-14 | Integrate audit writing into MCP proxy allow/deny path |
| B-15 | Add Pro-tier API upload in `lattice-run` |
| B-16 | e2e tests: audit file written, deny events captured, param redaction verified |
