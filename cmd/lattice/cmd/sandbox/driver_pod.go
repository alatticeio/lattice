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

// Start runs the pod-mode sandbox. It blocks until SIGINT/SIGTERM or ctx is cancelled.
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
		Logger:      logger,
		Port:        51820,
		ShowLog:     false,
		Flags:       agentconfig.Conf,
		CustomTUN:   tunDev,
		CustomName:  cfg.SandboxName,
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
