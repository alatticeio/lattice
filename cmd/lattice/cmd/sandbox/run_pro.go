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
	"strings"
	"time"

	shimfwd "github.com/alatticeio/lattice-shim/shim"
	"github.com/spf13/cobra"
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
		Short: "Run an AI agent with egress control and audit (Pro)",
		Long: `Run registers a sandbox with the Lattice control plane, starts an embedded
gVisor netstack WireGuard node (no kernel wg0), transparently intercepts the
AI agent's TCP traffic, enforces egress policy, and writes an audit log.

Pro features:
  --egress-allow        Comma-separated overlay CIDRs the AI agent can reach
  --egress-default-deny  Deny all egress except --egress-allow CIDRs

Every AI agent connection flows through:
  PolicyChecker → egress CIDR allow/deny
  AuditWriter   → /tmp/lattice-audit.jsonl

Requires --cap-add NET_ADMIN for iptables.

Example:
  docker run --rm --cap-add NET_ADMIN ghcr.io/alatticeio/lattice \
    sandbox run my-agent --server-url http://latticed:8080 --token lt-xxx \
    --egress-allow 10.0.0.0/8 --egress-default-deny \
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

func runRun(_ *cobra.Command, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: lattice sandbox run <name> -- <command> [args...]")
	}
	agentName := args[0]
	cmdArgs := args[1:]

	// Build egress policy for gVisor netstack PolicyChecker.
	egressPolicy := shimfwd.EgressPolicy{DefaultDeny: runEgressDeny}
	if runEgressAllow != "" {
		for _, entry := range strings.Split(runEgressAllow, ",") {
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

	policyChecker := shimfwd.NewEgressFilter(egressPolicy)
	auditWriter, auditErr := newFileAuditWriter(auditLogPath)
	if auditErr != nil {
		fmt.Printf("[sandbox-run] warning: open audit log: %v\n", auditErr)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	return runSandbox(ctx, cancel, agentName, runServerURL, runToken, policyChecker, auditWriter, cmdArgs)
}
