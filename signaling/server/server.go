package server

import (
	"context"
	"fmt"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
	"io"
	"linkany/management/grpc/client"
	"linkany/management/service"
	"linkany/pkg/drp"
	"linkany/pkg/linkerrors"
	"linkany/pkg/log"
	"linkany/signaling/grpc/signaling"
	"net"
	"sync"
	"time"
)

type Server struct {
	mu     sync.RWMutex
	logger *log.Logger
	signaling.UnimplementedSignalingServiceServer
	listen      string
	userService service.UserService
	clients     *drp.IndexTable
	mgtClient   *client.Client

	forwardManager *ForwardManager

	clientset map[string]*ClientInfo
}

type ClientInfo struct {
	ID       string
	LastSeen time.Time
	Stream   signaling.SignalingService_HeartbeatServer
}

type ServerConfig struct {
	Logger      *log.Logger
	Port        int
	Listen      string
	UserService service.UserService
	Table       *drp.IndexTable
}

func NewServer(cfg *ServerConfig) (*Server, error) {

	mgtClient, err := client.NewClient(&client.GrpcConfig{
		Addr:   "console.linkany.io:32051",
		Logger: log.NewLogger(log.Loglevel, "mgtclient"),
	})
	if err != nil {
		return nil, err
	}

	return &Server{
		logger:         cfg.Logger,
		mgtClient:      mgtClient,
		clientset:      make(map[string]*ClientInfo),
		forwardManager: NewForwardManager(),
	}, nil
}

func (s *Server) Start() error {
	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", 32132))
	if err != nil {
		return err
	}
	kasp := keepalive.ServerParameters{
		MaxConnectionIdle:     15 * time.Minute, // 如果连接空闲超过此时间，发送 GOAWAY
		MaxConnectionAge:      30 * time.Minute, // 连接最大存活时间
		MaxConnectionAgeGrace: 5 * time.Second,  // 强制关闭连接前的等待时间
		Time:                  5 * time.Second,  // 如果没有 ping，每5秒发送 ping
		Timeout:               3 * time.Second,  // ping 响应超时时间
	}

	//服务端强制策略
	kaep := keepalive.EnforcementPolicy{
		MinTime:             5 * time.Second, // 客户端两次 ping 之间的最小时间间隔
		PermitWithoutStream: true,            // 即使没有活跃的流也允许保持连接
	}

	grpcServer := grpc.NewServer(
		grpc.KeepaliveParams(kasp),
		grpc.KeepaliveEnforcementPolicy(kaep))
	signaling.RegisterSignalingServiceServer(grpcServer, s)
	s.logger.Verbosef("Signaling grpc server listening at %v", listen.Addr())
	return grpcServer.Serve(listen)
}

// Register will register a client to signaling server, will check token
func (s *Server) Register(ctx context.Context, message *signaling.EncryptMessage) (*signaling.EncryptMessage, error) {
	s.logger.Verbosef("register client: %v", message)
	var req signaling.EncryptMessageReqAndResp
	if err := proto.Unmarshal(message.Body, &req); err != nil {
		s.logger.Errorf("unmarshal failed: %v", err)
		return nil, err
	}

	_, err := s.mgtClient.VerifyToken(req.Token)
	if err != nil {
		s.logger.Errorf("verify token failed: %v", err)
		return nil, err
	}

	s.forwardManager.CreateChannel(message.PublicKey)
	s.logger.Verbosef("register '%v' client channel success", req.SrcPublicKey)

	var resp = &signaling.EncryptMessageReqAndResp{
		SrcPublicKey: req.SrcPublicKey,
		DstPublicKey: req.DstPublicKey,
	}

	body, err := proto.Marshal(resp)
	if err != nil {
		s.logger.Errorf("marshal failed: %v", err)
		return nil, err
	}

	return &signaling.EncryptMessage{
		Body: body,
	}, nil
}

