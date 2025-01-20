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
		Use:          "signaler [command]",
		SilenceUsage: true,
		Short:        "signaler is a signaling server",
		Long:         `signaler used for signaling`,

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

// run drp
func runSignaling(opts signalerOptions) error {
	return signaling.Start(opts.Listen)
}
