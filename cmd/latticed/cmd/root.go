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
	"github.com/alatticeio/lattice/internal/agent/config"
	"os"

	"github.com/spf13/cobra"
)

var cfgManager = config.NewConfigManager()

var rootCmd = &cobra.Command{
	Use:           "latticed",
	Short:         "Lattice All-in-One Control Plane",
	Long:          `Latticed manages encrypted private networks with embedded NATS and SQLite.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return cfgManager.LoadConf(cmd)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		isVersion, _ := cmd.Flags().GetBool("version")
		if isVersion {
			fmt.Println("latticed version: dev")
			return nil
		}
		// pre-flight: 服务端模式——自动补全 signaling-url / database.dsn，不报错中断
		if err := config.ValidateAndReport(config.GlobalConfig, true); err != nil {
			return err
		}
		return runLatticed(config.GlobalConfig)
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
	fs.StringP("config-dir", "", "", "config directory (default ~/.lattice)")
	fs.StringP("server-url", "", "", "management server url")
	fs.StringP("signaling-url", "", "", "NATS server URL for agent signaling (server-side)")
	fs.BoolP("version", "", false, "Print version information")
	fs.BoolP("save", "", false, "whether save config to file")

}
