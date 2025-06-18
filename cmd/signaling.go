package cmd

import (
	"github.com/spf13/cobra"
	"linkany/drp"
	"linkany/pkg/log"
)

type signalerOptions struct {
	Listen   string
	LogLevel string
}

func signalingCmd() *cobra.Command {
	var opts signalerOptions
	var cmd = &cobra.Command{
		Use:          "signaling [command]",
		SilenceUsage: true,
		Short:        "signaling is a signaling server",
		Long:         `signaling will start a signaling server, signaling server is used to exchange the network information between the clients. which is our core feature.`,

		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			return runSignaling(opts)
		},
	}
	fs := cmd.Flags()
	fs.StringVarP(&opts.Listen, "", "l", "", "http port for drp over http")
	fs.StringVarP(&opts.LogLevel, "log-level", "", "silent", "log level (silent, info, error, warn, verbose)")
	return cmd
}

// run signaling server
func runSignaling(opts signalerOptions) error {
	if opts.LogLevel == "" {
		opts.LogLevel = "error"
	}
	log.Loglevel = log.SetLogLevel(opts.LogLevel)
	return drp.Start(opts.Listen)
}
