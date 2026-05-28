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
	"fmt"
	"os"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/alatticeio/lattice/internal/agent"
	agentconfig "github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/infra"
	agentlog "github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/agent/runsc"
)

// RunscDriver launches the AI agent inside a gVisor runsc container.
//
// Architecture (two-phase):
//
//	Phase 1 (pod kernel): NATS registration + WireGuard (wg0) is created on
//	  the real host kernel. This gives the AI agent access to the overlay
//	  network via the pod's native network stack.
//	Phase 2 (gVisor): runsc runs the AI agent binary as PID 1 with
//	  --network=host, inheriting the pod's pre-configured network namespace.
//	  gVisor's sentry intercepts all syscalls for security isolation.
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

// bootstrapAgent performs Phase 1: registers with the control plane via NATS
// and creates a WireGuard (wg0) interface on the pod's real kernel. It returns
// a running *agent.Node that must be stopped when the sandbox exits.
func (d *RunscDriver) bootstrapAgent(ctx context.Context) (*agent.Node, error) {
	cfg := d.cfg

	agentconfig.Conf.AppId = cfg.SandboxName
	agentconfig.Conf.ServerUrl = cfg.ServerURL
	agentconfig.Conf.WgPort = 51820

	// Ensure /dev/net/ exists so wireguard-go can create /dev/net/tun.
	_ = os.MkdirAll("/dev/net", 0o755)

	var privKey wgtypes.Key
	var currentPeer *infra.Peer

	// Attempt to resume from persisted credentials (container restart path).
	if creds, loadErr := loadSandboxCredentials(); loadErr == nil {
		if key, parseErr := wgtypes.ParseKey(creds.PrivateKey); parseErr == nil {
			privKey = key
			resumed, resumeErr := agent.ResumeSandboxViaNATS(ctx, cfg.ServerURL, creds.JWT, cfg.SandboxName, key)
			if resumeErr == nil {
				currentPeer = resumed
				localIP := ""
				if currentPeer.Address != nil {
					localIP = *currentPeer.Address
				}
				fmt.Printf("[sandbox-bootstrap] resumed %q, overlay IP=%s\n", cfg.SandboxName, localIP)
			} else {
				fmt.Printf("[sandbox-bootstrap] resume failed (%v), registering fresh...\n", resumeErr)
			}
		}
	}

	if currentPeer == nil {
		var err error
		privKey, err = wgtypes.GeneratePrivateKey()
		if err != nil {
			return nil, fmt.Errorf("generate WireGuard key: %w", err)
		}
		fmt.Printf("[sandbox-bootstrap] registering %q via NATS...\n", cfg.SandboxName)
		currentPeer, err = agent.RegisterSandboxViaNATS(ctx, cfg.ServerURL, cfg.Token, cfg.SandboxName, privKey)
		if err != nil {
			return nil, fmt.Errorf("sandbox registration failed: %w", err)
		}
		if saveErr := saveSandboxCredentials(privKey, currentPeer.Token); saveErr != nil {
			fmt.Printf("[sandbox-bootstrap] warning: failed to persist credentials: %v\n", saveErr)
		}
	}

	localIP := ""
	if currentPeer.Address != nil {
		localIP = *currentPeer.Address
	}
	fmt.Printf("[sandbox-bootstrap] overlay IP=%s\n", localIP)

	if currentPeer.LrpUrl != "" {
		agentconfig.Conf.EnableLrp = true
		agentconfig.Conf.RelayURL = currentPeer.LrpUrl
	}

	logger := agentlog.GetLogger("sandbox-bootstrap")

	// Create node on the real kernel TUN — no CustomTUN, no ProvisionerFactory.
	// This creates a real wg0 interface that the gVisor container will route
	// through because it shares the pod network namespace (--network=host).
	nodeCfg := &agent.NodeConfig{
		Logger:      logger,
		Port:        51820,
		ShowLog:     false,
		Flags:       agentconfig.Conf,
		CustomName:  cfg.SandboxName,
		CurrentPeer: currentPeer,
	}

	node, err := agent.NewNode(ctx, nodeCfg)
	if err != nil {
		return nil, fmt.Errorf("create node: %w", err)
	}

	node.GetNetworkMap = func() (*infra.Message, error) {
		msg, getErr := node.GetNetMap(currentPeer.Token)
		if getErr != nil {
			logger.Error("get network map failed", getErr)
			return nil, getErr
		}
		return msg, nil
	}

	if err := node.Start(ctx); err != nil {
		return nil, fmt.Errorf("start node: %w", err)
	}

	go node.StartHeartbeat(ctx)

	return node, nil
}

// Start runs the two-phase sandbox lifecycle:
//  1. Bootstrap the Lattice overlay on the pod kernel (NATS + wg0).
//  2. Launch the AI agent inside a gVisor runsc container with --network=host.
//
// Start blocks until the runsc container exits or ctx is cancelled.
func (d *RunscDriver) Start(ctx context.Context) error {
	cfg := d.cfg

	// Phase 1: register with control plane and create wg0 on pod kernel.
	node, err := d.bootstrapAgent(ctx)
	if err != nil {
		return fmt.Errorf("bootstrap agent: %w", err)
	}
	defer node.Stop() //nolint:errcheck

	// Phase 2: run AI agent inside gVisor with host networking.
	mgr, err := runsc.NewManager(runsc.Config{
		SandboxID:   cfg.SandboxName,
		RootFS:      cfg.RootFS,
		AgentBinary: cfg.AgentBinary,
		AgentArgs:   cfg.AgentArgs,
		BundleDir:   cfg.BundleDir,
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
