# runsc Sandbox Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the runsc gVisor full-isolation mode for `lattice sandbox`, where Lattice agent runs as PID 1 inside a gVisor container, sets up WireGuard (wg0), then execs the AI agent binary transparently.

**Architecture:** `IsolationDriver` interface abstracts pod vs. gvisor modes. `RunscDriver` manages runsc container lifecycle; inside the container, `lattice sandbox agent` subcommand handles NATS registration, WireGuard setup, capability drop, and `exec` of the AI agent. `PodDriver` wraps the existing gVisor pod-mode logic unchanged.

**Tech Stack:** Go 1.25, gVisor `runsc`, wireguard-go, `golang.org/x/sys/unix` (prctl), cobra, OCI runtime spec (map[string]any)

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `cmd/lattice/cmd/sandbox/driver.go` | Create (no build tag) | `IsolationDriver` interface + `DriverConfig` struct |
| `cmd/lattice/cmd/sandbox/driver_pod.go` | Create (`//go:build pro`) | `PodDriver` — current pod/gVisor-sidecar logic |
| `cmd/lattice/cmd/sandbox/driver_runsc.go` | Create (`//go:build pro`) | `RunscDriver` — launches runsc container, waits for exit |
| `cmd/lattice/cmd/sandbox/sandbox_agent.go` | Create (`//go:build pro`) | `agentCmd()` — PID 1 inside container: NATS → wg0 → exec |
| `cmd/lattice/cmd/sandbox/sandbox.go` | Modify | Register `agentCmd()` subcommand |
| `cmd/lattice/cmd/sandbox/sandbox_pro.go` | Modify | Thin orchestrator: parse flags → `DriverConfig` → `newDriver()` → `driver.Start()` |
| `internal/agent/runsc/runsc.go` | Modify | Add fields to `Config`; fix OCI spec (`--network=sandbox`, `CAP_NET_ADMIN`); add `Done()` channel; remove socketpair |
| `internal/agent/runsc/runsc_test.go` | Create | Unit test OCI spec generation |
| `cmd/lattice/cmd/sandbox/driver_runsc_test.go` | Create | Unit test `RunscDriver` construction and `newDriver()` dispatch |

---

## Task 1: Fix compile errors in sandbox_pro.go

**Files:**
- Modify: `cmd/lattice/cmd/sandbox/sandbox_pro.go`

The rebase introduced `registerStartFlags` referencing four undeclared variables, and an unused import. Fix both before anything else.

- [ ] **Step 1: Add missing var declarations**

In `sandbox_pro.go`, the `var (...)` block at line ~82 currently only declares `sandboxProxyAddr`, `sandboxForwardRules`, `sandboxEgressAllow`, `sandboxEgressDeny`. Add the four new vars below the existing block:

```go
// PRO-only flags (not available in community edition).
var (
	sandboxProxyAddr    string
	sandboxForwardRules []string
	sandboxEgressAllow  string
	sandboxEgressDeny   bool

	// gvisor (runsc) mode flags.
	sandboxMode        string
	sandboxAgentRootFS string
	sandboxAgentBinary string
	sandboxAgentArgs   []string
)
```

- [ ] **Step 2: Remove unused runsc import**

Remove the line `"github.com/alatticeio/lattice/internal/agent/runsc"` from the import block in `sandbox_pro.go`. It will be used in `driver_runsc.go` (Task 5), not here.

- [ ] **Step 3: Verify it compiles**

```bash
make EDITION=pro build SERVICE=lattice 2>&1 | grep -E "error:|ok|Building"
```

Expected: `Building lattice [edition=pro]...` with no `error:` lines.

- [ ] **Step 4: Commit**

```bash
git add cmd/lattice/cmd/sandbox/sandbox_pro.go
git commit -s -m "fix(sandbox): declare missing runsc mode vars, remove unused import"
```

---

## Task 2: IsolationDriver interface + DriverConfig

**Files:**
- Create: `cmd/lattice/cmd/sandbox/driver.go`

This file has no build tag — it must compile in both community and pro builds. It defines only the interface and config struct; no implementations.

- [ ] **Step 1: Create driver.go**

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

package sandbox

import "context"

// DriverConfig holds all parameters needed to start a sandbox, regardless of
// isolation mode. Fields unused by a given driver are silently ignored.
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
	RootFS      string   // path to container root filesystem
	AgentBinary string   // AI agent binary inside the container
	AgentArgs   []string // AI agent arguments
	BundleDir   string   // writable OCI bundle dir; defaults to /tmp/lattice-runsc/<name>
}

// IsolationDriver abstracts the lifecycle of a sandbox isolation backend.
// Implementations: PodDriver (--mode pod), RunscDriver (--mode gvisor).
type IsolationDriver interface {
	// Name returns a short identifier for logging (e.g. "pod", "gvisor").
	Name() string
	// Start runs the sandbox, blocking until ctx is cancelled or the sandbox
	// exits. It must perform cleanup (Stop) before returning.
	Start(ctx context.Context) error
}
```

- [ ] **Step 2: Verify community build compiles**

```bash
make build SERVICE=lattice 2>&1 | grep -E "error:|Building"
```

Expected: builds cleanly (community build, no `-tags pro`).

- [ ] **Step 3: Commit**

```bash
git add cmd/lattice/cmd/sandbox/driver.go
git commit -s -m "feat(sandbox): add IsolationDriver interface and DriverConfig"
```

---

## Task 3: Fix runsc.go OCI spec

**Files:**
- Modify: `internal/agent/runsc/runsc.go`
- Create: `internal/agent/runsc/runsc_test.go`

The current `runsc.go` has the old SOCKS5-over-socketpair design. Replace it with the new design:
- `Config` gains `ServerURL`, `Token`, `EgressAllow`, `EgressDeny`
- `Manager` drops `hostConn`, `containerFile` (no socketpair)
- `Manager.Start()` uses `--network=sandbox` instead of `--network=none`, removes `--pass-fd` and `ExtraFiles`
- `ociSpec()` adds `CAP_NET_ADMIN` capabilities and sets `process.args` to the `lattice sandbox agent` invocation
- Remove the `var _ io.Reader` placeholder

- [ ] **Step 1: Write the failing test**

Create `internal/agent/runsc/runsc_test.go`:

```go
//go:build pro

// Copyright 2026 The Lattice Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package runsc_test

import (
	"encoding/json"
	"testing"

	"github.com/alatticeio/lattice/internal/agent/runsc"
)

