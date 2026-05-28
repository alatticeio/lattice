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

	shimfwd "github.com/alatticeio/lattice-shim/shim"
	"github.com/spf13/cobra"
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
	cmd.Flags().StringVar(&sandboxMode, "mode", "pod", "Isolation mode: pod | gvisor")
	cmd.Flags().StringVar(&sandboxAgentRootFS, "agent-rootfs", "", "Root filesystem path for runsc container (gvisor mode)")
	cmd.Flags().StringVar(&sandboxAgentBinary, "agent-binary", "", "Agent entrypoint binary (gvisor mode)")
	cmd.Flags().StringSliceVar(&sandboxAgentArgs, "agent-args", nil, "Agent entrypoint arguments (gvisor mode)")
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
	sandboxWgPort       int

	// gvisor (runsc) mode flags.
	sandboxMode        string
	sandboxAgentRootFS string
	sandboxAgentBinary string
	sandboxAgentArgs   []string
)

// auditLogPath is where the sandbox writes JSONL audit events.
const auditLogPath = "/tmp/lattice-audit.jsonl"

func runStart(_ *cobra.Command, _ []string) error {
	cfg := DriverConfig{
		SandboxName:  sandboxName,
		ServerURL:    sandboxServerURL,
		Token:        sandboxToken,
		EgressAllow:  sandboxEgressAllow,
		EgressDeny:   sandboxEgressDeny,
		ProxyAddr:    sandboxProxyAddr,
		ForwardRules: sandboxForwardRules,
		RootFS:       sandboxAgentRootFS,
		AgentBinary:  sandboxAgentBinary,
		AgentArgs:    sandboxAgentArgs,
	}

	if err := validateDriverConfig(sandboxMode, cfg); err != nil {
		return err
	}

	driver := NewDriver(sandboxMode, cfg)
	if driver == nil {
		return fmt.Errorf("unknown isolation mode %q: choose pod or gvisor", sandboxMode)
	}

	ctx := context.Background()
	fmt.Printf("Starting sandbox %q in %s mode...\n", sandboxName, driver.Name())
	return driver.Start(ctx)
}

// validateDriverConfig checks that required fields are present for the given mode.
func validateDriverConfig(mode string, cfg DriverConfig) error {
	if mode == "gvisor" {
		if cfg.RootFS == "" {
			return fmt.Errorf("--agent-rootfs is required for gvisor mode")
		}
		if cfg.AgentBinary == "" {
			return fmt.Errorf("--agent-binary is required for gvisor mode")
		}
	}
	if cfg.EgressAllow != "" {
		for _, entry := range strings.Split(cfg.EgressAllow, ",") {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			if _, _, err := net.ParseCIDR(entry); err != nil {
				return fmt.Errorf("invalid egress CIDR %q: %w", entry, err)
			}
		}
	}
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
