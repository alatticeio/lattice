package client

import (
	"context"
	"errors"
	"flag"
	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"io"
	"k8s.io/klog/v2"
	"linkany/management/grpc/mgt"
	"time"
)

type GrpcConfig struct {
	Addr string
}

type Client struct {
	client mgt.ManagementServiceClient
}

func NewClient(config *GrpcConfig) (*Client, error) {
	flag.Parse()
	keepAliveArgs := keepalive.ClientParameters{
		Time:    20 * time.Second,
		Timeout: 20 * time.Second,
	}
	// Set up a connection to the server.
	conn, err := grpc.NewClient(config.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		klog.Errorf("did not connect: %v", err)
		return nil, err
	}
	grpc.WithKeepaliveParams(keepAliveArgs)
	c := mgt.NewManagementServiceClient(conn)

	return &Client{client: c}, nil

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
	stream, err := c.client.Watch(ctx)
	if err != nil {
		klog.Errorf("client watch failed: %v", err)
	}

	if err = stream.Send(in); err != nil {
		klog.Errorf("client watch: stream.Send(%v) failed: %v", in, err)
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
				klog.Errorf("err: %v", err)
			}

			var watchMessage mgt.WatchMessage
			if err := proto.Unmarshal(in.Body, &watchMessage); err != nil {
				klog.Errorf("Failed to parse network map: %v", err)
				continue
			}

			if err = callback(&watchMessage); err != nil {
				klog.Errorf("Failed to callback: %v", err)
			}
		}
	}()

	<-ch
	return nil
}

func (c *Client) Keepalive(ctx context.Context, in *mgt.ManagementMessage) error {
	stream, err := c.client.Keepalive(ctx)
	var errChan = make(chan error, 1)
	if err != nil {
		klog.Errorf("client keep alive failed: %v", err)
		return err
	}

	if err = stream.Send(in); err != nil {
		klog.Errorf("client keepalive: stream.Send(%v) failed: %v", in, err)
	}
	defer func() {
		if err = stream.CloseSend(); err != nil {
			klog.Errorf("close send failed: %v", err)
		}
		err = <-errChan
		if err != nil {
			klog.Errorf("keepalive failed: %v", err)
		}

	}()

	for {
		msg, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			klog.Warningf("server closed the stream")
			return nil
		} else if err != nil {
			klog.Errorf("recv msg failed: %v", err)
			errChan <- err
			return err
		}

		var req mgt.Request
		if err = proto.Unmarshal(msg.Body, &req); err != nil {
			klog.Errorf("failed unmarshal check packet: %v", err)
			return err
		}
		klog.Infof("got check packet from server: %v", &req)

		if err = stream.Send(in); err != nil {
			if errors.Is(err, io.EOF) {
				klog.Warningf("server closed the stream")
				return nil
			}
			klog.Errorf("send check packet failed: %v", err)
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
