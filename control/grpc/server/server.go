package server

import (
	"fmt"
	"google.golang.org/grpc"
	pb "linkany/control/grpc/peer"
	"linkany/control/mapper"
	"log"
	"net"
)

// Server is used to implement helloworld.GreeterServer.
type Server struct {
	pb.UnimplementedListWatcherServer
	userMapper mapper.UserInterface
	port       int
	queue      chan *pb.WatchResponse
}

type ServerConfig struct {
	Port            int
	DataBaseService *mapper.DatabaseService
	Queue           chan *pb.WatchResponse
}

func NewServer(cfg *ServerConfig) *Server {
	return &Server{
		port:       cfg.Port,
		userMapper: mapper.NewUserMapper(cfg.DataBaseService),
		queue:      cfg.Queue,
	}
}

// ListWatch once request, will return a stream of watched response
func (s *Server) Watch(in *pb.WatchRequest, stream pb.ListWatcher_WatchServer) error {
	log.Printf("Received username: %v, appId; %v", in.GetUsername(), in.GetAppId())
	//TODO implement the logic here

	for {
		select {
		case msg := <-s.queue:
			if err := stream.Send(msg); err != nil {
				log.Fatal(err)
			}
		}
	}

	return nil
}

func (s *Server) Start() error {
	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		return err
	}
	grpcServer := grpc.NewServer()
	pb.RegisterListWatcherServer(grpcServer, s)
	log.Printf("Server listening at %v", listen.Addr())
	return grpcServer.Serve(listen)
}