func (s *Server) Forward(stream grpc.BidiStreamingServer[signaling.EncryptMessage, signaling.EncryptMessage]) error {

	done := make(chan interface{})
	defer func() {
		s.logger.Errorf("close server signaling stream")
		close(done)
	}()

	req, err, body := s.recv(stream)
	if err != nil {
		return err
	}

	channel, b := s.forwardManager.GetChannel(req.SrcPublicKey)
	if !b {
		s.logger.Errorf("channel not exists: %v", req.SrcPublicKey)
		return linkerrors.ErrChannelNotExists
	}

	logger := s.logger

	go func() {
		for {
			select {
			case forwardMsg := <-channel:
				logger.Verbosef("forward message to client: %v", req.SrcPublicKey)
				if err := stream.Send(&signaling.EncryptMessage{Body: forwardMsg.Body}); err != nil {
					s, ok := status.FromError(err)
					if ok && s.Code() == codes.Canceled {
						logger.Infof("client canceled")
						return
					} else if err == io.EOF {
						logger.Infof("client closed")
						return
					}
					return
				}
			case <-done:
				s.forwardManager.DeleteChannel(req.SrcPublicKey) // because client closed
				logger.Infof("close forward signaling stream, delete channel: %v", req.SrcPublicKey)
				return
			}
		}
	}()

	logger.Verbosef("forward message: %v, body: %v", req.Type, body)
	s.forward(&req, body)

	for {
		req, err, body := s.recv(stream)
		if err != nil {
			return err
		}

		logger.Verbosef("forward message: %v, body: %v", req.Type, body)
		s.forward(&req, body)

		logger.Verbosef("forward message success")

	}
}

func (s *Server) forward(req *signaling.EncryptMessageReqAndResp, body []byte) {
	dstChannel, ok := s.forwardManager.GetChannel(req.DstPublicKey)
	if !ok {
		s.logger.Errorf("channel not exists: %v", req.DstPublicKey)
	}

	if dstChannel != nil {
		dstChannel <- &ForwardMessage{
			Body: body,
		}
	}

}

func (s *Server) recv(stream grpc.BidiStreamingServer[signaling.EncryptMessage, signaling.EncryptMessage]) (signaling.EncryptMessageReqAndResp, error, []byte) {
	msg, err := stream.Recv()
	if err != nil {
		state, ok := status.FromError(err)
		if ok && state.Code() == codes.Canceled {
			s.logger.Infof("client canceled")
			return signaling.EncryptMessageReqAndResp{}, linkerrors.ErrClientCanceled, nil
		} else if err == io.EOF {
			s.logger.Infof("client closed")
			return signaling.EncryptMessageReqAndResp{}, linkerrors.ErrClientClosed, nil
		}

		s.logger.Errorf("receive msg failed: %v", err)
		return signaling.EncryptMessageReqAndResp{}, err, nil
	}

	// forward message to client
	var req signaling.EncryptMessageReqAndResp
	if err := proto.Unmarshal(msg.Body, &req); err != nil {
		return signaling.EncryptMessageReqAndResp{}, err, nil
	}
	return req, nil, msg.Body
}

func (s *Server) Heartbeat(stream signaling.SignalingService_HeartbeatServer) error {
	var clientID string
	// 设置超时检测
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.mu.RLock()
				client, exists := s.clientset[clientID]
				s.mu.RUnlock()

				if exists && time.Since(client.LastSeen) > 60*time.Second {
					// 客户端超时，关闭连接
					s.removeClient(clientID)
					return
				}
			case <-stream.Context().Done():
				s.removeClient(clientID)
				return
			}
		}
	}()

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			s.removeClient(clientID)
			return nil
		}
		if err != nil {
			s.removeClient(clientID)
			return err
		}

		// 更新客户端状态
		clientID = req.ClientId
		s.mu.Lock()
		s.clientset[clientID] = &ClientInfo{
			ID:       clientID,
			LastSeen: time.Now(),
			Stream:   stream,
		}
		s.mu.Unlock()

		// 发送响应
		err = stream.Send(&signaling.HeartbeatResponse{
			Timestamp: time.Now().UnixNano(),
			ServerId:  "signaling",
			Status:    signaling.Status_OK,
		})
		if err != nil {
			s.removeClient(clientID)
			return err
		}
	}

}

func (s *Server) removeClient(clientID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clientset, clientID)
}
