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

package cmd

import (
	"fmt"
	"github.com/alatticeio/lattice/cmd/lattice/cmd/peer"
	"github.com/alatticeio/lattice/cmd/lattice/cmd/policy"
	"github.com/alatticeio/lattice/cmd/lattice/cmd/sandbox"
	"github.com/alatticeio/lattice/cmd/lattice/cmd/token"
	"github.com/alatticeio/lattice/cmd/lattice/cmd/workspace"
	"github.com/alatticeio/lattice/internal/agent/config"
	"os"

	"github.com/spf13/cobra"
)

var cfgManager = config.NewConfigManager()

var rootCmd = &cobra.Command{
	Use:   "lattice",
	Short: "WireGuard overlay networking with policy control and agent sandboxing",
	Long: `Lattice connects agents across networks using WireGuard tunnels,
enforces traffic policies, and isolates workloads in secure agent sandboxes.
Agents join a workspace via enrollment tokens issued from the control plane.

Quick start (token enrollment):
  lattice up --token <token> --server-url https://your-server

Quick start (interactive):
  lattice init
  lattice up`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return cfgManager.LoadConf(cmd)
	},
	Run: func(cmd *cobra.Command, args []string) {
		isVersion, _ := cmd.Flags().GetBool("version")
		if isVersion {
			err := runVersion() // calls the server version logic
			if err != nil {
				fmt.Println(err)
			}
			return
		}

		err := cmd.Help()
		if err != nil {
			fmt.Println(err)
		}
	},
}

// Execute executes the root command.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func init() {
	fs := rootCmd.PersistentFlags()
	fs.StringP("config-dir", "", "", "config directory (default: ~/.lattice)")
	fs.StringP("server-url", "", "", "management server URL")
	fs.BoolP("version", "", false, "print version information")
	fs.BoolP("save", "", false, "persist flags to config file")

	rootCmd.AddCommand(initCmd())
	rootCmd.AddCommand(loginCmd())
	rootCmd.AddCommand(upCmd())
	rootCmd.AddCommand(statusCmd())
	rootCmd.AddCommand(token.NewTokenCommand())
	rootCmd.AddCommand(workspace.NewWorkspaceCommand())
	rootCmd.AddCommand(policy.NewPolicyCommand())
	rootCmd.AddCommand(peer.NewPeerCommand())
	rootCmd.AddCommand(sandbox.SandboxCmd())
}
