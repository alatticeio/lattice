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

package network

import (
	"wireflow/internal/config"

	"github.com/spf13/cobra"
)

func newRemoveCmd() *cobra.Command {
	var opts config.NetworkOptions
	var cmd = &cobra.Command{
		Use:          "rm [command]",
		SilenceUsage: true,
		Short:        "rm a network",
		Long:         `rm a network you created`,

		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			return runRemove(opts)
		},
	}
	//fs := cmd.Flags()
	//fs.StringVarP(&opts.Listen, "", "l", "", "http port for drp over http")
	//fs.StringVarP(&opts.LogLevel, "log-level", "", "silent", "log level (silent, info, error, warn, verbose)")
	return cmd
}

func runRemove(opts config.NetworkOptions) error {
	return nil
}
