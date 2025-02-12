package management

import (
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
	grpcserver "linkany/management/grpc/server"
	"linkany/management/mapper"
	"linkany/management/server"
	"linkany/pkg/redis"
)

func Start(listen string) error {
	viper.AddConfigPath("/app/")
	viper.AddConfigPath("conf/")
	viper.SetConfigName("control")
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		return err
	}

	var cfg server.ServerConfig
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
	dbService := mapper.NewDatabaseService(&cfg.Database)
	gServer := grpcserver.NewServer(&grpcserver.ServerConfig{
		Port:            32051,
		DataBaseService: dbService,
		Rdb:             redisClient,
	})
	// go run a grpc server
	go func() {
		if err := gServer.Start(); err != nil {
			klog.Errorf("grpc server start failed: %v", err)
		}
	}()

	cfg.DatabaseService = dbService
	// Create a new server
	s := server.NewServer(&cfg)
	// Start the server
	return s.Start()
}