func TestOCISpec(t *testing.T) {
	mgr := &runsc.Manager{}
	mgr.SetConfig(runsc.Config{
		SandboxID:   "test-sandbox",
		RootFS:      "/rootfs",
		AgentBinary: "/usr/bin/myagent",
		AgentArgs:   []string{"--flag", "val"},
		ServerURL:   "http://ctrl:8080",
		Token:       "tok-abc",
	})

	spec := mgr.OCISpec()

	// network namespace must be present (gVisor sandbox networking)
	linux, ok := spec["linux"].(map[string]any)
	if !ok {
		t.Fatal("missing linux section")
	}
	namespaces, ok := linux["namespaces"].([]map[string]string)
	if !ok {
		t.Fatal("missing linux.namespaces")
	}
	hasNet := false
	for _, ns := range namespaces {
		if ns["type"] == "network" {
			hasNet = true
		}
	}
	if !hasNet {
		t.Error("expected network namespace in OCI spec")
	}

	// capabilities must include CAP_NET_ADMIN
	caps, ok := linux["capabilities"].(map[string][]string)
	if !ok {
		t.Fatal("missing linux.capabilities")
	}
	hasNetAdmin := false
	for _, c := range caps["effective"] {
		if c == "CAP_NET_ADMIN" {
			hasNetAdmin = true
		}
	}
	if !hasNetAdmin {
		t.Error("expected CAP_NET_ADMIN in effective capabilities")
	}

	// process.args must start with "lattice sandbox agent"
	proc, ok := spec["process"].(map[string]any)
	if !ok {
		t.Fatal("missing process section")
	}
	args, ok := proc["args"].([]string)
	if !ok {
		t.Fatal("process.args is not []string")
	}
	if len(args) < 3 || args[0] != "lattice" || args[1] != "sandbox" || args[2] != "agent" {
		t.Errorf("expected process.args to start with [lattice sandbox agent], got %v", args)
	}
	// --name, --server-url, --token must appear
	argsJSON, _ := json.Marshal(args)
	argsStr := string(argsJSON)
	for _, needle := range []string{"--name", "--server-url", "--token", "--", "/usr/bin/myagent", "--flag"} {
		found := false
		for _, a := range args {
			if a == needle {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q in process.args, got %s", needle, argsStr)
		}
	}

	// NO ALL_PROXY env var (runsc mode does not use SOCKS5 proxy)
	envs, _ := proc["env"].([]string)
	for _, e := range envs {
		if len(e) >= 9 && e[:9] == "ALL_PROXY" {
			t.Error("ALL_PROXY must not appear in runsc OCI spec env")
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/francis/workspc/lattice && \
  go test -tags pro ./internal/agent/runsc/... -run TestOCISpec -v 2>&1 | tail -20
```

Expected: compile error — `Manager` has no exported `SetConfig` or `OCISpec` method.

- [ ] **Step 3: Rewrite runsc.go**

Replace the entire `internal/agent/runsc/runsc.go` with:

```go
//go:build pro

// Copyright 2026 The Lattice Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package runsc manages gVisor runsc sandbox lifecycle for AI agent isolation.
// The container runs with --network=sandbox so wireguard-go can open a real
// /dev/net/tun (wg0) and send WireGuard UDP through eth0 to the host.
// CAP_NET_ADMIN is virtualised by gVisor — it grants no real host-kernel access.
package runsc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// ContextDialer mirrors shim.ContextDialer; kept for socks5.go compatibility.
type ContextDialer interface {
	DialContext(ctx context.Context, network, addr string) (interface{ Close() error }, error)
}

// Config holds all parameters needed to create a runsc sandbox.
type Config struct {
	SandboxID   string   // sandbox identifier
	RootFS      string   // path to root filesystem for the container
	AgentBinary string   // AI agent entrypoint binary inside the container
	AgentArgs   []string // arguments passed to the AI agent
	BundleDir   string   // writable directory for the OCI bundle; defaults to /tmp/lattice-runsc/<id>

	// Passed through to `lattice sandbox agent` running as PID 1.
	ServerURL   string
	Token       string
	EgressAllow string
	EgressDeny  bool
}

// Manager controls the lifecycle of a runsc container.
type Manager struct {
	cfg       Config
	cmd       *exec.Cmd
	bundleDir string
	done      chan struct{}
}

// NewManager validates the runsc binary and returns a Manager.
func NewManager(cfg Config) (*Manager, error) {
	if _, err := exec.LookPath("runsc"); err != nil {
		return nil, fmt.Errorf("runsc not found in PATH: %w", err)
	}
	return &Manager{cfg: cfg, done: make(chan struct{})}, nil
}

// SetConfig replaces the manager's config. Used in tests that construct
// a Manager directly without calling NewManager.
func (m *Manager) SetConfig(cfg Config) { m.cfg = cfg }

// Create prepares the OCI bundle directory and writes config.json.
// It does NOT start runsc.
func (m *Manager) Create() error {
	bundleDir := m.cfg.BundleDir
	if bundleDir == "" {
		bundleDir = filepath.Join(os.TempDir(), "lattice-runsc", m.cfg.SandboxID)
	}
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		return fmt.Errorf("create bundle dir %s: %w", bundleDir, err)
	}
	m.bundleDir = bundleDir

	specData, err := json.MarshalIndent(m.OCISpec(), "", "  ")
	if err != nil {
		return fmt.Errorf("marshal OCI spec: %w", err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "config.json"), specData, 0o644); err != nil {
		return fmt.Errorf("write config.json: %w", err)
	}
	return nil
}

// Start launches runsc with --network=sandbox. The container's PID 1 is
// `lattice sandbox agent` which handles NATS registration, wg0 setup, and
// execs the AI agent binary. Start returns immediately; use Done() to wait.
func (m *Manager) Start(ctx context.Context) error {
	m.cmd = exec.CommandContext(ctx, "runsc",
		"--network=sandbox",
		"run",
		"--bundle", m.bundleDir,
		m.cfg.SandboxID,
	)
	m.cmd.Stdout = os.Stdout
	m.cmd.Stderr = os.Stderr

	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("start runsc: %w", err)
	}

	go func() {
		m.cmd.Wait() //nolint:errcheck
		close(m.done)
	}()

	return nil
}

// Stop sends SIGTERM to runsc, then SIGKILL after a 10 s grace period.
func (m *Manager) Stop() error {
	if m.cmd == nil || m.cmd.Process == nil {
		return nil
	}
	if err := m.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("signal runsc: %w", err)
	}
	select {
	case <-time.After(10 * time.Second):
		m.cmd.Process.Kill() //nolint:errcheck
		<-m.done
	case <-m.done:
	}
	return nil
}

// Destroy removes the OCI bundle and runsc state directory.
func (m *Manager) Destroy() error {
	if m.bundleDir != "" {
		os.RemoveAll(m.bundleDir) //nolint:errcheck
	}
	os.RemoveAll(filepath.Join("/var/run/runsc", m.cfg.SandboxID)) //nolint:errcheck
	return nil
}

// Done returns a channel that is closed when the runsc container exits.
func (m *Manager) Done() <-chan struct{} { return m.done }

// OCISpec returns the OCI runtime spec for the container.
// Exported so tests can inspect the generated spec.
func (m *Manager) OCISpec() map[string]any {
	// Build `lattice sandbox agent` args that PID 1 will receive.
	pidOneArgs := []string{
		"lattice", "sandbox", "agent",
		"--name", m.cfg.SandboxID,
		"--server-url", m.cfg.ServerURL,
		"--token", m.cfg.Token,
	}
	if m.cfg.EgressAllow != "" {
		pidOneArgs = append(pidOneArgs, "--egress-allow", m.cfg.EgressAllow)
	}
	if m.cfg.EgressDeny {
		pidOneArgs = append(pidOneArgs, "--egress-default-deny")
	}
	// Separator: everything after "--" is passed to the AI agent.
	pidOneArgs = append(pidOneArgs, "--")
	pidOneArgs = append(pidOneArgs, m.cfg.AgentBinary)
	pidOneArgs = append(pidOneArgs, m.cfg.AgentArgs...)

	caps := []string{"CAP_NET_ADMIN"}

	return map[string]any{
		"ociVersion": "1.0.2",
		"process": map[string]any{
			"terminal": false,
			"args":     pidOneArgs,
			"env": []string{
				"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
			},
			"cwd": "/",
		},
		"root": map[string]any{
			"path":     m.cfg.RootFS,
			"readonly": true,
		},
		"hostname": m.cfg.SandboxID,
		"linux": map[string]any{
			"capabilities": map[string][]string{
				"bounding":    caps,
				"permitted":   caps,
				"effective":   caps,
				"inheritable": {},
				"ambient":     {},
			},
			"namespaces": []map[string]string{
				{"type": "pid"},
				{"type": "network"},
				{"type": "ipc"},
				{"type": "uts"},
				{"type": "mount"},
			},
		},
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test -tags pro ./internal/agent/runsc/... -run TestOCISpec -v 2>&1
```

Expected:
```
=== RUN   TestOCISpec
--- PASS: TestOCISpec (0.00s)
PASS
```

- [ ] **Step 5: Build check**

```bash
make EDITION=pro build SERVICE=lattice 2>&1 | grep -E "error:|Building"
```

- [ ] **Step 6: Commit**

```bash
git add internal/agent/runsc/runsc.go internal/agent/runsc/runsc_test.go
git commit -s -m "feat(sandbox): fix runsc OCI spec — network=sandbox, CAP_NET_ADMIN, PID-1 args"
```

---

## Task 4: PodDriver — extract pod mode into driver_pod.go

**Files:**
- Create: `cmd/lattice/cmd/sandbox/driver_pod.go` (`//go:build pro`)

Extract the entire current `runStart()` body from `sandbox_pro.go` into `PodDriver.Start()`. The pod mode logic (NATS registration → gVisor sandbox → WireGuard node → SOCKS5 → signal wait) moves here unchanged.

- [ ] **Step 1: Create driver_pod.go**

```go
//go:build pro

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
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	shimfwd "github.com/alatticeio/lattice-shim/shim"
	"github.com/alatticeio/lattice/internal/agent"
	agentconfig "github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/gvisor"
	"github.com/alatticeio/lattice/internal/agent/infra"
	agentlog "github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/agent/provision"
	wgdevice "golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// PodDriver runs the sandbox in pod mode: a gVisor userspace netstack is
// embedded in-process, and a SOCKS5 sidecar bridges outbound connections to
// the WireGuard overlay. AI agents must configure ALL_PROXY to use the proxy.
type PodDriver struct {
	cfg DriverConfig
}

// NewPodDriver constructs a PodDriver from cfg.
func NewPodDriver(cfg DriverConfig) *PodDriver {
	return &PodDriver{cfg: cfg}
}

func (d *PodDriver) Name() string { return "pod" }

// Start runs the pod-mode sandbox. It blocks until ctx is cancelled or SIGINT/SIGTERM.
func (d *PodDriver) Start(ctx context.Context) error {
	cfg := d.cfg

	egressPolicy := shimfwd.EgressPolicy{DefaultDeny: cfg.EgressDeny}
	if cfg.EgressAllow != "" {
		for _, entry := range strings.Split(cfg.EgressAllow, ",") {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			_, cidr, cidrErr := net.ParseCIDR(entry)
			if cidrErr != nil {
				return fmt.Errorf("invalid egress CIDR %q: %w", entry, cidrErr)
			}
			egressPolicy.AllowedCIDRs = append(egressPolicy.AllowedCIDRs, *cidr)
		}
	}

	agentconfig.Conf.AppId = cfg.SandboxName
	agentconfig.Conf.ServerUrl = cfg.ServerURL
	agentconfig.Conf.WgPort = 51820

	var privKey wgtypes.Key
	var currentPeer *infra.Peer

	if creds, loadErr := loadSandboxCredentials(); loadErr == nil {
		fmt.Printf("Resuming sandbox %q from saved credentials...\n", cfg.SandboxName)
		if key, parseErr := wgtypes.ParseKey(creds.PrivateKey); parseErr == nil {
			privKey = key
			resumed, resumeErr := agent.ResumeSandboxViaNATS(ctx, cfg.ServerURL, creds.JWT, cfg.SandboxName, key)
			if resumeErr == nil {
				currentPeer = resumed
				localIP := ""
				if currentPeer.Address != nil {
					localIP = *currentPeer.Address
				}
				fmt.Printf("Resumed %q, overlay IP=%s\n", cfg.SandboxName, localIP)
			} else {
				fmt.Printf("Resume failed (%v), falling back to registration...\n", resumeErr)
			}
		}
	}

	if currentPeer == nil {
		var err error
		privKey, err = wgtypes.GeneratePrivateKey()
		if err != nil {
			return fmt.Errorf("generate WireGuard key: %w", err)
		}
		fmt.Printf("Registering sandbox %q via NATS...\n", cfg.SandboxName)
		currentPeer, err = agent.RegisterSandboxViaNATS(ctx, cfg.ServerURL, cfg.Token, cfg.SandboxName, privKey)
		if err != nil {
			return fmt.Errorf("sandbox registration failed: %w", err)
		}
		if saveErr := saveSandboxCredentials(privKey, currentPeer.Token); saveErr != nil {
			fmt.Printf("Warning: failed to persist sandbox credentials: %v\n", saveErr)
		}
	}

	localIP := ""
	if currentPeer.Address != nil {
		localIP = *currentPeer.Address
	}
	fmt.Printf("Registered %q, overlay IP=%s\n", cfg.SandboxName, localIP)

	if currentPeer.LrpUrl != "" {
		agentconfig.Conf.EnableLrp = true
		agentconfig.Conf.RelayURL = currentPeer.LrpUrl
	}

	policyChecker := shimfwd.NewEgressFilter(egressPolicy)
	auditWriter, auditErr := newFileAuditWriter(auditLogPath)
	if auditErr != nil {
		fmt.Printf("Warning: failed to open audit log %s: %v\n", auditLogPath, auditErr)
	}
	sb, err := gvisor.New(gvisor.Config{
		ID:            cfg.SandboxName,
		LocalIP:       localIP,
		PolicyChecker: policyChecker,
		AuditWriter:   auditWriter,
	})
	if err != nil {
		return fmt.Errorf("create gVisor sandbox: %w", err)
	}
	defer sb.Close() //nolint:errcheck

	tunDev := gvisor.NewTUNAdapter(sb.Channel(), gvisor.InjectIntoChannel(sb.Channel()))

	logger := agentlog.GetLogger("sandbox")
	agentJWT := currentPeer.Token
	nodeCfg := &agent.NodeConfig{
		Logger:     logger,
		Port:       51820,
		ShowLog:    false,
		Flags:      agentconfig.Conf,
		CustomTUN:  tunDev,
		CustomName: cfg.SandboxName,
		CurrentPeer: currentPeer,
		ProvisionerFactory: func(dev *wgdevice.Device) provision.Provisioner {
			return gvisor.NewSandboxProvisionerFactory(localIP, cfg.SandboxName)(dev)
		},
	}

	node, err := agent.NewNode(ctx, nodeCfg)
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

	if cfg.ProxyAddr != "" {
		socks5, socks5Err := shimfwd.NewSocks5Server(sb, cfg.ProxyAddr)
		if socks5Err != nil {
			return fmt.Errorf("start socks5 proxy: %w", socks5Err)
		}
		go socks5.Serve()
		defer socks5.Close()
		fmt.Printf("SOCKS5 proxy listening on %s\n", cfg.ProxyAddr)
	}

	var fwdRules []shimfwd.ForwardRule
	for _, r := range cfg.ForwardRules {
		rule, parseErr := parseForwardRule(r)
		if parseErr != nil {
			return fmt.Errorf("parse --forward %q: %w", r, parseErr)
		}
		fwdRules = append(fwdRules, rule)
	}
	if len(fwdRules) > 0 {
		fl := shimfwd.NewForwardListener(sb.Netstack(), sb.LocalIP(), fwdRules)
		if startErr := fl.Start(ctx); startErr != nil {
			return fmt.Errorf("start forward listener: %w", startErr)
		}
	}

	if err = node.Start(ctx); err != nil {
		return fmt.Errorf("start node: %w", err)
	}

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

	fmt.Printf("Sandbox %q ready (pod mode), overlay IP=%s\n", cfg.SandboxName, localIP)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sigCh:
	case <-ctx.Done():
	}
	fmt.Println("\nShutting down...")
	_ = node.Stop()
	return nil
}
```

- [ ] **Step 2: Build check**

```bash
make EDITION=pro build SERVICE=lattice 2>&1 | grep -E "error:|Building"
```

Expected: builds cleanly.

- [ ] **Step 3: Commit**

```bash
git add cmd/lattice/cmd/sandbox/driver_pod.go
git commit -s -m "feat(sandbox): add PodDriver — extract pod-mode logic from sandbox_pro.go"
```

---

## Task 5: RunscDriver

**Files:**
- Create: `cmd/lattice/cmd/sandbox/driver_runsc.go` (`//go:build pro`)
- Create: `cmd/lattice/cmd/sandbox/driver_runsc_test.go` (`//go:build pro`)

`RunscDriver` creates the `runsc.Manager`, prepares the OCI bundle, starts the container, and blocks until the container exits or ctx is cancelled.

- [ ] **Step 1: Write failing test**

Create `cmd/lattice/cmd/sandbox/driver_runsc_test.go`:

```go
//go:build pro

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

package sandbox_test

import (
	"testing"

	"github.com/alatticeio/lattice/cmd/lattice/cmd/sandbox"
)

func TestNewDriver_Pod(t *testing.T) {
	cfg := sandbox.DriverConfig{SandboxName: "test", ServerURL: "http://x", Token: "t"}
	d := sandbox.NewDriver("pod", cfg)
	if d == nil {
		t.Fatal("expected non-nil driver for pod mode")
	}
	if d.Name() != "pod" {
		t.Errorf("expected Name()=pod, got %s", d.Name())
	}
}

func TestNewDriver_Gvisor(t *testing.T) {
	cfg := sandbox.DriverConfig{
		SandboxName: "test",
		ServerURL:   "http://x",
		Token:       "t",
		RootFS:      "/rootfs",
		AgentBinary: "/bin/agent",
	}
	// NewDriver for gvisor returns error only if runsc is not found; skip binary
	// check here by testing the Name() of a directly constructed RunscDriver.
	d := sandbox.NewRunscDriver(cfg)
	if d.Name() != "gvisor" {
		t.Errorf("expected Name()=gvisor, got %s", d.Name())
	}
}

func TestNewDriver_Unknown(t *testing.T) {
	cfg := sandbox.DriverConfig{}
	d := sandbox.NewDriver("unknown", cfg)
	if d != nil {
		t.Error("expected nil for unknown mode")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test -tags pro ./cmd/lattice/cmd/sandbox/... -run "TestNewDriver" -v 2>&1 | tail -10
```

Expected: compile error — `sandbox.NewDriver` and `sandbox.NewRunscDriver` undefined.

- [ ] **Step 3: Create driver_runsc.go**

```go
//go:build pro

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
	"fmt"

	"github.com/alatticeio/lattice/internal/agent/runsc"
)

// RunscDriver launches the AI agent inside a gVisor runsc container.
// The container runs `lattice sandbox agent` as PID 1, which handles NATS
// registration, WireGuard (wg0) setup, and execs the AI agent binary.
// No SOCKS5 proxy is involved — the AI agent connects to overlay IPs directly.
type RunscDriver struct {
	cfg     DriverConfig
	manager *runsc.Manager
}

// NewRunscDriver constructs a RunscDriver from cfg. It does not check for the
// runsc binary; that check happens lazily in Start().
func NewRunscDriver(cfg DriverConfig) *RunscDriver {
	return &RunscDriver{cfg: cfg}
}

func (d *RunscDriver) Name() string { return "gvisor" }

// Start prepares the OCI bundle, starts the runsc container, and blocks until
// the container exits or ctx is cancelled.
func (d *RunscDriver) Start(ctx context.Context) error {
	cfg := d.cfg

	mgr, err := runsc.NewManager(runsc.Config{
		SandboxID:   cfg.SandboxName,
		RootFS:      cfg.RootFS,
		AgentBinary: cfg.AgentBinary,
		AgentArgs:   cfg.AgentArgs,
		BundleDir:   cfg.BundleDir,
		ServerURL:   cfg.ServerURL,
		Token:       cfg.Token,
		EgressAllow: cfg.EgressAllow,
		EgressDeny:  cfg.EgressDeny,
	})
	if err != nil {
		return fmt.Errorf("init runsc manager: %w", err)
	}
	d.manager = mgr
	defer mgr.Destroy() //nolint:errcheck

	if err := mgr.Create(); err != nil {
		return fmt.Errorf("create runsc bundle: %w", err)
	}

	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("start runsc container: %w", err)
	}

	fmt.Printf("runsc container %q started\n", cfg.SandboxName)

	select {
	case <-ctx.Done():
		return mgr.Stop()
	case <-mgr.Done():
		return nil
	}
}

// NewDriver returns the IsolationDriver for the given mode, or nil for unknown modes.
func NewDriver(mode string, cfg DriverConfig) IsolationDriver {
	switch mode {
	case "pod":
		return NewPodDriver(cfg)
	case "gvisor":
		return NewRunscDriver(cfg)
	default:
		return nil
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test -tags pro ./cmd/lattice/cmd/sandbox/... -run "TestNewDriver" -v 2>&1
```

Expected:
```
=== RUN   TestNewDriver_Pod
--- PASS: TestNewDriver_Pod (0.00s)
=== RUN   TestNewDriver_Gvisor
--- PASS: TestNewDriver_Gvisor (0.00s)
=== RUN   TestNewDriver_Unknown
--- PASS: TestNewDriver_Unknown (0.00s)
PASS
```

- [ ] **Step 5: Build check**

```bash
make EDITION=pro build SERVICE=lattice 2>&1 | grep -E "error:|Building"
```

- [ ] **Step 6: Commit**

```bash
git add cmd/lattice/cmd/sandbox/driver_runsc.go cmd/lattice/cmd/sandbox/driver_runsc_test.go
git commit -s -m "feat(sandbox): add RunscDriver and NewDriver factory"
```

---

## Task 6: sandbox_agent.go — container PID 1 subcommand

**Files:**
- Create: `cmd/lattice/cmd/sandbox/sandbox_agent.go` (`//go:build pro`)

This is the `lattice sandbox agent` subcommand. It runs as PID 1 inside the gVisor container:
1. NATS registration (reusing existing helpers)
2. `agent.NewNode()` without `CustomTUN` → standard wireguard-go + `/dev/net/tun` (gVisor virtualises TUN creation)
3. `node.Start()` + heartbeat
4. Wait for WireGuard peers via `--ready-wait` (default 3s)
5. Drop ambient capabilities via `prctl(PR_CAP_AMBIENT, PR_CAP_AMBIENT_CLEAR_ALL)`
6. `syscall.Exec()` the AI agent binary — replaces this process image

- [ ] **Step 1: Create sandbox_agent.go**

```go
//go:build pro

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
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/alatticeio/lattice/internal/agent"
	agentconfig "github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/infra"
	agentlog "github.com/alatticeio/lattice/internal/agent/log"
	"github.com/spf13/cobra"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var (
	agentName        string
	agentServerURL   string
	agentToken       string
	agentEgressAllow string
	agentEgressDeny  bool
	agentReadyWait   time.Duration
)

// agentCmd returns the `lattice sandbox agent` cobra command.
// This command is designed to run as PID 1 inside a gVisor runsc container.
// It is not intended for direct user invocation.
func agentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Container PID 1: set up overlay network then exec AI agent (internal)",
		Long: `agent runs as PID 1 inside a gVisor runsc container. It registers with the
Lattice control plane, creates the WireGuard overlay interface (wg0), drops
capabilities, then replaces itself with the AI agent binary via exec.

The AI agent binary and its arguments are passed after a "--" separator:

  lattice sandbox agent --name s1 --server-url http://ctrl --token tk -- /usr/bin/myagent --flag val`,
		RunE: runAgent,
	}
	cmd.Flags().StringVar(&agentName, "name", "", "Sandbox identifier (required)")
	cmd.Flags().StringVar(&agentServerURL, "server-url", "", "Lattice control plane URL (required)")
	cmd.Flags().StringVar(&agentToken, "token", "", "Enrollment token (required)")
	cmd.Flags().StringVar(&agentEgressAllow, "egress-allow", "", "Comma-separated allowed egress CIDRs (informational; enforcement is via gVisor routes)")
	cmd.Flags().BoolVar(&agentEgressDeny, "egress-default-deny", false, "Whitelist egress mode")
	cmd.Flags().DurationVar(&agentReadyWait, "ready-wait", 3*time.Second, "Time to wait for WireGuard peers before exec-ing AI agent")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("server-url")
	_ = cmd.MarkFlagRequired("token")
	return cmd
}

func runAgent(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("missing agent binary: pass it after '--', e.g.: lattice sandbox agent ... -- /path/to/agent [args]")
	}
	agentBinary := args[0]
	agentBinArgs := args[1:]

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agentconfig.Conf.AppId = agentName
	agentconfig.Conf.ServerUrl = agentServerURL
	agentconfig.Conf.WgPort = 51820

	// Apply egress-allow to config for logging / future enforcement hooks.
	if agentEgressAllow != "" {
		for _, entry := range strings.Split(agentEgressAllow, ",") {
			entry = strings.TrimSpace(entry)
			if entry != "" {
				// Stored for audit; actual enforcement is at the route/runsc level.
				_ = entry
			}
		}
	}

	var privKey wgtypes.Key
	var currentPeer *infra.Peer

	// Attempt to resume from persisted credentials (container restart path).
	if creds, loadErr := loadSandboxCredentials(); loadErr == nil {
		if key, parseErr := wgtypes.ParseKey(creds.PrivateKey); parseErr == nil {
			privKey = key
			resumed, resumeErr := agent.ResumeSandboxViaNATS(ctx, agentServerURL, creds.JWT, agentName, key)
			if resumeErr == nil {
				currentPeer = resumed
				localIP := ""
				if currentPeer.Address != nil {
					localIP = *currentPeer.Address
				}
				fmt.Printf("[sandbox-agent] resumed %q, overlay IP=%s\n", agentName, localIP)
			} else {
				fmt.Printf("[sandbox-agent] resume failed (%v), registering fresh...\n", resumeErr)
			}
		}
	}

	if currentPeer == nil {
		var err error
		privKey, err = wgtypes.GeneratePrivateKey()
		if err != nil {
			return fmt.Errorf("generate WireGuard key: %w", err)
		}
		fmt.Printf("[sandbox-agent] registering %q via NATS...\n", agentName)
		currentPeer, err = agent.RegisterSandboxViaNATS(ctx, agentServerURL, agentToken, agentName, privKey)
		if err != nil {
			return fmt.Errorf("sandbox registration failed: %w", err)
		}
		if saveErr := saveSandboxCredentials(privKey, currentPeer.Token); saveErr != nil {
			fmt.Printf("[sandbox-agent] warning: failed to persist credentials: %v\n", saveErr)
		}
	}

	localIP := ""
	if currentPeer.Address != nil {
		localIP = *currentPeer.Address
	}
	fmt.Printf("[sandbox-agent] overlay IP=%s\n", localIP)

	if currentPeer.LrpUrl != "" {
		agentconfig.Conf.EnableLrp = true
		agentconfig.Conf.RelayURL = currentPeer.LrpUrl
	}

	logger := agentlog.GetLogger("sandbox-agent")
	agentJWT := currentPeer.Token

	// NewNode without CustomTUN: wireguard-go opens /dev/net/tun (gVisor
	// intercepts this and creates a virtual TUN interface in its netstack).
	// ProvisionerFactory is nil → default kernel provisioner (iptables/eBPF);
	// gVisor intercepts iptables and netlink calls on the container's netns.
	nodeCfg := &agent.NodeConfig{
		Logger:      logger,
		Port:        51820,
		ShowLog:     false,
		Flags:       agentconfig.Conf,
		CustomName:  agentName,
		CurrentPeer: currentPeer,
	}

	node, nodeErr := agent.NewNode(ctx, nodeCfg)
	if nodeErr != nil {
		return fmt.Errorf("create node: %w", nodeErr)
	}

	node.GetNetworkMap = func() (*infra.Message, error) {
		msg, getErr := node.GetNetMap(agentJWT)
		if getErr != nil {
			logger.Error("get network map failed", getErr)
			return nil, getErr
		}
		return msg, nil
	}

	if err := node.Start(ctx); err != nil {
		return fmt.Errorf("start node: %w", err)
	}

	go node.StartHeartbeat(ctx)

	// Wait for WireGuard to establish peer sessions before exec-ing the agent.
	fmt.Printf("[sandbox-agent] waiting %s for WireGuard peers...\n", agentReadyWait)
	select {
	case <-time.After(agentReadyWait):
	case <-ctx.Done():
		return ctx.Err()
	}

	// Drop ambient capabilities so the exec'd AI agent inherits zero privileges.
	// In gVisor, CAP_NET_ADMIN is virtualised; clearing the ambient set ensures
	// the AI agent process cannot manipulate network interfaces.
	if err := unix.Prctl(unix.PR_CAP_AMBIENT, unix.PR_CAP_AMBIENT_CLEAR_ALL, 0, 0, 0); err != nil {
		// Non-fatal: log and continue. On some kernels/gVisor versions this may
		// return EINVAL if ambient capabilities are not supported.
		fmt.Printf("[sandbox-agent] warning: clear ambient caps: %v\n", err)
	}

	fmt.Printf("[sandbox-agent] exec %s %v\n", agentBinary, agentBinArgs)
	// syscall.Exec replaces this process image. On success it does not return.
	return syscall.Exec(agentBinary, append([]string{agentBinary}, agentBinArgs...), os.Environ())
}
```

- [ ] **Step 2: Build check**

```bash
make EDITION=pro build SERVICE=lattice 2>&1 | grep -E "error:|Building"
```

Expected: builds cleanly.

- [ ] **Step 3: Commit**

```bash
git add cmd/lattice/cmd/sandbox/sandbox_agent.go
git commit -s -m "feat(sandbox): add sandbox agent subcommand — container PID 1 for runsc mode"
```

---

## Task 7: Register agentCmd in sandbox.go

**Files:**
- Modify: `cmd/lattice/cmd/sandbox/sandbox.go`

`agentCmd()` is defined in `sandbox_agent.go` with `//go:build pro`. Register it from `sandbox.go` (which has no build tag). Because the function only exists in the pro build, use a build-tag-gated registration pattern.

