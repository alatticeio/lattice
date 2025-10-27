package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
	"wireflow/internal"
	"wireflow/management/controller"
	"wireflow/management/db"
	"wireflow/management/dto"
	"wireflow/management/grpc/mgt"
	"wireflow/management/resource"
	"wireflow/management/utils"
	"wireflow/management/vo"
	"wireflow/pkg/log"
	"wireflow/pkg/loop"
	"wireflow/pkg/redis"
	"wireflow/pkg/wferrors"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type ServerInterface interface {
	Login(ctx context.Context, in *mgt.ManagementMessage) (*mgt.ManagementMessage, error)
	Registry(ctx context.Context, in *mgt.ManagementMessage) (*mgt.ManagementMessage, error)
}

// Server is grpc server used to list watch resources to nodes.
type Server struct {
	ctx          context.Context
	stopCh       chan struct{}
	logger       *log.Logger
	mu           sync.Mutex
	watchManager *internal.WatchManager
	mgt.UnimplementedManagementServiceServer
	userController     *controller.UserController
	nodeController     *controller.NodeController
	nodeResource       *resource.NodeResource
	port               int
	tokenController    *controller.TokenController
	resourceController *resource.Controller
	loop               *loop.TaskLoop
	checkInterval      time.Duration
}

// ServerConfig used for Server builder
type ServerConfig struct {
	Ctx                context.Context
	Logger             *log.Logger
	Port               int
	Database           db.DatabaseConfig
	DataBaseService    *gorm.DB
	Rdb                *redis.Client
	ResourceController *resource.Controller
}

// RegRequest used for register to grpc server
type RegRequest struct {
	ID                  int64            `json:"id"`
	UserID              int64            `json:"user_id"`
	Name                string           `json:"name"`
	Hostname            string           `json:"hostname"`
	Description         string           `json:"description"`
	AppID               string           `json:"app_id"`
	Address             string           `json:"address"`
	Endpoint            string           `json:"endpoint"`
	PersistentKeepalive int              `json:"persistent_keepalive"`
	PublicKey           string           `json:"public_key"`
	PrivateKey          string           `json:"private_key"`
	AllowedIPs          string           `json:"allowed_ips"`
	RelayIP             string           `json:"relay_ip"`
	TieBreaker          uint32           `json:"tie_breaker"`
	UpdatedAt           time.Time        `json:"updated_at"`
	DeletedAt           *time.Time       `json:"deleted_at"`
	CreatedAt           time.Time        `json:"created_at"`
	Ufrag               string           `json:"ufrag"`
	Pwd                 string           `json:"pwd"`
	Port                int              `json:"port"`
	Status              utils.NodeStatus `json:"status"`
	Token               string           `json:"token"`
}

func NewServer(cfg *ServerConfig) *Server {

	stopCh := make(chan struct{})
	wt := internal.NewWatchManager()
	resourceController, err := resource.NewController(cfg.Ctx, "", wt)
	if err != nil {
		panic(err)
	}

	resourceController.Run(cfg.Ctx)

	return &Server{
		ctx:                cfg.Ctx,
		stopCh:             stopCh,
		logger:             cfg.Logger,
		port:               cfg.Port,
		userController:     controller.NewUserController(cfg.DataBaseService, cfg.Rdb),
		nodeController:     controller.NewPeerController(cfg.DataBaseService),
		tokenController:    controller.NewTokenController(cfg.DataBaseService),
		watchManager:       wt,
		resourceController: resourceController,
		loop:               loop.NewTaskLoop(100),
		nodeResource:       resource.NewNodeResource(resourceController),
		checkInterval:      30,
	}
}

