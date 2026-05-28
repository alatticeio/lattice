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
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	shim "github.com/alatticeio/lattice-shim/shim"
	latticeagent "github.com/alatticeio/lattice/internal/agent"
	agentconfig "github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/gvisor"
	"github.com/alatticeio/lattice/internal/agent/infra"
	agentlog "github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/agent/provision"
	"github.com/alatticeio/lattice/internal/agent/tproxy"
	"github.com/spf13/cobra"
	wgdevice "golang.zx2c4.com/wireguard/device"
)

var (
	sidecarServerURL   string
	sidecarToken       string
	sidecarProxyPort   int
	sidecarEgressAllow string
	sidecarEgressDeny  bool
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

Pro accounts get egress policy control (--egress-allow / --egress-default-deny)
and full audit logging to /tmp/lattice-audit.jsonl.

Run as a Kubernetes sidecar container alongside the AI agent container:
  securityContext:
    runAsUser: 1337  # must match --skip-uid in agent init

Example:
  lattice sandbox sidecar my-agent \
    --server-url http://latticed:8080 --token lt-xxx \
    [--egress-allow 10.0.0.0/8] [--egress-default-deny]`,
		Args: cobra.ExactArgs(1),
		RunE: runSidecar,
	}
	cmd.Flags().StringVar(&sidecarServerURL, "server-url", "", "Lattice control plane URL (required)")
	cmd.Flags().StringVar(&sidecarToken, "token", "", "Enrollment token (required)")
	cmd.Flags().IntVar(&sidecarProxyPort, "proxy-port", 15001,
		"TCP port to listen on for transparent proxy connections")
	cmd.Flags().StringVar(&sidecarEgressAllow, "egress-allow", "",
		"Comma-separated overlay CIDRs the AI agent is allowed to reach (Pro)")
	cmd.Flags().BoolVar(&sidecarEgressDeny, "egress-default-deny", false,
		"Deny all egress except --egress-allow CIDRs (Pro)")
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

	currentPeer, err := registerOrResume(ctx, agentName, sidecarServerURL, sidecarToken)
	if err != nil {
		return err
	}

	// Build policy and audit based on account tier.
	var policyChecker shim.PolicyChecker
	var auditWriter shim.AuditWriter

	if currentPeer.Tier == "pro" {
		egressPolicy := shim.EgressPolicy{DefaultDeny: sidecarEgressDeny}
		if sidecarEgressAllow != "" {
			for _, entry := range strings.Split(sidecarEgressAllow, ",") {
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
		policyChecker = shim.NewEgressFilter(egressPolicy)
		auditWriter, _ = newFileAuditWriter(auditLogPath)
	} else if sidecarEgressDeny || sidecarEgressAllow != "" {
		fmt.Fprintln(os.Stderr, "[agent-sidecar] WARNING: egress policy flags require a Pro account. These flags will be ignored.")
	}

	localIP := overlayAddr(currentPeer)
	fmt.Printf("[agent-sidecar] %q registered, overlay IP=%s\n", agentName, localIP)

	if currentPeer.LrpUrl != "" {
		agentconfig.Conf.EnableLrp = true
		agentconfig.Conf.RelayURL = currentPeer.LrpUrl
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
	go runPeriodicRefresh(ctx, node, logger)

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
