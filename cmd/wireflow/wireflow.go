// Copyright 2025 wireflowio.com, Inc.
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

package main

import (
	"wireflow/client"
	"wireflow/pkg/log"

	"github.com/spf13/cobra"
)

func NewClientCmd() *cobra.Command {
	var flags client.Flags
	cmd := &cobra.Command{
		Short:        "client",
		Use:          "client [command]",
		SilenceUsage: true,
		Long:         `wireflow startup, will create a wireguard interface and join your wireflow network,and also will config the interface automatically`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWireflow(&flags)
		},
	}

	fs := cmd.Flags()
	fs.StringVarP(&flags.InterfaceName, "interface-name", "u", "", "name which create interface use")
	fs.BoolVarP(&flags.ForceRelay, "force-relay", "f", false, "force relay mode")
	fs.StringVarP(&flags.LogLevel, "log-level", "l", "silent", "log level (silent, info, error, warn, verbose)")
	fs.StringVarP(&flags.ManagementUrl, "control-url", "", "", "management server url, need not give when you are using our service")
	fs.StringVarP(&flags.TurnServerUrl, "turn-url", "", "", "just need modify when you custom your own relay server")
	fs.StringVarP(&flags.SignalingUrl, "", "", "", "signaling service, not need to modify")
	fs.BoolVarP(&flags.DaemonGround, "daemon", "d", false, "run in daemon mode, default is forground mode")
	fs.BoolVarP(&flags.MetricsEnable, "metrics", "m", false, "enable metrics")
	fs.BoolVarP(&flags.DnsEnable, "dns", "", false, "enable dns")

	return cmd
}

func runWireflow(flags *client.Flags) error {
	if flags.LogLevel == "" {
		flags.LogLevel = "error"
	}
	log.SetLogLevel(flags.LogLevel)
	return client.Start(flags)
}
