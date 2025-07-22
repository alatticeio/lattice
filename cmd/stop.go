package cmd

import (
	"github.com/spf13/cobra"
	"linkany/node"
	"linkany/pkg/log"
)

func stop() *cobra.Command {
	var flags node.LinkFlags
	cmd := &cobra.Command{
		Short:        "stop",
		Use:          "up",
		SilenceUsage: true,
		Long:         `linkany stop,will stop the linkany daemon and remove the wireguard interface`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLinkanyd(&flags)
		},
	}

	fs := cmd.Flags()
	fs.StringVarP(&flags.InterfaceName, "interface-name", "u", "", "name which create interface use")

	return cmd
}

func stopLinkanyd(flags *node.LinkFlags) error {
	if flags.LogLevel == "" {
		flags.LogLevel = "error"
	}
	log.Loglevel = log.SetLogLevel(flags.LogLevel)
	return node.Start(flags)
}
