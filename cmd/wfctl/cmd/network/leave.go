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
	"context"
	"fmt"
	"wireflow/pkg/cli/network"
	"wireflow/pkg/config"

	"github.com/spf13/cobra"
)

func newLeaveCmd() *cobra.Command {
	var opts config.NetworkOptions
	var cmd = &cobra.Command{
		Use:          "leave [command]",
		SilenceUsage: true,
		Short:        "leave a network",
		Long:         `leave network wireflow has joined`,

		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 0 {
				return fmt.Errorf("Network name is required")
			}
			opts.Name = args[0]
			return runLeave(&opts)
		},
	}
	fs := cmd.Flags()
	fs.StringVarP(&opts.ServerUrl, "server-url", "", "", "management server url")
	return cmd
}

func runLeave(opts *config.NetworkOptions) error {
	manager, err := network.NewNetworkManager(opts.ServerUrl)
	if err != nil {
		return err
	}
	return manager.LeaveNetwork(context.Background(), opts)
}
