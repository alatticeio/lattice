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
	"wireflow/client"
	"wireflow/internal/config"

	"github.com/spf13/cobra"
)

func upCmd() *cobra.Command {
	var flags config.Flags
	// upCmd 代表 config 顶层命令
	var upCmd = &cobra.Command{
		Use:     "up",
		Short:   "wireflow startup command",
		Example: "wireflow up --token <token> --server-url <server-url> --signaling-url <signaling-url>",
		RunE: func(cmd *cobra.Command, args []string) error {
			return client.Start(&flags)
		},
	}

	fs := upCmd.Flags()
	fs.StringVarP(&flags.Token, "token", "", "", "token using for creating or joining network")
	fs.StringVarP(&flags.LogLevel, "level", "", "", "log level (debug, info, warn, error)")
	return upCmd
}
