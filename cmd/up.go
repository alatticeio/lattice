package cmd

import (
	"github.com/spf13/cobra"
	"linkany/client"
)

type anyOptions struct {
	interfaceName string
	forceRelay    bool
}

func up() *cobra.Command {
	var opts anyOptions
	cmd := &cobra.Command{
		Short:        "up",
		Use:          "up [command]",
		SilenceUsage: true,
		Long:         `linkanyd start up`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLinkanyd(opts)
		},
	}

	fs := cmd.Flags()
	fs.StringVarP(&opts.interfaceName, "interface-name", "u", "", "name of will create interface")
	fs.BoolVarP(&opts.forceRelay, "force-relay", "f", false, "force relay mode")

	return cmd
}

func runLinkanyd(opts anyOptions) error {
	return client.Start(opts.interfaceName, opts.forceRelay)
}
