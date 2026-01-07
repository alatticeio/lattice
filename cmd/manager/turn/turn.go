// Copyright 2025 The Wireflow Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package turn

import (
	"fmt"
	"wireflow/internal/config"
	"wireflow/internal/core/infra"
	"wireflow/internal/log"
	"wireflow/management/client"
	"wireflow/management/nats"
	"wireflow/turn"

	"github.com/spf13/cobra"
)

type turnOptions struct {
	PublicIP string
	Port     int
	LogLevel string
}

func NewTurnCmd() *cobra.Command {
	var opts turnOptions
	var cmd = &cobra.Command{
		Use:          "turn",
		SilenceUsage: true,
		Short:        "start a turn server",
		Long:         `start a turn serer will provided stun service for you, you can use it to get public IP and port, also you can deploy you own turn server when direct(P2P) unavailable.`,
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
	fs.StringVarP(&opts.LogLevel, "log-level", "", "silent", "log level (silent, info, error, warn, verbose)")
	return cmd
}

func runTurn(opts turnOptions) error {
	if opts.LogLevel == "" {
		opts.LogLevel = "error"
	}

	log.SetLevel(opts.LogLevel)
	if config.GlobalConfig.SignalUrl == "" {
		config.GlobalConfig.SignalUrl = fmt.Sprintf("nats://%s:%d", infra.SignalingDomain, infra.DefaultSignalingPort)
		config.WriteConfig("signal-url", config.GlobalConfig.SignalUrl)
	}
	signalService, err := nats.NewNatsService(config.GlobalConfig.SignalUrl)
	if err != nil {
		return err
	}
	client, err := client.NewClient(signalService, nil)
	if err != nil {
		return err
	}

	return turn.Start(&turn.TurnServerConfig{
		Logger:   log.GetLogger("turnserver"),
		PublicIP: opts.PublicIP,
		Port:     opts.Port,
		Client:   client,
	})
}
