package cmd

import (
	"github.com/spf13/cobra"
	"linkany/node"
	"linkany/pkg/log"
)

func status() *cobra.Command {
	var flags node.LinkFlags
	cmd := &cobra.Command{
		Short:        "status",
		Use:          "status",
		SilenceUsage: true,
		Long:         `linkany status command is used to check the status of the linkany daemon.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return linkanyInfo(&flags)
		},
	}

	fs := cmd.Flags()
	fs.StringVarP(&flags.InterfaceName, "interface-name", "u", "", "name which create interface use")

	return cmd
}

func linkanyInfo(flags *node.LinkFlags) error {
	if flags.LogLevel == "" {
		flags.LogLevel = "error"
	}
	log.Loglevel = log.SetLogLevel(flags.LogLevel)
	return node.Status(flags)
}
