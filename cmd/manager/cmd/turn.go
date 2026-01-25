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

package cmd

import (
	"context"
	"wireflow/internal/config"
	"wireflow/internal/log"
	"wireflow/management/client"
	"wireflow/management/nats"
	"wireflow/turn"

	"github.com/spf13/cobra"
)

func newTurnCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:          "turn",
		SilenceUsage: true,
		Short:        "start a turn server",
		Long:         `start a turn serer will provided stun service for you, you can use it to get public IP and port, also you can deploy you own turn server when direct(P2P) unavailable.`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			return runTurn()
		},
	}
	fs := cmd.Flags()
	fs.StringP("public-ip", "u", "", "public ip for turn")
	fs.IntP("port", "p", 3478, "port for turn")
	fs.StringP("level", "", "silent", "log level (debug, info, warn, error)")
	return cmd
}

func runTurn() error {
	signalService, err := nats.NewNatsService(context.Background(), config.Conf.SignalingURL)
	if err != nil {
		return err
	}
	client, err := client.NewClient(signalService)
	if err != nil {
		return err
	}

	log.SetLevel(config.Conf.Level)
	return turn.Start(&turn.TurnServerConfig{
		Logger:   log.GetLogger("turnserver"),
		PublicIP: config.Conf.PublicIP,
		Port:     config.Conf.Port,
		Client:   client,
	})
}
