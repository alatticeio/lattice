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
	"fmt"
	"os/exec"
	"strconv"

	"github.com/spf13/cobra"
)

var (
	initProxyPort int
	initSkipUID   int
)

func addInitCmd(parent *cobra.Command) {
	parent.AddCommand(initCmd())
}

func initCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write iptables REDIRECT rules for K8s sidecar mode (init container)",
		Long: `Init sets up iptables rules that redirect all outbound TCP traffic from the
pod to the Lattice sidecar transparent proxy port. Traffic from the sidecar
process itself (identified by --skip-uid) is exempted to prevent loops.

Run as a Kubernetes initContainer with NET_ADMIN capability. Exits 0 on success.

Example K8s manifest:
  initContainers:
  - name: lattice-init
    image: ghcr.io/alattice/lattice
    command: ["lattice", "agent", "init"]
    securityContext:
      capabilities:
        add: ["NET_ADMIN"]`,
		RunE: runInit,
	}
	cmd.Flags().IntVar(&initProxyPort, "proxy-port", 15001, "TCP port the sidecar transparent proxy listens on")
	cmd.Flags().IntVar(&initSkipUID, "skip-uid", 1337, "UID whose outbound traffic is exempt from redirect (the sidecar's UID)")
	return cmd
}

func runInit(_ *cobra.Command, _ []string) error {
	fmt.Printf("[lattice-init] setting up iptables redirect -> port %d (skip UID %d)\n", initProxyPort, initSkipUID)
	for _, args := range buildIPTablesRules(initProxyPort, initSkipUID) {
		out, err := exec.Command("iptables", args...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("iptables %v: %s: %w", args, out, err)
		}
	}
	fmt.Println("[lattice-init] iptables rules installed successfully")
	return nil
}

// buildIPTablesRules returns the ordered list of iptables argument slices to install.
func buildIPTablesRules(proxyPort, skipUID int) [][]string {
	port := strconv.Itoa(proxyPort)
	uid := strconv.Itoa(skipUID)
	return [][]string{
		// Create the LATTICE_REDIRECT chain.
		{"-t", "nat", "-N", "LATTICE_REDIRECT"},
		// Exempt sidecar's own traffic (by UID) to prevent redirect loops.
		{"-t", "nat", "-A", "LATTICE_REDIRECT", "-m", "owner", "--uid-owner", uid, "-j", "RETURN"},
		// Redirect all other TCP to the proxy port.
		{"-t", "nat", "-A", "LATTICE_REDIRECT", "-p", "tcp", "-j", "REDIRECT", "--to-ports", port},
		// Hook the chain into OUTPUT (outbound traffic from all processes).
		{"-t", "nat", "-A", "OUTPUT", "-p", "tcp", "-j", "LATTICE_REDIRECT"},
	}
}
