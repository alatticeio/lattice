package server

import (
	"context"
	"encoding/json"
	"fmt"
	"google.golang.org/grpc"
	"linkany/control/controller"
	pb "linkany/control/grpc/peer"
	"linkany/control/mapper"
	"log"
	"net"
)

// Server is used to implement helloworld.GreeterServer.
type Server struct {
	pb.UnimplementedListWatcherServer
	userController *controller.UserController
	peerController *controller.PeerController
	port           int
	queue          chan *pb.WatchResponse
}

type ServerConfig struct {
	Port            int
	DataBaseService *mapper.DatabaseService
	Queue           chan *pb.WatchResponse
}

func NewServer(cfg *ServerConfig) *Server {
	return &Server{
		port:           cfg.Port,
		userController: controller.NewUserController(mapper.NewUserMapper(cfg.DataBaseService)),
		peerController: controller.NewPeerController(mapper.NewPeerMapper(cfg.DataBaseService)),
		queue:          cfg.Queue,
	}
}

// List, will return a list of response
func (s *Server) List(ctx context.Context, in *pb.Request) (*pb.ListResponse, error) {
	log.Printf("Received username: %v, appId; %v", in.GetUsername(), in.GetAppId())
	user, err := s.userController.Get(in.GetUsername())
	if err != nil {
		return nil, err
	}
	peers, err := s.peerController.List(fmt.Sprintf("%v", user.ID))
	if err != nil {
		return nil, err
	}

	var result []*pb.Peer
	for _, peer := range peers {
		b, err := json.Marshal(peer)
		if err != nil {
			return nil, err
		}

		result = append(result, &pb.Peer{Message: string(b)})
	}

	return &pb.ListResponse{Peer: result}, nil
}

// ListWatch once request, will return a stream of watched response
func (s *Server) Watch(in *pb.Request, stream pb.ListWatcher_WatchServer) error {
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
