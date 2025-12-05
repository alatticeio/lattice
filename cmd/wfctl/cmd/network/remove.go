package network

import (
	"wireflow/pkg/config"

	"github.com/spf13/cobra"
)

func newRemoveCmd() *cobra.Command {
	var opts config.NetworkOptions
	var cmd = &cobra.Command{
		Use:          "rm [command]",
		SilenceUsage: true,
		Short:        "rm a network",
		Long:         `rm a network you created`,

		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			return runRemove(opts)
		},
	}
	//fs := cmd.Flags()
	//fs.StringVarP(&opts.Listen, "", "l", "", "http port for drp over http")
	//fs.StringVarP(&opts.LogLevel, "log-level", "", "silent", "log level (silent, info, error, warn, verbose)")
	return cmd
}

func runRemove(opts config.NetworkOptions) error {
	return nil
}
