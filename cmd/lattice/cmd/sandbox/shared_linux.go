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
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	shim "github.com/alatticeio/lattice-shim/shim"
	latticeagent "github.com/alatticeio/lattice/internal/agent"
	agentconfig "github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/infra"
	agentlog "github.com/alatticeio/lattice/internal/agent/log"
)

const (
	auditLogPath = "/tmp/lattice-audit.jsonl"
)

// policyDialer wraps net.Dial with optional egress policy checking and audit.
// Traffic goes through the kernel (wf0) to the WireGuard overlay.
type policyDialer struct {
	identity string
	checker  shim.PolicyChecker // nil = no policy enforcement
	auditor  shim.AuditWriter   // nil = no audit
}

var defaultDialer = &net.Dialer{}

var _ shim.ContextDialer = (*policyDialer)(nil)

func (d *policyDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("policyDialer: invalid addr %q: %w", addr, err)
	}

	ip := net.ParseIP(host)

	if d.checker != nil {
		if ip == nil {
			// Hostname — can't check IP-based policy; deny to prevent bypass.
			if d.auditor != nil {
				_ = d.auditor.Write(shim.AuditEvent{
					Identity: d.identity,
					DstIP:    host,
					Protocol: network,
					Verdict:  shim.VerdictDrop,
				})
			}
			return nil, fmt.Errorf("egress policy: hostname %q not allowed (IP-based policy only)", host)
		}
		var port uint16
		if p, parseErr := strconv.ParseUint(portStr, 10, 16); parseErr == nil {
			port = uint16(p)
		}
		if !d.checker.Allow(d.identity, ip, port) {
			if d.auditor != nil {
				_ = d.auditor.Write(shim.AuditEvent{
					Identity: d.identity,
					DstIP:    host,
					DstPort:  port,
					Protocol: network,
					Verdict:  shim.VerdictDrop,
				})
			}
			return nil, fmt.Errorf("egress policy denied: %s", addr)
		}
	}

	conn, connErr := defaultDialer.DialContext(ctx, network, addr)
	if connErr == nil && d.auditor != nil {
		var port uint16
		if p, parseErr := strconv.ParseUint(portStr, 10, 16); parseErr == nil {
			port = uint16(p)
		}
		_ = d.auditor.Write(shim.AuditEvent{
			Identity: d.identity,
			DstIP:    host,
			DstPort:  port,
			Protocol: network,
			Verdict:  shim.VerdictAllow,
		})
	}
	return conn, connErr
}

// fileAuditWriter implements shim.AuditWriter by appending JSON lines to a file.
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

func (w *fileAuditWriter) Write(event shim.AuditEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	_, err = fmt.Fprintf(w.f, "%s\n", data)
	return err
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

// forkAgent forks the AI agent as a child process. The child inherits the same
// network namespace and can reach overlay peers directly via kernel wf0 routing.
// No proxy injection — WireGuard routing handles overlay connectivity natively.
func forkAgent(ctx context.Context, cancel context.CancelFunc, cmdArgs []string) error {
	child := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	child.Env = os.Environ()

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

// runSandbox is the shared sandbox engine for both community and PRO editions.
// It creates a standard kernel wf0 (identical to a regular lattice agent) and
// forks the AI agent as a child process. The child inherits the same network
// namespace and routes overlay traffic directly through wf0 — no SOCKS5 proxy,
// no iptables, no UID tricks.
//
// checker and auditor are reserved for Phase B (MCP proxy policy enforcement).
func runSandbox(
	ctx context.Context,
	cancel context.CancelFunc,
	agentName string,
	currentPeer *infra.Peer,
	_ shim.PolicyChecker,
	_ shim.AuditWriter,
	cmdArgs []string,
) error {
	agentconfig.Conf.AppId = agentName

	localIP := overlayAddr(currentPeer)
	fmt.Printf("[sandbox-run] %q registered, overlay IP=%s\n", agentName, localIP)

	if currentPeer.LrpUrl != "" {
		agentconfig.Conf.EnableLrp = true
		agentconfig.Conf.RelayURL = currentPeer.LrpUrl
	}

	agentJWT := currentPeer.Token
	logger := agentlog.GetLogger("sandbox-run")

	nodeCfg := &latticeagent.NodeConfig{
		Logger:      logger,
		Port:        0,
		ShowLog:     false,
		Flags:       agentconfig.Conf,
		CurrentPeer: currentPeer,
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

	select {
	case <-time.After(runReadyWait):
	case <-ctx.Done():
		return ctx.Err()
	}

	return forkAgent(ctx, cancel, cmdArgs)
}
