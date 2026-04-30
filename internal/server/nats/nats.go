// Copyright 2026 The Lattice Authors, Inc.
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

package nats

import (
	"context"
	"errors"
	"fmt"
	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/agent/log"
	"time"

	"github.com/alatticeio/lattice/internal/grpc"

	natsgo "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"
)

var (
	_ infra.SignalService = (*NatsSignalService)(nil)
	_ infra.SignalService = (*noopSignalService)(nil)
)

// noopSignalService 是 NATS 不可用时的降级实现，所有操作静默忽略。
type noopSignalService struct {
	log *log.Logger
}

func (n *noopSignalService) Send(_ context.Context, _ infra.PeerID, _ []byte) error {
	return nil
}

func (n *noopSignalService) Request(_ context.Context, _, _ string, _ []byte) ([]byte, error) {
	return nil, fmt.Errorf("nats: not connected (noop service)")
}

func (n *noopSignalService) Flush() error {
	return nil
}

func (n *noopSignalService) Service(_, _ string, _ func([]byte) ([]byte, error)) {
	n.log.Warn("nats: Service() called on noop signal service, subscription skipped")
}

func (n *noopSignalService) Close() error { return nil }

// NewNoopSignalService 返回一个无操作的 SignalService，用于 NATS 不可用时的降级。
func NewNoopSignalService() infra.SignalService {
	return &noopSignalService{log: log.GetLogger("nats-noop")}
}

type SignalHandler func(ctx context.Context, peerId infra.PeerID, packet *grpc.SignalPacket) error

type NatsSignalService struct {
	log *log.Logger
	nc  *natsgo.Conn
	sub *natsgo.Subscription
}

func NewNatsService(ctx context.Context, name, role, url string) (*NatsSignalService, error) {
	clientName := fmt.Sprintf("lattice-%s-%s-%d", role, name, time.Now().UnixNano())
	// 1. 使用更稳健的连接配置
	opts := []natsgo.Option{
		natsgo.Name(clientName),
		natsgo.MaxReconnects(-1), // 无限重连，防止网络抖动导致服务彻底挂掉
		natsgo.ReconnectWait(2 * time.Second),
		// 关键：增加断开连接后的报错回调，便于排查你提到的 Hold 住问题
		natsgo.DisconnectErrHandler(func(nc *natsgo.Conn, err error) {
			fmt.Printf("NATS disconnected: %v\n", err)
		}),
	}

	logger := log.GetLogger("nats-signal")
	logger.Info("connecting to NATS server", "url", url)

	nc, err := natsgo.Connect(url, opts...)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}

	// 3. 必须执行 Flush！
	// 这一步会同步等待握手完成。如果你连到了 Telnet 端口，Flush 会立刻报错。
	if err = nc.Flush(); err != nil {
		nc.Close()
		return nil, fmt.Errorf("nats handshake failed (check if protocol is correct): %w", err)
	}

	s := &NatsSignalService{
		nc: nc,
	}
	s.log = logger

	// JetStream 初始化逻辑
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close() // 初始化失败记得关掉连接
		return nil, err
	}

	// 建议：将 Stream 的创建/检查逻辑封装成一个内部方法，保持 New 函数整洁
	if err := s.ensureStream(ctx, js); err != nil {
		nc.Close()
		return nil, err
	}

	return s, nil
}

func (s *NatsSignalService) ensureStream(ctx context.Context, js jetstream.JetStream) error {
	streamName := "LATTICE"
	_, err := js.Stream(ctx, streamName)
	if err != nil {
		if errors.Is(err, jetstream.ErrStreamNotFound) {
			s.log.Debug("Stream not found, creating", "stream", streamName)
			_, err = js.CreateStream(ctx, jetstream.StreamConfig{
				Name:     streamName,
				Subjects: []string{"signals.>"},
				Storage:  jetstream.FileStorage,
			})
		}
	}

	return err
}

func (s *NatsSignalService) Subscribe(subject string, onMessage SignalHandler) error {
	sub, err := s.nc.Subscribe(subject, func(m *natsgo.Msg) {
		var packet grpc.SignalPacket
		if err := proto.Unmarshal(m.Data, &packet); err != nil {
			s.log.Error("failed to unmarshal packet", err)
			return
		}

		err := onMessage(context.Background(), infra.FromUint64(packet.SenderId), &packet)
		if err != nil {
			s.log.Error("onMessage failed", err)
		}
	})

	s.sub = sub
	if err != nil {
		return err
	}

	return nil
}

func (s *NatsSignalService) Flush() error {
	return s.nc.Flush()
}

func (s *NatsSignalService) Send(_ context.Context, peerId infra.PeerID, data []byte) error {
	subject := fmt.Sprintf("lattice.signals.peers.%s", peerId)
	return s.nc.Publish(subject, data)
}

// Request sends a request-reply message with exponential backoff retry on ErrNoResponders.
// ErrNoResponders occurs when the server-side QueueSubscribe is temporarily unavailable
// (e.g. during a NATS reconnection window after a client restart).
func (s *NatsSignalService) Request(ctx context.Context, subject, method string, data []byte) ([]byte, error) {
	if !s.nc.IsConnected() {
		return nil, fmt.Errorf("nats: connection is not ready")
	}

	const maxRetries = 5
	const baseDelay = 100 * time.Millisecond

	fullSubject := fmt.Sprintf("%s.%s", subject, method)
	var resp *natsgo.Msg
	var err error

	for i := 0; i < maxRetries; i++ {
		resp, err = s.nc.RequestWithContext(ctx, fullSubject, data)
		if err == nil {
			break
		}
		if !errors.Is(err, natsgo.ErrNoResponders) {
			return nil, err
		}
		delay := baseDelay << i // 100ms, 200ms, 400ms, 800ms, 1600ms
		s.log.Warn("no responders, retrying", "subject", fullSubject, "attempt", i+1, "delay", delay)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
	if err != nil {
		return nil, err
	}

	if resp.Header.Get("error") != "" {
		return nil, fmt.Errorf("%s", resp.Header.Get("error"))
	}

	return resp.Data, nil
}

// SetReconnectedHandler registers a callback that is invoked each time the
// NATS client successfully reconnects to the server.  Use this to re-register
// application state that the server loses on restart (e.g. peer registration,
// network-map re-fetch).  The callback is run in a new goroutine so it must
// not block the NATS internal reconnect loop.
func (s *NatsSignalService) SetReconnectedHandler(fn func()) {
	s.nc.SetReconnectHandler(func(_ *natsgo.Conn) {
		s.log.Info("NATS reconnected, triggering re-registration")
		go fn()
	})
}

// Close drains in-flight messages and closes the NATS connection, immediately
// notifying the server to remove all subscriptions for this client.
func (s *NatsSignalService) Close() error {
	return s.nc.Drain()
}

func (s *NatsSignalService) Service(subject, queue string, service func(data []byte) ([]byte, error)) {
	_, err := s.nc.QueueSubscribe(subject, queue, func(msg *natsgo.Msg) {
		go func(msg *natsgo.Msg) {
			data, err := service(msg.Data)
			if err != nil {
				resp := natsgo.NewMsg(msg.Reply)
				resp.Header.Add("error", err.Error())
				resp.Header.Add("status", "400")

				if err = msg.RespondMsg(resp); err != nil {
					s.log.Error("failed to respond to message", err)
				}
				return
			}
			if err = msg.Respond(data); err != nil {
				s.log.Error("failed to respond to message", err)
			}
		}(msg)
	})

	if err != nil {
		s.log.Error("failed to subscribe", err)
	}
}
