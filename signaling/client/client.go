package client

import (
	"context"
	"fmt"
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
		Time:                10 * time.Second,
		Timeout:             3 * time.Second,
		PermitWithoutStream: true,
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

	errChan := make(chan error)
	go c.sendMessages(stream, ch, errChan)
	go c.receiveMessages(stream, errChan, callback)

	select {
	case err = <-errChan:
		if err == io.EOF {
			return nil
		}

		if status.Code(err) == codes.Canceled {
			c.logger.Infof("stream closed")
			return nil
		}

		return err
	}
}

func (c *Client) receiveMessages(stream signaling.SignalingService_ForwardClient, errChan chan error, callback func(message *signaling.EncryptMessage) error) {
	for {
		msg, err := stream.Recv()
		if err != nil {
			s, ok := status.FromError(err)
			if ok && s.Code() == codes.Canceled {
				c.logger.Infof("stream canceled")
				errChan <- fmt.Errorf("stream canceled")
				return
			} else if err == io.EOF {
				c.logger.Infof("stream closed")
				errChan <- fmt.Errorf("stream closed")
				return
			}

			c.logger.Errorf("recv message failed: %v", err)
			errChan <- fmt.Errorf("recv message failed: %v", err)
			return
		}

		callback(msg)
	}
}

func (c *Client) sendMessages(stream signaling.SignalingService_ForwardClient, ch chan *signaling.EncryptMessage, errChan chan error) {
	for {
		select {
		case msg := <-ch:
			if err := stream.Send(msg); err != nil {
				s, ok := status.FromError(err)
				if ok && s.Code() == codes.Canceled {
					c.logger.Infof("stream canceled")
					errChan <- fmt.Errorf("stream canceled")
					return
				} else if err == io.EOF {
					c.logger.Infof("stream closed")
					errChan <- fmt.Errorf("stream closed")
					return
				}

				c.logger.Errorf("send message failed: %v", err)
				errChan <- fmt.Errorf("send message failed: %v", err)
				return
			}
		}
	}
}

func (c *Client) Close() error {
	c.logger.Infof("close signaling client connection")
	return c.conn.Close()
}
