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
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/alatticeio/lattice/internal/agent/config"
	"github.com/spf13/cobra"
)

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Interactively configure Lattice and save to config file",
		Long: `Prompt for required connection parameters and save them to
~/.lattice/lattice.yaml. After init, run "lattice up" with no flags.`,
		Example: `  lattice init
  lattice init --config-dir /etc/lattice`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd)
		},
	}
}

func runInit(cmd *cobra.Command) error {
	cfgPath := config.GetConfigFilePath()
	scanner := bufio.NewScanner(os.Stdin)

	// 如果配置文件已存在，询问是否覆盖
	if _, err := os.Stat(cfgPath); err == nil {
		fmt.Printf("Config file already exists at %s\n", cfgPath)
		fmt.Print("Overwrite existing config? [y/N]: ")
		scanner.Scan()
		answer := strings.TrimSpace(scanner.Text())
		if !strings.EqualFold(answer, "y") {
			fmt.Println("Aborted. Existing config unchanged.")
			return nil
		}
	}

	v := cfgManager.Viper()

	// 必填项
	serverURL := prompt(scanner, "Management server URL (--server-url)", v.GetString("server-url"))
	signalingURL := prompt(scanner, "Signaling server URL (--signaling-url)", v.GetString("signaling-url"))
	token := prompt(scanner, "Enrollment token (--token)", v.GetString("token"))

	// 可选项
	relayURL := promptOptional(scanner, "Relay TCP URL (--relay-url, optional)")
	relayQuicURL := promptOptional(scanner, "Relay QUIC URL (--relay-quic-url, optional)")

	// 写入 Viper 并保存
	v.Set("server-url", serverURL)
	v.Set("signaling-url", signalingURL)
	v.Set("token", token)
	if relayURL != "" {
		v.Set("relay-url", relayURL)
	}
	if relayQuicURL != "" {
		v.Set("relay-quic-url", relayQuicURL)
	}

	if err := cfgManager.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("\nConfig saved to %s\n", cfgPath)
	fmt.Println(`Run "lattice up" to connect.`)
	return nil
}

// prompt 打印提示并读取输入；若用户直接回车则返回 defaultVal。
func prompt(scanner *bufio.Scanner, label, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("? %s [%s]: ", label, defaultVal)
	} else {
		fmt.Printf("? %s: ", label)
	}
	scanner.Scan()
	val := strings.TrimSpace(scanner.Text())
	if val == "" {
		return defaultVal
	}
	return val
}

// promptOptional 打印可选提示；用户直接回车返回空字符串。
func promptOptional(scanner *bufio.Scanner, label string) string {
	fmt.Printf("? %s (press Enter to skip): ", label)
	scanner.Scan()
	return strings.TrimSpace(scanner.Text())
}