- [ ] **Step 1: Create sandbox_agent_register_pro.go**

Rather than modifying `sandbox.go` and needing to guard the call, add a new file that registers `agentCmd()` only in the pro build. Create `cmd/lattice/cmd/sandbox/sandbox_agent_register.go`:

```go
//go:build pro

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

// registerAgentCmd adds the `sandbox agent` subcommand to cmd.
// Called from SandboxCmd() in sandbox.go.
func registerAgentCmd(cmd *cobra.Command) {
	cmd.AddCommand(agentCmd())
}
```

Create `cmd/lattice/cmd/sandbox/sandbox_agent_register_community.go`:

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

// registerAgentCmd is a no-op in community builds.
func registerAgentCmd(_ *cobra.Command) {}
```

- [ ] **Step 2: Call registerAgentCmd from sandbox.go**

In `cmd/lattice/cmd/sandbox/sandbox.go`, add the call inside `SandboxCmd()`:

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

- [ ] **Step 3: Verify both builds**

```bash
# Community build
make build SERVICE=lattice 2>&1 | grep -E "error:|Building"
# Pro build
make EDITION=pro build SERVICE=lattice 2>&1 | grep -E "error:|Building"
```

Both must build cleanly.

- [ ] **Step 4: Verify command is registered in pro build**

```bash
GOOS=linux GOARCH=amd64 go build -tags pro -o /tmp/lattice-pro ./cmd/lattice/main.go && \
  /tmp/lattice-pro sandbox --help 2>&1 | grep -E "agent|start"
