package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"linkany/management/controller"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/grpc/mgt"
	"linkany/management/service"
	"linkany/management/vo"
	"linkany/pkg/linkerrors"
	"linkany/pkg/log"
	"linkany/pkg/redis"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server is used to implement helloworld.GreeterServer.
type Server struct {
	logger   *log.Logger
	mu       sync.Mutex
	channels map[string]chan *vo.Message
	mgt.UnimplementedManagementServiceServer
	userController  *controller.UserController
	peerController  *controller.NodeController
	port            int
	tokenController *controller.TokenController
}

type ServerConfig struct {
	Logger          *log.Logger
	Port            int
	Database        service.DatabaseConfig
	DataBaseService *service.DatabaseService
	Rdb             *redis.Client
}

type RegistryRequest struct {
	ID                  int64             `json:"id"`
	UserID              int64             `json:"user_id"`
	Name                string            `json:"name"`
	Hostname            string            `json:"hostname"`
	Description         string            `json:"description"`
	AppID               string            `json:"app_id"`
	Address             string            `json:"address"`
	Endpoint            string            `json:"endpoint"`
	PersistentKeepalive int               `json:"persistent_keepalive"`
	PublicKey           string            `json:"public_key"`
	PrivateKey          string            `json:"private_key"`
	AllowedIPs          string            `json:"allowed_ips"`
	RelayIP             string            `json:"relay_ip"`
	TieBreaker          uint64            `json:"tie_breaker"`
	UpdatedAt           time.Time         `json:"updated_at"`
	DeletedAt           *time.Time        `json:"deleted_at"`
	CreatedAt           time.Time         `json:"created_at"`
	Ufrag               string            `json:"ufrag"`
	Pwd                 string            `json:"pwd"`
	Port                int               `json:"port"`
	Status              entity.NodeStatus `json:"status"`
	Token               string            `json:"token"`
}

func NewServer(cfg *ServerConfig) *Server {
	return &Server{
		logger:          cfg.Logger,
		port:            cfg.Port,
		userController:  controller.NewUserController(cfg.DataBaseService, cfg.Rdb),
		peerController:  controller.NewPeerController(cfg.DataBaseService),
		tokenController: controller.NewTokenController(cfg.DataBaseService),
	}
}

func (s *Server) Login(ctx context.Context, in *mgt.ManagementMessage) (*mgt.ManagementMessage, error) {
	var req mgt.LoginRequest
	if err := proto.Unmarshal(in.Body, &req); err != nil {
		return nil, err
	}
	s.logger.Infof("Received username: %s, password: %s", req.Username, req.Password)

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

// Registry will return a list of response
func (s *Server) Registry(ctx context.Context, in *mgt.ManagementMessage) (*mgt.ManagementMessage, error) {
	var req RegistryRequest
	if err := json.Unmarshal(in.Body, &req); err != nil {
		return nil, err
	}
	s.logger.Infof("Received peer info: %+v", req)
	user, err := s.userController.Get(req.Token)
	if err != nil {
		s.logger.Errorf("get user info err: %s\n", err.Error())
		return nil, err
	}

	peer, err := s.peerController.Registry(&dto.NodeDto{
		Hostname:            req.Hostname,
		UserID:              user.ID,
		AppID:               req.AppID,
		Address:             req.Address,
		PersistentKeepalive: req.PersistentKeepalive,
		PublicKey:           req.PublicKey,
		PrivateKey:          req.PrivateKey,
		AllowedIPs:          req.AllowedIPs,
		TieBreaker:          int64(req.TieBreaker),
		UpdatedAt:           time.Now(),
		CreatedAt:           time.Now(),
		Ufrag:               req.Ufrag,
		Pwd:                 req.Pwd,
		Port:                req.Port,
		Status:              req.Status,
	})
	if err != nil {
		return nil, err
	}

	bs, err := json.Marshal(peer)
	if err != nil {
		return nil, err
	}

	return &mgt.ManagementMessage{Body: bs}, nil
}

func (s *Server) Get(ctx context.Context, in *mgt.ManagementMessage) (*mgt.ManagementMessage, error) {
	var req mgt.Request
	if err := proto.Unmarshal(in.Body, &req); err != nil {
		return nil, err
	}
	user, err := s.userController.Get(req.Token)
	if err != nil {
		return nil, err
	}

	peer, count, err := s.peerController.GetByAppId(req.AppId, strconv.Itoa(int(user.ID)))
	if err != nil {
		return nil, err
	}

	type result struct {
		Peer  *entity.Node
		Count int64
	}
	body := &result{
		Peer:  peer,
		Count: count,
	}
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	return &mgt.ManagementMessage{Body: b}, nil
}

// List will return a list of response
func (s *Server) List(ctx context.Context, in *mgt.ManagementMessage) (*mgt.ManagementMessage, error) {
	var req mgt.Request
	if err := proto.Unmarshal(in.Body, &req); err != nil {
		return nil, status.Errorf(codes.Internal, "unmarshal failed: %v", err)
	}
	user, err := s.userController.Get(req.GetToken())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get user info err: %v", err)
	}
	s.logger.Infof("%v", user)
	networkMap, err := s.peerController.GetNetworkMap(req.AppId, strconv.Itoa(int(user.ID)))
	if err != nil {
		return nil, err
	}

	bs, err := json.Marshal(networkMap)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal failed: %v", err)
	}

	return &mgt.ManagementMessage{Body: bs}, nil
}

