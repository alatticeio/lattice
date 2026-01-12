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
	"wireflow/internal/core/infra"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "wireflow",
	Short: "wireflow: High-performance WireGuard proxy tunneling\n A tool for creating fast and secure network proxies using WireGuard protocol.",
	Run: func(cmd *cobra.Command, args []string) {
		isVersion, _ := cmd.Flags().GetBool("version")
		if isVersion {
			runVersion() // 在这里调用你联网获取 Server 版本的逻辑
			return
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func init() {
	fs := rootCmd.PersistentFlags()
	fs.StringVarP(&infra.ServerUrl, "server-url", "", "", "management server url")
	fs.StringVarP(&infra.SignalUrl, "signal-url", "", "", "signaling server url")
	fs.BoolP("version", "v", false, "Print version information")
	rootCmd.AddCommand(upCmd())
	rootCmd.AddCommand(configCmd)
}
