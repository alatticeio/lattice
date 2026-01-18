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

package config

import (
	"fmt"
	"os"
	"sync"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cm   *ConfigManager
	once sync.Once
	Conf *Flags
)

type ConfigManager struct {
	v *viper.Viper
}

// NewConfigManager return ConfigManager instance.
func NewConfigManager() *ConfigManager {
	once.Do(func() {
		cm = &ConfigManager{v: viper.New()}
	})
	return cm
}

// Viper return viper instance.
func (cm *ConfigManager) Viper() *viper.Viper {
	return cm.v
}

func (cm *ConfigManager) LoadConf(cmd *cobra.Command) error {
	// 1. 设置配置文件名和类型
	v := cm.v
	v.SetConfigType("yaml")

	// 2. 设置默认值 (这些值将成为初始配置文件的内容)
	v.SetDefault("LogLevel", "info")
	v.SetDefault("ManagementUrl", "http://wireflow.run")
	v.SetDefault("SignalingURL", "nats://signaling.wireflow.run:4222")
	v.SetDefault("TurnServerURL", "stun.wireflow.run:3478")

	configName := GetConfigFilePath()
	v.SetConfigFile(configName)
	// 3. 检查文件是否存在，不存在则创建
	if _, err := os.Stat(configName); os.IsNotExist(err) {
		fmt.Printf("未找到配置文件，正在创建默认配置: %s\n", configName)
		// SafeWriteConfig 会按照当前的默认值创建一个新文件
		// 如果文件已存在则会报错（所以配合 os.IsNotExist 很安全）
		if err := v.SafeWriteConfigAs(configName); err != nil {
			return fmt.Errorf("创建配置文件失败: %v", err)
		}
	}

	// 2. 尝试读取配置文件
	if err := v.ReadInConfig(); err != nil {
		// 如果没找到配置文件，可以忽略，继续使用默认值或 Flag
	}

	// 3. 开启环境变量支持
	v.SetEnvPrefix("WIREFLOW")
	v.AutomaticEnv()

	// 4. 【高阶技巧】一次性绑定所有 Flag
	// 这样你就不需要在 init 里手动一个个 BindPFlag 了
	if err := v.BindPFlags(cmd.Flags()); err != nil {
		return err
	}

	return v.Unmarshal(&Conf)
}

// GetConfigFilePath get config filepath.
func GetConfigFilePath() string {
	// 1. first get from env
	if path := os.Getenv("WIREFLOW_CONFIG_DIR"); path != "" {
		return path + "/.wireflow.yaml"
	}
	// 2. if home == '/' return etc
	home, _ := os.UserHomeDir()
	if home == "/" {
		return "/etc/wireflow/.wireflow.yaml"
	}
	// 3. using home dir
	return home + "/.wireflow.yaml"
}

// Flags is a struct that contains the flags that are passed to the mgtClient.
type Flags struct {
	LogLevel      string `mapstructure:"log-level,omitempty"`
	InterfaceName string `mapstructure:"interface-name,omitempty"`
	Auth          string `mapstructure:"auth,omitempty"`
	AppId         string `mapstructure:"app-id,omitempty"`
	Debug         bool   `mapstructure:"debug,omitempty"`
	Token         string `mapstructure:"token,omitempty"`
	SignalingURL  string `mapstructure:"signaling-url,omitempty"`
	ServerUrl     string `mapstructure:"server-url,omitempty"`
	TurnServerURL string `mapstructure:"stun-url,omitempty"`
	ShowSystemLog bool   `mapstructure:"show-system-log,omitempty"`
	DaemonGround  bool   `mapstructure:"daemon-ground,omitempty"`
	MetricsEnable bool   `mapstructure:"metrics-enable,omitempty"`
	DnsEnable     bool   `mapstructure:"dns-enable,omitempty"`

	// for controller
	MetricsAddr          string `mapstructure:"metrics-addr,omitempty"`
	WebhookCertPath      string `mapstructure:"webhook-cert-path,omitempty"`
	WebhookCertName      string `mapstructure:"webhook-cert-name,omitempty"`
	WebhookCertKey       string `mapstructure:"webhook-cert-key,omitempty"`
	MetricsCertPath      string `mapstructure:"metrics-cert-path,omitempty"`
	MetricsCertName      string `mapstructure:"metrics-cert-name,omitempty"`
	MetricsCertKey       string `mapstructure:"metrics-cert-key,omitempty"`
	EnableLeaderElection bool   `mapstructure:"leader-elect,omitempty"`
	ProbeAddr            string `mapstructure:"probe-addr,omitempty"`
	SecureMetrics        bool   `mapstructure:"metrics-secure,omitempty"`
	EnableHTTP2          bool   `mapstructure:"enable-http2,omitempty"`
	Listen               string `mapstructure:"listen,omitempty"`
}

// NetworkOptions for network operations.
type NetworkOptions struct {
	AppId      string
	Identifier string
	Name       string
	CIDR       string
	ServerUrl  string
}

// config/manager.go

// Save 将当前内存中的配置（包括 Flag、Env 和手动 Set 的值）写入文件
func (cm *ConfigManager) Save() error {
	path := GetConfigFilePath()

	// WriteConfig 会覆盖当前指定的配置文件
	if err := cm.v.WriteConfig(); err != nil {
		// 如果文件不存在，WriteConfig 会报错，此时可以使用 WriteConfigAs
		return cm.v.WriteConfigAs(path)
	}
	return nil
}
