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
using the same ICE/LRP infrastructure as a regular agent. Policy is enforced by gVisor.

Examples:

  # Start with auto-registration to a Lattice control plane:
  lattice sandbox start --name agent-1 --server-url http://localhost:8080 --token lt-xxx

  # Expose a local service on the overlay:
  lattice sandbox start --name agent-1 --server-url http://localhost:8080 --token lt-xxx \
    --forward 8080:127.0.0.1:8080`,
		RunE: runStart,
	}
	registerStartFlags(cmd)
	return cmd
}

func registerStartFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&sandboxName, "name", "", "Sandbox identifier (required)")
	cmd.Flags().StringVar(&sandboxServerURL, "server-url", "", "Lattice control plane URL (required)")
	cmd.Flags().StringVar(&sandboxToken, "token", "", "Enrollment token (required)")
	cmd.Flags().StringVar(&sandboxProxyAddr, "proxy-addr", "", "SOCKS5 proxy listen address (e.g. 127.0.0.1:1080)")
	cmd.Flags().StringArrayVar(&sandboxForwardRules, "forward", nil, "Inbound forward rule: overlayPort:targetAddr")
	cmd.Flags().StringVar(&sandboxEgressAllow, "egress-allow", "", "Comma-separated allowed egress CIDRs")
	cmd.Flags().BoolVar(&sandboxEgressDeny, "egress-default-deny", false, "Whitelist egress mode")
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
)

// auditLogPath is where the sandbox writes JSONL audit events.
const auditLogPath = "/tmp/lattice-audit.jsonl"

func runStart(_ *cobra.Command, _ []string) error {
	// Build egress policy from flags.
	egressPolicy := shimfwd.EgressPolicy{DefaultDeny: sandboxEgressDeny}
	if sandboxEgressAllow != "" {
		for _, entry := range strings.Split(sandboxEgressAllow, ",") {
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
			privKey = key
			resumed, resumeErr := agent.ResumeSandboxViaNATS(ctx, sandboxServerURL, creds.JWT, sandboxName, key)
			if resumeErr == nil {
				currentPeer = resumed
				fmt.Printf("Resumed %q, overlay IP=%s\n", sandboxName, func() string {
					if currentPeer.Address != nil {
						return *currentPeer.Address
					}
					return ""
				}())
			} else {
				fmt.Printf("Resume failed (%v), falling back to registration...\n", resumeErr)
			}
		}
	}

	if currentPeer == nil {
		// Fresh registration via NATS enrollment token.
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

		// Persist credentials so container restarts can resume without re-registration.
		if saveErr := saveSandboxCredentials(privKey, currentPeer.Token); saveErr != nil {
			fmt.Printf("Warning: failed to persist sandbox credentials: %v\n", saveErr)
		}
	}
	localIP := ""
	if currentPeer.Address != nil {
		localIP = *currentPeer.Address
	}
	fmt.Printf("Registered %q, overlay IP=%s\n", sandboxName, localIP)

	// Enable LRP relay fallback if the control plane assigned a relay URL.
	// Sandbox has no kernel TUN / iptables, but the DefaultBind and QUIC/TCP
	// relay clients work identically to a regular agent.
	if currentPeer.LrpUrl != "" {
		agentconfig.Conf.EnableLrp = true
		agentconfig.Conf.RelayURL = currentPeer.LrpUrl
	}

	// Create the gVisor sandbox and TUN adapter now that we know the VPN IP.
	policyChecker := shimfwd.NewEgressFilter(egressPolicy)
	auditWriter, auditErr := newFileAuditWriter(auditLogPath)
	if auditErr != nil {
		fmt.Printf("Warning: failed to open audit log %s: %v\n", auditLogPath, auditErr)
	}
	sb, err := gvisor.New(gvisor.Config{
		ID:            sandboxName,
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
		CustomName: sandboxName,
		CurrentPeer: currentPeer, // skip NATS register; already done above
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

	if sandboxProxyAddr != "" {
		socks5, err := shimfwd.NewSocks5Server(sb, sandboxProxyAddr)
		if err != nil {
			return fmt.Errorf("start socks5 proxy: %w", err)
		}
		go socks5.Serve()
		defer socks5.Close()
		fmt.Printf("SOCKS5 proxy listening on %s\n", sandboxProxyAddr)
	}

	var fwdRules []shimfwd.ForwardRule
	for _, r := range sandboxForwardRules {
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

	// Heartbeat: keeps the server aware of this sandbox's online status so it
	// remains in other peers' ComputedPeers and receives config pushes.
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
