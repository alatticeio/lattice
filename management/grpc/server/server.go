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
	wgrpc "wireflow/internal/grpc"
	"wireflow/management/controller"
	"wireflow/management/db"
	"wireflow/management/dto"
	"wireflow/management/resource"
	"wireflow/management/vo"
	"wireflow/pkg/log"
	"wireflow/pkg/loop"
	"wireflow/pkg/redis"
	"wireflow/pkg/utils"
	"wireflow/pkg/wferrors"

	"github.com/golang/protobuf/proto"
	"github.com/wireflowio/wireflow-controller/api/v1alpha1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

// Server is grpc server used to list watch resources to nodes.
type Server struct {
	ctx          context.Context
	stopCh       chan struct{}
	logger       *log.Logger
	mu           sync.Mutex
	watchManager *internal.WatchManager
	wgrpc.UnimplementedManagementServiceServer
	userController  *controller.UserController
	nodeController  *controller.NodeController
	client          *resource.Client
	port            int
	tokenController *controller.TokenController
	loop            *loop.TaskLoop
	checkInterval   time.Duration
}

// ServerConfig used for Server builder
type ServerConfig struct {
	Ctx             context.Context
	Logger          *log.Logger
	Port            int
	Database        db.DatabaseConfig
	DataBaseService *gorm.DB
	Rdb             *redis.Client
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
	client, err := resource.NewClient(wt)
	if err != nil {
		panic(err)
	}

	go func() {
		client.Start()
	}()

	return &Server{
		ctx:             cfg.Ctx,
		stopCh:          stopCh,
		logger:          cfg.Logger,
		port:            cfg.Port,
		userController:  controller.NewUserController(cfg.DataBaseService, cfg.Rdb),
		nodeController:  controller.NewPeerController(cfg.DataBaseService),
		tokenController: controller.NewTokenController(cfg.DataBaseService),
		watchManager:    wt,
		client:          client,
		loop:            loop.NewTaskLoop(100),
		checkInterval:   30,
	}
}

// Login used for node login using grpc protocol
func (s *Server) Login(ctx context.Context, in *wgrpc.ManagementMessage) (*wgrpc.ManagementMessage, error) {
	var req wgrpc.LoginRequest
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

	b, err := proto.Marshal(&wgrpc.LoginResponse{Token: token.Token})
	if err != nil {
		return nil, err
	}

	return &wgrpc.ManagementMessage{
		Body: b,
	}, nil
}

// Registry will return a list of response
func (s *Server) Registry(ctx context.Context, in *wgrpc.ManagementMessage) (*wgrpc.ManagementMessage, error) {
	var dto dto.NodeDto
	if err := json.Unmarshal(in.Body, &dto); err != nil {
		return nil, err
	}
	s.logger.Infof("Received peer info: %+v", dto)
	node, err := s.client.Register(ctx, &dto)

	if err != nil {
		return nil, err
	}

	bs, err := json.Marshal(node)
	if err != nil {
		return nil, err
	}

	return &wgrpc.ManagementMessage{Body: bs}, nil
}

