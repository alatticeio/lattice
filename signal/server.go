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

package signal

import (
	"context"
	"fmt"
	"wireflow/internal/core/domain"
	internalgrpc "wireflow/internal/grpc"
	"wireflow/internal/log"
	"wireflow/internal/wferrors"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"

	"io"
	"net"
	"sync"
	"time"
)

type Server struct {
	mu     sync.RWMutex
	ru     sync.Mutex
	logger *log.Logger
	internalgrpc.UnimplementedDrpServerServer
	listen     string
	clients    map[string]chan *internalgrpc.DrpMessage
	msgManager *MessagePool
}

type ServerConfig struct {
	Logger *log.Logger
	Port   int
	Listen string
}

func NewServer(cfg *ServerConfig) (*Server, error) {

	return nil, nil
}

func (s *Server) Start() error {
	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", domain.DefaultSignalingPort))
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
	internalgrpc.RegisterDrpServerServer(grpcServer, s)
	s.logger.Infof("Signaling grpc server listening at %v", listen.Addr())
	return grpcServer.Serve(listen)
}

func (s *Server) HandleMessage(stream grpc.BidiStreamingServer[internalgrpc.DrpMessage, internalgrpc.DrpMessage]) error {

	var (
		msgChan chan *internalgrpc.DrpMessage
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
	case internalgrpc.MessageType_MessageRegisterType:
		// create channel for client
		s.ru.Lock()
		if msgChan, ok = s.clients[msg.From]; !ok {
			msgChan = make(chan *internalgrpc.DrpMessage, 10000)
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

func (s *Server) sendLoop(ctx context.Context, msgChan chan *internalgrpc.DrpMessage, stream grpc.BidiStreamingServer[internalgrpc.DrpMessage, internalgrpc.DrpMessage]) error {
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

func (s *Server) receiveLoop(ctx context.Context, stream grpc.BidiStreamingServer[internalgrpc.DrpMessage, internalgrpc.DrpMessage]) error {
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
			case internalgrpc.MessageType_MessageHeartBeatType:
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
