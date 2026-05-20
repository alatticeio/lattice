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
//
// Architecture (two-phase):
//   Phase 1 (pod kernel): NATS registration + WireGuard (wg0) creation on the
//     real host kernel via /dev/net/tun. Routes and iptables rules are applied
//     directly on the pod's network namespace.
//   Phase 2 (gVisor): runsc runs the AI agent binary as PID 1 with
//     --network=host, inheriting the pod's network stack. The AI agent's traffic
//     is routed through the pod → wg0 → overlay. gVisor's sentry still
//     intercepts all syscalls for security isolation.
//
// No virtual TUN, no CAP_NET_ADMIN, no static eth0 config — networking is
// handled entirely outside gVisor.
package runsc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// Config holds all parameters needed to create a runsc sandbox.
type Config struct {
	SandboxID   string   // sandbox identifier
	RootFS      string   // path to root filesystem for the container
	AgentBinary string   // AI agent entrypoint binary inside the container
	AgentArgs   []string // arguments passed to the AI agent
	BundleDir   string   // writable directory for the OCI bundle; defaults to /tmp/lattice-runsc/<id>
}

// Manager controls the lifecycle of a runsc container.
type Manager struct {
	cfg       Config
	cmd       *exec.Cmd
	bundleDir string
	done      chan struct{}
	stopOnce  sync.Once
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

// Start launches runsc in host network mode so the sandbox shares the pod's
// network namespace and can reach K8s services directly.
// Start returns immediately; use Done() to wait.
func (m *Manager) Start(ctx context.Context) error {
	m.cmd = exec.CommandContext(ctx, "runsc",
		"--network=host",
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
	var stopErr error
	m.stopOnce.Do(func() {
		if m.cmd == nil || m.cmd.Process == nil {
			return
		}
		if err := m.cmd.Process.Signal(syscall.SIGTERM); err != nil {
			stopErr = fmt.Errorf("signal runsc: %w", err)
			return
		}
		select {
		case <-time.After(10 * time.Second):
			m.cmd.Process.Kill() //nolint:errcheck
			<-m.done
		case <-m.done:
		}
	})
	return stopErr
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
// PID 1 runs the AI agent binary directly. Phase 1 (NATS registration,
// WireGuard wg0) already completed on the pod kernel before runsc starts.
// The container uses --network=host to inherit the pod network namespace,
// so no virtual TUN, CAP_NET_ADMIN, or static eth0 config is needed.
// Exported so tests can inspect the generated spec.
func (m *Manager) OCISpec() map[string]any {
	pidOneArgs := append([]string{m.cfg.AgentBinary}, m.cfg.AgentArgs...)

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
		"mounts": []map[string]any{
			{
				"destination": "/etc/resolv.conf",
				"source":      "/etc/resolv.conf",
				"type":        "bind",
				"options":     []string{"ro", "rbind"},
			},
		},
		"linux": map[string]any{
			"namespaces": []map[string]string{
				{"type": "pid"},
				{"type": "ipc"},
				{"type": "uts"},
				{"type": "mount"},
			},
		},
	}
}
