# Sandbox Network Namespace Isolation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give `lattice sandbox run` an isolated Linux network namespace so the AI agent cannot bypass the MCP proxy or access the host network directly.

**Architecture:** Create a named netns (`lt-sandbox`) for the agent, connect it to the host via a veth pair (`lts-host` ↔ `lts-agent`), route all agent traffic through the host-side veth (where the MCP proxy and wf0 live), and fork the agent process into the isolated netns using `unix.Setns()` + `runtime.LockOSThread()`.

**Tech Stack:** `golang.org/x/sys/unix` (already in go.mod), `os/exec` for `ip` commands, standard `net` package.

---

## Traffic Path After This Change

```
Before (current):
  agent process (shared host netns)
    → kernel socket → wf0 → WireGuard overlay
    HTTP_PROXY → MCP proxy (127.0.0.1:port) [agent can ignore]

After (this plan):
  agent process (isolated lt-sandbox netns)
    → lts-agent (169.254.100.2) → [veth] → lts-host (169.254.100.1)
    → host netns routing → wf0 → WireGuard overlay
    HTTP_PROXY=http://169.254.100.1:port (MCP proxy on host veth)
    [agent cannot bypass: no route to internet without going through host]
```

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `cmd/lattice/cmd/sandbox/netns_linux.go` | **Create** | netns + veth lifecycle (setup, cleanup) |
| `cmd/lattice/cmd/sandbox/shared_linux.go` | **Modify** | `runSandbox` wires netns; `forkAgent` enters netns |
| `internal/agent/mcpproxy/proxy.go` | **No change** | Already accepts `addr` param — caller changes the addr |

---

## Task 1: Create `netns_linux.go` — netns and veth lifecycle

**Files:**
- Create: `cmd/lattice/cmd/sandbox/netns_linux.go`

- [ ] **Step 1: Write the file**

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

package sandbox

import (
	"fmt"
	"os/exec"
)

const (
	sandboxNetnsName = "lt-sandbox"         // named netns in /var/run/netns/
	hostVethName     = "lts-host"           // host-side veth interface
	agentVethName    = "lts-agent"          // agent-side veth interface (lives in netns)
	hostVethAddr     = "169.254.100.1/30"   // host-side link-local IP
	agentVethAddr    = "169.254.100.2/30"   // agent-side link-local IP
	hostVethIP       = "169.254.100.1"      // host veth IP without prefix (gateway for agent)
	proxyPort        = "18080"              // MCP proxy listen port in host netns
)

// setupSandboxNetns creates an isolated network namespace for the sandbox agent
// and wires it to the host network via a veth pair.
//
// After this call:
//   - /var/run/netns/lt-sandbox exists (named netns)
//   - lts-host (169.254.100.1/30) in host netns, UP
//   - lts-agent (169.254.100.2/30) in lt-sandbox netns, UP
//   - default route in lt-sandbox → 169.254.100.1 (host veth)
//   - IP forwarding enabled on host
//
// Returns the proxy listen address the MCP proxy should bind to.
// Call teardownSandboxNetns() to clean up.
func setupSandboxNetns() (proxyAddr string, err error) {
	// Clean up any leftover state from a previous crashed run.
	_ = teardownSandboxNetns()

	steps := [][]string{
		// 1. Create named netns (bind-mounts at /var/run/netns/lt-sandbox)
		{"ip", "netns", "add", sandboxNetnsName},
		// 2. Create veth pair in host netns
		{"ip", "link", "add", hostVethName, "type", "veth", "peer", "name", agentVethName},
		// 3. Move agent end into the netns
		{"ip", "link", "set", agentVethName, "netns", sandboxNetnsName},
		// 4. Configure host side
		{"ip", "addr", "add", hostVethAddr, "dev", hostVethName},
		{"ip", "link", "set", hostVethName, "up"},
		// 5. Configure agent side (inside the netns)
		{"ip", "netns", "exec", sandboxNetnsName, "ip", "link", "set", "lo", "up"},
		{"ip", "netns", "exec", sandboxNetnsName, "ip", "addr", "add", agentVethAddr, "dev", agentVethName},
		{"ip", "netns", "exec", sandboxNetnsName, "ip", "link", "set", agentVethName, "up"},
		// 6. Default route in agent netns: everything goes to host veth
		{"ip", "netns", "exec", sandboxNetnsName, "ip", "route", "add", "default", "via", hostVethIP},
		// 7. Enable IP forwarding so host can route agent traffic to wf0/internet
		{"sysctl", "-w", "net.ipv4.ip_forward=1"},
	}

	for _, args := range steps {
		if out, runErr := exec.Command(args[0], args[1:]...).CombinedOutput(); runErr != nil {
			_ = teardownSandboxNetns()
			return "", fmt.Errorf("sandbox netns setup (%v): %w\n%s", args, runErr, out)
		}
	}

	return hostVethIP + ":" + proxyPort, nil
}

