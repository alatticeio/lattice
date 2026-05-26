# `lattice sandbox run` Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `lattice sandbox run -- <command>` that starts a gVisor sandbox, injects `ALL_PROXY` into the child process environment, execs the AI agent, and auto-cleans up when the agent exits.

**Architecture:** `PodDriver.Start()` is refactored to signal a `ReadyCh` channel when the sandbox and SOCKS5 proxy are ready, then blocks on ctx. `sandbox run` starts the driver in a goroutine, waits for the ready signal, injects `ALL_PROXY=socks5://<addr>` into the child env, execs the child, and cancels the driver context when the child exits. Community edition registers a no-op (the command doesn't appear in help).

**Tech Stack:** Go stdlib (`os/exec`, `context`, `net`, `syscall`), Cobra, existing sandbox package (`DriverConfig`, `PodDriver`, `IsolationDriver`)

---

## File Map

| Action | File | Purpose |
|--------|------|---------|
| Modify | `cmd/lattice/cmd/sandbox/driver.go` | Add `ReadyCh chan<- struct{}` to `DriverConfig` |
| Modify | `cmd/lattice/cmd/sandbox/driver_pod.go` | Signal `ReadyCh` after sandbox+SOCKS5 is ready |
| Modify | `cmd/lattice/cmd/sandbox/sandbox.go` | Register `runCmd` via `registerRunCmd(cmd)` |
| Create | `cmd/lattice/cmd/sandbox/sandbox_run_community.go` | `//go:build !pro` stub — `registerRunCmd` no-op |
| Create | `cmd/lattice/cmd/sandbox/sandbox_run_pro.go` | `//go:build pro && linux` — full run implementation |

---

### Task 1: Add `ReadyCh` to `DriverConfig`

**Files:**
- Modify: `cmd/lattice/cmd/sandbox/driver.go`

- [ ] **Step 1: Add `ReadyCh` field**

Open `driver.go`. After the `BundleDir` field, add:

```go
// ReadyCh, if non-nil, receives a signal when the sandbox is fully ready
// (SOCKS5 listening, WireGuard node started). Used by sandbox run to know
// when to inject ALL_PROXY and exec the child process.
ReadyCh chan<- struct{}
```

Final `DriverConfig` struct:

```go
type DriverConfig struct {
	// Common fields.
	SandboxName string
	ServerURL   string
	Token       string
	EgressAllow string
	EgressDeny  bool

	// pod mode fields.
	ProxyAddr    string
	ForwardRules []string

	// gvisor (runsc) mode fields.
	RootFS      string
	AgentBinary string
	AgentArgs   []string
	BundleDir   string

	// ReadyCh, if non-nil, receives a signal when the sandbox is fully ready
	// (SOCKS5 listening, WireGuard node started). Used by sandbox run to know
	// when to inject ALL_PROXY and exec the child process.
	ReadyCh chan<- struct{}
}
```

- [ ] **Step 2: Build to verify no compile errors**

```bash
cd /Users/francis/workspc/lattice
make build SERVICE=lattice 2>&1 | tail -5
```

Expected: build succeeds (or only pre-existing errors).

- [ ] **Step 3: Commit**

```bash
git add cmd/lattice/cmd/sandbox/driver.go
git commit -s -m "feat(sandbox): add ReadyCh to DriverConfig for run command readiness"
```

---

### Task 2: Signal `ReadyCh` in `PodDriver.Start()`

**Files:**
- Modify: `cmd/lattice/cmd/sandbox/driver_pod.go` (lines ~218–228, after node start)

- [ ] **Step 1: Add readiness signal after node.Start()**

In `driver_pod.go`, find the block:

```go
	fmt.Printf("Sandbox %q ready (pod mode), overlay IP=%s\n", cfg.SandboxName, localIP)

	sigCh := make(chan os.Signal, 1)
```

Replace with:

```go
	fmt.Printf("Sandbox %q ready (pod mode), overlay IP=%s\n", cfg.SandboxName, localIP)

	if cfg.ReadyCh != nil {
		cfg.ReadyCh <- struct{}{}
	}

	sigCh := make(chan os.Signal, 1)
```

- [ ] **Step 2: Build PRO to verify**

```bash
cd /Users/francis/workspc/lattice
make EDITION=pro build SERVICE=lattice 2>&1 | tail -5
```

Expected: build succeeds.

- [ ] **Step 3: Commit**

```bash
git add cmd/lattice/cmd/sandbox/driver_pod.go
git commit -s -m "feat(sandbox): signal ReadyCh when pod sandbox is ready"
```

---

### Task 3: Register `runCmd` in `sandbox.go`

**Files:**
- Modify: `cmd/lattice/cmd/sandbox/sandbox.go`

- [ ] **Step 1: Add `registerRunCmd` call**

In `sandbox.go`, find `SandboxCmd()`. Change:

```go
func SandboxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sandbox",
		Short: "Manage sandboxed agent environments (Pro)",
	}
	cmd.AddCommand(startCmd())
	registerAgentCmd(cmd)
	return cmd
}
```

To:

```go
func SandboxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sandbox",
		Short: "Manage sandboxed agent environments (Pro)",
	}
	cmd.AddCommand(startCmd())
	registerAgentCmd(cmd)
	registerRunCmd(cmd)
	return cmd
}
```

- [ ] **Step 2: Build to verify (both editions)**

```bash
cd /Users/francis/workspc/lattice
make build SERVICE=lattice 2>&1 | tail -5
make EDITION=pro build SERVICE=lattice 2>&1 | tail -5
```

Both should fail with "undefined: registerRunCmd" — that is expected, we add it next.

- [ ] **Step 3: Don't commit yet** — wait for Task 4 which provides the missing symbol.

---

### Task 4: Community stub

**Files:**
- Create: `cmd/lattice/cmd/sandbox/sandbox_run_community.go`

- [ ] **Step 1: Create the stub file**

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

package sandbox

import "github.com/spf13/cobra"

// registerRunCmd is a no-op in community builds; sandbox run requires Pro.
func registerRunCmd(_ *cobra.Command) {}
```

- [ ] **Step 2: Build community to verify**

```bash
cd /Users/francis/workspc/lattice
make build SERVICE=lattice 2>&1 | tail -5
```

Expected: build succeeds.

- [ ] **Step 3: Commit Tasks 3 + 4 together**

```bash
git add cmd/lattice/cmd/sandbox/sandbox.go cmd/lattice/cmd/sandbox/sandbox_run_community.go
git commit -s -m "feat(sandbox): register sandbox run command (community stub)"
```

---

### Task 5: PRO implementation of `sandbox run`

**Files:**
- Create: `cmd/lattice/cmd/sandbox/sandbox_run_pro.go`

This is the core implementation. It:
1. Pre-allocates a random local TCP port for SOCKS5 (avoids needing `socks5.Addr()`)
2. Builds a `DriverConfig` with `ReadyCh` and the chosen proxy addr
3. Runs `PodDriver.Start()` in a goroutine
4. Waits for the ready signal (or timeout / driver failure)
5. Execs the child with `ALL_PROXY` injected
6. When child exits: cancels driver context, propagates exit code

- [ ] **Step 1: Create `sandbox_run_pro.go`**

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

package sandbox

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var (
	sandboxRunProxyAddr    string
	sandboxRunReadyTimeout time.Duration
)

// registerRunCmd adds `lattice sandbox run` to the sandbox parent command.
func registerRunCmd(parent *cobra.Command) {
	parent.AddCommand(runCmd())
}

func runCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run an AI agent inside a Lattice sandbox (Pro)",
		Long: `Run starts a gVisor-based network sandbox, injects ALL_PROXY into the
child process environment, and executes the given command. When the child
process exits, the sandbox is automatically cleaned up.

The child process can use any standard HTTP/HTTPS client that respects
ALL_PROXY (curl, Python requests/httpx, Node.js fetch, Go net/http, etc.)
to route traffic through the Lattice overlay network without any code changes.

Examples:

  # Run a Python agent through the sandbox:
  lattice sandbox run --name my-agent --server-url http://latticed:8080 --token lt-xxx \
    -- python agent.py --task "analyze data"

  # Run Claude CLI through the sandbox:
  lattice sandbox run --name my-agent --server-url http://latticed:8080 --token lt-xxx \
    -- claude --model claude-opus-4-6

  # Restrict egress to the overlay subnet only:
  lattice sandbox run --name my-agent --server-url http://latticed:8080 --token lt-xxx \
    --egress-allow 10.0.0.0/8 --egress-default-deny \
    -- python agent.py`,
		RunE: runRun,
		// Args after -- are the child command; Cobra passes them as args.
		Args: cobra.ArbitraryArgs,
	}

	// Shared identity flags (reuse package-level vars from sandbox_pro.go).
	cmd.Flags().StringVar(&sandboxName, "name", "", "Sandbox identifier (required)")
	cmd.Flags().StringVar(&sandboxServerURL, "server-url", "", "Lattice control plane URL (required)")
	cmd.Flags().StringVar(&sandboxToken, "token", "", "Enrollment token (required)")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("server-url")
	_ = cmd.MarkFlagRequired("token")

	// Network flags.
	cmd.Flags().StringVar(&sandboxRunProxyAddr, "proxy-addr", "127.0.0.1:0",
		"SOCKS5 proxy listen address; :0 picks a random port")
	cmd.Flags().StringVar(&sandboxEgressAllow, "egress-allow", "", "Comma-separated allowed egress CIDRs")
	cmd.Flags().BoolVar(&sandboxEgressDeny, "egress-default-deny", false, "Whitelist egress mode")
	cmd.Flags().StringArrayVar(&sandboxForwardRules, "forward", nil, "Inbound forward rule: overlayPort:targetAddr")

	// Lifecycle flags.
	cmd.Flags().DurationVar(&sandboxRunReadyTimeout, "ready-timeout", 10*time.Second,
		"Maximum time to wait for sandbox to be ready")

	return cmd
}

func runRun(_ *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing command: use -- <command> [args...], e.g.: lattice sandbox run ... -- python agent.py")
	}

	// Pre-allocate a local port for SOCKS5 so we know the address before the
	// driver starts (avoids needing a Addr() method on the SOCKS5 server).
	proxyAddr := sandboxRunProxyAddr
	if proxyAddr == "" || proxyAddr == "127.0.0.1:0" || proxyAddr == ":0" {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return fmt.Errorf("allocate proxy port: %w", err)
		}
		proxyAddr = ln.Addr().String()
		ln.Close()
	}

	readyCh := make(chan struct{}, 1)
	cfg := DriverConfig{
		SandboxName:  sandboxName,
		ServerURL:    sandboxServerURL,
		Token:        sandboxToken,
		ProxyAddr:    proxyAddr,
		EgressAllow:  sandboxEgressAllow,
		EgressDeny:   sandboxEgressDeny,
		ForwardRules: sandboxForwardRules,
		ReadyCh:      readyCh,
	}

	if err := validateDriverConfig("pod", cfg); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	driver := NewPodDriver(cfg)
	driverDone := make(chan error, 1)
	go func() {
		driverDone <- driver.Start(ctx)
	}()

	// Wait for sandbox to be ready.
	select {
	case <-readyCh:
		// sandbox is up, SOCKS5 is listening
	case <-time.After(sandboxRunReadyTimeout):
		cancel()
		<-driverDone
		return fmt.Errorf("sandbox not ready after %s", sandboxRunReadyTimeout)
	case err := <-driverDone:
		if err != nil {
			return fmt.Errorf("sandbox failed to start: %w", err)
		}
		return fmt.Errorf("sandbox exited before becoming ready")
	}

	// Build child environment with proxy injected.
	env := append(os.Environ(),
		"ALL_PROXY=socks5://"+proxyAddr,
		"all_proxy=socks5://"+proxyAddr,
		"LATTICE_SANDBOX_NAME="+sandboxName,
	)

	fmt.Printf("Sandbox %q ready. Proxy: socks5://%s\n", sandboxName, proxyAddr)
	fmt.Printf("Executing: %v\n", args)

	child := exec.CommandContext(ctx, args[0], args[1:]...)
	child.Env = env
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr

	if err := child.Start(); err != nil {
		cancel()
		<-driverDone
		return fmt.Errorf("start child process: %w", err)
	}

	childDone := make(chan error, 1)
	go func() {
		childDone <- child.Wait()
	}()

	var childErr error
	select {
	case childErr = <-childDone:
		// Child exited normally (or with error) — cancel sandbox.
		cancel()
		<-driverDone
	case driverErr := <-driverDone:
		// Sandbox died unexpectedly — terminate child.
		fmt.Fprintf(os.Stderr, "sandbox terminated unexpectedly: %v\n", driverErr)
		_ = child.Process.Signal(syscall.SIGTERM)
		select {
		case <-childDone:
		case <-time.After(5 * time.Second):
			_ = child.Process.Kill()
			<-childDone
		}
		return fmt.Errorf("sandbox terminated unexpectedly: %w", driverErr)
	}

	// Propagate child's exit code.
	var exitErr *exec.ExitError
	if errors.As(childErr, &exitErr) {
		os.Exit(exitErr.ExitCode())
	}
	return childErr
}
```

- [ ] **Step 2: Build PRO to verify it compiles**

```bash
cd /Users/francis/workspc/lattice
make EDITION=pro build SERVICE=lattice 2>&1 | tail -10
```

Expected: build succeeds with no errors.

- [ ] **Step 3: Build community to verify it still compiles**

```bash
make build SERVICE=lattice 2>&1 | tail -5
```

Expected: build succeeds.

- [ ] **Step 4: Run lint**

```bash
make lint 2>&1 | grep -A3 "sandbox_run"
```

Fix any issues reported.

- [ ] **Step 5: Commit**

```bash
git add cmd/lattice/cmd/sandbox/sandbox_run_pro.go
git commit -s -m "feat(sandbox): implement sandbox run command (PRO)"
```

---

### Task 6: Verify existing `NewDriver` test still passes

**Files:**
- Read: `cmd/lattice/cmd/sandbox/driver_runsc_test.go`

- [ ] **Step 1: Run existing driver tests**

```bash
cd /Users/francis/workspc/lattice
go test ./cmd/lattice/cmd/sandbox/... -v -tags pro 2>&1 | tail -20
```

Expected: `TestNewDriver_Pod`, `TestNewDriver_Gvisor`, `TestNewDriver_Unknown` all PASS.

If they fail due to missing build constraints (test file uses `sandbox.NewDriver` which is in a `pro && linux` file), note this — it's a pre-existing constraint and not caused by this change.

- [ ] **Step 2: Run unit tests for the whole lattice binary**

```bash
go test ./cmd/lattice/... 2>&1 | tail -10
```

Expected: PASS (community build).

---

### Task 7: Manual smoke test (if a Lattice server is available)

This task verifies end-to-end behavior. Skip if no server is available.

- [ ] **Step 1: Build the PRO binary**

```bash
make EDITION=pro build SERVICE=lattice
```

- [ ] **Step 2: Run sandbox with a simple echo command**

```bash
./bin/lattice sandbox run \
  --name test-sandbox \
  --server-url http://localhost:8080 \
  --token lt-xxx \
  -- env | grep -E "ALL_PROXY|LATTICE_SANDBOX"
```

Expected output includes:
```
ALL_PROXY=socks5://127.0.0.1:<port>
all_proxy=socks5://127.0.0.1:<port>
LATTICE_SANDBOX_NAME=test-sandbox
```

- [ ] **Step 3: Verify help text**

```bash
./bin/lattice sandbox --help
```

Expected: `run` appears in the command list under `sandbox`.

```bash
./bin/lattice sandbox run --help
```

Expected: shows flags `--name`, `--server-url`, `--token`, `--proxy-addr`, `--ready-timeout`, `--egress-allow`, `--egress-default-deny`.

- [ ] **Step 4: Commit cleanup if needed**

```bash
git add -A && git commit -s -m "fix(sandbox): smoke test fixes"
```

---

## Self-Review

**Spec coverage:**
- ✅ Single command: `sandbox run -- <cmd>` — Task 5
- ✅ Auto-allocate random SOCKS5 port — Task 5 (`net.Listen(":0")`)
- ✅ Inject `ALL_PROXY` + `all_proxy` + `LATTICE_SANDBOX_NAME` — Task 5
- ✅ AI agent lifecycle controls sandbox — Task 5 (childDone → cancel → driverDone)
- ✅ Sandbox crash → SIGTERM child → 5s → SIGKILL — Task 5
- ✅ Community edition: command not registered — Task 4
- ✅ Exit code propagation — Task 5 (`os.Exit(exitErr.ExitCode())`)
- ✅ `ReadyCh` mechanism — Tasks 1 + 2

**Placeholder scan:** None found.

**Type consistency:**
- `ReadyCh chan<- struct{}` defined in Task 1, signaled in Task 2, received in Task 5 — consistent.
- `NewPodDriver(cfg)` called in Task 5 — matches signature in `driver_pod.go`.
- `validateDriverConfig("pod", cfg)` called in Task 5 — matches signature in `sandbox_pro.go`.
- Package-level vars `sandboxName`, `sandboxServerURL`, `sandboxToken`, `sandboxEgressAllow`, `sandboxEgressDeny`, `sandboxForwardRules` — defined in `sandbox_pro.go` (same build tag), reused safely.
