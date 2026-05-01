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
	"github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/relay"

	"github.com/spf13/cobra"
)

func newWrrpCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:          "wrrper",
		SilenceUsage: true,
		Short:        "wrrp using as relay server for lattice",
		Long:         `wrrp using as relay server for lattice`,

		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Map renamed server flags to their viper keys before config loading.
			// PersistentPreRunE overrides parent's, so we call LoadConf here.
			_ = cfgManager.Viper().BindPFlag("listen", cmd.Flags().Lookup("addr"))
			_ = cfgManager.Viper().BindPFlag("relay-quic-url", cmd.Flags().Lookup("quic-addr"))
			return cfgManager.LoadConf(cmd)
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			return runWrrp(config.Conf)
		},
	}
	fs := cmd.Flags()
	fs.StringP("addr", "l", "", "TCP relay listen address")
	fs.BoolP("enable-tls", "", false, "using tls")
	fs.StringP("level", "", "silent", "log level (debug, info, warn, error)")
	fs.StringP("quic-addr", "", "", "QUIC relay listen address (e.g. :6267)")
	return cmd
}

// run signaling server
func runWrrp(flags *config.Config) error {
	log.SetLevel(flags.Level)
	server := relay.NewServer(flags)

	if flags.RelayQuicURL != "" {
		tlsCfg, err := relay.GenerateSelfSignedTLS()
		if err != nil {
			log.GetLogger("wrrp").Warn("failed to generate self-signed TLS, skipping QUIC", "err", err)
		} else {
			qs := relay.NewQUICServer(server.Manager())
			go func() {
				if err := qs.Start(flags.RelayQuicURL, tlsCfg); err != nil {
					log.GetLogger("wrrp").Error("QUIC server error", err)
				}
			}()
		}
	}

	return server.Start()
}
