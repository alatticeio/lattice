# Sentry-Backed Process Sandbox Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `shim/sentry` package to lattice-shim that wraps gVisor Sentry as a Go library, then simplify `sandbox run` to use it — achieving full syscall isolation with no iptables, no tproxy, no runsc, zero extra Docker flags.

**Architecture:** lattice-shim gains a `sentry` package that imports `gvisor.dev/gvisor/runsc/boot` to create a ptrace-based Sentry, injects our `*stack.Stack` (WireGuard-attached) into it, and forks the AI agent. All AI agent syscalls (file, network, process, memory) are intercepted by Sentry. Network syscalls go directly through our WireGuard stack without host kernel involvement.

**Tech Stack:** Go 1.25, gVisor pinned at v0.0.0-20260509025911 (lattice-shim's current dep), lattice-shim `shim` package.

**Repos affected:** lattice-shim (primary), lattice (consumer).

---

## Current State

Working tree has uncommitted changes from an earlier embedded-netstack approach (aborted in favor of Sentry):

- `cmd/lattice/cmd/sandbox/shared_linux.go` — NEW: helpers + `installRunIPTables` + `forkAndWait` with UID tricks
- `cmd/lattice/cmd/sandbox/run_community.go` — MODIFIED: `!pro && linux`, embedded netstack + iptables + tproxy
- `cmd/lattice/cmd/sandbox/run_community_stub.go` — NEW: empty `addRunCmd` for `!pro && !linux`

These files evolve into the Sentry approach in Tasks 5-7 below.

---

## File Structure

### lattice-shim repo:

| Action | File | Purpose |
|--------|------|---------|
| Create | `shim/sentry/sentry.go` | `Start()`, `Wait()`, `Kill()` — main Sentry integration |
| Create | `shim/sentry/sentry_stub.go` | Stub for non-Linux builds |
| Create | `shim/sentry/sentry_test.go` | Unit tests |

### lattice repo:

| Action | File | Purpose |
|--------|------|---------|
| Modify | `cmd/lattice/cmd/sandbox/run_community.go` | Simplify: remove iptables/tproxy, call `sentry.Start` |
| Modify | `cmd/lattice/cmd/sandbox/run_pro.go` | Same + egress via `shimfwd.NewEgressFilter` |
| Modify | `cmd/lattice/cmd/sandbox/shared_linux.go` | Remove `installRunIPTables` and `sandboxAgentUID` constant |
| Keep | `cmd/lattice/cmd/sandbox/run_community_stub.go` | No changes needed |
| Keep | `cmd/lattice/cmd/sandbox/run_pro_stub.go` | No changes needed |
| Modify | `frontend/src/components/SandboxDemoModal.vue` | Remove `--cap-add NET_ADMIN` from docker command |

---

### Task 1: lattice-shim — explore gVisor Sentry network stack injection point

**Files:**
- Research: `gvisor.dev/gvisor/runsc/boot` (at pinned version)
- Research: `gvisor.dev/gvisor/pkg/sentry/kernel`

- [ ] **Step 1: Find the network stack injection API**

Read the gVisor source at the pinned version to find how to inject a custom `*stack.Stack` into a Sentry kernel. Key files to examine:

```bash
# In the gvisor module cache:
go doc gvisor.dev/gvisor/runsc/boot.Loader
go doc gvisor.dev/gvisor/pkg/sentry/kernel.Kernel
```

The goal is to find one of:
1. A `NetworkStack()` or `SetNetworkStack()` method on `kernel.Kernel`
2. A config option in `boot.New()` to pass a custom stack
3. A way to create the kernel separately and pass it to the Loader

At the pinned gVisor version (`v0.0.0-20260509025911`), examine `runsc/boot/loader.go` for how the kernel is created. The `createKernel()` function calls `kernel.New()` which creates a `*stack.Stack` internally.

Expected finding: `kernel.Kernel` has a `SetNetworkStack(*stack.Stack)` method (used in gVisor's own tests). If this exists, proceed. If not, the fallback is to create the kernel's stack by calling `kernel.NetworkStack()` and attaching our WireGuard channel endpoint as a NIC to it.

- [ ] **Step 2: Document the injection approach**

Write a 3-5 line comment summarizing the chosen injection approach. This guides Task 2 implementation.

---

### Task 2: lattice-shim — sentry package

**Files:**
- Create: `shim/sentry/sentry.go`

- [ ] **Step 1: Write the failing tests**

Create `shim/sentry/sentry_test.go`:

```go
//go:build linux

package sentry_test

import (
    "bytes"
    "context"
    "os"
    "testing"
    "time"

    "github.com/alatticeio/lattice-shim/shim/sentry"
)

// TestSentryStartEcho verifies that a trivial command runs under Sentry
// and produces expected stdout.
func TestSentryStartEcho(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    var stdout bytes.Buffer
    proc, err := sentry.Start(ctx, sentry.Config{
        Args:   []string{"/bin/echo", "hello-sentry"},
        Stdout: &stdout,
    })
    if err != nil {
        t.Fatalf("Start: %v", err)
    }

    code, err := proc.Wait()
    if err != nil {
        t.Fatalf("Wait: %v", err)
    }
    if code != 0 {
        t.Errorf("expected exit 0, got %d", code)
    }
    if got := stdout.String(); got != "hello-sentry\n" {
        t.Errorf("expected 'hello-sentry\\n', got %q", got)
    }
}

// TestSentryStartExitCode verifies non-zero exit codes are propagated.
func TestSentryStartExitCode(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    proc, err := sentry.Start(ctx, sentry.Config{
        Args: []string{"/bin/sh", "-c", "exit 42"},
    })
    if err != nil {
        t.Fatalf("Start: %v", err)
    }

    code, err := proc.Wait()
    if err != nil {
        t.Fatalf("Wait: %v", err)
    }
    if code != 42 {
        t.Errorf("expected exit 42, got %d", code)
    }
}

// TestSentryKill verifies that Kill terminates the sandboxed process.
func TestSentryKill(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    proc, err := sentry.Start(ctx, sentry.Config{
        Args: []string{"/bin/sleep", "60"},
    })
    if err != nil {
        t.Fatalf("Start: %v", err)
    }

    if err := proc.Kill(); err != nil {
        t.Fatalf("Kill: %v", err)
    }

    code, err := proc.Wait()
    if err != nil {
        t.Fatalf("Wait: %v", err)
    }
    // Killed processes should have non-zero exit code
    if code == 0 {
        t.Error("expected non-zero exit after Kill")
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /Users/francis/workspc/lattice-shim && go test ./shim/sentry/ -v
```

Expected: FAIL — `sentry.go` doesn't exist yet.

- [ ] **Step 3: Write sentry.go implementation**

Create `shim/sentry/sentry.go`:

```go
//go:build linux

// Copyright 2026 The Lattice Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// ...

// Package sentry provides process sandboxing via gVisor Sentry (ptrace).
// It wraps gVisor's runsc/boot and runsc/container as a Go library so
// callers (like lattice sandbox run) can launch processes with full
// syscall isolation — no runsc binary required.
package sentry

import (
    "context"
    "fmt"
    "io"
    "syscall"

    "gvisor.dev/gvisor/pkg/tcpip/stack"
    "gvisor.dev/gvisor/runsc/boot"
    "gvisor.dev/gvisor/runsc/container"
    "gvisor.dev/gvisor/runsc/specutils"
    "gvisor.dev/gvisor/pkg/sentry/platform"
    "gvisor.dev/gvisor/pkg/sentry/platform/ptrace"
)

// Config describes the sandboxed process to launch.
type Config struct {
    // Args is the command to execute (e.g. ["python", "agent.py"]).
    Args []string
    // Env is environment variables. nil = inherit parent's env.
    Env []string
    // WorkDir is the working directory. "" = inherit parent's cwd.
    WorkDir string
    // Network is the injected gVisor netstack. nil = no networking.
    // Callers inject a *stack.Stack with WireGuard channel endpoint attached.
    Network *stack.Stack
    // Stdout is the writer for the child's stdout. nil = os.Stdout.
    Stdout io.Writer
    // Stderr is the writer for the child's stderr. nil = os.Stderr.
    Stderr io.Writer
    // Stdin is the reader for the child's stdin. nil = os.Stdin.
    Stdin io.Reader
}

// Process represents a running Sentry-sandboxed child process.
type Process struct {
    c *container.Container
}

// Start launches a child process wrapped in gVisor Sentry via ptrace.
// Sentry intercepts all syscalls. The caller injects *stack.Stack to
// control network routing (e.g. WireGuard overlay via channel endpoint).
func Start(ctx context.Context, cfg Config) (*Process, error) {
    // Build minimal OCI spec.
    spec := specutils.NewSimpleSpec(cfg.Args, cfg.Env, cfg.WorkDir)

    // Configure Sentry platform: ptrace (most compatible, no hardware virt needed).
    conf := &boot.Config{
        // Platform: use ptrace for syscall interception.
        Platform: platform.Ptrace,
        // Network: if cfg.Network is nil, use "none"; otherwise inject our stack.
        // The kernel.NetworkStack() method is used to replace the default stack.
        Network: boot.NetworkNone,
        // FileAccess: "exclusive" for syscall-level file interception.
        FileAccess: boot.FileAccessExclusive,
    }

    // Create the Sentry container.
    cid := fmt.Sprintf("sentry-%d", syscall.Gettid())
    c, err := container.New(cid, spec, conf, nil, nil)
    if err != nil {
        return nil, fmt.Errorf("sentry: create container: %w", err)
    }

    // Inject our custom network stack if provided.
    if cfg.Network != nil {
        k := c.Sandbox().Kernel()
        if k == nil {
            c.Destroy()
            return nil, fmt.Errorf("sentry: kernel not available for stack injection")
        }
        k.SetNetworkStack(cfg.Network)
    }

    // Set stdout/stderr.
    if cfg.Stdout != nil {
        c.SetStdout(cfg.Stdout)
    }
    if cfg.Stderr != nil {
        c.SetStderr(cfg.Stderr)
    }

    // Start the container (forks child, ptrace attaches).
    if err := c.Start(conf); err != nil {
        c.Destroy()
        return nil, fmt.Errorf("sentry: start container: %w", err)
    }

    return &Process{c: c}, nil
}

// Wait blocks until the process exits and returns its exit code.
func (p *Process) Wait() (int, error) {
    ws, err := p.c.Wait()
    if err != nil {
        return -1, err
    }
    p.c.Destroy()
    return ws.ExitStatus(), nil
}

// Kill forcefully terminates the sandboxed process.
func (p *Process) Kill() error {
    return p.c.SignalContainer(syscall.SIGKILL)
}
```

> **Note for implementer:** The exact container API (`container.New`, `c.Sandbox().Kernel()`, `k.SetNetworkStack`, `c.Start`, `c.SetStdout`, etc.) at the pinned gVisor version may differ slightly from the pseudo-code above. Adapt function signatures to match the actual API. The core logic is:
> 1. Build minimal OCI spec → `container.New()` → inject custom stack via kernel → `c.Start()` → `c.Wait()`.

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/francis/workspc/lattice-shim && go test ./shim/sentry/ -v
```

Expected: 3/3 PASS.

- [ ] **Step 5: Commit**

```bash
git -C /Users/francis/workspc/lattice-shim add shim/sentry/sentry.go shim/sentry/sentry_test.go
git -C /Users/francis/workspc/lattice-shim commit -s -m "feat(shim): add sentry package for gVisor process sandboxing"
```

---

### Task 3: lattice-shim — sentry stub for non-Linux

**Files:**
- Create: `shim/sentry/sentry_stub.go`

- [ ] **Step 1: Write the stub**

```go
//go:build !linux

package sentry

import (
    "context"
    "fmt"
)

func Start(ctx context.Context, cfg Config) (*Process, error) {
    return nil, fmt.Errorf("sentry: process sandboxing requires Linux (ptrace)")
}
```

- [ ] **Step 2: Verify it compiles on non-Linux**

```bash
cd /Users/francis/workspc/lattice-shim && GOOS=darwin go build ./shim/sentry/
```

Expected: no errors (the stub is used).

- [ ] **Step 3: Commit**

```bash
git -C /Users/francis/workspc/lattice-shim add shim/sentry/sentry_stub.go
git -C /Users/francis/workspc/lattice-shim commit -s -m "feat(shim): add non-linux sentry stub"
```

---

### Task 4: lattice — simplify shared_linux.go

**Files:**
- Modify: `cmd/lattice/cmd/sandbox/shared_linux.go` (uncommitted, NEW file)

The current uncommitted `shared_linux.go` contains:
- `sandboxAgentUID` constant → REMOVE
- `registerOrResume` → KEEP
- `runPeriodicRefresh` → KEEP
- `installRunIPTables` → REMOVE
- `forkAndWait` (with UID tricks) → REPLACE with version that calls `sentry.Start`

- [ ] **Step 1: Rewrite shared_linux.go to remove iptables/UID code, update forkAndWait**

```go
//go:build linux

package sandbox

import (
    "context"
    "fmt"
    "os"
    "time"

    "github.com/alatticeio/lattice-shim/shim/sentry"
    latticeagent "github.com/alatticeio/lattice/internal/agent"
    "github.com/alatticeio/lattice/internal/agent/gvisor"
    "github.com/alatticeio/lattice/internal/agent/infra"
    "golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// registerOrResume tries to resume from persisted credentials; falls back to
// fresh NATS registration.
func registerOrResume(ctx context.Context, agentName, serverURL, token string) (*infra.Peer, error) {
    if creds, err := loadSandboxCredentials(); err == nil {
        if key, parseErr := wgtypes.ParseKey(creds.PrivateKey); parseErr == nil {
            if peer, resumeErr := latticeagent.ResumeSandboxViaNATS(ctx, serverURL, creds.JWT, agentName, key); resumeErr == nil {
                fmt.Printf("[sandbox-run] resumed %q from saved credentials\n", agentName)
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
        fmt.Printf("[sandbox-run] warning: persist credentials: %v\n", saveErr)
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

// forkInSentry launches the AI agent under gVisor Sentry, which provides
// full syscall isolation (file, network, process, memory). The caller's
// gVisor netstack (*stack.Stack with WireGuard endpoint) is injected so
// all AI agent network traffic routes through the WireGuard overlay without
// iptables, tproxy, or kernel TUN device.
func forkInSentry(ctx context.Context, sb *gvisor.Sandbox, cmdArgs []string) (int, error) {
    proc, err := sentry.Start(ctx, sentry.Config{
        Args:    cmdArgs,
        Env:     os.Environ(),
        Stdin:   os.Stdin,
        Stdout:  os.Stdout,
        Stderr:  os.Stderr,
        Network: sb.Netstack().Stack(),
    })
    if err != nil {
        return -1, fmt.Errorf("sentry start: %w", err)
    }
    return proc.Wait()
}
```

- [ ] **Step 2: Commit**

```bash
git add cmd/lattice/cmd/sandbox/shared_linux.go
git commit -s -m "refactor(sandbox): simplify shared_linux.go, remove iptables/UID code"
```

---

### Task 5: lattice — simplify run_community.go

**Files:**
- Modify: `cmd/lattice/cmd/sandbox/run_community.go` (uncommitted, MODIFIED)

Current state: uses embedded netstack + `installRunIPTables` + `tproxy.Proxy` + `forkAndWait`.
Target: remove iptables/tproxy, call `forkInSentry` instead of `forkAndWait`.

- [ ] **Step 1: Rewrite run_community.go**

```go
//go:build !pro && linux

package sandbox

import (
    "context"
    "fmt"
    "os"
    "time"

    latticeagent "github.com/alatticeio/lattice/internal/agent"
    agentconfig "github.com/alatticeio/lattice/internal/agent/config"
    "github.com/alatticeio/lattice/internal/agent/gvisor"
    "github.com/alatticeio/lattice/internal/agent/infra"
    agentlog "github.com/alatticeio/lattice/internal/agent/log"
    "github.com/alatticeio/lattice/internal/agent/provision"
    "github.com/spf13/cobra"
    wgdevice "golang.zx2c4.com/wireguard/device"
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
        Short: "Run an AI agent under gVisor Sentry with zero-privilege isolation",
        Long: `Run registers a sandbox with the Lattice control plane, starts a WireGuard
node backed by an embedded gVisor netstack, then forks the given command under
gVisor Sentry (ptrace). Sentry provides full syscall isolation — no runsc,
no iptables, no capabilities required.

Example:
  docker run ghcr.io/alatticeio/lattice \
    sandbox run my-agent --server-url http://latticed:8080 --token lt-xxx \
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
        return fmt.Errorf("usage: lattice sandbox run <name> -- <command> [args...]")
    }
    agentName := args[0]
    cmdArgs := args[1:]

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    agentconfig.Conf.AppId = agentName
    agentconfig.Conf.ServerUrl = runServerURL
    agentconfig.Conf.WgPort = 0 // embedded netstack, no kernel wg0

    currentPeer, err := registerOrResume(ctx, agentName, runServerURL, runToken)
    if err != nil {
        return err
    }

    localIP := overlayAddr(currentPeer)
    fmt.Printf("[sandbox-run] %q registered, overlay IP=%s\n", agentName, localIP)

    if currentPeer.LrpUrl != "" {
        agentconfig.Conf.EnableLrp = true
        agentconfig.Conf.RelayURL = currentPeer.LrpUrl
    }

    // Create gVisor netstack (community: no PolicyChecker or AuditWriter).
    sb, err := gvisor.New(gvisor.Config{
        ID:      agentName,
        LocalIP: localIP,
    })
    if err != nil {
        return fmt.Errorf("create gVisor sandbox: %w", err)
    }
    defer sb.Close() //nolint:errcheck

    tunDev := gvisor.NewTUNAdapter(sb.Channel(), gvisor.InjectIntoChannel(sb.Channel()))

    logger := agentlog.GetLogger("sandbox-run")
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
    go runPeriodicRefresh(ctx, node, logger)

    select {
    case <-time.After(runReadyWait):
    case <-ctx.Done():
        return ctx.Err()
    }

    // Fork AI agent under Sentry — full syscall isolation.
    // Sentry's network stack IS our WireGuard-attached netstack.
    code, err := forkInSentry(ctx, sb, cmdArgs)
    if err != nil {
        return err
    }
    os.Exit(code)
    return nil // unreachable
}
```

- [ ] **Step 2: Commit**

```bash
git add cmd/lattice/cmd/sandbox/run_community.go
git commit -s -m "refactor(sandbox): simplify run_community to use sentry.Start, remove iptables/tproxy"
```

---

### Task 6: lattice — simplify run_pro.go

**Files:**
- Modify: `cmd/lattice/cmd/sandbox/run_pro.go` (committed, needs full rewrite)

Current state: old kernel wg0 + runsc approach.
Target: same as run_community.go + egress policy via `shimfwd.NewEgressFilter` + `fileAuditWriter`.

- [ ] **Step 1: Rewrite run_pro.go**

```go
//go:build pro && linux

package sandbox

import (
    "context"
    "encoding/json"
    "fmt"
    "net"
    "os"
    "strings"
    "sync"
    "time"

    shimfwd "github.com/alatticeio/lattice-shim/shim"
    latticeagent "github.com/alatticeio/lattice/internal/agent"
    agentconfig "github.com/alatticeio/lattice/internal/agent/config"
    "github.com/alatticeio/lattice/internal/agent/gvisor"
    "github.com/alatticeio/lattice/internal/agent/infra"
    agentlog "github.com/alatticeio/lattice/internal/agent/log"
    "github.com/alatticeio/lattice/internal/agent/provision"
    "github.com/spf13/cobra"
    wgdevice "golang.zx2c4.com/wireguard/device"
)

var (
    runServerURL   string
    runToken       string
    runReadyWait   time.Duration
    runEgressAllow string
    runEgressDeny  bool
)

func addRunCmd(parent *cobra.Command) {
    parent.AddCommand(runCmd())
}

func runCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "run <name> -- <command> [args...]",
        Short: "Run an AI agent under gVisor Sentry with full isolation (Pro)",
        Long: `Run registers a sandbox, starts WireGuard via embedded netstack, then forks
the given command under gVisor Sentry. Pro adds egress policy and audit logging.

Example:
  docker run ghcr.io/alatticeio/lattice \
    sandbox run my-agent --server-url http://latticed:8080 --token lt-xxx \
    --egress-allow 10.0.0.0/8 --egress-default-deny \
    -- python agent.py`,
        Args: cobra.ArbitraryArgs,
        RunE: runRun,
    }
    cmd.Flags().StringVar(&runServerURL, "server-url", "", "Lattice control plane URL (required)")
    cmd.Flags().StringVar(&runToken, "token", "", "Enrollment token (required)")
    cmd.Flags().DurationVar(&runReadyWait, "ready-wait", 3*time.Second,
        "Time to wait for WireGuard peer sessions")
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
        return fmt.Errorf("usage: lattice sandbox run <name> -- <command> [args...]")
    }
    agentName := args[0]
    cmdArgs := args[1:]

    // Build egress policy.
    egressPolicy := shimfwd.EgressPolicy{DefaultDeny: runEgressDeny}
    if runEgressAllow != "" {
        for _, entry := range strings.Split(runEgressAllow, ",") {
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
    agentconfig.Conf.ServerUrl = runServerURL
    agentconfig.Conf.WgPort = 0

    currentPeer, err := registerOrResume(ctx, agentName, runServerURL, runToken)
    if err != nil {
        return err
    }

    localIP := overlayAddr(currentPeer)
    fmt.Printf("[sandbox-run] %q registered, overlay IP=%s\n", agentName, localIP)

    if currentPeer.LrpUrl != "" {
        agentconfig.Conf.EnableLrp = true
        agentconfig.Conf.RelayURL = currentPeer.LrpUrl
    }

    policyChecker := shimfwd.NewEgressFilter(egressPolicy)
    auditWriter, auditErr := newFileAuditWriter(auditLogPath)
    if auditErr != nil {
        fmt.Printf("[sandbox-run] warning: open audit log: %v\n", auditErr)
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

    logger := agentlog.GetLogger("sandbox-run")
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
    go runPeriodicRefresh(ctx, node, logger)

    select {
    case <-time.After(runReadyWait):
    case <-ctx.Done():
        return ctx.Err()
    }

    code, err := forkInSentry(ctx, sb, cmdArgs)
    if err != nil {
        return err
    }
    os.Exit(code)
    return nil // unreachable
}
```

- [ ] **Step 2: Commit**

```bash
git add cmd/lattice/cmd/sandbox/run_pro.go
git commit -s -m "refactor(sandbox): simplify run_pro to use sentry.Start with egress policy"
```

---

### Task 7: lattice — commit run_community_stub.go and update deps

**Files:**
- Keep: `cmd/lattice/cmd/sandbox/run_community_stub.go` (uncommitted, NEW)
- Modify: `go.mod` — ensure lattice-shim dependency is updated

- [ ] **Step 1: Commit the stub file**

```bash
git add cmd/lattice/cmd/sandbox/run_community_stub.go
git commit -s -m "feat(sandbox): add non-linux stub for run command"
```

- [ ] **Step 2: Update lattice-shim dependency**

After lattice-shim Tasks 1-3 are done and pushed, update lattice's dependency:

```bash
go get github.com/alatticeio/lattice-shim@latest
go mod tidy
```

- [ ] **Step 3: Commit dependency update**

```bash
git add go.mod go.sum
git commit -s -m "chore(deps): update lattice-shim for sentry sandbox integration"
```

---

### Task 8: lattice — update frontend

**Files:**
- Modify: `frontend/src/components/SandboxDemoModal.vue`

- [ ] **Step 1: Update docker command in runCmd computed**

The current command includes `--cap-add NET_ADMIN`. Remove it since Sentry-based sandbox needs zero extra Docker flags.

Find in `SandboxDemoModal.vue`:

```ts
const runCmd = computed(() => {
  if (!session.value) return ''
  const p = presets.find(x => x.value === preset.value) ?? presets[0]
  return `docker run --rm --cap-add NET_ADMIN ghcr.io/alatticeio/lattice sandbox run demo-agent --server-url ${session.value.server_url} --token ${session.value.token} ${p.suffix}`
})
```

Replace with:

```ts
const runCmd = computed(() => {
  if (!session.value) return ''
  const p = presets.find(x => x.value === preset.value) ?? presets[0]
  return `docker run --rm ghcr.io/alatticeio/lattice sandbox run demo-agent --server-url ${session.value.server_url} --token ${session.value.token} ${p.suffix}`
})
```

The prerequisite notice text changes from "Requires Docker with gVisor runtime" to:

```html
<div class="rounded-lg border border-border bg-muted/40 px-3 py-2 text-xs text-muted-foreground">
  Zero-privilege sandbox — no gVisor runtime required. AI agent runs under Lattice's embedded Sentry with full syscall isolation.
</div>
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/SandboxDemoModal.vue
git commit -s -m "feat(sandbox): remove --cap-add from try-sandbox command (Sentry-based)"
```

---

### Task 9: lattice — verify build and lint

**Files:**
- All changed files

- [ ] **Step 1: Build community version**

```bash
cd /Users/francis/workspc/lattice && make build
```

Expected: build succeeds, no errors.

- [ ] **Step 2: Run lint**

```bash
cd /Users/francis/workspc/lattice && make lint
```

Expected: 0 issues.

- [ ] **Step 3: Build PRO version**

```bash
cd /Users/francis/workspc/lattice && make build EDITION=pro
```

Expected: build succeeds, no errors.

- [ ] **Step 4: Run unit tests**

```bash
cd /Users/francis/workspc/lattice && go test ./cmd/lattice/cmd/sandbox/...
```

Expected: all tests pass.

- [ ] **Step 5: Fix any issues and commit**

```bash
git add -A
git commit -s -m "chore(sandbox): fix build and lint after sentry refactor"
```
