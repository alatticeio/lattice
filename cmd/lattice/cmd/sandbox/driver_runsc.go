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
		SandboxID:      cfg.SandboxName,
		RootFS:         cfg.RootFS,
		AgentBinary:    cfg.AgentBinary,
		AgentArgs:      cfg.AgentArgs,
		BundleDir:      cfg.BundleDir,
		ServerURL:      cfg.ServerURL,
		Token:          cfg.Token,
		EgressAllow:    cfg.EgressAllow,
		EgressDeny:     cfg.EgressDeny,
		SandboxIP:      cfg.SandboxIP,
		SandboxGateway: cfg.SandboxGateway,
		SandboxCIDR:    cfg.SandboxCIDR,
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
