package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"linkany/management/client"
	"linkany/pkg/config"
	"linkany/pkg/log"
	"linkany/turn"
	"linkany/turn/server"
)

type turnOptions struct {
	PublicIP string
	Port     int
}

func turnCmd() *cobra.Command {
	var opts turnOptions
	var cmd = &cobra.Command{
		Use:          "turn",
		SilenceUsage: true,
		Short:        "start a turn server",
		Long:         `a turn serer will provided stun service for up, you can use it to get public IP and port`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			return runTurn(opts)
		},
	}
	fs := cmd.Flags()
	fs.StringVarP(&opts.PublicIP, "public-ip", "u", "", "public ip for turn")
	fs.IntVarP(&opts.Port, "port", "p", 3478, "port for turn")

	return cmd
}

func runTurn(opts turnOptions) error {
	conf, err := config.GetLocalConfig()
	if err != nil {
		return err
	}
	client := client.NewClient(&client.ClientConfig{
		Conf: conf,
	})

	return turn.Start(&server.TurnServerConfig{
		Logger:   log.NewLogger(log.LogLevelVerbose, fmt.Sprintf("[%s] ", "turnserver")),
		PublicIP: opts.PublicIP,
		Port:     opts.Port,
		Client:   client,
	})
}
