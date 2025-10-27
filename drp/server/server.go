package server

import (
	"context"
	"fmt"
	"wireflow/internal"
	"wireflow/pkg/wferrors"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"

	"io"
	"net"
	"sync"
	"time"
	drpclient "wireflow/drp/client"
	drpgrpc "wireflow/drp/grpc"
	"wireflow/management/grpc/client"
	"wireflow/management/service"
	"wireflow/pkg/drp"
	"wireflow/pkg/log"
)

type Server struct {
	mu     sync.RWMutex
	ru     sync.Mutex
	logger *log.Logger
	drpgrpc.UnimplementedDrpServerServer
	listen      string
	userService service.UserService
	mgtClient   *client.Client
	clients     map[string]chan *drpgrpc.DrpMessage
	msgManager  *drpclient.MessageManager
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
		Addr:   fmt.Sprintf("%s:%d", internal.ManagementDomain, internal.DefaultSignalingPort),
		Logger: log.NewLogger(log.Loglevel, "mgt-client"),
	})
	if err != nil {
		return nil, err
	}

	return &Server{
		logger:     cfg.Logger,
		mgtClient:  mgtClient,
		msgManager: drpclient.NewMessageManager(),
		clients:    make(map[string]chan *drpgrpc.DrpMessage, 1),
	}, nil
}

func (s *Server) Start() error {
	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", internal.DefaultSignalingPort))
	if err != nil {
		return err
	}
	kasp := keepalive.ServerParameters{
		MaxConnectionIdle:     15 * time.Minute, // 如果连接空闲超过此时间，发送 GOAWAY
		MaxConnectionAge:      30 * time.Minute, // 连接最大存活时间
		MaxConnectionAgeGrace: 5 * time.Second,  // 强制关闭连接前的等待时间
		Time:                  10 * time.Second, // 如果没有 ping，每5秒发送 ping
		Timeout:               5 * time.Second,  // ping 响应超时时间
	}

	//服务端强制策略
	kaep := keepalive.EnforcementPolicy{
		MinTime:             5 * time.Second, // 客户端两次 ping 之间的最小时间间隔
		PermitWithoutStream: true,            // 即使没有活跃的流也允许保持连接
	}

	grpcServer := grpc.NewServer(
		grpc.InitialWindowSize(1024*1024),
		grpc.InitialConnWindowSize(1024*1024),
		grpc.MaxRecvMsgSize(4*1024*1024),
		grpc.WriteBufferSize(64*1024),
		grpc.ReadBufferSize(64*1024),
		grpc.MaxConcurrentStreams(1000),
		grpc.KeepaliveParams(kasp),
		grpc.KeepaliveEnforcementPolicy(kaep))
	drpgrpc.RegisterDrpServerServer(grpcServer, s)
	s.logger.Verbosef("Signaling grpc server listening at %v", listen.Addr())
	return grpcServer.Serve(listen)
}

func (s *Server) HandleMessage(stream grpc.BidiStreamingServer[drpgrpc.DrpMessage, drpgrpc.DrpMessage]) error {

	var (
		msgChan chan *drpgrpc.DrpMessage
		ok      bool
		err     error
	)

	done := make(chan interface{})
	defer func() {
		s.logger.Errorf("close server signaling stream")
		close(done)
	}()

	msg := s.msgManager.GetMessage()
	if err = stream.RecvMsg(msg); err != nil {
		s.msgManager.ReleaseMessage(msg)
		return err
	}

	s.logger.Verbosef("received drp request from %s, to: %s, msgType: %v,  data: %s", msg.From, msg.To, msg.MsgType, msg.Body)

	switch msg.MsgType {
	case drpgrpc.MessageType_MessageRegisterType:
		// create channel for client
		s.ru.Lock()
		if msgChan, ok = s.clients[msg.From]; !ok {
			msgChan = make(chan *drpgrpc.DrpMessage, 10000)
			s.clients[msg.From] = msgChan
			s.logger.Infof("create channel for %v success", msg.From)
		} else {
			s.logger.Infof("channel already exists for %v", msg.From)
		}
		s.ru.Unlock()
		s.msgManager.ReleaseMessage(msg)
	default:

	}

	eg, ctx := errgroup.WithContext(stream.Context())
	eg.Go(func() error {
		s.logger.Verbosef("start sendLoop for client: %v", msg.From)
		return s.sendLoop(ctx, msgChan, stream)
	})

	eg.Go(func() error {

		return s.receiveLoop(ctx, stream)
	})

	return eg.Wait()
}

func (s *Server) sendLoop(ctx context.Context, msgChan chan *drpgrpc.DrpMessage, stream grpc.BidiStreamingServer[drpgrpc.DrpMessage, drpgrpc.DrpMessage]) error {
	for {
		select {
		case forwardMsg := <-msgChan:
			if err := stream.Send(forwardMsg); err != nil {
				st, ok := status.FromError(err)
				if ok && st.Code() == codes.Canceled {
					s.logger.Errorf("client canceled")
				} else if err == io.EOF {
					s.logger.Errorf("client closed")
				}
				return err
			}

			s.logger.Verbosef("send message from: %v, to: %v,  cost time: %v", forwardMsg.From, forwardMsg.To, time.Since(time.UnixMilli(forwardMsg.Timestamp)).Milliseconds())
			s.msgManager.ReleaseMessage(forwardMsg)

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (s *Server) receiveLoop(ctx context.Context, stream grpc.BidiStreamingServer[drpgrpc.DrpMessage, drpgrpc.DrpMessage]) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			var err error
			msg := s.msgManager.GetMessage()
			if err = stream.RecvMsg(msg); err != nil {
				state, ok := status.FromError(err)
				if ok && state.Code() == codes.Canceled {
					s.logger.Infof("client canceled")
					return wferrors.ErrClientCanceled
				} else if err == io.EOF {
					s.logger.Infof("client closed")
					return wferrors.ErrClientClosed
				}

				s.logger.Errorf("receive msg failed: %v", err)
				s.msgManager.ReleaseMessage(msg)
				return err
			}

			switch msg.MsgType {
			case drpgrpc.MessageType_MessageHeartBeatType:
				s.msgManager.ReleaseMessage(msg)
			default:
				s.mu.RLock()
				targetChan, ok := s.clients[msg.To]
				if !ok {
					s.msgManager.ReleaseMessage(msg)
					s.logger.Errorf("channel not exists for client: %v", msg.To)
					continue
				}

				s.logger.Verbosef("drp server received msg time slapped: %v", time.Since(time.UnixMilli(msg.Timestamp)).Milliseconds())
				if targetChan != nil {
					targetChan <- msg
				}
				s.mu.RUnlock()
			}
		}
	}
}
