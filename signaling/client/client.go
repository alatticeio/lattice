package client

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
	"io"
	"k8s.io/klog/v2"
	"linkany/signaling/grpc/signaling"
	"time"
)

type Client struct {
	conn   *grpc.ClientConn
	client signaling.SignalingServiceClient
}

type ClientConfig struct {
	Addr string
}

func NewClient(cfg *ClientConfig) (*Client, error) {

	keepAliveArgs := keepalive.ClientParameters{
		Time:    20 * time.Second,
		Timeout: 20 * time.Second,
	}
	// Set up a connection to the server.
	conn, err := grpc.NewClient(cfg.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		klog.Errorf("connect failed: %v", err)
		return nil, err
	}
	grpc.WithKeepaliveParams(keepAliveArgs)
	c := signaling.NewSignalingServiceClient(conn)
	return &Client{
		conn:   conn,
		client: c,
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
		klog.Infof("close signaling stream")
		if err = stream.CloseSend(); err != nil {
			klog.Errorf("close send failed: %v", err)
		}
	}()

	go func() {
		for {
			select {
			case msg := <-ch:
				if err := stream.Send(msg); err != nil {
					s, ok := status.FromError(err)
					if ok && s.Code() == codes.Canceled {
						klog.Infof("stream canceled")
						return
					} else if err == io.EOF {
						klog.Infof("stream EOF")
						return
					}

					klog.Errorf("send message failed: %v", err)
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
				klog.Infof("client canceled")
				return nil
			} else if err == io.EOF {
				klog.Infof("client closed")
				return nil
			}

			klog.Errorf("recv msg failed: %v", err)
		}

		if err = callback(msg); err != nil {
			klog.Errorf("callback failed: %v", err)
		}

	}

}

func (c *Client) Close() error {
	klog.Infof("close signaling client connection")
	return c.conn.Close()
}