// Login used for node login using grpc protocol
func (s *Server) Login(ctx context.Context, in *mgt.ManagementMessage) (*mgt.ManagementMessage, error) {
	var req mgt.LoginRequest
	if err := proto.Unmarshal(in.Body, &req); err != nil {
		return nil, err
	}
	s.logger.Infof("Received login username: %s, password: %s", req.Username, req.Password)

	token, err := s.userController.Login(ctx, &dto.UserDto{
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
	var dto dto.NodeDto
	if err := json.Unmarshal(in.Body, &dto); err != nil {
		return nil, err
	}
	s.logger.Infof("Received peer info: %+v", dto)
	node, err := s.nodeResource.Register(ctx, &dto)

	if err != nil {
		return nil, err
	}

	bs, err := json.Marshal(node)
	if err != nil {
		return nil, err
	}

	return &mgt.ManagementMessage{Body: bs}, nil
}

// Get used to get a node info by node's appId
func (s *Server) Get(ctx context.Context, in *mgt.ManagementMessage) (*mgt.ManagementMessage, error) {
	var req mgt.Request
	if err := proto.Unmarshal(in.Body, &req); err != nil {
		return nil, err
	}
	//_, err := s.userController.Get(ctx, req.Token)
	//if err != nil {
	//	return nil, err
	//}

	node, err := s.nodeResource.GetByAppId(ctx, req.AppId)
	if err != nil {
		return nil, err
	}

	type result struct {
		Peer  *internal.Node
		Count int64
	}
	body := &result{
		Peer: &internal.Node{
			UserId:              node.UserId,
			Name:                node.Name,
			Description:         node.Description,
			Hostname:            node.Hostname,
			AppID:               node.AppID,
			Address:             node.Address,
			Endpoint:            node.Endpoint,
			PersistentKeepalive: node.PersistentKeepalive,
			PublicKey:           node.PublicKey,
			PrivateKey:          node.PrivateKey,
			AllowedIPs:          node.AllowedIPs,
			GroupName:           node.Group.GroupName,
			NetworkId:           node.Group.NetworkId,
			DrpAddr:             node.DrpAddr,
			ConnectType:         node.ConnectType,
		},
	}

	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	s.logger.Verbosef("get node info: %v", string(b))

	return &mgt.ManagementMessage{Body: b}, nil
}

// List list-watch is like k8s's api design. list will return nodes list in the group that current node lived in.
// watch will catching the event in the group, when a node join in or leave away, send actual event message to every other group node
// lived in
func (s *Server) List(ctx context.Context, in *mgt.ManagementMessage) (*mgt.ManagementMessage, error) {
	var req mgt.Request
	if err := proto.Unmarshal(in.Body, &req); err != nil {
		return nil, status.Errorf(codes.Internal, "unmarshal failed: %v", err)
	}
	user, err := s.userController.Get(ctx, req.GetToken())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get user info err: %v", err)
	}
	s.logger.Infof("%v", user)
	networkMap, err := s.nodeController.GetNetworkMap(ctx, req.AppId, fmt.Sprintf("%d", user.ID))
	if err != nil {
		return nil, err
	}

	bs, err := json.Marshal(networkMap)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal failed: %v", err)
	}

	return &mgt.ManagementMessage{Body: bs}, nil
}

// GetNetMap used to get node's net map, to connect to when node starting
func (s *Server) GetNetMap(ctx context.Context, in *mgt.ManagementMessage) (*mgt.ManagementMessage, error) {
	var req mgt.Request
	if err := proto.Unmarshal(in.Body, &req); err != nil {
		return nil, status.Errorf(codes.Internal, "unmarshal failed: %v", err)
	}
	user, err := s.userController.Get(ctx, req.GetToken())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get user info err: %v", err)
	}
	s.logger.Infof("%v", user)
	networkMap, err := s.nodeController.GetNetworkMap(ctx, req.AppId, fmt.Sprintf("%d", user.ID))
	if err != nil {
		return nil, err
	}

	bs, err := json.Marshal(networkMap)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal failed: %v", err)
	}

	return &mgt.ManagementMessage{Body: bs}, nil
}

