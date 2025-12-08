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
	"wireflow/cmd/wfctl/cmd/network"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "wfctl",
	Short: "wfctl: High-performance WireGuard proxy tunneling\n A tool for creating fast and secure network proxies using WireGuard protocol.",
	Long: `wfctl: High-performance WireGuard proxy tunneling
A tool for creating fast and secure network proxies using WireGuard protocol.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(network.NewNetworkCommand())
}
