//go:build !pro && linux

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
	"time"

	"github.com/spf13/cobra"
)

var (
	runServerURL string
	runToken     string
	runReadyWait time.Duration
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Community: no policy checker, no audit writer.
	return runSandbox(ctx, cancel, agentName, runServerURL, runToken, nil, nil, cmdArgs)
}
