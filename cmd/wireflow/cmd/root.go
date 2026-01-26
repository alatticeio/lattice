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
	"wireflow/cmd/wireflow/cmd/token"
	"wireflow/internal/config"

	"github.com/spf13/cobra"
)

var cfgManager = config.NewConfigManager()

var rootCmd = &cobra.Command{
	Use:           "wireflow",
	Short:         "wireflow: High-performance WireGuard proxy tunneling\n A tool for creating fast and secure network proxies using WireGuard protocol.",
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return cfgManager.LoadConf(cmd)
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		// 检查 --save 是否被触发
		save, _ := cmd.Flags().GetBool("save")
		if save {
			if err := cfgManager.Viper().WriteConfigAs(".wireflow.yaml"); err != nil {
				fmt.Printf("cann't save config: %v\n", err)
			} else {
				fmt.Println("save success to config")
			}
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		isVersion, _ := cmd.Flags().GetBool("version")
		if isVersion {
			runVersion() // 在这里调用你联网获取 Server 版本的逻辑
			return
		}

		cmd.Help()
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
	fs.StringP("server-url", "", "", "management server url")
	fs.StringP("signaling-url", "", "", "signaling server url")
	fs.BoolP("version", "", false, "Print version information")
	fs.BoolP("show-system-log", "", false, "whether show (wireguard/ice) detail log")
	fs.BoolP("save", "", false, "whether save config to file")

	rootCmd.AddCommand(upCmd())
	rootCmd.AddCommand(token.NewTokenCommand())
}
