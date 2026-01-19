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
	"wireflow/internal/config"
	"wireflow/wrrper"

	"github.com/spf13/cobra"
)

func newWrrpCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:          "wrrp [command]",
		SilenceUsage: true,
		Short:        "wrrp using as relay server for wireflow",
		Long:         `wrrp using as relay server for wireflow`,

		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			return runWrrp(config.Conf)
		},
	}
	fs := cmd.Flags()
	fs.StringP("listen", "l", "", "port for wrrp server")
	fs.BoolP("enable-tls", "", false, "using tls")
	return cmd
}

// run signaling server
func runWrrp(flags *config.Flags) error {
	server := wrrper.NewServer(flags)
	return server.Start()
}
