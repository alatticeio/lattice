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
	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/server"
	"os"

	"github.com/spf13/cobra"
)

func newManagementCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:          "manager [command]",
		SilenceUsage: true,
		Short:        "manager is control server",
		Long:         `manager used for starting management server, management providing our all control plane features.`,

		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			return runManagement(config.Conf)
		},
	}
	fs := cmd.Flags()
	fs.StringP("listen", "l", "", "management server listen address")
	fs.StringP("level", "", "silent", "log level (silent, info, error, warn, verbose)")
	fs.StringP("env", "", "dev", "run environment (dev, pre-run, production) ")
	return cmd
}

// run drp
func runManagement(flags *config.Config) error {
	log.SetLevel(flags.Level)
	// pre-flight: only print a warning when signaling-url is empty (management can run degraded, but with limited functionality)
	if flags.SignalingURL == "" {
		fmt.Fprintln(os.Stderr, "[pre-flight] WARNING: signaling-url is not configured; NATS signaling will be disabled, and agents will not receive WireGuard peer updates")
	}
	return management.Start(flags)
}
