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
	"fmt"
	"github.com/alatticeio/lattice/internal/agent"
	"github.com/alatticeio/lattice/internal/agent/config"
	wflog "github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/pkg/utils"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var log = wflog.GetLogger("node")

func upCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "up",
		Short: "Connect this node to a Lattice workspace",
		Long: `Start the Lattice node and establish a WireGuard tunnel to the workspace
identified by the enrollment token. The node will register with the management
server, negotiate peer connections via the signaling server, and apply the
workspace's network policies.

Configuration is read from ~/.lattice/config.yaml. CLI flags override file values.
Use --save to persist the current flags back to the config file.`,
		Example: `  # minimal startup
  lattice up --token <token> --server-url <server-url> --signaling-url <signaling-url>

  # save flags to config file for future runs
  lattice up --token <token> --server-url <server-url> --signaling-url <signaling-url> --save

  # enable the WRRP relay for restrictive NAT environments
  lattice up --token <token> --server-url <server-url> --signaling-url <signaling-url> --enable-wrrp --wrrper-url <wrrp-url>`,
		RunE: func(cmd *cobra.Command, args []string) error {

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			// pre-flight: 严格校验客户端必须配置项（signaling-url / server-url / token）
			if err := config.ValidateAndReport(config.Conf, false); err != nil {
				return err
			}

			// check appId is empty
			if config.Conf.AppId == "" {
				log.Info("no AppId detected, generating one")

				hostName, err := os.Hostname()
				if err != nil {
					return err
				}
				newId := utils.StringFormatter(hostName)

				cfgManager.Viper().Set("app-id", newId)
				config.Conf.AppId = newId

				if err = cfgManager.Viper().WriteConfig(); err != nil {
					return err
				}
				log.Info("AppId saved to config file", "app-id", newId)
			}

			// 1. 检查用户是否传了 --save
			save, _ := cmd.Flags().GetBool("save")
			if save {
				if err := cfgManager.Save(); err != nil {
					return fmt.Errorf("failed to save config: %w", err)
				}
				log.Info("config saved", "path", config.GetConfigFilePath())
			}

			// Propagate flat CLI flags into nested config fields.
			// BindPFlags binds "vm-endpoint" → Viper key "vm-endpoint" (flat),
			// but Unmarshal reads from "telemetry.vmendpoint" (nested), so they
			// never match. Manually forward the value here.
			if ep, _ := cmd.Flags().GetString("vm-endpoint"); ep != "" {
				config.Conf.Telemetry.VMEndpoint = ep
			}
			if em, _ := cmd.Flags().GetBool("enable-metric"); em {
				config.Conf.EnableMetric = em
			}

			return agent.Start(ctx, config.Conf)
		},
	}

	fs := cmd.Flags()
	fs.StringP("token", "", "", "enrollment token to authenticate and join a workspace")
	fs.StringP("level", "", "", "log level: debug, info, warn, error")
	fs.StringP("wrrper-url", "", "", "WRRP relay server URL (required when --enable-wrrp)")
	fs.StringP("wrrp-quic-url", "", "", "QUIC WRRP relay server address (e.g. server:6267)")
	fs.BoolP("enable-wrrp", "", false, "use WRRP relay for NAT traversal")
	fs.StringP("vm-endpoint", "", "", "use to push tele")
	fs.BoolP("enable-metric", "", false, "expose Prometheus metrics endpoint")
	fs.BoolP("enable-sys-log", "", false, "enable verbose WireGuard and ICE debug logging")
	fs.IntP("wg-port", "", 51820, "UDP port for WireGuard and ICE (default 51820)")
	return cmd
}
