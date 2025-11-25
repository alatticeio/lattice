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
	"wireflow/drp"
	"wireflow/pkg/log"

	"github.com/spf13/cobra"
)

type signalerOptions struct {
	Listen   string
	LogLevel string
}

func signalingCmd() *cobra.Command {
	var opts signalerOptions
	var cmd = &cobra.Command{
		Use:          "signaling [command]",
		SilenceUsage: true,
		Short:        "signaling is a signaling server",
		Long:         `signaling will start a signaling server, signaling server is used to exchange the network information between the clients. which is our core feature.`,

		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			return runSignaling(opts)
		},
	}
	fs := cmd.Flags()
	fs.StringVarP(&opts.Listen, "", "l", "", "http port for drp over http")
	fs.StringVarP(&opts.LogLevel, "log-level", "", "silent", "log level (silent, info, error, warn, verbose)")
	return cmd
}

// run signaling server
func runSignaling(opts signalerOptions) error {
	if opts.LogLevel == "" {
		opts.LogLevel = "error"
	}
	log.SetLogLevel(opts.LogLevel)
	return drp.Start(opts.Listen)
}
