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
	"wireflow/internal/config"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// configCmd 代表 config 顶层命令
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "管理 wfctl 的本地配置",
}

// setCmd 用于设置具体的配置项
var setCmd = &cobra.Command{
	Use:     "set <key> <value>",
	Short:   "设置配置项的值",
	Example: "  wfctl config set server-url http://wireflowio.com",
	Args:    cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]
		value := args[1]

		// 将配置写入 viper 内存并持久化到文件
		viper.Set(key, value)
		err := viper.WriteConfig()
		if err != nil {
			// 如果文件不存在，则创建新文件
			err = viper.SafeWriteConfig()
		}

		if err != nil {
			fmt.Printf(" >> 保存配置失败: %v\n", err)
			return
		}
		fmt.Printf(" >> 配置已更新: %s = %s\n", key, value)
	},
}

// getCmd 用于查看当前配置
var getCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "查看配置项的值",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]
		value := viper.GetString(key)
		if value == "" {
			fmt.Printf(" >> 未找到配置项: %s\n", key)
		} else {
			fmt.Printf(" >> %s: %s\n", key, value)
		}
	},
}

func init() {
	viper.SetConfigFile(config.GetConfigFilePath())
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(setCmd)
	configCmd.AddCommand(getCmd)
}
