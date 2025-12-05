package network

import (
	"context"
	"fmt"
	"wireflow/pkg/cli/network"
	"wireflow/pkg/config"

	"github.com/spf13/cobra"
)

func newJoinCmd() *cobra.Command {
	var opts config.NetworkOptions
	var cmd = &cobra.Command{
		Use:          "join [network name]",
		SilenceUsage: true,
		Short:        "join into a network",
		Long:         `join into a network you created`,

		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("must specify network name")
			}
			opts.Name = args[0]
			return runJoin(&opts)
		},
	}
	fs := cmd.Flags()
	fs.StringVarP(&opts.ServerUrl, "server-url", "", "", "management server url")
	return cmd
}

func runJoin(opts *config.NetworkOptions) error {
	manager, err := network.NewNetworkManager(opts.ServerUrl)
	if err != nil {
		return err
	}
	return manager.JoinNetwork(context.Background(), opts)
}
