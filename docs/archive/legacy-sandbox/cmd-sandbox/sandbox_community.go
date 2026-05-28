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

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alatticeio/lattice/internal/agent"
	agentconfig "github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/gvisor"
	"github.com/alatticeio/lattice/internal/agent/infra"
	agentlog "github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/agent/provision"
	"github.com/spf13/cobra"
	wgdevice "golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func startCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a sandboxed agent environment",
		Long: `Start creates a gVisor-based network sandbox attached to the Lattice overlay network.
It registers with the control plane via NATS, receives a VPN IP, and connects to peers
using the same ICE/LRP infrastructure as a regular agent.

Examples:

  # Start with auto-registration to a Lattice control plane:
  lattice sandbox start --name agent-1 --server-url http://localhost:8080 --token lt-xxx`,
		RunE: runStart,
	}
	registerStartFlags(cmd)
	return cmd
}

func registerStartFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&sandboxName, "name", "", "Sandbox identifier (required)")
	cmd.Flags().StringVar(&sandboxServerURL, "server-url", "", "Lattice control plane URL (required)")
	cmd.Flags().StringVar(&sandboxToken, "token", "", "Enrollment token (required)")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("server-url")
	_ = cmd.MarkFlagRequired("token")
}

// communityAuditLogPath writes JSONL audit events per-sandbox to /tmp.
func communityAuditLogPath(name string) string {
	return "/tmp/lattice-audit-" + name + ".jsonl"
}

func runStart(_ *cobra.Command, _ []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agentconfig.Conf.AppId = sandboxName
	agentconfig.Conf.ServerUrl = sandboxServerURL
	agentconfig.Conf.WgPort = 51820

	var privKey wgtypes.Key
	var currentPeer *infra.Peer

	// On container restart, reuse persisted credentials instead of consuming the
	// one-time enrollment token again.
	if creds, loadErr := loadSandboxCredentials(); loadErr == nil {
		fmt.Printf("Resuming sandbox %q from saved credentials...\n", sandboxName)
		if key, parseErr := wgtypes.ParseKey(creds.PrivateKey); parseErr == nil {
			resumed, resumeErr := agent.ResumeSandboxViaNATS(ctx, sandboxServerURL, creds.JWT, sandboxName, key)
			if resumeErr == nil {
				currentPeer = resumed
				fmt.Printf("Resumed %q, overlay IP=%s\n", sandboxName, overlayAddr(currentPeer))
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

		fmt.Printf("Registering sandbox %q via NATS...\n", sandboxName)
		currentPeer, err = agent.RegisterSandboxViaNATS(ctx, sandboxServerURL, sandboxToken, sandboxName, privKey)
		if err != nil {
			return fmt.Errorf("sandbox registration failed: %w", err)
		}

		if saveErr := saveSandboxCredentials(privKey, currentPeer.Token); saveErr != nil {
			fmt.Printf("Warning: failed to persist sandbox credentials: %v\n", saveErr)
		}
	}

	localIP := overlayAddr(currentPeer)
	fmt.Printf("Registered %q, overlay IP=%s\n", sandboxName, localIP)

	// Enable LRP relay fallback if the control plane assigned a relay URL.
	if currentPeer.LrpUrl != "" {
		agentconfig.Conf.EnableLrp = true
		agentconfig.Conf.RelayURL = currentPeer.LrpUrl
	}

	// Local-file audit writer: community edition records flow events to /tmp.
	auditWriter, auditErr := newFileAuditWriter(communityAuditLogPath(sandboxName))
	if auditErr != nil {
		fmt.Printf("Warning: failed to open audit log: %v\n", auditErr)
	}

	// Community: PolicyChecker is nil → all egress is allowed.
	sb, err := gvisor.New(gvisor.Config{
		ID:          sandboxName,
		LocalIP:     localIP,
		AuditWriter: auditWriter,
	})
	if err != nil {
		return fmt.Errorf("create gVisor sandbox: %w", err)
	}
	defer sb.Close() //nolint:errcheck

	tunDev := gvisor.NewTUNAdapter(sb.Channel(), gvisor.InjectIntoChannel(sb.Channel()))

	logger := agentlog.GetLogger("sandbox")
	agentJWT := currentPeer.Token

	nodeCfg := &agent.NodeConfig{
		Logger:      logger,
		Port:        51820,
		ShowLog:     false,
		Flags:       agentconfig.Conf,
		CustomTUN:   tunDev,
		CustomName:  sandboxName,
		CurrentPeer: currentPeer,
		ProvisionerFactory: func(dev *wgdevice.Device) provision.Provisioner {
			return gvisor.NewSandboxProvisionerFactory(localIP, sandboxName)(dev)
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

	if err = node.Start(ctx); err != nil {
		return fmt.Errorf("start node: %w", err)
	}

	// Heartbeat: keeps the server aware of this sandbox's online status.
	go node.StartHeartbeat(ctx)

	// Periodic refresh as NATS push fallback (15s).
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

	fmt.Printf("Sandbox %q ready, overlay IP=%s\n", sandboxName, localIP)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	cancel()
	fmt.Println("\nShutting down...")
	_ = node.Stop()
	return nil
}

func overlayAddr(p *infra.Peer) string {
	if p != nil && p.Address != nil {
		return *p.Address
	}
	return ""
}
