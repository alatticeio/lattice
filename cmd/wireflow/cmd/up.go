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
	"wireflow/agent"
	"wireflow/internal/config"
	"wireflow/pkg/utils"

	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
)

func upCmd() *cobra.Command {
	// upCmd 代表 config 顶层命令
	var cmd = &cobra.Command{
		Use:     "up",
		Short:   "wireflow startup command",
		Example: "wireflow up --token <token> --server-url <server-url> --signaling-url <signaling-url> --wrrp-url <wrrp-url>",
		RunE: func(cmd *cobra.Command, args []string) error {
			// check appId is empty
			if config.Conf.AppId == "" {
				fmt.Println("未检测到 AppId，正在生成...")

				// create appId
				newId := utils.GenerateAppId() // 你的生成逻辑

				// set appId
				cfgManager.Viper().Set("app-id", newId) // 注入 Viper，确保 WriteConfig 时能写进文件
				config.Conf.AppId = newId               // 同步到内存结构体，方便本次运行后续逻辑使用

				// save appId
				err := cfgManager.Viper().WriteConfig()
				if err != nil {
					fmt.Printf("cann't save appId: %v\n", err)
					return err
				} else {
					fmt.Println("save appId to file successfully")
				}
			}

			// 1. 检查用户是否传了 --save
			save, _ := cmd.Flags().GetBool("save")
			if save {
				// 2. 执行保存
				fmt.Println("Saving configuration...")

				if err := cfgManager.Save(); err != nil {
					return fmt.Errorf("failed to save config: %w", err)
				}

				fmt.Printf("Config saved to: %s\n", config.GetConfigFilePath())
			}

			ctx := signals.SetupSignalHandler()

			return agent.Start(ctx, config.Conf)
		},
	}

	fs := cmd.Flags()
	fs.StringP("token", "", "", "token using for creating or joining network")
	fs.StringP("level", "", "", "log level (debug, info, warn, error)")
	return cmd
}