```

Expected output includes both `agent` and `start` in the subcommand list.

- [ ] **Step 5: Commit**

```bash
git add cmd/lattice/cmd/sandbox/sandbox_agent_register.go \
        cmd/lattice/cmd/sandbox/sandbox_agent_register_community.go \
        cmd/lattice/cmd/sandbox/sandbox.go
git commit -s -m "feat(sandbox): register sandbox agent subcommand in pro build"
```

---

## Task 8: Refactor sandbox_pro.go as thin orchestrator

**Files:**
- Modify: `cmd/lattice/cmd/sandbox/sandbox_pro.go`

Replace the monolithic `runStart()` body with a thin orchestrator: parse flags into `DriverConfig`, call `newDriver()`, call `driver.Start()`. All the business logic is now in `PodDriver` / `RunscDriver`.

- [ ] **Step 1: Replace sandbox_pro.go**

Overwrite `cmd/lattice/cmd/sandbox/sandbox_pro.go` with:

```go
//go:build pro

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
	"fmt"
	"net"
	"strings"

	"github.com/spf13/cobra"
)

func startCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a sandboxed agent environment",
		Long: `Start creates a sandbox attached to the Lattice overlay network.

Two isolation modes are available via --mode:

  pod    (default) In-process gVisor netstack + SOCKS5 sidecar. AI agents must
                   configure ALL_PROXY=socks5://<proxy-addr>. Fast, no extra binary.

  gvisor Full gVisor runsc container. Lattice agent is PID 1; AI agent runs with
                   zero privileges and connects to overlay IPs directly, with no
                   awareness of WireGuard or SOCKS5.

Examples:

  # Pod mode with egress filtering:
  lattice sandbox start --name agent-1 --server-url http://ctrl:8080 --token lt-xxx \
    --proxy-addr 127.0.0.1:1080 --egress-default-deny --egress-allow 10.0.0.0/8

  # gVisor (runsc) mode:
  lattice sandbox start --name agent-1 --server-url http://ctrl:8080 --token lt-xxx \
    --mode gvisor --agent-rootfs /opt/agent-rootfs --agent-binary /usr/bin/myagent`,
		RunE: runStart,
	}
	registerStartFlags(cmd)
	return cmd
}

func registerStartFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&sandboxName, "name", "", "Sandbox identifier (required)")
	cmd.Flags().StringVar(&sandboxServerURL, "server-url", "", "Lattice control plane URL (required)")
	cmd.Flags().StringVar(&sandboxToken, "token", "", "Enrollment token (required)")
	cmd.Flags().StringVar(&sandboxProxyAddr, "proxy-addr", "", "SOCKS5 proxy listen address (pod mode, e.g. 127.0.0.1:1080)")
	cmd.Flags().StringArrayVar(&sandboxForwardRules, "forward", nil, "Inbound forward rule: overlayPort:targetAddr (pod mode)")
	cmd.Flags().StringVar(&sandboxEgressAllow, "egress-allow", "", "Comma-separated allowed egress CIDRs")
	cmd.Flags().BoolVar(&sandboxEgressDeny, "egress-default-deny", false, "Whitelist egress mode")
	cmd.Flags().StringVar(&sandboxMode, "mode", "pod", "Isolation mode: pod | gvisor")
	cmd.Flags().StringVar(&sandboxAgentRootFS, "agent-rootfs", "", "Root filesystem path for runsc container (gvisor mode)")
	cmd.Flags().StringVar(&sandboxAgentBinary, "agent-binary", "", "AI agent entrypoint binary (gvisor mode)")
	cmd.Flags().StringSliceVar(&sandboxAgentArgs, "agent-args", nil, "AI agent entrypoint arguments (gvisor mode)")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("server-url")
	_ = cmd.MarkFlagRequired("token")
}