// Watch once request, will return a stream of watched response
func (s *Server) Watch(server mgt.ManagementService_WatchServer) error {
	var err error
	var msg *mgt.ManagementMessage
	msg, err = server.Recv()
	if err != nil {
		return status.Errorf(codes.Internal, "receive watcher failed: %v", err)
	}

	var req mgt.Request
	if err = proto.Unmarshal(msg.Body, &req); err != nil {
		return status.Errorf(codes.Internal, "unmarshal failed: %v", err)
	}

	clientId := req.PubKey
	// create a chan for the peer
	watchChannel := CreateChannel(clientId)
	s.logger.Infof("node %v is now watching, channel: %v", req.PubKey, watchChannel)

	defer func() {
		s.mu.Lock()
		s.logger.Infof("close watch channel")
		RemoveChannel(clientId)
		close(watchChannel)
		s.mu.Unlock()
	}()

	for {
		select {
		case wm := <-watchChannel:
			s.logger.Infof("sending watch message: %v to node: %v", wm, req.PubKey)
			bs, err := json.Marshal(wm)
			if err != nil {
				return status.Errorf(codes.Internal, "marshal failed: %v", err)
			}

			msg := &mgt.ManagementMessage{PubKey: req.PubKey, Body: bs}
			if err = server.Send(msg); err != nil {
				return status.Errorf(codes.Internal, "send failed: %v", err)
			}
		case <-server.Context().Done():
			return nil
		}
	}
}

// Keepalive acts as a client is living， server will send 'ping' packet to client
// client will response packet to server with in 10 seconds, if not, client is offline, otherwise onlie.
func (s *Server) Keepalive(stream mgt.ManagementService_KeepaliveServer) error {
	var (
		err    error
		req    *mgt.Request
		pubKey string
		userId string
	)

	ctx := context.Background()
	req, err = s.recv(stream)
	pubKey = req.PubKey
	logger := s.logger

	current, err := s.peerController.QueryNodes(&dto.QueryParams{PubKey: &pubKey})
	if err != nil {
		return err
	}

	if len(current) == 0 {
		return fmt.Errorf("node not found")
	}

	s.logger.Infof("receive keepalive packet from client, pubkey: %v, userId: %v", pubKey, userId)
	k := NewWatchKeeper()
	check := func(ctx context.Context) error {
		req, err = s.recv(stream)
		if err != nil {
			return err
		}
		s.logger.Verbosef("got keepalive resp packet from client,pubKey: %s", req.PubKey)
		return nil
	}

	timer := time.NewTimer(10 * time.Second)
	for {
		select {
		case <-timer.C:
			// check 10s receive the response
			newCtx, cancel := context.WithTimeout(ctx, 20*time.Second)

			checkReq := &mgt.Request{PubKey: pubKey}
			body, err := proto.Marshal(checkReq)
			if err != nil {
				s.logger.Errorf("marshal check request failed: %v", err)
				cancel()
				return err
			}

			checkChannel := make(chan interface{})

			// work
			go func() {
				// got resp, check success
				var err error
				defer func() {
					if err != nil {
						cancel()
					}
				}()

				if err = stream.Send(&mgt.ManagementMessage{Body: body}); err != nil {
					s, ok := status.FromError(err)
					if ok && s.Code() == codes.Canceled {
						logger.Errorf("stream canceled")
						return
					} else if errors.Is(err, io.EOF) {
						// client exit
						logger.Verbosef("node %s is disconnected", pubKey)
						return
					}
				}

				if err = check(newCtx); err != nil {
					logger.Errorf("check failed: %v", err)
					return
				}

				close(checkChannel)
				timer.Reset(10 * time.Second)

			}()

			select {
			case <-newCtx.Done():
				logger.Infof("timeout or cancel")
				//timeout or cancel
				s.pushWatchMessage(&vo.MessageConfig{
					EventType: vo.EventTypeNodeRemove,
					GroupMessage: &vo.GroupMessage{
						Nodes: current,
					},
				})
				if err = s.UpdateStatus(current[0], 0); err != nil {
					s.logger.Errorf("update node status: %v", err)
				}
				k.Online.Store(false)
				return fmt.Errorf("exit stream: %v", stream)
			case <-checkChannel:
				s.logger.Verbosef("node %s is online", pubKey)
				//if !k.Status.Load() {
				s.pushWatchMessage(&vo.MessageConfig{
					EventType: vo.EventTypeNodeAdd,
					GroupMessage: &vo.GroupMessage{
						Nodes: current,
					},
				})
				if err = s.UpdateStatus(current[0], 1); err != nil {
					s.logger.Errorf("update node status: %v", err)
				}
				k.Online.Store(true)

			}
		}
	}
}

