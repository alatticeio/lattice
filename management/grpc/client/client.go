package client

import (
	"context"
	"errors"
	"flag"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
	"io"
	"linkany/management/grpc/mgt"
	"linkany/pkg/log"
	"time"
)

type GrpcConfig struct {
	Logger *log.Logger
	Addr   string
}

type Client struct {
	client mgt.ManagementServiceClient
	logger *log.Logger
}

func NewClient(cfg *GrpcConfig) (*Client, error) {
	flag.Parse()
	keepAliveArgs := keepalive.ClientParameters{
		Time:    20 * time.Second,
		Timeout: 20 * time.Second,
	}
	// Set up a connection to the server.
	conn, err := grpc.NewClient(cfg.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		cfg.Logger.Errorf("did not connect: %v", err)
		return nil, err
	}
	grpc.WithKeepaliveParams(keepAliveArgs)
	c := mgt.NewManagementServiceClient(conn)

	return &Client{client: c, logger: cfg.Logger}, nil

}

func (c *Client) Get(ctx context.Context, in *mgt.ManagementMessage) (*mgt.ManagementMessage, error) {
	return c.client.Get(ctx, in)
}

func (c *Client) List(ctx context.Context, in *mgt.ManagementMessage) (*mgt.ManagementMessage, error) {
	return c.client.List(ctx, in)
}

func (c *Client) Login(ctx context.Context, in *mgt.ManagementMessage) (*mgt.ManagementMessage, error) {
	return c.client.Login(ctx, in)
}

func (c *Client) Watch(ctx context.Context, in *mgt.ManagementMessage, callback func(wm *mgt.WatchMessage) error) error {
	//ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	//defer cancel()
	logger := c.logger
	stream, err := c.client.Watch(ctx)
	if err != nil {
		logger.Errorf("client watch failed: %v", err)
	}

	if err = stream.Send(in); err != nil {
		logger.Errorf("client watch: stream.Send(%v) failed: %v", in, err)
	}

	ch := make(chan struct{})
	go func() {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				// read done.
				close(ch)
				return
			}
			if err != nil {
				logger.Errorf("err: %v", err)
			}

			var watchMessage mgt.WatchMessage
			if err := proto.Unmarshal(in.Body, &watchMessage); err != nil {
				logger.Errorf("Failed to parse network map: %v", err)
				continue
			}

			if err = callback(&watchMessage); err != nil {
				c.logger.Errorf("Failed to callback: %v", err)
			}
		}
	}()

	c.logger.Verbosef("client watching peers events")
	<-ch
	c.logger.Verbosef("client break watching peers events")
	return nil
}

func (c *Client) Keepalive(ctx context.Context, in *mgt.ManagementMessage) error {
	stream, err := c.client.Keepalive(ctx)
	var errChan = make(chan error, 1)
	if err != nil {
		c.logger.Errorf("client keep alive failed: %v", err)
		return err
	}

	if err = stream.Send(in); err != nil {
		c.logger.Errorf("client keepalive: stream.Send(%v) failed: %v", in, err)
	}
	defer func() {
		if err = stream.CloseSend(); err != nil {
			c.logger.Errorf("close send failed: %v", err)
		}
		err = <-errChan
		if err != nil {
			c.logger.Errorf("keepalive failed: %v", err)
		}

	}()

	for {
		msg, err := stream.Recv()
		s, ok := status.FromError(err)
		if ok && s.Code() == codes.Canceled {
			c.logger.Infof("stream canceled")
			return err
		} else if err == io.EOF {
			c.logger.Infof("stream EOF")
			return err
		}

		var req mgt.Request
		if err = proto.Unmarshal(msg.Body, &req); err != nil {
			c.logger.Errorf("failed unmarshal check packet: %v", err)
			return err
		}
		c.logger.Infof("receive check living packet from server: %v", &req)

		if err = stream.Send(in); err != nil {
			if errors.Is(err, io.EOF) {
				c.logger.Errorf("server closed the stream")
				return nil
			}
			c.logger.Errorf("send check living packet failed: %v", err)
		}

	}

}

// VerifyToken verify token for sso
func (c *Client) VerifyToken(token string) (*mgt.LoginResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	in := &mgt.Request{Token: token}

	body, err := proto.Marshal(in)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.VerifyToken(ctx, &mgt.ManagementMessage{Body: body})
	if err != nil {
		return nil, err
	}

	var loginResp mgt.LoginResponse
	if err = proto.Unmarshal(resp.Body, &loginResp); err != nil {
		return nil, err
	}

	return &loginResp, nil
}

func (c *Client) Registry(ctx context.Context, in *mgt.ManagementMessage) (*mgt.ManagementMessage, error) {
	return c.client.Registry(ctx, in)
}
