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
