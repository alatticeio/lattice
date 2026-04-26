// Copyright 2025 The Wireflow Authors, Inc.
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
	"os"
	"wireflow/cmd/wireflow/cmd/peer"
	"wireflow/cmd/wireflow/cmd/policy"
	"wireflow/cmd/wireflow/cmd/token"
	"wireflow/cmd/wireflow/cmd/workspace"
	"wireflow/internal/config"

	"github.com/spf13/cobra"
)

var cfgManager = config.NewConfigManager()

var rootCmd = &cobra.Command{
	Use:   "wireflow",
	Short: "High-performance WireGuard-based overlay network manager",
	Long: `Wireflow connects agents across networks using WireGuard tunnels and a
centralized management plane. Agents join a workspace via enrollment tokens,
and traffic is governed by explicit allow/deny policies.

Quick start:
  wireflow workspace add dev
  wireflow token create dev-team -n <namespace>
  wireflow up --token <token> --server-url <server-url> --signaling-url <signaling-url>`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return cfgManager.LoadConf(cmd)
	},
	Run: func(cmd *cobra.Command, args []string) {
		isVersion, _ := cmd.Flags().GetBool("version")
		if isVersion {
			err := runVersion() // 在这里调用你联网获取 Server 版本的逻辑
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
	fs.StringP("config-dir", "", "", "config directory (default: ~/.wireflow)")
	fs.StringP("server-url", "", "", "management server URL")
	fs.StringP("signaling-url", "", "", "signaling server URL")
	fs.BoolP("version", "", false, "print version information")
	fs.BoolP("save", "", false, "persist flags to config file")

	rootCmd.AddCommand(upCmd())
	rootCmd.AddCommand(statusCmd())
	rootCmd.AddCommand(token.NewTokenCommand())
	rootCmd.AddCommand(workspace.NewWorkspaceCommand())
	rootCmd.AddCommand(policy.NewPolicyCommand())
	rootCmd.AddCommand(peer.NewPeerCommand())
}
