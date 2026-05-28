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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	shim "github.com/alatticeio/lattice-shim/shim"
	latticeagent "github.com/alatticeio/lattice/internal/agent"
	agentconfig "github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/gvisor"
	"github.com/alatticeio/lattice/internal/agent/infra"
	agentlog "github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/agent/provision"
	"github.com/alatticeio/lattice/internal/agent/tproxy"
	wgdevice "golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const sandboxAgentUID = 999

// registerOrResume tries to resume from persisted credentials; falls back to
// fresh NATS registration.
func registerOrResume(ctx context.Context, agentName, serverURL, token string) (*infra.Peer, error) {
	if creds, err := loadSandboxCredentials(); err == nil {
		if key, parseErr := wgtypes.ParseKey(creds.PrivateKey); parseErr == nil {
			if peer, resumeErr := latticeagent.ResumeSandboxViaNATS(ctx, serverURL, creds.JWT, agentName, key); resumeErr == nil {
				fmt.Printf("[sandbox-run] resumed %q from saved credentials\n", agentName)
				return peer, nil
			}
		}
	}

	privKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("generate WireGuard key: %w", err)
	}
	peer, err := latticeagent.RegisterSandboxViaNATS(ctx, serverURL, token, agentName, privKey)
	if err != nil {
		return nil, fmt.Errorf("registration failed: %w", err)
	}
	if saveErr := saveSandboxCredentials(privKey, peer.Token); saveErr != nil {
		fmt.Printf("[sandbox-run] warning: persist credentials: %v\n", saveErr)
	}
	return peer, nil
}

// runPeriodicRefresh polls the network map every 15 s as a NATS push fallback.
func runPeriodicRefresh(ctx context.Context, node *latticeagent.Node, logger interface {
	Warn(msg string, args ...any)
}) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := node.RefreshConfig(ctx); err != nil {
				logger.Warn("periodic config refresh failed", "err", err)
			}
		}
	}
}

// installRunIPTables sets up iptables REDIRECT rules.
// UID 0 (root, the sandbox-run parent) is exempt. The AI agent runs as
// sandboxAgentUID (999) → its TCP gets redirected → tproxy → netstack → WireGuard.
func installRunIPTables(proxyPort int) error {
	// Tear down any leftover chain from a previous run.
	exec.Command("iptables", "-t", "nat", "-D", "OUTPUT", "-p", "tcp", "-j", "LATTICE_REDIRECT").Run() //nolint:errcheck
	exec.Command("iptables", "-t", "nat", "-F", "LATTICE_REDIRECT").Run()                              //nolint:errcheck
	exec.Command("iptables", "-t", "nat", "-X", "LATTICE_REDIRECT").Run()                              //nolint:errcheck

	for _, args := range buildIPTablesRules(proxyPort, 0) {
		if out, err := exec.Command("iptables", args...).CombinedOutput(); err != nil {
			return fmt.Errorf("iptables %v: %s: %w", args, out, err)
		}
	}
	return nil
}

// forkAndWait forks the AI agent as sandboxAgentUID (999) so iptables
// redirects its TCP connections. Parent (UID 0) is exempt.
func forkAndWait(ctx context.Context, cancel context.CancelFunc, cmdArgs []string) error {
	child := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	child.Env = os.Environ()
	child.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: sandboxAgentUID,
			Gid: sandboxAgentUID,
		},
	}

	if err := child.Start(); err != nil {
		return fmt.Errorf("start agent process: %w", err)
	}

	childDone := make(chan error, 1)
	go func() { childDone <- child.Wait() }()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	var childErr error
	select {
	case childErr = <-childDone:
		cancel()
	case <-sigCh:
		_ = child.Process.Signal(syscall.SIGTERM)
		select {
		case childErr = <-childDone:
		case <-time.After(5 * time.Second):
			_ = child.Process.Kill()
			childErr = <-childDone
		}
		cancel()
	}

	var exitErr *exec.ExitError
	if errors.As(childErr, &exitErr) {
		os.Exit(exitErr.ExitCode())
	}
	return childErr
}

const runProxyPort = 15001

// runSandbox is the shared sandbox engine for both community and PRO editions.
// policyChecker and auditWriter may be nil (community: no policy, no audit).
func runSandbox(
	ctx context.Context,
	cancel context.CancelFunc,
	agentName, serverURL, token string,
	policyChecker shim.PolicyChecker,
	auditWriter shim.AuditWriter,
	cmdArgs []string,
) error {
	agentconfig.Conf.AppId = agentName
	agentconfig.Conf.ServerUrl = serverURL
	agentconfig.Conf.WgPort = 0 // embedded netstack, no kernel wg0

	currentPeer, err := registerOrResume(ctx, agentName, serverURL, token)
	if err != nil {
		return err
	}

	localIP := overlayAddr(currentPeer)
	fmt.Printf("[sandbox-run] %q registered, overlay IP=%s\n", agentName, localIP)

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

	logger := agentlog.GetLogger("sandbox-run")
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

	if err := installRunIPTables(runProxyPort); err != nil {
		return fmt.Errorf("iptables setup: %w", err)
	}
	fmt.Printf("[sandbox-run] iptables REDIRECT installed (exempt UID 0, port %d)\n", runProxyPort)

	proxy := &tproxy.Proxy{
		Addr: fmt.Sprintf("0.0.0.0:%d", runProxyPort),
		Dial: sb.DialContext,
	}
	if err := proxy.Start(ctx); err != nil {
		return fmt.Errorf("start transparent proxy: %w", err)
	}
	fmt.Printf("[sandbox-run] transparent proxy listening on :%d\n", runProxyPort)

	select {
	case <-time.After(runReadyWait):
	case <-ctx.Done():
		return ctx.Err()
	}

	return forkAndWait(ctx, cancel, cmdArgs)
}
