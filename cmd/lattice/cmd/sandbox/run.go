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
	"strings"
	"time"

	shim "github.com/alatticeio/lattice-shim/shim"
	latticeagent "github.com/alatticeio/lattice/internal/agent"
	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/spf13/cobra"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var (
	runServerURL   string
	runToken       string
	runReadyWait   time.Duration
	runEgressAllow string
	runEgressDeny  bool
)

func addRunCmd(parent *cobra.Command) {
	parent.AddCommand(runCmd())
}

func runCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <name> -- <command> [args...]",
		Short: "Run an AI agent with transparent WireGuard overlay networking",
		Long: `Run registers a sandbox with the Lattice control plane, starts an embedded
gVisor netstack WireGuard node (no kernel wg0), transparently intercepts the
AI agent's TCP traffic via iptables, and routes it through the overlay.

All network observability happens at the gVisor netstack layer — every
connection flows through policy and audit hooks (available in Pro).

The AI agent needs zero configuration — no SOCKS5, no proxy env vars.

Pro accounts get egress policy control (--egress-allow / --egress-default-deny)
and full audit logging to /tmp/lattice-audit.jsonl.

Requires --cap-add NET_ADMIN for iptables.

Example:
  docker run --rm --cap-add NET_ADMIN ghcr.io/alatticeio/lattice \
    sandbox run my-agent --server-url http://latticed:8080 --token lt-xxx \
    -- python agent.py`,
		Args: cobra.ArbitraryArgs,
		RunE: runRun,
	}
	cmd.Flags().StringVar(&runServerURL, "server-url", "", "Lattice control plane URL (required)")
	cmd.Flags().StringVar(&runToken, "token", "", "Enrollment token (required)")
	cmd.Flags().DurationVar(&runReadyWait, "ready-wait", 3*time.Second,
		"Time to wait for WireGuard peer sessions before starting the AI agent")
	cmd.Flags().StringVar(&runEgressAllow, "egress-allow", "",
		"Comma-separated overlay CIDRs the AI agent is allowed to reach (Pro)")
	cmd.Flags().BoolVar(&runEgressDeny, "egress-default-deny", false,
		"Deny all egress except --egress-allow CIDRs (Pro)")
	_ = cmd.MarkFlagRequired("server-url")
	_ = cmd.MarkFlagRequired("token")
	return cmd
}

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

func runRun(_ *cobra.Command, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: lattice sandbox run <name> -- <command> [args...]")
	}
	agentName := args[0]
	cmdArgs := args[1:]

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register first to get the tier from the server response.
	currentPeer, err := registerOrResume(ctx, agentName, runServerURL, runToken)
	if err != nil {
		return err
	}

	// Build policy and audit based on account tier.
	var policyChecker shim.PolicyChecker
	var auditWriter shim.AuditWriter

	if currentPeer.Tier == "pro" {
		egressPolicy := shim.EgressPolicy{DefaultDeny: runEgressDeny}
		if runEgressAllow != "" {
			for _, entry := range strings.Split(runEgressAllow, ",") {
				entry = strings.TrimSpace(entry)
				if entry == "" {
					continue
				}
				_, cidr, parseErr := net.ParseCIDR(entry)
				if parseErr != nil {
					return fmt.Errorf("invalid egress CIDR %q: %w", entry, parseErr)
				}
				egressPolicy.AllowedCIDRs = append(egressPolicy.AllowedCIDRs, *cidr)
			}
		}

		policyChecker = shim.NewEgressFilter(egressPolicy)
		auditWriter, err = newFileAuditWriter(auditLogPath)
		if err != nil {
			fmt.Printf("[sandbox-run] warning: open audit log: %v\n", err)
		}
	} else if runEgressDeny || runEgressAllow != "" {
		fmt.Fprintln(os.Stderr, "[sandbox-run] WARNING: egress policy flags require a Pro account. These flags will be ignored.")
	}

	return runSandbox(ctx, cancel, agentName, currentPeer, policyChecker, auditWriter, cmdArgs)
}
