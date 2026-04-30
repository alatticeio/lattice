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

// wrrper is the standalone Lattice relay server.
// It bridges WireGuard peers that cannot establish a direct ICE path
// (e.g. symmetric NAT on both sides) by forwarding encrypted datagrams
// over TCP (HTTP upgrade) and/or QUIC.
package main

import (
	"fmt"
	"github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/relay"
	"os"

	"github.com/spf13/cobra"
)

var cfgManager = config.NewConfigManager()

func main() {
	cmd := &cobra.Command{
		Use:          "wrrper",
		Short:        "LRP relay server for Lattice",
		Long:         `Standalone LRP relay server. Bridges WireGuard peers that cannot reach each other directly.`,
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return cfgManager.LoadConf(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(config.Conf)
		},
	}

	fs := cmd.PersistentFlags()
	fs.StringP("config-dir", "", "", "config directory (default ~/.lattice)")
	fs.StringP("listen", "l", ":6266", "TCP LRP listen address")
	fs.BoolP("enable-tls", "", false, "enable TLS on TCP listener")
	fs.StringP("wrrp-quic-url", "", "", "QUIC LRP listen address (e.g. :6267); empty disables QUIC")
	fs.StringP("level", "", "info", "log level: debug, info, warn, error, silent")

	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func run(flags *config.Config) error {
	log.SetLevel(flags.Level)

	server := relay.NewServer(flags)

	if flags.WrrpQuicURL != "" {
		tlsCfg, err := relay.GenerateSelfSignedTLS()
		if err != nil {
			log.GetLogger("wrrper").Warn("failed to generate TLS cert, QUIC disabled", "err", err)
		} else {
			qs := relay.NewQUICServer(server.Manager())
			go func() {
				if startErr := qs.Start(flags.WrrpQuicURL, tlsCfg); startErr != nil {
					log.GetLogger("wrrper").Error("QUIC server stopped", startErr)
				}
			}()
		}
	}

	return server.Start()
}
