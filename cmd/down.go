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
	"wireflow/node"
	"wireflow/pkg/log"

	"github.com/spf13/cobra"
)

func stop() *cobra.Command {
	var flags node.LinkFlags
	cmd := &cobra.Command{
		Short:        "down",
		Use:          "down",
		SilenceUsage: true,
		Long:         `linkany will stop the linkany daemon and remove the wireguard interface`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return stopLinkanyd(&flags)
		},
	}

	fs := cmd.Flags()
	fs.StringVarP(&flags.InterfaceName, "interface-name", "u", "", "name which create interface use")

	return cmd
}

func stopLinkanyd(flags *node.LinkFlags) error {
	if flags.LogLevel == "" {
		flags.LogLevel = "error"
	}
	log.Loglevel = log.SetLogLevel(flags.LogLevel)
	return node.Stop(flags)
}
