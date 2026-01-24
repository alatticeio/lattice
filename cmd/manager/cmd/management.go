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
	"wireflow/internal/log"
	"wireflow/management"

	"github.com/spf13/cobra"
)

type managementOptions struct {
	Listen   string
	LogLevel string
}

func newManagementCmd() *cobra.Command {
	var opts managementOptions
	var cmd = &cobra.Command{
		Use:          "manager [command]",
		SilenceUsage: true,
		Short:        "manager is control server",
		Long:         `manager used for starting management server, management providing our all control plane features.`,

		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			return runManagement(opts)
		},
	}
	fs := cmd.Flags()
	fs.StringP("listen", "l", "", "management server listen address")
	fs.StringP("level", "", "silent", "log level (silent, info, error, warn, verbose)")
	return cmd
}

// run drp
func runManagement(opts managementOptions) error {
	if opts.LogLevel == "" {
		opts.LogLevel = "error"
	}
	log.SetLevel(opts.LogLevel)
	return management.Start(opts.Listen)
}