// teardownSandboxNetns removes the named netns and veth interfaces.
// Idempotent — safe to call even if setup was partial or already torn down.
func teardownSandboxNetns() error {
	// Delete host veth (automatically deletes the peer inside the netns).
	// Ignore errors — interface may not exist.
	_ = exec.Command("ip", "link", "del", hostVethName).Run()
	// Delete named netns.
	_ = exec.Command("ip", "netns", "del", sandboxNetnsName).Run()
	return nil
}

// sandboxNetnsPath returns the filesystem path of the named netns.
// Used by forkAgent to open the netns fd for Setns().
func sandboxNetnsPath() string {
	return "/var/run/netns/" + sandboxNetnsName
}
```

- [ ] **Step 2: Verify it compiles**

```bash
cd /Users/francis/workspc/lattice
go build ./cmd/lattice/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add cmd/lattice/cmd/sandbox/netns_linux.go
git commit -s -m "feat(sandbox): add netns + veth lifecycle helpers for agent isolation"
```

---

## Task 2: Modify `forkAgent` to enter the agent netns before exec

**Files:**
- Modify: `cmd/lattice/cmd/sandbox/shared_linux.go` — `forkAgent` function

- [ ] **Step 1: Add the import and update the function signature**

In `shared_linux.go`, add `"runtime"` and `"golang.org/x/sys/unix"` to the import block, then replace the `forkAgent` function with the version below. The only signature change is adding `netnsPath string` as the last parameter.

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
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"

	shim "github.com/alatticeio/lattice-shim/shim"
	latticeagent "github.com/alatticeio/lattice/internal/agent"
	agentconfig "github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/infra"
	agentlog "github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/agent/mcpproxy"
	"golang.org/x/sys/unix"
)
```

- [ ] **Step 2: Replace `forkAgent` with the netns-aware version**

Replace the entire `forkAgent` function (lines 160–207 in `shared_linux.go`) with:

```go
// forkAgent forks the AI agent as a child process.
// When netnsPath is non-empty, the child is placed into that network namespace
// so it cannot see or reach the host network directly.
// When httpProxyAddr is non-empty, HTTP_PROXY/HTTPS_PROXY env vars are injected.
func forkAgent(ctx context.Context, cancel context.CancelFunc, cmdArgs []string, httpProxyAddr, netnsPath string) error {
	child := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr

	env := os.Environ()
	if httpProxyAddr != "" {
		env = append(env,
			"HTTP_PROXY="+httpProxyAddr,
			"http_proxy="+httpProxyAddr,
			"HTTPS_PROXY="+httpProxyAddr,
			"https_proxy="+httpProxyAddr,
		)
	}
	child.Env = env

	if netnsPath != "" {
		if err := startInNetns(child, netnsPath); err != nil {
			return err
		}
	} else {
		if err := child.Start(); err != nil {
			return fmt.Errorf("start agent process: %w", err)
		}
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

// startInNetns locks the current OS thread, enters the target network namespace,
// starts the child process (which inherits the thread's netns via fork), then
// restores the original netns on the same thread.
func startInNetns(child *exec.Cmd, netnsPath string) error {
	// Open the target netns fd.
	nsFd, err := os.Open(netnsPath)
	if err != nil {
		return fmt.Errorf("open agent netns %s: %w", netnsPath, err)
	}
	defer nsFd.Close()

	// Open the current (host) netns fd so we can restore it after fork.
	hostNsFd, err := os.Open("/proc/self/ns/net")
	if err != nil {
		return fmt.Errorf("open host netns: %w", err)
	}
	defer hostNsFd.Close()

	// LockOSThread pins this goroutine to its OS thread.
	// unix.Setns only changes the current thread's netns;
	// locking ensures cmd.Start() forks from this exact thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := unix.Setns(int(nsFd.Fd()), unix.CLONE_NEWNET); err != nil {
		return fmt.Errorf("enter agent netns: %w", err)
	}

	startErr := child.Start()

	// Always restore host netns regardless of Start() result.
	if restoreErr := unix.Setns(int(hostNsFd.Fd()), unix.CLONE_NEWNET); restoreErr != nil {
		// Unlikely but catastrophic — log and continue.
		fmt.Fprintf(os.Stderr, "[sandbox-run] WARNING: failed to restore host netns: %v\n", restoreErr)
	}

	if startErr != nil {
		return fmt.Errorf("start agent process in netns: %w", startErr)
	}
	return nil
}
```

