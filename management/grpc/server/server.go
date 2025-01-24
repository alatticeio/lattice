package server

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"linkany/management/controller"
	"linkany/management/dto"
	"linkany/management/grpc/mgt"
	"linkany/management/mapper"
	"log"
	"net"
)

// Server is used to implement helloworld.GreeterServer.
type Server struct {
	mgt.UnimplementedManagementServiceServer
	userController *controller.UserController
	peerController *controller.PeerController
	port           int
}

type ServerConfig struct {
	Port            int
	Database        mapper.DatabaseConfig
	DataBaseService *mapper.DatabaseService
}

func NewServer(cfg *ServerConfig) *Server {
	return &Server{
		port:           cfg.Port,
		userController: controller.NewUserController(mapper.NewUserMapper(cfg.DataBaseService)),
		peerController: controller.NewPeerController(mapper.NewPeerMapper(cfg.DataBaseService)),
	}
}

func (s *Server) Login(ctx context.Context, in *mgt.ManagementMessage) (*mgt.ManagementMessage, error) {
	var req mgt.LoginRequest
	if err := proto.Unmarshal(in.Body, &mgt.LoginRequest{}); err != nil {
		return nil, err
	}

	log.Printf("Received username: %s, password: %s", req.Username, req.Password)

	token, err := s.userController.Login(&dto.UserDto{
		Username: req.Username,
		Password: req.Password,
	})

	if err != nil {
		return nil, err
	}

	b, err := proto.Marshal(&mgt.LoginResponse{Token: token.Token})
	if err != nil {
		return nil, err
	}

	return &mgt.ManagementMessage{
		Body: b,
	}, nil
}

// List, will return a list of response
func (s *Server) List(ctx context.Context, in *mgt.ManagementMessage) (*mgt.ManagementMessage, error) {
	var req mgt.LoginRequest
	if err := proto.Unmarshal(in.Body, &mgt.LoginRequest{}); err != nil {
		return nil, err
	}
	user, err := s.userController.Get(req.GetUsername())
	if err != nil {
		return nil, err
	}
	peers, err := s.peerController.List(fmt.Sprintf("%v", user.ID))
	if err != nil {
		return nil, err
	}

	bs, err := json.Marshal(peers)
	if err != nil {
		return nil, err
	}

	return &mgt.ManagementMessage{Body: bs}, nil
}

// ListWatch once request, will return a stream of watched response
func (s *Server) Watch(server mgt.ManagementService_WatchServer) error {
	//TODO implement the logic here
	var err error
	var msg *mgt.ManagementMessage
	msg, err = server.Recv()
	if err != nil {
		return err
	}

	var req mgt.Request
	if err = proto.Unmarshal(msg.Body, &req); err != nil {
		return err
	}

	// create a chan for the peer
	ch := CreateChannel(req.PubKey)
	for {
		select {
		case peer := <-ch:
			bs, err := json.Marshal(peer)
			if err != nil {
				return err
			}

			msg = &mgt.ManagementMessage{Body: bs}
			if err = server.Send(msg); err != nil {
				return err
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
	mgt.RegisterManagementServiceServer(grpcServer, s)
	log.Printf("Server listening at %v", listen.Addr())
	return grpcServer.Serve(listen)
}
