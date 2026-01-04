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
	"wireflow/internal/core/domain"
	drpgrpc "wireflow/internal/grpc"
	"wireflow/internal/log"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"

	"io"
	"time"
)

// Client signaling client
type Client struct {
	logger *log.Logger
	conn   *grpc.ClientConn
	client drpgrpc.DrpServerClient

	done   chan struct{}
	from   string
	config struct {
		heartbeatInterval time.Duration
		timeout           time.Duration
	}
	keyManager  domain.KeyManager
	messagePool *MessagePool
}

type ClientConfig struct {
	Logger       *log.Logger
	Addr         string
	ClientID     string
	KeyManager   domain.KeyManager
	SignalingUrl string
	MessagePool  *MessagePool
}

type Heart struct {
	From   string
	Status string
	Last   string
}

func NewClient(cfg *ClientConfig) (*Client, error) {

	// grpc连接优化
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithInitialWindowSize(1 << 24),
		grpc.WithInitialConnWindowSize(1 << 24),
		//compress
		grpc.WithDefaultCallOptions(
			grpc.MaxCallSendMsgSize(4*1024*1024),
			grpc.MaxCallRecvMsgSize(4*1024*1024),
		),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                30 * time.Second,
			Timeout:             10 * time.Second,
			PermitWithoutStream: true,
		}),
	}
	// Set up a connection to the server.
	conn, err := grpc.NewClient(cfg.Addr, opts...)
	if err != nil {
		cfg.Logger.Errorf("connect failed: %v", err)
		return nil, err
	}
	c := drpgrpc.NewDrpServerClient(conn)
	drpClient := &Client{
		conn:   conn,
		client: c,
		from:   cfg.ClientID,
		logger: cfg.Logger,
		config: struct {
			heartbeatInterval time.Duration
			timeout           time.Duration
		}{
			heartbeatInterval: 20 * time.Second,
			timeout:           60 * time.Second,
		},
		messagePool: cfg.MessagePool,
	}

	return drpClient, nil
}

type ClientOption func(*Client) error

func (c *Client) Configure(opts ...ClientOption) error {
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return err
		}
	}
	return nil
}

func WithKeyManager(keyManager domain.KeyManager) ClientOption {
	return func(c *Client) error {
		c.keyManager = keyManager
		return nil
	}
}

func (c *Client) HandleMessage(ctx context.Context, outBoundQueue chan *drpgrpc.DrpMessage, receive func(ctx context.Context, msg *drpgrpc.DrpMessage) error) error {
	stream, err := c.client.HandleMessage(ctx)
	if err != nil {
		return err
	}

	g, ctx := errgroup.WithContext(ctx)

	if err := stream.SendMsg(&drpgrpc.DrpMessage{
		From:    c.keyManager.GetPublicKey(),
		MsgType: drpgrpc.MessageType_MessageRegisterType,
	}); err != nil {
		return err
	}

	g.Go(func() error {
		return c.sendLoop(stream, outBoundQueue)
	})

	g.Go(func() error {
		return c.receiveLoop(stream, receive)
	})

	return g.Wait()
}

func (c *Client) receiveLoop(stream drpgrpc.DrpServer_HandleMessageClient, callback func(ctx context.Context, message *drpgrpc.DrpMessage) error) error {
	for {
		msg, err := stream.Recv()
		if err != nil {
			s, ok := status.FromError(err)
			if ok && s.Code() == codes.Canceled {
				return err
			} else if err == io.EOF {
				return err
			}

			return err
		}

		c.logger.Infof("received message msgType: %v, from %s, to: %v, data size: %v", msg.MsgType, msg.From, msg.To, len(msg.Body))
		switch msg.MsgType {
		case drpgrpc.MessageType_MessageHeartBeatType:
		default:
			callback(context.Background(), msg)
		}
	}
}

func (c *Client) sendLoop(stream drpgrpc.DrpServer_HandleMessageClient, ch chan *drpgrpc.DrpMessage) error {
	for {
		select {
		case msg := <-ch:
			if err := stream.Send(msg); err != nil {
				s, ok := status.FromError(err)
				if ok && s.Code() == codes.Canceled {
					c.logger.Infof("stream canceled")
					return err
				} else if err == io.EOF {
					c.logger.Infof("stream closed")
					return err
				}

				c.logger.Errorf("send message failed: %v", err)
				c.messagePool.ReleaseMessage(msg)
				return err
			}

			c.logger.Verbosef("send data to drp server msgType: %v, from: %v, to: %v,", msg.MsgType, msg.From, msg.To)
			c.messagePool.ReleaseMessage(msg)
		}
	}
}

func (c *Client) Close() error {
	c.logger.Infof("close signaling client connection")
	return c.conn.Close()
}
