package management

import (
	"github.com/spf13/viper"
	"linkany/management/db"
	grpcserver "linkany/management/grpc/server"
	"linkany/management/http"
	"linkany/pkg/log"
	"linkany/pkg/redis"
)

func Start(listen string) error {
	logger := log.NewLogger(log.Loglevel, "management")
	viper.AddConfigPath("/app/")
	viper.AddConfigPath("conf/")
	viper.SetConfigName("control")
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		return err
	}

	var cfg http.ServerConfig
	if err := viper.Unmarshal(&cfg); err != nil {
		return err
	}

	redisClient, err := redis.NewClient(&redis.ClientConfig{
		Addr:     viper.GetString("redis.addr"),
		Password: viper.GetString("redis.password"),
	})

	if err != nil {
		return err
	}

	cfg.Rdb = redisClient
	dbService := db.GetDB(&cfg.Database)
	gServer := grpcserver.NewServer(&grpcserver.ServerConfig{
		Logger:          logger,
		Port:            32051,
		DataBaseService: dbService,
		Rdb:             redisClient,
	})
	// go run a grpc server
	go func() {
		if err := gServer.Start(); err != nil {
			logger.Errorf("grpc server start failed: %v", err)
		}
	}()

	cfg.DatabaseService = dbService
	// Create a new server
	s := http.NewServer(&cfg)
	// Start the server
	return s.Start()
}
