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

package nats

import (
	"context"
	"fmt"
	"time"
	"wireflow/internal/infra"
	"wireflow/internal/log"

	"wireflow/internal/grpc"

	natsgo "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"google.golang.org/protobuf/proto"
)

var (
	_ infra.SignalService = (*NatsSignalService)(nil)
)

type SignalHandler func(ctx context.Context, peerId string, packet *grpc.SignalPacket) error

type NatsSignalService struct {
	log       *log.Logger
	nc        *natsgo.Conn
	localID   string // local  publickey
	sub       *natsgo.Subscription
	onMessage SignalHandler
}

func NewNatsService(ctx context.Context, url string) (*NatsSignalService, error) {
	nc, err := natsgo.Connect(url)
	if err != nil {
		return nil, err
	}

	s := &NatsSignalService{
		log: log.GetLogger("nats-signal"),
		nc:  nc,
	}

	// 2. 创建 JetStream 管理实例
	js, err := jetstream.New(nc)
	if err != nil {
		s.log.Error("Failed to connect to NATS JetStream", err)
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	streamName := "WIREFLOW"

	// 3. 检查 Stream 是否存在，不存在则创建
	_, err = js.Stream(ctx, streamName)
	if err != nil {
		if err == jetstream.ErrStreamNotFound {
			s.log.Info("Stream not found, creating", "stream", streamName)

			// 创建 Stream 的配置
			_, err = js.CreateStream(ctx, jetstream.StreamConfig{
				Name:     streamName,
				Subjects: []string{"signals.>"}, // 监听以 signals. 开头的所有主题
				Storage:  jetstream.FileStorage, // 持久化存储
			})
			if err != nil {
				s.log.Error("Failed to create stream", err, "stream", streamName)
				return nil, err
			}
			fmt.Println("Stream 创建成功")
		} else {
			s.log.Error("Failed to create stream", err, "stream", streamName)
			return nil, err
		}
	} else {
		s.log.Info("stream exists.")
	}
	return s, nil
}

func (s *NatsSignalService) Subscribe(subject string, onMessage SignalHandler) error {
	sub, err := s.nc.Subscribe(subject, func(m *natsgo.Msg) {
		var packet grpc.SignalPacket
		if err := proto.Unmarshal(m.Data, &packet); err != nil {
			s.log.Error("failed to unmarshal packet", err)
			return
		}
		err := onMessage(context.Background(), packet.SenderId, &packet)
		if err == nil {
			m.Ack()
		}
	})

	s.sub = sub
	if err != nil {
		return err
	}

	return nil
}

func (s *NatsSignalService) Send(ctx context.Context, peerId string, data []byte) error {
	subject := fmt.Sprintf("wireflow.signals.peers.%s", peerId)
	return s.nc.Publish(subject, data)
}

// Request req/resp
func (s *NatsSignalService) Request(ctx context.Context, subject, method string, data []byte) ([]byte, error) {
	resp, err := s.nc.Request(fmt.Sprintf("%s.%s", subject, method), data, 10*time.Second)
	if err != nil {
		return nil, err
	}

	return resp.Data, nil
}

func (s *NatsSignalService) Service(subject, queue string, service func(data []byte) ([]byte, error)) {
	s.nc.QueueSubscribe(subject, queue, func(msg *natsgo.Msg) {
		data, err := service(msg.Data)
		if err != nil {
			resp := natsgo.NewMsg(msg.Reply)
			resp.Header.Add("error", err.Error())
			resp.Header.Add("status", "400")

			msg.RespondMsg(resp)
			return
		}
		msg.Respond(data)
	})
}