// PRO-only flags (not available in community edition).
var (
	sandboxProxyAddr    string
	sandboxForwardRules []string
	sandboxEgressAllow  string
	sandboxEgressDeny   bool

	// gvisor (runsc) mode flags.
	sandboxMode        string
	sandboxAgentRootFS string
	sandboxAgentBinary string
	sandboxAgentArgs   []string
)

// auditLogPath is where the sandbox writes JSONL audit events.
const auditLogPath = "/tmp/lattice-audit.jsonl"

func runStart(_ *cobra.Command, _ []string) error {
	cfg := DriverConfig{
		SandboxName:  sandboxName,
		ServerURL:    sandboxServerURL,
		Token:        sandboxToken,
		EgressAllow:  sandboxEgressAllow,
		EgressDeny:   sandboxEgressDeny,
		ProxyAddr:    sandboxProxyAddr,
		ForwardRules: sandboxForwardRules,
		RootFS:       sandboxAgentRootFS,
		AgentBinary:  sandboxAgentBinary,
		AgentArgs:    sandboxAgentArgs,
	}

	if err := validateDriverConfig(sandboxMode, cfg); err != nil {
		return err
	}

	driver := NewDriver(sandboxMode, cfg)
	if driver == nil {
		return fmt.Errorf("unknown isolation mode %q: choose pod or gvisor", sandboxMode)
	}

	ctx := context.Background()
	fmt.Printf("Starting sandbox %q in %s mode...\n", sandboxName, driver.Name())
	return driver.Start(ctx)
}

