package client

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
	"io"
	"linkany/pkg/log"
	"linkany/signaling/grpc/signaling"
	"time"
)

type Client struct {
	logger *log.Logger
	conn   *grpc.ClientConn
	client signaling.SignalingServiceClient
}

type ClientConfig struct {
	Logger *log.Logger
	Addr   string
}

func NewClient(cfg *ClientConfig) (*Client, error) {

	keepAliveArgs := keepalive.ClientParameters{
		Time:    20 * time.Second,
		Timeout: 20 * time.Second,
	}
	// Set up a connection to the server.
	conn, err := grpc.NewClient(cfg.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		cfg.Logger.Errorf("connect failed: %v", err)
		return nil, err
	}
	grpc.WithKeepaliveParams(keepAliveArgs)
	c := signaling.NewSignalingServiceClient(conn)
	return &Client{
		conn:   conn,
		client: c,
		logger: cfg.Logger,
	}, nil
}

func (c *Client) Register(ctx context.Context, in *signaling.EncryptMessage) (*signaling.EncryptMessage, error) {
	return c.client.Register(ctx, in)
}

func (c *Client) Forward(ctx context.Context, ch chan *signaling.EncryptMessage, callback func(message *signaling.EncryptMessage) error) error {
	stream, err := c.client.Forward(ctx)
	if err != nil {
		return err
	}

	defer func() {
		c.logger.Infof("close signaling stream")
		if err = stream.CloseSend(); err != nil {
			c.logger.Errorf("close send failed: %v", err)
		}
	}()

	go func() {
		for {
			select {
			case msg := <-ch:
				if err := stream.Send(msg); err != nil {
					s, ok := status.FromError(err)
					if ok && s.Code() == codes.Canceled {
						c.logger.Infof("stream canceled")
						return
					} else if err == io.EOF {
						c.logger.Infof("stream EOF")
						return
					}

					c.logger.Errorf("send message failed: %v", err)
					return
				}
			}
		}
	}()

	for {
		msg, err := stream.Recv()
		if err != nil {
			s, ok := status.FromError(err)
			if ok && s.Code() == codes.Canceled {
				c.logger.Infof("client canceled")
				return nil
			} else if err == io.EOF {
				c.logger.Infof("client closed")
				return nil
			}

			c.logger.Errorf("recv msg failed: %v", err)
			return err
		}

		if err = callback(msg); err != nil {
			c.logger.Errorf("callback failed: %v", err)
		}

	}

}

func (c *Client) Close() error {
	c.logger.Infof("close signaling client connection")
	return c.conn.Close()
}
