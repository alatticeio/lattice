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
	"wireflow/internal/config"
	"wireflow/pkg/cmd/network"

	"github.com/spf13/cobra"
)

func newJoinCmd() *cobra.Command {
	var opts config.NetworkOptions
	var cmd = &cobra.Command{
		Use:          "join <network-name>",
		SilenceUsage: true,
		Short:        "join into a network",
		Long:         `join into a network you created`,

		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Name = args[0]
			return runJoin(&opts)
		},
	}
	return cmd
}

func runJoin(opts *config.NetworkOptions) error {
	manager, err := network.NewNetworkManager(opts.ServerUrl)
	if err != nil {
		return err
	}
	return manager.JoinNetwork(context.Background(), opts)
}
