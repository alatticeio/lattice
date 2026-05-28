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
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	shimfwd "github.com/alatticeio/lattice-shim/shim"
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

const auditLogPath = "/tmp/lattice-audit.jsonl"

var (
	sidecarServerURL   string
	sidecarToken       string
	sidecarProxyPort   int
	sidecarEgressAllow string
	sidecarEgressDeny  bool
)

// fileAuditWriter implements shimfwd.AuditWriter by appending JSON lines to a file.
type fileAuditWriter struct {
	mu sync.Mutex
	f  *os.File
}

func newFileAuditWriter(path string) (*fileAuditWriter, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	return &fileAuditWriter{f: f}, nil
}

func (w *fileAuditWriter) Write(event shimfwd.AuditEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	_, err = fmt.Fprintf(w.f, "%s\n", data)
	return err
}

func addSidecarCmd(parent *cobra.Command) {
	parent.AddCommand(sidecarCmd())
}

func sidecarCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sidecar <name>",
		Short: "Run a Lattice sidecar with transparent proxy for K8s pods (Pro)",
		Long: `Sidecar registers with the Lattice control plane, starts an embedded gVisor
netstack for WireGuard (no kernel wg0), enforces egress policy, and listens
as a transparent TCP proxy.

Example:
  lattice agent sidecar my-agent \
    --server-url http://latticed:8080 --token lt-xxx \
    --egress-allow 10.0.0.0/8 --egress-default-deny`,
		Args: cobra.ExactArgs(1),
		RunE: runSidecar,
	}
	cmd.Flags().StringVar(&sidecarServerURL, "server-url", "", "Lattice control plane URL (required)")
	cmd.Flags().StringVar(&sidecarToken, "token", "", "Enrollment token (required)")
	cmd.Flags().IntVar(&sidecarProxyPort, "proxy-port", 15001,
		"TCP port to listen on for transparent proxy connections")
	cmd.Flags().StringVar(&sidecarEgressAllow, "egress-allow", "",
		"Comma-separated overlay CIDRs the AI agent is allowed to reach")
	cmd.Flags().BoolVar(&sidecarEgressDeny, "egress-default-deny", false,
		"Deny all egress except --egress-allow CIDRs")
	_ = cmd.MarkFlagRequired("server-url")
	_ = cmd.MarkFlagRequired("token")
	return cmd
}

func runSidecar(_ *cobra.Command, args []string) error {
	agentName := args[0]

	egressPolicy := shimfwd.EgressPolicy{DefaultDeny: sidecarEgressDeny}
	if sidecarEgressAllow != "" {
		for _, entry := range strings.Split(sidecarEgressAllow, ",") {
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
	agentconfig.Conf.ServerUrl = sidecarServerURL
	agentconfig.Conf.WgPort = 0

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
	_ = privKey

	localIP := overlayAddr(currentPeer)
	fmt.Printf("[agent-sidecar] %q registered, overlay IP=%s\n", agentName, localIP)

	if currentPeer.LrpUrl != "" {
		agentconfig.Conf.EnableLrp = true
		agentconfig.Conf.RelayURL = currentPeer.LrpUrl
	}

	policyChecker := shimfwd.NewEgressFilter(egressPolicy)
	auditWriter, auditErr := newFileAuditWriter(auditLogPath)
	if auditErr != nil {
		fmt.Printf("[agent-sidecar] warning: open audit log: %v\n", auditErr)
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

	proxy := &tproxy.Proxy{
		Addr: fmt.Sprintf("0.0.0.0:%d", sidecarProxyPort),
		Dial: sb.DialContext,
	}
	if err := proxy.Start(ctx); err != nil {
		return fmt.Errorf("start transparent proxy: %w", err)
	}
	fmt.Printf("[agent-sidecar] transparent proxy listening on :%d (egress-deny=%v)\n",
		sidecarProxyPort, sidecarEgressDeny)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sigCh:
	case <-ctx.Done():
	}
	fmt.Println("[agent-sidecar] shutting down")
	return nil
}
