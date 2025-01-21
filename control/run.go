package control

import (
	"github.com/spf13/viper"
	pb "linkany/control/grpc/peer"
	grpcserver "linkany/control/grpc/server"
	"linkany/control/server"
	"log"
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
	queue := make(chan *pb.WatchResponse)
	cfg.Queue = queue

	gServer := grpcserver.NewServer(&grpcserver.ServerConfig{
		Port:  50015,
		Queue: queue,
	})
	// go run a grpc server
	go func() {
		if err := gServer.Start(); err != nil {
			log.Fatal(err)
		}
	}()

	// Create a new server
	s := server.NewServer(&cfg)
	// Start the server
	return s.Start()
}
