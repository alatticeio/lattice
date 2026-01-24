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
	"wireflow/internal/config"
	"wireflow/internal/infra"
	"wireflow/pkg/cmd/network"

	"github.com/spf13/cobra"
)

func newCreateCmd() *cobra.Command {
	var opts config.NetworkOptions
	var cmd = &cobra.Command{
		Use:          "create <network-name>",
		SilenceUsage: true,
		Short:        "create a network",
		Long:         `create a network for nodes ip allocation`,

		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				opts.Name = args[0]
			} else {
				opts.Name = network.GenerateNetworkID()
			}

			return runCreate(&opts)
		},
	}
	fs := cmd.Flags()
	fs.StringVarP(&opts.Name, "name", "n", "", "network name")
	fs.StringVarP(&opts.CIDR, "cidr", "", "", "network cidr used to allocate IP address for wireflow peers")
	return cmd
}

func runCreate(opts *config.NetworkOptions) error {
	if infra.ServerUrl == "" {
		infra.ServerUrl = config.Conf.ServerUrl
	}
	manager, err := network.NewNetworkManager(infra.ServerUrl)
	if err != nil {
		return err
	}
	if err = manager.CreateNetwork(context.Background(), opts); err != nil {
		return err
	}
	fmt.Printf(" >> Create network '%s' successfully!\n", opts.Name)
	return nil
}