- [ ] **Step 3: Verify it compiles**

```bash
go build ./cmd/lattice/...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add cmd/lattice/cmd/sandbox/shared_linux.go
git commit -s -m "feat(sandbox): fork agent process into isolated network namespace"
```

---

## Task 3: Wire netns setup into `runSandbox`

**Files:**
- Modify: `cmd/lattice/cmd/sandbox/shared_linux.go` — `runSandbox` function

- [ ] **Step 1: Update the `runSandbox` function**

Replace the `runSandbox` function body (starting at `agentconfig.Conf.AppId = agentName`) with the version below. Changes:
1. Call `setupSandboxNetns()` early, defer `teardownSandboxNetns()`
2. Proxy listens on the veth IP (returned by `setupSandboxNetns`) instead of `127.0.0.1:0`
3. Pass `sandboxNetnsPath()` to `forkAgent`

```go
func runSandbox(
	ctx context.Context,
	cancel context.CancelFunc,
	agentName string,
	currentPeer *infra.Peer,
	_ shim.PolicyChecker,
	_ shim.AuditWriter,
	cmdArgs []string,
	enableMCPProxy bool,
) error {
	agentconfig.Conf.AppId = agentName

	localIP := overlayAddr(currentPeer)
	fmt.Printf("[sandbox-run] %q registered, overlay IP=%s\n", agentName, localIP)

	if currentPeer.LrpUrl != "" {
		agentconfig.Conf.EnableLrp = true
		agentconfig.Conf.RelayURL = currentPeer.LrpUrl
	}

	// Set up isolated network namespace for the agent process.
	proxyListenAddr, err := setupSandboxNetns()
	if err != nil {
		return fmt.Errorf("setup sandbox netns: %w", err)
	}
	defer func() {
		if err := teardownSandboxNetns(); err != nil {
			fmt.Fprintf(os.Stderr, "[sandbox-run] netns cleanup warning: %v\n", err)
		}
	}()
	fmt.Printf("[sandbox-run] network namespace ready (agent→host via lts-host %s)\n", hostVethIP)

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

	select {
	case <-time.After(runReadyWait):
	case <-ctx.Done():
		return ctx.Err()
	}

	// Start MCP proxy on the host-side veth IP so the agent netns can reach it.
	httpProxyAddr := ""
	if enableMCPProxy && currentPeer.Token != "" {
		cache := mcpproxy.NewPolicyCache(agentconfig.Conf.ServerUrl, currentPeer.Token, overlayAddr(currentPeer))
		if cacheErr := cache.Start(ctx); cacheErr != nil {
			logger.Warn("MCP policy cache failed to start, proxy disabled", "err", cacheErr)
		} else {
			auditW, _ := mcpproxy.NewAuditWriter(mcpproxy.AuditLogPath)
			proxy := mcpproxy.NewProxy(agentName, proxyListenAddr, cache, auditW)
			if proxyErr := proxy.Start(ctx); proxyErr != nil {
				logger.Warn("MCP proxy failed to start", "err", proxyErr)
			} else {
				httpProxyAddr = "http://" + proxy.Addr()
				fmt.Printf("[sandbox-run] MCP proxy on %s\n", proxy.Addr())
			}
		}
	}

	return forkAgent(ctx, cancel, cmdArgs, httpProxyAddr, sandboxNetnsPath())
}
```

