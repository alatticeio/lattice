package cmd

import (
	"github.com/spf13/cobra"
	"linkany/client"
	"linkany/pkg/log"
)

func up() *cobra.Command {
	var flags client.ClientFlags
	cmd := &cobra.Command{
		Short:        "up",
		Use:          "up [command]",
		SilenceUsage: true,
		Long:         `linkany startup, will create a wireguard interface and join your linkany network,and also will config the interface automatically`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLinkanyd(&flags)
		},
	}

	fs := cmd.Flags()
	fs.StringVarP(&flags.InterfaceName, "interface-name", "u", "", "name which create interface use")
	fs.BoolVarP(&flags.ForceRelay, "force-relay", "f", false, "force relay mode")
	fs.StringVarP(&flags.LogLevel, "log-level", "l", "silent", "log level (silent, info, error, warn, verbose)")
	fs.StringVarP(&flags.ManagementUrl, "control-url", "", "", "management server url, need not give when you are using our service")
	fs.StringVarP(&flags.TurnServerUrl, "turn-url", "", "", "just need modify when you custom your own relay server")
	fs.StringVarP(&flags.SignalingUrl, "", "", "", "signaling service, not need to modify")

	return cmd
}

func runLinkanyd(flags *client.ClientFlags) error {
	if flags.LogLevel == "" {
		flags.LogLevel = "error"
	}
	log.Loglevel = log.SetLogLevel(flags.LogLevel)
	return client.Start(flags)
}
