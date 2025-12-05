package client

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"time"
	"wireflow/internal"
	wgrpc "wireflow/internal/grpc"
	"wireflow/pkg/log"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
)

type GrpcConfig struct {
	Logger        *log.Logger
	Addr          string
	KeepaliveChan chan struct{}
	WatchChan     chan struct{}
}

type Client struct {
	client wgrpc.ManagementServiceClient
	logger *log.Logger
	//channel for close for keepalive
	keepaliveChan chan struct{}
	watchChan     chan struct{}
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
	c := wgrpc.NewManagementServiceClient(conn)

	return &Client{client: c, logger: cfg.Logger, keepaliveChan: cfg.KeepaliveChan, watchChan: cfg.WatchChan}, nil

}

func (c *Client) Get(ctx context.Context, in *wgrpc.ManagementMessage) (*wgrpc.ManagementMessage, error) {
	return c.client.Get(ctx, in)
}

func (c *Client) List(ctx context.Context, in *wgrpc.ManagementMessage) (*wgrpc.ManagementMessage, error) {
	return c.client.List(ctx, in)
}

func (c *Client) GetNetMap(ctx context.Context, in *wgrpc.ManagementMessage) (*wgrpc.ManagementMessage, error) {
	return c.client.GetNetMap(ctx, in)
}

func (c *Client) Login(ctx context.Context, in *wgrpc.ManagementMessage) (*wgrpc.ManagementMessage, error) {
	return c.client.Login(ctx, in)
}

func (c *Client) Watch(ctx context.Context, in *wgrpc.ManagementMessage, fn func(message *internal.Message) error) error {
	logger := c.logger
	stream, err := c.client.Watch(ctx)
	if err != nil {
		logger.Errorf("client watch failed: %v", err)
		return err
	}

	if err = stream.Send(in); err != nil {
		logger.Errorf("client watch: stream.Push(%v) failed: %v", in, err)
		return err
	}

	defer func() {
		c.logger.Infof("close watching stream")
		if err = stream.CloseSend(); err != nil {
			logger.Errorf("close send failed: %v", err)
		}
	}()

	errChan := make(chan error, 1)
	go func() {
		for {
			select {
			case <-c.watchChan:
				logger.Infof("watching stream closed by user")
				return
			default:
				in, err := stream.Recv()
				if err == io.EOF {
					c.logger.Errorf("%v", err)
					errChan <- err
					return
				}
				if err != nil {
					logger.Errorf("err: %v", err)
					errChan <- err
					return
				}

				var message internal.Message
				if err := json.Unmarshal(in.Body, &message); err != nil {
					logger.Errorf("Failed to parse network map: %v", err)
					errChan <- err
					return
				}

				if err = fn(&message); err != nil {
					c.logger.Errorf("Failed to callback: %v", err)
					errChan <- err
					return
				}
			}

		}
	}()

	return <-errChan
}

func (c *Client) Keepalive(ctx context.Context, in *wgrpc.ManagementMessage) error {
	stream, err := c.client.Keepalive(ctx)
	var errChan = make(chan error, 1)
	if err != nil {
		c.logger.Errorf("client keep alive failed: %v", err)
		return err
	}

	if err = stream.Send(in); err != nil {
		c.logger.Errorf("client keepalive: stream.Push(%v) failed: %v", in, err)
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
		select {
		case <-c.keepaliveChan:
			c.logger.Infof("keepalive stream closed by user")
			return nil
		default:
			msg, err := stream.Recv()
			s, ok := status.FromError(err)

			if ok && s.Code() == codes.Canceled {
				errChan <- err
				return err
			} else if err == io.EOF {
				errChan <- err
				return err
			} else if err != nil {
				c.logger.Errorf("receive check living packet failed: %v", err)
				errChan <- err
				return err
			}

			var req wgrpc.Request
			if err = proto.Unmarshal(msg.Body, &req); err != nil {
				c.logger.Errorf("failed unmarshal check packet: %v", err)
				return err
			}
			c.logger.Verbosef("receive check living packet from server: %v, elapsed: %v", &req, time.Since(time.UnixMilli(msg.Timestamp)).Milliseconds())

			if err = stream.Send(in); err != nil {
				if errors.Is(err, io.EOF) {
					c.logger.Errorf("server closed the stream")
					return nil
				}
				c.logger.Errorf("send check living packet failed: %v", err)
			}
		}

	}

	//select {
	//case err = <-errChan:
	//	if err == io.EOF {
	//		return nil
	//	}
	//	c.logger.Errorf("keepalive failed: %v", err)
	//	return err
	//}

}

// VerifyToken verify token for sso
func (c *Client) VerifyToken(token string) (*wgrpc.LoginResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	in := &wgrpc.Request{Token: token}

	body, err := proto.Marshal(in)
	if err != nil {
		return nil, err
	}

	resp, err := c.client.VerifyToken(ctx, &wgrpc.ManagementMessage{Body: body})
	if err != nil {
		return nil, err
	}

	var loginResp wgrpc.LoginResponse
	if err = proto.Unmarshal(resp.Body, &loginResp); err != nil {
		return nil, err
	}

	return &loginResp, nil
}

func (c *Client) Registry(ctx context.Context, in *wgrpc.ManagementMessage) (*wgrpc.ManagementMessage, error) {
	return c.client.Registry(ctx, in)
}

func (c *Client) Request(ctx context.Context, in *wgrpc.ManagementMessage) (*wgrpc.ManagementMessage, error) {
	return c.client.Do(ctx, in)
}