// Get used to get a node info by node's appId
func (s *Server) Get(ctx context.Context, in *wgrpc.ManagementMessage) (*wgrpc.ManagementMessage, error) {
	var req wgrpc.Request
	if err := proto.Unmarshal(in.Body, &req); err != nil {
		return nil, err
	}
	//_, err := s.userController.Get(ctx, req.Token)
	//if err != nil {
	//	return nil, err
	//}

	node, err := s.client.GetByAppId(ctx, req.AppId)
	if err != nil {
		return nil, err
	}

	type result struct {
		Peer  *internal.Peer
		Count int64
	}
	body := &result{
		Peer: &internal.Peer{
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

	return &wgrpc.ManagementMessage{Body: b}, nil
}

// List list-watch is like k8s's api design. list will return nodes list in the group that current node lived in.
// watch will catching the event in the group, when a node join in or leave away, send actual event message to every other group node
// lived in
func (s *Server) List(ctx context.Context, in *wgrpc.ManagementMessage) (*wgrpc.ManagementMessage, error) {
	var req wgrpc.Request
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

	return &wgrpc.ManagementMessage{Body: bs}, nil
}

// GetNetMap used to get node's net map, to connect to when node starting
func (s *Server) GetNetMap(ctx context.Context, in *wgrpc.ManagementMessage) (*wgrpc.ManagementMessage, error) {
	logger := s.logger
	logger.Infof("GetNetMap starting")
	var req wgrpc.Request
	if err := proto.Unmarshal(in.Body, &req); err != nil {
		return nil, status.Errorf(codes.Internal, "unmarshal failed: %v", err)
	}
	networkMap, err := s.client.GetNetworkMap(ctx, "default", req.AppId)
	if err != nil {
		return nil, err
	}

	bs, err := json.Marshal(networkMap)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal failed: %v", err)
	}

	return &wgrpc.ManagementMessage{Body: bs}, nil
}

// Watch list-watch is like k8s's api design. list will return nodes list in the group that current node lived in.
// watch will catching the event in the group, when a node join in or leave away, send actual event message to every other group node
// lived in
func (s *Server) Watch(stream wgrpc.ManagementService_WatchServer) error {
	var err error
	var msg *wgrpc.ManagementMessage
	ctx := stream.Context()
	msg, err = stream.Recv()
	if err != nil {
		return status.Errorf(codes.Internal, "receive watcher failed: %v", err)
	}

	var req wgrpc.Request
	if err = proto.Unmarshal(msg.Body, &req); err != nil {
		return status.Errorf(codes.Internal, "unmarshal failed: %v", err)
	}

	appId := req.AppId
	// create a chan for the peer
	watchChannel := CreateChannel(appId)
	s.logger.Infof("node %v is now watching, channel: %v", req.AppId, watchChannel)

	defer func() {
		s.mu.Lock()
		s.logger.Infof("close watch channel for client: %s", appId)
		RemoveChannel(appId)
		// Update node status to inactive when watch connection is closed，give 30 second to receive watch message
		newCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err = s.client.UpdateNodeStatus(newCtx, "default", appId, func(status *v1alpha1.NodeStatus) {
			status.Status = v1alpha1.InActive
		}); err != nil {
			s.logger.Errorf("update node %s status to inactive failed: %v", appId, err)
		}
		s.mu.Unlock()
	}()

	for {
		select {
		case wm, ok := <-watchChannel.GetChannel():
			if !ok {
				s.logger.Infof("watch channel closed for client: %s", appId)
			}
			s.logger.Infof("sending watch message: %v to node: %v", wm, req.PubKey)
			bs, err := json.Marshal(wm)
			if err != nil {
				s.logger.Errorf("marshal failed: %v", err)
				continue
			}

			msg = &wgrpc.ManagementMessage{PubKey: req.PubKey, Body: bs}
			if err = stream.Send(msg); err != nil {
				s.logger.Errorf("send failed: %v", err)
				// Check if it's a client disconnect error
				if st, ok := status.FromError(err); ok && (st.Code() == codes.Canceled || st.Code() == codes.Unavailable) {
					s.logger.Errorf("client %s disconnected, stopping watch", appId)
				}
				if errors.Is(err, io.EOF) {
					s.logger.Errorf("client %s closed connection, stopping watch", appId)
				}
			}
		case <-ctx.Done():
			s.logger.Infof("watch context cancelled for client: %s", appId)
			return fmt.Errorf("watch context cancelled")
		}
	}
}

// Keepalive used to check whether a node is living， server will send 'ping' packet to nodes
// and node will response packet to server with in 10 seconds, if not, node is offline, otherwise online.
func (s *Server) Keepalive(stream wgrpc.ManagementService_KeepaliveServer) error {
	var (
		err      error
		body     []byte
		req      *wgrpc.Request
		clientId string
		appId    string
	)

	ctx := stream.Context()
	req, err = s.recv(ctx, stream)
	if err != nil {
		return status.Errorf(codes.Internal, "receive keepalive packet failed: %v", err)
	}
	clientId, appId = req.PubKey, req.AppId

	s.logger.Infof("receive keepalive packet from client, pubkey: %v, appId: %v", req.PubKey, req.AppId)
	var check func() error
	check = func() error {
		checkReq := &wgrpc.Request{PubKey: clientId}
		body, err = proto.Marshal(checkReq)
		if err != nil {
			s.logger.Errorf("marshal check request failed: %v", err)
		}
		if err = stream.Send(&wgrpc.ManagementMessage{Body: body, Timestamp: time.Now().UnixMilli()}); err != nil {
			st, ok := status.FromError(err)
			if ok && st.Code() == codes.Canceled {
				s.logger.Errorf("stream canceled")
				return status.Errorf(codes.Canceled, "stream canceled")
			} else if errors.Is(err, io.EOF) {
				s.logger.Verbosef("node %s is disconnected", clientId)
				return status.Errorf(codes.Internal, "client closed")
			}
		}

		newCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		req, err = s.recv(newCtx, stream)
		if err != nil {
			if err = s.client.UpdateNodeStatus(ctx, "default", appId, func(status *v1alpha1.NodeStatus) {
				status.Status = v1alpha1.InActive
			}); err != nil {
				s.logger.Errorf("update node %s status to inactive failed: %v", appId, err)
			}
			return status.Errorf(codes.Internal, "receive keepalive packet failed: %v", err)
		}

		s.logger.Infof("recv keepalive packet from app, appId: %s", appId)
		if err = s.client.UpdateNodeStatus(ctx, "default", appId, func(status *v1alpha1.NodeStatus) {
			status.Status = v1alpha1.Active
		}); err != nil {
			s.logger.Errorf("update node %s status to active failed: %v", req.AppId, err)
		}
		return nil
	}

	ticker := time.NewTicker(s.checkInterval * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err = check(); err != nil {
				s.logger.Errorf("keepalive check failed: %v", err)
			}
		case <-ctx.Done():
			s.logger.Infof("keepalive server closed")
			return nil
		}
	}
}

