package server

import (
	"fmt"
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
	mgtClient   *client.Client
	clients     map[string]chan *signaling.SignalingMessage

	//forwardManager *ForwardManager
}

//type ClientInfo struct {
//	ID       string
//	LastSeen time.Time
//	Stream   signaling.SignalingService_HeartbeatServer
//}

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
		logger:    cfg.Logger,
		mgtClient: mgtClient,
		clients:   make(map[string]chan *signaling.SignalingMessage, 1),
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

func (s *Server) Signaling(stream grpc.BidiStreamingServer[signaling.SignalingMessage, signaling.SignalingMessage]) error {

	var (
		msgChan chan *signaling.SignalingMessage
		ok      bool
		req     *signaling.SignalingMessage
		err     error
		body    []byte
	)

	done := make(chan interface{})
	defer func() {
		s.logger.Errorf("close server signaling stream")
		close(done)
	}()

	req, err, body = s.recv(stream)
	if err != nil {
		return err
	}

	s.logger.Verbosef("received signaling request from %s, to: %s, msgType: %v,  content: %s", req.From, req.To, req.MsgType, string(body))

	// create channel for client
	s.mu.Lock()
	if msgChan, ok = s.clients[req.From]; !ok {
		msgChan = make(chan *signaling.SignalingMessage, 1000)
		s.clients[req.From] = msgChan
	}
	s.mu.Unlock()
	s.logger.Infof("create channel for %v success", req.From)

	logger := s.logger

	go func() {
		for {
			select {
			case forwardMsg := <-msgChan:
				if err := stream.Send(forwardMsg); err != nil {
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
				logger.Verbosef("signaling message to client: %v, to: %v,  content: %v", req.From, req.To, string(forwardMsg.Body))
			case <-done:
				//s.forwardManager.DeleteChannel(req.From) // because client closed
				//logger.Infof("close signaling signaling stream, delete channel: %v", req.From)
				return
			}
		}
	}()

	for {
		req, err, body = s.recv(stream)
		if err != nil {
			return err
		}

		s.signaling(req, body)
	}
}

func (s *Server) signaling(req *signaling.SignalingMessage, body []byte) {
	switch req.MsgType {
	case signaling.MessageType_MessageHeartBeatType:
		s.logger.Verbosef("received heartbeat message from %s, content: %s", req.From, string(body))
	// do nothing, heartbeat message is not forwarded
	default:
		s.logger.Verbosef("receiving forward message from %s, to: %v,  content: %s", req.From, req.To, string(body))
		s.mu.RLock()
		targetChan, ok := s.clients[req.To]
		if !ok {
			s.logger.Errorf("channel not exists for client: %v", req.To)
		}

		if targetChan != nil {
			targetChan <- req
		}
		s.mu.RUnlock()
	}
}

func (s *Server) recv(stream grpc.BidiStreamingServer[signaling.SignalingMessage, signaling.SignalingMessage]) (*signaling.SignalingMessage, error, []byte) {
	msg, err := stream.Recv()
	if err != nil {
		state, ok := status.FromError(err)
		if ok && state.Code() == codes.Canceled {
			s.logger.Infof("client canceled")
			return nil, linkerrors.ErrClientCanceled, nil
		} else if err == io.EOF {
			s.logger.Infof("client closed")
			return nil, linkerrors.ErrClientClosed, nil
		}

		s.logger.Errorf("receive msg failed: %v", err)
		return nil, err, nil
	}

	return msg, nil, msg.Body
}

//
//func (s *Server) Heartbeat(stream signaling.SignalingService_HeartbeatServer) error {
//	var clientID string
//	// 设置超时检测
//	go func() {
//		ticker := time.NewTicker(30 * time.Second)
//		defer ticker.Stop()
//
//		for {
//			select {
//			case <-ticker.C:
//				s.mu.RLock()
//				client, exists := s.clientset[clientID]
//				s.mu.RUnlock()
//
//				if exists && time.Since(client.LastSeen) > 60*time.Second {
//					// 客户端超时，关闭连接
//					s.removeClient(clientID)
//					return
//				}
//			case <-stream.Context().Done():
//				s.removeClient(clientID)
//				return
//			}
//		}
//	}()
//
//	for {
//		req, err := stream.Recv()
//		if err == io.EOF {
//			s.removeClient(clientID)
//			return nil
//		}
//		if err != nil {
//			s.removeClient(clientID)
//			return err
//		}
//
//		// 更新客户端状态
//		clientID = req.ClientId
//		s.mu.Lock()
//		s.clientset[clientID] = &ClientInfo{
//			ID:       clientID,
//			LastSeen: time.Now(),
//			Stream:   stream,
//		}
//		s.mu.Unlock()
//
//		// 发送响应
//		err = stream.Send(&signaling.HeartbeatResponse{
//			Timestamp: time.Now().UnixNano(),
//			ServerId:  "signaling",
//			Status:    signaling.Status_OK,
//		})
//		if err != nil {
//			s.removeClient(clientID)
//			return err
//		}
//	}
//
//}

//func (s *Server) removeClient(clientID string) {
//	s.mu.Lock()
//	defer s.mu.Unlock()
//	delete(s.clientset, clientID)
//}
