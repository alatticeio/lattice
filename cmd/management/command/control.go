package command

import (
	"github.com/spf13/cobra"
	"linkany/management"
)

type managementOptions struct {
	Listen string
}

func ManagementCmd() *cobra.Command {
	var opts managementOptions
	var cmd = &cobra.Command{
		Use:          "manager [command]",
		SilenceUsage: true,
		Short:        "manager is control server",
		Long:         `manager used for starting management server, management providing our all control plance features.`,

		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			return runManagement(opts)
		},
	}
	fs := cmd.Flags()
	fs.StringVarP(&opts.Listen, "", "l", "", "management server listen address")
	return cmd
}

// run drp
func runManagement(opts managementOptions) error {
	return management.Start(opts.Listen)
}
