package network

import (
	"wireflow/pkg/config"

	"github.com/spf13/cobra"
)

func newUpdateCmd() *cobra.Command {
	var opts config.NetworkOptions
	var cmd = &cobra.Command{
		Use:          "update [command]",
		SilenceUsage: true,
		Short:        "update into a network",
		Long:         `update into a network you created`,

		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(opts)
		},
	}
	//fs := cmd.Flags()
	//fs.StringVarP(&opts.Listen, "", "l", "", "http port for drp over http")
	//fs.StringVarP(&opts.LogLevel, "log-level", "", "silent", "log level (silent, info, error, warn, verbose)")
	return cmd
}

func runUpdate(opts config.NetworkOptions) error {
	return nil
}