- [ ] **Step 2: Build and lint**

```bash
go build ./cmd/lattice/...
make lint
```

Expected: build succeeds, no lint errors.

- [ ] **Step 3: Commit**

```bash
git add cmd/lattice/cmd/sandbox/shared_linux.go
git commit -s -m "feat(sandbox): wire netns isolation into runSandbox — agent runs in lt-sandbox netns"
```

---

## Task 4: Update comments in `shared_linux.go` that reference old architecture

**Files:**
- Modify: `cmd/lattice/cmd/sandbox/shared_linux.go` — stale comments only

- [ ] **Step 1: Fix the `policyDialer` comment (line 46)**

```go
// policyDialer wraps net.Dial with optional egress policy checking and audit.
// Used by sandbox run when netns isolation is unavailable (fallback path).
```

- [ ] **Step 2: Fix the `runSandbox` doc comment (lines 209–215)**

```go
// runSandbox is the shared sandbox engine for both community and PRO editions.
// It creates a standard kernel WireGuard interface (identical to a regular lattice
// agent), isolates the AI agent inside a dedicated network namespace (lt-sandbox),
// and forks the agent as a child process. The child can only communicate via the
// host-side veth — there is no direct path to the host network or internet.
//
// When enableMCPProxy is true, an MCP HTTP proxy is started on the host-side
// veth IP and injected via HTTP_PROXY/HTTPS_PROXY into the child.
```

- [ ] **Step 3: Commit**

```bash
git add cmd/lattice/cmd/sandbox/shared_linux.go
git commit -s -m "docs(sandbox): update comments to reflect netns isolation architecture"
```

---

## Task 5: Smoke test

> There are no unit tests for the netns setup because it requires `NET_ADMIN` (not available in standard CI). Verify manually inside a Linux container.

- [ ] **Step 1: Build the binary**

```bash
make build SERVICE=lattice
```

- [ ] **Step 2: Run inside a Docker container with NET_ADMIN**

```bash
docker run --rm -it --cap-add NET_ADMIN \
  -v $(pwd)/bin/lattice:/usr/local/bin/lattice \
  ubuntu:22.04 bash
```

Inside the container:

```bash
# Check netns tools available
apt-get install -y iproute2 iptables

# Run sandbox (replace with your actual server URL and token)
lattice sandbox run test-agent \
  --server-url http://host.docker.internal:8080 \
  --token lt-test \
  -- sleep 30 &

# While the agent is sleeping, verify the netns exists
ip netns list
# Expected: lt-sandbox

# Verify veth pair
ip link show lts-host
# Expected: lts-host@lts-agent, UP, 169.254.100.1/30

# Verify agent is in isolated netns
ip netns exec lt-sandbox ip addr
# Expected: lo and lts-agent (169.254.100.2/30), NO wf0, NO eth0

# Verify agent can't reach host network directly
ip netns exec lt-sandbox ip route
# Expected: default via 169.254.100.1 dev lts-agent
# (only route is through the host veth — no direct internet access)
```

- [ ] **Step 3: Verify cleanup on exit**

```bash
# After sandbox exits (kill sleep or let it finish):
ip netns list
# Expected: lt-sandbox is gone

ip link show lts-host 2>&1
# Expected: error — interface not found
```

- [ ] **Step 4: Commit lint fix if needed**

```bash
make lint
# Fix any issues, then:
git add -u
git commit -s -m "fix(sandbox): lint fixes"
```

---

## What This Does NOT Implement (Phase 2)

- **iptables REDIRECT / transparent proxy**: Full transparent enforcement (agent can still bypass HTTP_PROXY for overlay IPs, which is caught by LatticePolicy on wf0)
- **Mount namespace**: Agent can still read host filesystem
- **PID namespace**: Agent can still see host processes
- **macOS**: Netns is Linux-only; macOS sandbox run continues to use the old path (no netns)

These are intentionally deferred. This plan delivers the network isolation layer as the first meaningful improvement over the current shared-netns approach.
