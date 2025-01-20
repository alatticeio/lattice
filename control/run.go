package control

import (
	"github.com/spf13/viper"
	"linkany/control/server"
)

func Start(listen string) error {
	viper.SetConfigFile("conf/control.yaml")
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		return err
	}

	var cfg server.ServerConfig
	if err := viper.Unmarshal(&cfg); err != nil {
		return err
	}

	// Create a new server
	s := server.NewServer(&cfg)
	// Start the server
	return s.Start()
}