// Watch list-watch is like k8s's api design. list will return nodes list in the group that current node lived in.
// watch will catching the event in the group, when a node join in or leave away, send actual event message to every other group node
// lived in
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
		s.mu.Unlock()
	}()

	for {
		select {
		case wm := <-watchChannel.GetChannel():
			s.logger.Infof("sending watch message: %v to node: %v", wm, req.PubKey)
			bs, err := json.Marshal(wm)
			if err != nil {
				return status.Errorf(codes.Internal, "marshal failed: %v", err)
			}

			msg = &mgt.ManagementMessage{PubKey: req.PubKey, Body: bs}
			if err = server.Send(msg); err != nil {
				return status.Errorf(codes.Internal, "send failed: %v", err)
			}
		case <-server.Context().Done():
			return nil
		}
	}
}

// Keepalive used to check whether a node is living， server will send 'ping' packet to nodes
// and node will response packet to server with in 10 seconds, if not, node is offline, otherwise online.
func (s *Server) Keepalive(stream mgt.ManagementService_KeepaliveServer) error {
	var (
		err      error
		body     []byte
		req      *mgt.Request
		clientId string
	)

	ctx := context.Background()
	req, err = s.recv(ctx, stream)
	if err != nil {
		return status.Errorf(codes.Internal, "receive keepalive packet failed: %v", err)
	}
	clientId = req.PubKey
	logger := s.logger

	s.logger.Infof("receive keepalive packet from client, pubkey: %v, appId: %v", req.PubKey, req.AppId)
	check := func(ctx context.Context, checkChan chan struct{}) error {
		defer func() {
			close(checkChan)
		}()
		req, err = s.recv(ctx, stream)
		if req == nil || err != nil {
			return fmt.Errorf("receive keepalive packet failed: %v", err)
		}
		s.logger.Infof("recv keepalive packet from app, appId: %s", req.AppId)
		return s.nodeResource.UpdateNodeState(req.AppId, internal.Active)
	}

	timer := time.NewTimer(10 * time.Second)
	for {
		select {
		case <-timer.C:
			// check 10s receive the response
			newCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
			checkReq := &mgt.Request{PubKey: clientId}
			body, err = proto.Marshal(checkReq)
			if err != nil {
				s.logger.Errorf("marshal check request failed: %v", err)
				cancel()
				return err
			}
			checkChan := make(chan struct{})
			if err = s.loop.AddTask(newCtx, func(taskCtx context.Context) error {
				if err = stream.Send(&mgt.ManagementMessage{Body: body, Timestamp: time.Now().UnixMilli()}); err != nil {
					s, ok := status.FromError(err)
					if ok && s.Code() == codes.Canceled {
						logger.Errorf("stream canceled")
						return err
					} else if errors.Is(err, io.EOF) {
						logger.Verbosef("node %s is disconnected", clientId)
						return err
					}
				}

				return check(newCtx, checkChan)
			}); err != nil {
				return s.nodeResource.UpdateNodeState(req.AppId, internal.Inactive)
			}

			select {
			case <-newCtx.Done():
				logger.Infof("timeout or cancel")
				//timeout or cancel
				return s.nodeResource.UpdateNodeState(req.AppId, internal.Inactive)
			case <-checkChan:
				logger.Infof("node %s is active", req.AppId)
				timer.Reset(s.checkInterval * time.Second)
			}
		}
	}
}

func (s *Server) recv(ctx context.Context, stream mgt.ManagementService_KeepaliveServer) (*mgt.Request, error) {
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

func (s *Server) UpdateStatus(current *vo.NodeVo, status utils.NodeStatus) error {
	// update nodeVo online status
	dtoParam := &dto.NodeDto{PublicKey: current.PublicKey, Status: status}
	s.logger.Verbosef("update node status, publicKey: %v, status: %v", current.PublicKey, status)
	err := s.nodeController.UpdateStatus(context.Background(), dtoParam)

	current.Status = status
	return err
}

func (s *Server) Start() error {
	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", internal.DefaultManagementPort))
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

	b, _, err := s.tokenController.Verify(ctx, user.Username, user.Password)
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

	return nil, wferrors.ErrInvalidToken
}
