package server

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"linkany/management/controller"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/grpc/mgt"
	"linkany/management/mapper"
	"linkany/management/utils"
	"log"
	"net"
	"strconv"
)

// Server is used to implement helloworld.GreeterServer.
type Server struct {
	mgt.UnimplementedManagementServiceServer
	userController *controller.UserController
	peerController *controller.PeerController
	port           int
	tokenr         *utils.Tokener
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
	if err := proto.Unmarshal(in.Body, &req); err != nil {
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

// List will return a list of response
func (s *Server) List(ctx context.Context, in *mgt.ManagementMessage) (*mgt.ManagementMessage, error) {
	var req mgt.Request
	if err := proto.Unmarshal(in.Body, &req); err != nil {
		return nil, err
	}
	user, err := s.userController.Get(req.GetToken())
	if err != nil {
		return nil, err
	}
	log.Println(user)
	peers, err := s.peerController.GetNetworkMap(req.AppId, strconv.Itoa(int(user.ID)))
	if err != nil {
		return nil, err
	}

	bs, err := json.Marshal(peers)
	if err != nil {
		return nil, err
	}

	return &mgt.ManagementMessage{Body: bs}, nil
}

// Watch once request, will return a stream of watched response
func (s *Server) Watch(server mgt.ManagementService_WatchServer) error {
	//TODO implement the logic here
	var err error
	var msg *mgt.ManagementMessage
	for {
		msg, err = server.Recv()
		if err != nil {
			return err
		}

		var req mgt.Request
		if err = proto.Unmarshal(msg.Body, &req); err != nil {
			return err
		}

		// create a chan for the peer
		watchChannel := CreateChannel(req.PubKey)

		go func() {
			handleMessage(watchChannel, req, server)
		}()
	}

	return nil
}

func handleMessage(watchChannel chan *utils.WatchMessage, req mgt.Request, server mgt.ManagementService_WatchServer) error {
	for {
		select {
		case wc := <-watchChannel:
			bs, err := proto.Marshal(wc.Peer)
			if err != nil {
				return err
			}

			msg := &mgt.ManagementMessage{PubKey: req.PubKey, Body: bs}
			if err = server.Send(msg); err != nil {
				return err
			}
		}
	}
}

// Keepalive acts as a client is living, if 10s not receive the heartbeat, the client will set to offline,
// otherwise set to online.
// if recvice 3 times, will notify add event for peers.
func (s *Server) Keepalive(server mgt.ManagementService_KeepaliveServer) error {
	var err error
	var msg *mgt.ManagementMessage
	var pubKey string
	var count int
	var userId string

	msg, err = server.Recv()
	var req mgt.Request
	if err = proto.Unmarshal(msg.Body, &req); err != nil {
		return err
	}
	pubKey = req.PubKey

	user, err := s.tokenr.Parse(req.Token)
	if err != nil {
		log.Fatalf("invalid token")
		return err
	}

	userId = fmt.Sprintf("%v", user.ID)
	// record
	var wc chan *utils.WatchMessage
	wc = utils.NewWatchManager().Get(pubKey)
	if wc == nil {
		return fmt.Errorf("fatal error, peer has not connected to managent server")
	}

	currentPeer := &mgt.Peer{
		PublicKey: pubKey,
	}

	var online = 1
	var peers []*entity.Peer

	onlineChannel := make(chan interface{})
	close(onlineChannel)
	for {
		msg, err = server.Recv()
		if err != nil {
			log.Fatalf("peer %s connected broken, notify user's clients remove this peer", pubKey)
			peers, err = s.peerController.List(&mapper.QueryParams{
				PubKey: &pubKey,
				UserId: &userId,
				Online: &online,
			})

			if err != nil {
				log.Fatalf("list peers failed: %v", err)
			}

			s.handleKeepalive(mgt.DeleteEvent, currentPeer, peers)

			return err
		}

		var req mgt.Request
		if err = proto.Unmarshal(msg.Body, &req); err != nil {
			close(onlineChannel)
			return err
		}

		if count == 3 {
			peers, err = s.peerController.List(&mapper.QueryParams{
				PubKey: &pubKey,
				UserId: &userId,
				Online: &online,
			})

			if err != nil {
				close(onlineChannel)
				log.Fatalf("list peers failed: %v", err)
			}

			s.handleKeepalive(mgt.AddEvent, currentPeer, peers)
		}

		count++

		go func() {
			for {
				select {
				case <-onlineChannel:
					// offline
					dto := &dto.PeerDto{PubKey: pubKey, Online: 0}
					s.peerController.Update(dto)
				}
			}
		}()
		return nil
	}
}

func (s *Server) handleKeepalive(eventType mgt.EventType, current *mgt.Peer, peers []*entity.Peer) {
	manager := utils.NewWatchManager()
	for _, peer := range peers {
		wc := manager.Get(peer.PublicKey)
		message := utils.NewWatchMessage(eventType, current)
		// add to channel, will send to client
		wc <- message
	}
}

func (s *Server) Start() error {
	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", 50051))
	if err != nil {
		return err
	}
	grpcServer := grpc.NewServer()
	mgt.RegisterManagementServiceServer(grpcServer, s)
	log.Printf("Grpc server listening at %v", listen.Addr())
	return grpcServer.Serve(listen)
}
