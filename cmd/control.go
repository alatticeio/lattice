package cmd

import (
	"github.com/spf13/cobra"
	"linkany/management"
)

type managementOptions struct {
	Listen string
}

func managementCmd() *cobra.Command {
	var opts managementOptions
	var cmd = &cobra.Command{
		Use:          "manager [command]",
		SilenceUsage: true,
		Short:        "manager is control server",
		Long:         `manager used for`,

		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			return runManagement(opts)
		},
	}
	fs := cmd.Flags()
	fs.StringVarP(&opts.Listen, "", "l", "", "http port for drp over http")
	//fs.BoolVarP(&opts.RunDrp, "", "b", true, "run drp")
	return cmd
}

// run drp
func runManagement(opts managementOptions) error {
	return management.Start(opts.Listen)
}