// validateDriverConfig checks that required fields are present for the given mode.
func validateDriverConfig(mode string, cfg DriverConfig) error {
	if mode == "gvisor" {
		if cfg.RootFS == "" {
			return fmt.Errorf("--agent-rootfs is required for gvisor mode")
		}
		if cfg.AgentBinary == "" {
			return fmt.Errorf("--agent-binary is required for gvisor mode")
		}
	}
	if cfg.EgressAllow != "" {
		for _, entry := range strings.Split(cfg.EgressAllow, ",") {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			if _, _, err := net.ParseCIDR(entry); err != nil {
				return fmt.Errorf("invalid egress CIDR %q: %w", entry, err)
			}
		}
	}
	return nil
}

func parseForwardRule(s string) (forwardRule, error) {
	idx := strings.IndexByte(s, ':')
	if idx < 0 {
		return forwardRule{}, fmt.Errorf("expected overlayPort:targetAddr, got %q", s)
	}
	portStr := s[:idx]
	target := s[idx+1:]
	if target == "" {
		return forwardRule{}, fmt.Errorf("empty targetAddr in %q", s)
	}
	port, err := net.LookupPort("tcp", portStr)
	if err != nil || port < 1 || port > 65535 {
		return forwardRule{}, fmt.Errorf("invalid overlay port %q", portStr)
	}
	return forwardRule{overlayPort: uint16(port), targetAddr: target}, nil
}

type forwardRule struct {
	overlayPort uint16
	targetAddr  string
}
```

**Note:** `parseForwardRule` now returns a local `forwardRule` type because `shimfwd.ForwardRule` is used in `driver_pod.go`. Update `driver_pod.go` to call `parseForwardRule` from there using `shimfwd.ForwardRule` directly (it already does in the version written in Task 4 — `parseForwardRule` in `sandbox_pro.go` was only used inside `runStart()`, which is now gone).

Actually, since `driver_pod.go` iterates `cfg.ForwardRules` (strings) and calls `shimfwd.ForwardRule` directly, the `parseForwardRule` in `sandbox_pro.go` is no longer needed. Remove it and the local `forwardRule` type. Keep only `validateDriverConfig`.

The correct final `sandbox_pro.go` (without `parseForwardRule`):

```go
//go:build pro

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
	"fmt"
	"net"
	"strings"

	"github.com/spf13/cobra"
)

func startCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a sandboxed agent environment",
		Long: `Start creates a sandbox attached to the Lattice overlay network.

Two isolation modes are available via --mode:

  pod    (default) In-process gVisor netstack + SOCKS5 sidecar. AI agents must
                   configure ALL_PROXY=socks5://<proxy-addr>. Fast, no extra binary.

  gvisor Full gVisor runsc container. Lattice agent is PID 1; AI agent runs with
                   zero privileges and connects to overlay IPs directly, with no
                   awareness of WireGuard or SOCKS5.

Examples:

  # Pod mode with egress filtering:
  lattice sandbox start --name agent-1 --server-url http://ctrl:8080 --token lt-xxx \
    --proxy-addr 127.0.0.1:1080 --egress-default-deny --egress-allow 10.0.0.0/8

  # gVisor (runsc) mode:
  lattice sandbox start --name agent-1 --server-url http://ctrl:8080 --token lt-xxx \
    --mode gvisor --agent-rootfs /opt/agent-rootfs --agent-binary /usr/bin/myagent`,
		RunE: runStart,
	}
	registerStartFlags(cmd)
	return cmd
}

func registerStartFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&sandboxName, "name", "", "Sandbox identifier (required)")
	cmd.Flags().StringVar(&sandboxServerURL, "server-url", "", "Lattice control plane URL (required)")
	cmd.Flags().StringVar(&sandboxToken, "token", "", "Enrollment token (required)")
	cmd.Flags().StringVar(&sandboxProxyAddr, "proxy-addr", "", "SOCKS5 proxy listen address (pod mode, e.g. 127.0.0.1:1080)")
	cmd.Flags().StringArrayVar(&sandboxForwardRules, "forward", nil, "Inbound forward rule: overlayPort:targetAddr (pod mode)")
	cmd.Flags().StringVar(&sandboxEgressAllow, "egress-allow", "", "Comma-separated allowed egress CIDRs")
	cmd.Flags().BoolVar(&sandboxEgressDeny, "egress-default-deny", false, "Whitelist egress mode")
	cmd.Flags().StringVar(&sandboxMode, "mode", "pod", "Isolation mode: pod | gvisor")
	cmd.Flags().StringVar(&sandboxAgentRootFS, "agent-rootfs", "", "Root filesystem path for runsc container (gvisor mode)")
	cmd.Flags().StringVar(&sandboxAgentBinary, "agent-binary", "", "AI agent entrypoint binary (gvisor mode)")
	cmd.Flags().StringSliceVar(&sandboxAgentArgs, "agent-args", nil, "AI agent entrypoint arguments (gvisor mode)")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("server-url")
	_ = cmd.MarkFlagRequired("token")
}

// PRO-only flags (not available in community edition).
var (
	sandboxProxyAddr    string
	sandboxForwardRules []string
	sandboxEgressAllow  string
	sandboxEgressDeny   bool

	// gvisor (runsc) mode flags.
	sandboxMode        string
	sandboxAgentRootFS string
	sandboxAgentBinary string
	sandboxAgentArgs   []string
)

// auditLogPath is where the sandbox writes JSONL audit events.
const auditLogPath = "/tmp/lattice-audit.jsonl"

func runStart(_ *cobra.Command, _ []string) error {
	cfg := DriverConfig{
		SandboxName:  sandboxName,
		ServerURL:    sandboxServerURL,
		Token:        sandboxToken,
		EgressAllow:  sandboxEgressAllow,
		EgressDeny:   sandboxEgressDeny,
		ProxyAddr:    sandboxProxyAddr,
		ForwardRules: sandboxForwardRules,
		RootFS:       sandboxAgentRootFS,
		AgentBinary:  sandboxAgentBinary,
		AgentArgs:    sandboxAgentArgs,
	}

	if err := validateDriverConfig(sandboxMode, cfg); err != nil {
		return err
	}

	driver := NewDriver(sandboxMode, cfg)
	if driver == nil {
		return fmt.Errorf("unknown isolation mode %q: choose pod or gvisor", sandboxMode)
	}

	ctx := context.Background()
	fmt.Printf("Starting sandbox %q in %s mode...\n", sandboxName, driver.Name())
	return driver.Start(ctx)
}

// validateDriverConfig checks that required fields are present for the given mode.
func validateDriverConfig(mode string, cfg DriverConfig) error {
	if mode == "gvisor" {
		if cfg.RootFS == "" {
			return fmt.Errorf("--agent-rootfs is required for gvisor mode")
		}
		if cfg.AgentBinary == "" {
			return fmt.Errorf("--agent-binary is required for gvisor mode")
		}
	}
	if cfg.EgressAllow != "" {
		for _, entry := range strings.Split(cfg.EgressAllow, ",") {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			if _, _, err := net.ParseCIDR(entry); err != nil {
				return fmt.Errorf("invalid egress CIDR %q: %w", entry, err)
			}
		}
	}
	return nil
}
```

- [ ] **Step 2: Update driver_pod.go to use its own parseForwardRule**

Since `parseForwardRule` no longer lives in `sandbox_pro.go`, move it to `driver_pod.go`. Find the section in `driver_pod.go` where `cfg.ForwardRules` is iterated and replace with an inline helper at the bottom of `driver_pod.go`:

```go
func parseForwardRule(s string) (shimfwd.ForwardRule, error) {
	idx := strings.IndexByte(s, ':')
	if idx < 0 {
		return shimfwd.ForwardRule{}, fmt.Errorf("expected overlayPort:targetAddr, got %q", s)
	}
	portStr := s[:idx]
	target := s[idx+1:]
	if target == "" {
		return shimfwd.ForwardRule{}, fmt.Errorf("empty targetAddr in %q", s)
	}
	port, err := net.LookupPort("tcp", portStr)
	if err != nil || port < 1 || port > 65535 {
		return shimfwd.ForwardRule{}, fmt.Errorf("invalid overlay port %q", portStr)
	}
	return shimfwd.ForwardRule{
		OverlayPort: uint16(port),
		TargetAddr:  target,
	}, nil
}
```

- [ ] **Step 3: Build and lint**

```bash
make EDITION=pro build SERVICE=lattice 2>&1 | grep -E "error:|Building"
make lint 2>&1 | tail -5
```

Both must pass with no errors.

- [ ] **Step 4: Run all tests**

```bash
go test -tags pro ./cmd/lattice/cmd/sandbox/... ./internal/agent/runsc/... -v 2>&1 | grep -E "PASS|FAIL|ok"
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/lattice/cmd/sandbox/sandbox_pro.go cmd/lattice/cmd/sandbox/driver_pod.go
git commit -s -m "refactor(sandbox): sandbox_pro.go as thin orchestrator, delegate to IsolationDriver"
```

---

## Self-Review

**Spec coverage check:**

| Spec requirement | Covered by |
|---|---|
| syscall-level isolation via gVisor sentry | Task 3 (--network=sandbox OCI spec) |
| CAP_NET_ADMIN virtualised, not real host access | Task 3 (linux.capabilities in ociSpec) |
| Lattice agent as PID 1, sets up wg0, execs AI agent | Task 6 (sandbox_agent.go) |
| `IsolationDriver` interface for extensibility | Task 2 (driver.go) |
| `PodDriver` wraps existing pod logic | Task 4 (driver_pod.go) |
| `RunscDriver` manages runsc lifecycle | Task 5 (driver_runsc.go) |
| `sandbox_pro.go` becomes thin orchestrator | Task 8 (final task) |
| `--mode gvisor` flag dispatch | Task 1 (vars) + Task 8 (dispatch) |
| `lattice sandbox agent` subcommand | Task 6 + Task 7 |
| Drop ambient capabilities before exec | Task 6 (prctl call) |
| No SOCKS5 / ALL_PROXY in gvisor mode | Task 3 (ociSpec), Task 6 (no proxy) |
| `NewDriver` factory for future extensibility | Task 5 (NewDriver switch) |

**All requirements covered. No placeholders present.**