func (s *Server) recv(stream mgt.ManagementService_KeepaliveServer) (*mgt.Request, error) {
	msg, err := stream.Recv()
	if err != nil {
		state, ok := status.FromError(err)
		if ok && state.Code() == codes.Canceled {
			s.logger.Errorf("receive canceled")
			return nil, status.Errorf(codes.Canceled, "stream canceled")
		} else if errors.Is(err, io.EOF) {
			s.logger.Errorf("client closed")
			return nil, status.Errorf(codes.Internal, "client closed")
		}
		return nil, err
	}
	var req mgt.Request
	if err = proto.Unmarshal(msg.Body, &req); err != nil {
		return nil, err
	}

	return &req, nil

}

func (s *Server) pushWatchMessage(msg *vo.MessageConfig) {
	manager := vo.NewWatchManager()
	s.mu.Lock()
	defer s.mu.Unlock()
	// send to user's all group clients
	for _, wc := range manager.Clientsets() {
		wc <- vo.NewMessage(msg)
	}
}

func (s *Server) UpdateStatus(current *vo.NodeVo, status entity.NodeStatus) error {
	// update nodeVo online status
	dtoParam := &dto.NodeDto{PublicKey: current.PublicKey, Status: status}
	s.logger.Verbosef("update node status, publicKey: %v, status: %v", current.PublicKey, status)
	_, err := s.peerController.Update(dtoParam)
	return err
}

// NewWatchMessage creates a new HandleWatchMessage, when a peer is added, updated or deleted
func NewWatchMessage(eventType vo.EventType) *vo.Message {
	return &vo.Message{
		EventType: eventType,
	}
}

func (s *Server) Start() error {
	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", 32051))
	if err != nil {
		return err
	}
	grpcServer := grpc.NewServer()
	mgt.RegisterManagementServiceServer(grpcServer, s)
	s.logger.Verbosef("Grpc server listening at %v", listen.Addr())
	return grpcServer.Serve(listen)
}

func (s *Server) VerifyToken(ctx context.Context, in *mgt.ManagementMessage) (*mgt.ManagementMessage, error) {
	var req mgt.Request
	if err := proto.Unmarshal(in.Body, &req); err != nil {
		return nil, err
	}

	user, err := s.tokenController.Parse(req.Token)
	if err != nil {
		return nil, err
	}

	b, _, err := s.tokenController.Verify(user.Username, user.Password)
	if err != nil {
		return nil, err
	}

	if b {
		body, err := proto.Marshal(&mgt.LoginResponse{Token: req.Token})
		if err != nil {
			return nil, err
		}

		return &mgt.ManagementMessage{
			Body: body,
		}, nil
	}

	return nil, linkerrors.ErrInvalidToken
}
