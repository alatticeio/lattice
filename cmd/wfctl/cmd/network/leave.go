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
