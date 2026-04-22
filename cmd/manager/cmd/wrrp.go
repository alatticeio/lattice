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
	"wireflow/internal/config"
	"wireflow/internal/log"
	"wireflow/wrrper"

	"github.com/spf13/cobra"
)

func newWrrpCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:          "wrrper",
		SilenceUsage: true,
		Short:        "wrrp using as relay server for wireflow",
		Long:         `wrrp using as relay server for wireflow`,

		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			return runWrrp(config.Conf)
		},
	}
	fs := cmd.Flags()
	fs.StringP("listen", "l", "", "port for wrrp server")
	fs.BoolP("enable-tls", "", false, "using tls")
	fs.StringP("level", "", "silent", "log level (debug, info, warn, error)")
	fs.StringP("wrrp-quic-url", "", "", "QUIC WRRP relay server address (e.g. :6267)")
	return cmd
}

// run signaling server
func runWrrp(flags *config.Config) error {
	log.SetLevel(flags.Level)
	server := wrrper.NewServer(flags)

	if flags.WrrpQuicURL != "" {
		tlsCfg, err := wrrper.GenerateSelfSignedTLS()
		if err != nil {
			log.GetLogger("wrrp").Warn("failed to generate self-signed TLS, skipping QUIC", "err", err)
		} else {
			qs := wrrper.NewQUICServer(server.Manager())
			go func() {
				if err := qs.Start(flags.WrrpQuicURL, tlsCfg); err != nil {
					log.GetLogger("wrrp").Error("QUIC server error", err)
				}
			}()
		}
	}

	return server.Start()
}
