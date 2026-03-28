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
	"fmt"
	"os"
	"wireflow/internal/config"
	"wireflow/internal/log"
	"wireflow/management"

	"github.com/spf13/cobra"
)

func newManagementCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:          "manager [command]",
		SilenceUsage: true,
		Short:        "manager is control server",
		Long:         `manager used for starting management server, management providing our all control plane features.`,

		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			return runManagement(config.Conf)
		},
	}
	fs := cmd.Flags()
	fs.StringP("listen", "l", "", "management server listen address")
	fs.StringP("level", "", "silent", "log level (silent, info, error, warn, verbose)")
	fs.StringP("env", "", "dev", "run environment (dev, pre-run, production) ")
	return cmd
}

// run drp
func runManagement(flags *config.Config) error {
	log.SetLevel(flags.Level)
	// pre-flight: 仅在 signaling-url 为空时打印警告（management 可降级运行，但功能受限）
	if flags.SignalingURL == "" {
		fmt.Fprintln(os.Stderr, "[pre-flight] 警告: signaling-url 未配置，NATS 信令服务将禁用，Agent 将无法接收 WireGuard 对端更新")
	}
	return management.Start(flags)
}
