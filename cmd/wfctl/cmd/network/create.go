package network

import (
	"context"
	"wireflow/pkg/cli/network"
	"wireflow/pkg/config"

	"github.com/spf13/cobra"
)

func newCreateCmd() *cobra.Command {
	var opts config.NetworkOptions
	var cmd = &cobra.Command{
		Use:          "create [command]",
		SilenceUsage: true,
		Short:        "create into a network",
		Long:         `create into a network you created`,

		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				opts.Name = network.GenerateNetworkID()
			}
			return runCreate(&opts)
		},
	}
	fs := cmd.Flags()
	fs.StringVarP(&opts.Name, "name", "n", "", "network name")
	fs.StringVarP(&opts.CIDR, "cidr", "", "", "network cidr used to allocate IP address for wireflow peers")
	fs.StringVarP(&opts.ServerUrl, "server-url", "", "", "management server url")
	return cmd
}

func runCreate(opts *config.NetworkOptions) error {
	manager, err := network.NewNetworkManager(opts.ServerUrl)
	if err != nil {
		return err
	}
	return manager.CreateNetwork(context.Background(), opts)
}