func (s *Server) recv(ctx context.Context, stream wgrpc.ManagementService_KeepaliveServer) (*wgrpc.Request, error) {
	type recvResult struct {
		req *wgrpc.Request
		err error
	}

	resultChan := make(chan *recvResult, 1)

	go func() {
		msg, err := stream.Recv()
		if err != nil {
			resultChan <- &recvResult{nil, status.Errorf(codes.Canceled, "receive canceled")}
			return
		}
		var req wgrpc.Request
		if err = proto.Unmarshal(msg.Body, &req); err != nil {
			resultChan <- &recvResult{nil, err}
			return
		}

		resultChan <- &recvResult{&req, nil}
	}()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("timeout")
	case result := <-resultChan:
		return result.req, result.err

	}
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
	wgrpc.RegisterManagementServiceServer(grpcServer, s)
	s.logger.Verbosef("Grpc server listening at %v", listen.Addr())
	return grpcServer.Serve(listen)
}

func (s *Server) VerifyToken(ctx context.Context, in *wgrpc.ManagementMessage) (*wgrpc.ManagementMessage, error) {
	var req wgrpc.Request
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
		body, err := proto.Marshal(&wgrpc.LoginResponse{Token: req.Token})
		if err != nil {
			return nil, err
		}

		return &wgrpc.ManagementMessage{
			Body: body,
		}, nil
	}

	return nil, wferrors.ErrInvalidToken
}

// Do will handle cli request
func (s *Server) Do(ctx context.Context, in *wgrpc.ManagementMessage) (*wgrpc.ManagementMessage, error) {
	logger := s.logger
	logger.Infof("Handle cli request,pubKey: %s", in.PubKey)

	switch in.Type {
	case wgrpc.Type_MessageTypeJoinNetwork:
		var req struct {
			AppId     string `json:"appId"`
			NetworkId string `json:"networkId"`
		}
		if err := json.Unmarshal(in.Body, &req); err != nil {
			return nil, err
		}
		if err := s.JoinNetwork(ctx, req.AppId, req.NetworkId); err != nil {
			logger.Errorf("Join network failed: %v", err)
			return nil, err
		}

		return &wgrpc.ManagementMessage{
			Body: []byte("Join network success"),
		}, nil

	case wgrpc.Type_MessageTypeLeaveNetwork:
		var req struct {
			AppId     string `json:"appId"`
			NetworkId string `json:"networkId"`
		}

		if err := json.Unmarshal(in.Body, &req); err != nil {
			return nil, err
		}
		if err := s.LeaveNetwork(ctx, req.AppId, req.NetworkId); err != nil {
			logger.Errorf("Join network failed: %v", err)
			return nil, err
		}

		return &wgrpc.ManagementMessage{
			Body: []byte("Join network success"),
		}, nil
	}
	return nil, nil
}
