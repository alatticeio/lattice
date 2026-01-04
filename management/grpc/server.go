// Copyright 2025 The Wireflow Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package grpc

//
//import (
//	"context"
//	"encoding/json"
//	"errors"
//	"fmt"
//	"io"
//	"net"
//	"sync"
//	"time"
//	"wireflow/internal/core/domain"
//	"wireflow/internal/core/manager"
//	wgrpc "wireflow/internal/grpc"
//	"wireflow/internal/log"
//	"wireflow/management/dto"
//	"wireflow/management/resource"
//
//	"wireflow/api/v1alpha1"
//
//	"github.com/golang/protobuf/proto"
//	"google.golang.org/grpc"
//	"google.golang.org/grpc/codes"
//	"google.golang.org/grpc/status"
//)
//
//// Server is grpc server used to list watch resources to nodes.
//type Server struct {
//	ctx          context.Context
//	stopCh       chan struct{}
//	logger       *log.Logger
//	mu           sync.Mutex
//	watchManager domain.IWatchManager
//	wgrpc.UnimplementedManagementServiceServer
//	client        *resource.Client
//	port          int
//	checkInterval time.Duration
//}
//
//// ServerConfig used for Server builder
//type ServerConfig struct {
//	Ctx    context.Context
//	Logger *log.Logger
//	Port   int
//}
//
//// RegRequest used for register to grpc server
//type RegRequest struct {
//	ID                  int64      `json:"id"`
//	UserID              int64      `json:"user_id"`
//	Name                string     `json:"name"`
//	Hostname            string     `json:"hostname"`
//	Description         string     `json:"description"`
//	AppID               string     `json:"app_id"`
//	Address             string     `json:"address"`
//	Endpoint            string     `json:"endpoint"`
//	PersistentKeepalive int        `json:"persistent_keepalive"`
//	PublicKey           string     `json:"public_key"`
//	PrivateKey          string     `json:"private_key"`
//	AllowedIPs          string     `json:"allowed_ips"`
//	RelayIP             string     `json:"relay_ip"`
//	TieBreaker          uint32     `json:"tie_breaker"`
//	UpdatedAt           time.Time  `json:"updated_at"`
//	DeletedAt           *time.Time `json:"deleted_at"`
//	CreatedAt           time.Time  `json:"created_at"`
//	Ufrag               string     `json:"ufrag"`
//	Pwd                 string     `json:"pwd"`
//	Port                int        `json:"port"`
//	Token               string     `json:"token"`
//}
//
//func NewServer(cfg *ServerConfig) *Server {
//	stopCh := make(chan struct{})
//	wt := manager.NewWatchManager()
//	client, err := resource.NewClient(wt)
//	if err != nil {
//		panic(err)
//	}
//
//	go func() {
//		client.Start()
//	}()
//
//	return &Server{
//		ctx:           cfg.Ctx,
//		stopCh:        stopCh,
//		logger:        cfg.Logger,
//		port:          cfg.Port,
//		watchManager:  wt,
//		client:        client,
//		checkInterval: 30,
//	}
//}
//
//// Registry will return a list of response
//func (s *Server) Registry(ctx context.Context, in *wgrpc.ManagementMessage) (*wgrpc.ManagementMessage, error) {
//	var dto dto.PeerDto
//	if err := json.Unmarshal(in.Body, &dto); err != nil {
//		return nil, err
//	}
//	s.logger.Infof("Received peer info: %+v", dto)
//	node, err := s.client.Register(ctx, &dto)
//
//	if err != nil {
//		return nil, err
//	}
//
//	bs, err := json.Marshal(node)
//	if err != nil {
//		return nil, err
//	}
//
//	return &wgrpc.ManagementMessage{Body: bs}, nil
//}
//
//// Get used to get a node info by node's appId
//func (s *Server) Get(ctx context.Context, in *wgrpc.ManagementMessage) (*wgrpc.ManagementMessage, error) {
//	var req wgrpc.Request
//	if err := proto.Unmarshal(in.Body, &req); err != nil {
//		return nil, err
//	}
//	//_, err := s.userController.Get(ctx, req.Token)
//	//if err != nil {
//	//	return nil, err
//	//}
//
//	node, err := s.client.GetByAppId(ctx, req.AppId)
//	if err != nil {
//		return nil, err
//	}
//
//	type result struct {
//		Peer  *domain.Peer
//		Count int64
//	}
//	body := &result{
//		Peer: &domain.Peer{
//			UserId:              node.UserId,
//			Name:                node.Name,
//			Description:         node.Description,
//			Hostname:            node.Hostname,
//			AppID:               node.AppID,
//			Address:             node.Address,
//			Endpoint:            node.Endpoint,
//			PersistentKeepalive: node.PersistentKeepalive,
//			PublicKey:           node.PublicKey,
//			PrivateKey:          node.PrivateKey,
//			AllowedIPs:          node.AllowedIPs,
//			GroupName:           node.Group.GroupName,
//			NetworkId:           node.Group.NetworkId,
//		},
//	}
//
//	b, err := json.Marshal(body)
//	if err != nil {
//		return nil, err
//	}
//
//	s.logger.Verbosef("get node info: %v", string(b))
//
//	return &wgrpc.ManagementMessage{Body: b}, nil
//}
//
//// GetNetMap used to get node's net map, to connect to when node starting
//func (s *Server) GetNetMap(ctx context.Context, in *wgrpc.ManagementMessage) (*wgrpc.ManagementMessage, error) {
//	logger := s.logger
//	logger.Infof("GetNetMap starting")
//	var req wgrpc.Request
//	if err := proto.Unmarshal(in.Body, &req); err != nil {
//		return nil, status.Errorf(codes.Internal, "unmarshal failed: %v", err)
//	}
//	networkMap, err := s.client.GetNetworkMap(ctx, "default", req.AppId)
//	if err != nil {
//		return nil, err
//	}
//
//	bs, err := json.Marshal(networkMap)
//	if err != nil {
//		return nil, status.Errorf(codes.Internal, "marshal failed: %v", err)
//	}
//
//	return &wgrpc.ManagementMessage{Body: bs}, nil
//}
//
//// Keepalive used to check whether a node is living， server will send 'ping' packet to nodes
//// and node will response packet to server with in 10 seconds, if not, node is offline, otherwise online.
//func (s *Server) Keepalive(stream wgrpc.ManagementService_KeepaliveServer) error {
//	var (
//		err      error
//		body     []byte
//		req      *wgrpc.Request
//		clientId string
//		appId    string
//	)
//
//	ctx := stream.Context()
//	req, err = s.recv(ctx, stream)
//	if err != nil {
//		return status.Errorf(codes.Internal, "receive keepalive packet failed: %v", err)
//	}
//	clientId, appId = req.PubKey, req.AppId
//
//	s.logger.Infof("receive keepalive packet from client, pubkey: %v, appId: %v", req.PubKey, req.AppId)
//	var check func() error
//	check = func() error {
//		checkReq := &wgrpc.Request{PubKey: clientId}
//		body, err = proto.Marshal(checkReq)
//		if err != nil {
//			s.logger.Errorf("marshal check request failed: %v", err)
//		}
//		if err = stream.Send(&wgrpc.ManagementMessage{Body: body, Timestamp: time.Now().UnixMilli()}); err != nil {
//			st, ok := status.FromError(err)
//			if ok && st.Code() == codes.Canceled {
//				s.logger.Errorf("stream canceled")
//				return status.Errorf(codes.Canceled, "stream canceled")
//			} else if errors.Is(err, io.EOF) {
//				s.logger.Verbosef("node %s is disconnected", clientId)
//				return status.Errorf(codes.Internal, "client closed")
//			}
//		}
//
//		newCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
//		defer cancel()
//		req, err = s.recv(newCtx, stream)
//		if err != nil {
//			if err = s.client.UpdateNodeStatus(ctx, "default", appId, func(status *v1alpha1.NodeStatus) {
//				status.Status = v1alpha1.InActive
//			}); err != nil {
//				s.logger.Errorf("update node %s status to inactive failed: %v", appId, err)
//			}
//			return status.Errorf(codes.Internal, "receive keepalive packet failed: %v", err)
//		}
//
//		s.logger.Infof("recv keepalive packet from app, appId: %s", appId)
//		if err = s.client.UpdateNodeStatus(ctx, "default", appId, func(status *v1alpha1.NodeStatus) {
//			status.Status = v1alpha1.Active
//		}); err != nil {
//			s.logger.Errorf("update node %s status to active failed: %v", req.AppId, err)
//		}
//		return nil
//	}
//
//	ticker := time.NewTicker(s.checkInterval * time.Second)
//	defer ticker.Stop()
//
//	for {
//		select {
//		case <-ticker.C:
//			if err = check(); err != nil {
//				s.logger.Errorf("keepalive check failed: %v", err)
//			}
//		case <-ctx.Done():
//			s.logger.Infof("keepalive server closed")
//			return nil
//		}
//	}
//}
//
//func (s *Server) recv(ctx context.Context, stream wgrpc.ManagementService_KeepaliveServer) (*wgrpc.Request, error) {
//	type recvResult struct {
//		req *wgrpc.Request
//		err error
//	}
//
//	resultChan := make(chan *recvResult, 1)
//
//	go func() {
//		msg, err := stream.Recv()
//		if err != nil {
//			resultChan <- &recvResult{nil, status.Errorf(codes.Canceled, "receive canceled")}
//			return
//		}
//		var req wgrpc.Request
//		if err = proto.Unmarshal(msg.Body, &req); err != nil {
//			resultChan <- &recvResult{nil, err}
//			return
//		}
//
//		resultChan <- &recvResult{&req, nil}
//	}()
//
//	select {
//	case <-ctx.Done():
//		return nil, fmt.Errorf("timeout")
//	case result := <-resultChan:
//		return result.req, result.err
//
//	}
//}
//
//func (s *Server) Start() error {
//	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", domain.DefaultManagementPort))
//	if err != nil {
//		return err
//	}
//	grpcServer := grpc.NewServer()
//	wgrpc.RegisterManagementServiceServer(grpcServer, s)
//	s.logger.Verbosef("Grpc server listening at %v", listen.Addr())
//	return grpcServer.Serve(listen)
//}
//
//// Do will handle cli request
//func (s *Server) Do(ctx context.Context, in *wgrpc.ManagementMessage) (*wgrpc.ManagementMessage, error) {
//	logger := s.logger
//	logger.Infof("Handle cli request,pubKey: %s", in.PubKey)
//
//	var req dto.NetworkParams
//	if err := json.Unmarshal(in.Body, &req); err != nil {
//		return nil, err
//	}
//	switch in.Type {
//	case wgrpc.Type_MessageTypeCreateNetwork:
//		network, err := s.CreateNetwork(ctx, req.Name, req.CIDR)
//		if err != nil {
//			return nil, err
//		}
//
//		bs, err := json.Marshal(network)
//		if err != nil {
//			return nil, err
//		}
//		return &wgrpc.ManagementMessage{
//			Body: bs,
//		}, nil
//	case wgrpc.Type_MessageTypeJoinNetwork, wgrpc.Type_MessageTypeNetworkAddNode:
//		if err := s.JoinNetwork(ctx, req.AppIds, req.Name); err != nil {
//			logger.Errorf("Join network failed: %v", err)
//			return nil, err
//		}
//
//		return &wgrpc.ManagementMessage{
//			Body: []byte("Join network success"),
//		}, nil
//
//	case wgrpc.Type_MessageTypeLeaveNetwork, wgrpc.Type_MessageTypeNetworkRemoveNode:
//		if err := s.LeaveNetwork(ctx, req.AppIds, req.Name); err != nil {
//			logger.Errorf("Join network failed: %v", err)
//			return nil, err
//		}
//
//		return &wgrpc.ManagementMessage{
//			Body: []byte("Leave network success"),
//		}, nil
//	}
//	return nil, nil
//}
