package cmd

import (
	"github.com/spf13/cobra"
	"linkany/signaling"
)

type signalerOptions struct {
	Listen string
}

func drpCmd() *cobra.Command {
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
	return cmd
}

// run signaling server
func runSignaling(opts signalerOptions) error {
	return signaling.Start(opts.Listen)
}
