package management

import (
	grpcserver "wireflow/management/grpc"

	"github.com/spf13/viper"
	"github.com/wireflowio/wireflow-controller/pkg/signals"
	"k8s.io/klog/v2"

	"wireflow/management/http"
	"wireflow/pkg/log"
)

func Start(listen string) error {
	logger := log.NewLogger(log.Loglevel, "management")
	viper.AddConfigPath("/app/")
	viper.AddConfigPath("deploy/")
	viper.SetConfigName("control")
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		return err
	}

	var cfg http.ServerConfig
	if err := viper.Unmarshal(&cfg); err != nil {
		return err
	}

	//redisClient, err := redis.NewClient(&redis.ClientConfig{
	//	Addr:     viper.GetString("redis.addr"),
	//	Password: viper.GetString("redis.password"),
	//})

	//if err != nil {
	//	return err
	//}

	//cfg.Rdb = redisClient
	//dbService := db.GetDB(&cfg.Database)
	ctx := signals.SetupSignalHandler()
	gServer := grpcserver.NewServer(&grpcserver.ServerConfig{
		Ctx:    ctx,
		Logger: logger,
		Port:   32051,
	})
	// go run a grpc server
	go func() {
		if err := gServer.Start(); err != nil {
			logger.Errorf("grpc server start failed: %v", err)
		}
	}()

	go func() {
		http.NewPush()
	}()

	//cfg.DatabaseService = dbService
	// Create a new server
	//s := http.NewServer(&cfg)
	// Start the server
	//return s.Start()
	klog.Info("grpc server start successfully")
	<-ctx.Done()

	return nil
}
