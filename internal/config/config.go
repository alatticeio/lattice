package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	App      AppConfig      `mapstructure:"app"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Database DatabaseConfig `mapstructure:"database"`
}

type AppConfig struct {
	Listen     string        `mapstructure:"listen"`
	Name       string        `mapstructure:"name"`
	InitAdmins []AdminConfig `mapstructure:"init_admins"`
}

type AdminConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type JWTConfig struct {
	Secret      string `mapstructure:"secret"`
	ExpireHours int    `mapstructure:"expire_hours"`
}

type DatabaseConfig struct {
	Driver string `mapstructure:"driver"`
	DSN    string `mapstructure:"dsn"`
}

var (
	GlobalConfig *Config
)

// InitConfig 加载配置文件
func InitConfig(configPath string) *Config {
	serverOnce.Do(func() {
		viper.SetConfigFile(configPath) // 指定配置文件路径
		viper.SetConfigType("yaml")     // 配置文件类型

		// 读取环境变量，支持 APP_INIT_ADMINS 这种格式覆盖文件配置
		viper.AutomaticEnv()

		if err := viper.ReadInConfig(); err != nil {
			panic(fmt.Errorf("读取配置文件失败: %w", err))
		}

		if err := viper.Unmarshal(&GlobalConfig); err != nil {
			panic(fmt.Errorf("解析配置文件失败: %w", err))
		}
	})
	return GlobalConfig
}
