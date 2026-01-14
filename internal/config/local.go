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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

type Local struct {
	file *os.File
}

func GetConfigFilePath() string {
	home, _ := os.UserHomeDir()
	configPath := filepath.Join(home, ".wireflow.yaml") // 拼接完整路径
	return configPath
}

type Config struct {
	Auth       string `mapstructure:"auth,omitempty"`
	AppId      string `mapstructure:"app-id,omitempty"`
	Debug      bool   `mapstructure:"debug,omitempty"`
	Token      string `mapstructure:"token,omitempty"`
	SignalUrl  string `mapstructure:"signaling-url,omitempty"`
	ServerUrl  string `mapstructure:"server-url,omitempty"`
	PrivateKey string `mapstructure:"private-key,omitempty"`
	StunUrl    string `mapstructure:"stun-url,omitempty"`
}

var GlobalConfig *Config

func init() {
	var err error
	viper.SetConfigName(".wireflow") // 文件名（不含后缀）
	viper.SetConfigType("yaml")      // 预期的后缀

	viper.AddConfigPath("$HOME")                // 优先级 1
	viper.AddConfigPath(".")                    // 优先级 2
	if err = viper.ReadInConfig(); err != nil { // Handle errors reading the config file
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if errors.As(err, &configFileNotFoundError) {
			// using default configuration
		}
	}

	if err = viper.UnmarshalExact(&GlobalConfig); err != nil {
		panic(err)
	}

}

func WriteConfig(key, value string) error {
	if viper.ConfigFileUsed() == "" {
		viper.SetConfigFile(GetConfigFilePath())
	}
	// 将配置写入 viper 内存并持久化到文件
	viper.Set(key, value)
	err := viper.WriteConfig()
	if err != nil {
		// 如果文件不存在，则创建新文件
		err = viper.SafeWriteConfig()
	}

	if err != nil {
		fmt.Printf(" >> 保存配置失败: %v\n", err)
		return err
	}
	fmt.Printf(" >> 配置已更新: %s = %s\n", key, value)
	return nil
}

//
//func GetLocalConfig() (*Config, error) {
//	local, err := getLocal(os.O_RDWR)
//	defer local.Close()
//	if err != nil {
//		return nil, err
//	}
//	return local.ReadFile()
//}
//
//// UpdateLocalConfig update json file
//func UpdateLocalConfig(newCfg *Config) error {
//	local, err := getLocal(os.O_RDWR | os.O_CREATE)
//	defer local.Close()
//	if err != nil {
//		return err
//	}
//	defer local.Close()
//
//	err = local.WriteFile(newCfg)
//	if err != nil {
//		return err
//	}
//
//	return nil
//}
//
//// ReplaceLocalConfig update json file
//func ReplaceLocalConfig(newCfg *Config) error {
//	local, err := getLocal(os.O_RDWR | os.O_TRUNC)
//	defer local.Close()
//	if err != nil {
//		return err
//	}
//	defer local.Close()
//
//	err = local.WriteFile(newCfg)
//	if err != nil {
//		return err
//	}
//
//	return nil
//}
//
//func GetLocalUserInfo() (info *LocalInfo, err error) {
//	localCfg, err := GetLocalConfig()
//	if err != nil {
//		return nil, err
//	}
//
//	if localCfg.Auth == "" {
//		return nil, errors.New("please login first")
//	}
//	info = new(LocalInfo)
//	values := strings.Split(localCfg.Auth, ":")
//	info.Username = values[0]
//	info.Password, err = Base64Decode(values[1])
//	info.UserId = localCfg.UserId
//	if err != nil {
//		return nil, err
//	}
//
//	return info, nil
//}

func DecodeAuth(auth string) (string, string, error) {
	if auth == "" {
		return "", "", errors.New("auth is empty")
	}
	values := strings.Split(auth, ":")
	username := values[0]
	password, err := Base64Decode(values[1])
	if err != nil {
		return "", "", err
	}

	return username, password, nil

}
