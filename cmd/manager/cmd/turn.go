// Copyright 2026 The Lattice Authors, Inc.
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
	"github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/relay"

	"github.com/spf13/cobra"
)

func newTurnCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "turn",
		SilenceUsage: true,
		Short:        "start a turn server",
		Long:         `Start a TURN server that provides relay transport when direct (P2P) connections are unavailable.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTurn(cmd.Context())
		},
	}
	fs := cmd.Flags()
	fs.StringP("public-ip", "u", "", "public ip for turn")
	fs.IntP("port", "p", 3478, "port for turn")
	fs.StringP("level", "", "silent", "log level (debug, info, warn, error)")
	return cmd
}

func runTurn(ctx context.Context) error {
	log.SetLevel(config.Conf.Level)

	var users []*config.User
	for _, a := range config.Conf.App.InitAdmins {
		users = append(users, config.NewUser(a.Username, a.Password))
	}

	return relay.NewTurnServer(&relay.TurnServerConfig{
		Logger:   log.GetLogger("turnserver"),
		PublicIP: config.Conf.PublicIP,
		Port:     config.Conf.Port,
		Users:    users,
	}).Start(ctx)
}
