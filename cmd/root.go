// Copyright 2025 Wireflow.io, Inc.
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

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "wireflow",
	Short: "WireFlow: High-performance WireGuard proxy tunneling\n A tool for creating fast and secure network proxies using WireGuard protocol.",
	Long: `WireFlow: High-performance WireGuard proxy tunneling
A tool for creating fast and secure network proxies using WireGuard protocol.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Do Stuff Here
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(up())
	rootCmd.AddCommand(loginCmd())
	rootCmd.AddCommand(managementCmd())
	rootCmd.AddCommand(signalingCmd())
	rootCmd.AddCommand(turnCmd())
	rootCmd.AddCommand(stop())
	rootCmd.AddCommand(status())
}
