package client

import (
	"context"
	"encoding/json"
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

	done     chan struct{}
	clientID string
	config   struct {
		heartbeatInterval time.Duration
		timeout           time.Duration
	}
}

type ClientConfig struct {
	Logger   *log.Logger
	Addr     string
	ClientID string
}

type Heart struct {
	From   string
	Status string
	Last   string
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
		conn:     conn,
		client:   c,
		clientID: cfg.ClientID,
		logger:   cfg.Logger,
		config: struct {
			heartbeatInterval time.Duration
			timeout           time.Duration
		}{
			heartbeatInterval: 20 * time.Second,
			timeout:           60 * time.Second,
		},
	}, nil
}

//func (c *Client) Register(ctx context.Context, in *signaling.SignalingMessage) (*signaling.EncryptMessage, error) {
//	return c.client.Register(ctx, in)
//}

func (c *Client) Forward(ctx context.Context, ch chan *signaling.SignalingMessage, callback func(message *signaling.SignalingMessage) error) error {
	stream, err := c.client.Signaling(ctx)
	if err != nil {
		return err
	}

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

func (c *Client) Heartbeat(ctx context.Context, ch chan *signaling.SignalingMessage, clientId string) error {
	ticker := time.NewTicker(c.config.heartbeatInterval)
	ticker.Stop()

	sendHeart := func() error {
		heartInfo := &Heart{
			From:   clientId,
			Status: "alive",
			Last:   time.Now().Format(time.RFC3339),
		}
		body, err := json.Marshal(heartInfo)
		if err != nil {
			c.logger.Errorf("marshal heartbeat info failed: %v", err)
			return err
		}

		ch <- &signaling.SignalingMessage{
			From:    clientId,
			MsgType: signaling.MessageType_MessageHeartBeatType,
			Body:    body,
		}

		return nil
	}

	sendHeart()
	ticker.Reset(c.config.heartbeatInterval)
	for {
		select {
		case <-ctx.Done():
			c.logger.Infof("heartbeat context done: %v", ctx.Err())
			return ctx.Err()
		case <-ticker.C:
			sendHeart()
		}
	}
}

func (c *Client) receiveMessages(stream signaling.SignalingService_SignalingClient, errChan chan error, callback func(message *signaling.SignalingMessage) error) {
	for {
		msg, err := stream.Recv()
		c.logger.Verbosef("signaling received message >>>>>>>>>>>>>>>>>: %v", msg)
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

		switch msg.MsgType {
		case signaling.MessageType_MessageHeartBeatType:
			c.logger.Infof("received heartbeat message from %s, content: %v", msg.From, string(msg.Body))
		default:
			callback(msg)
		}
	}
}

func (c *Client) sendMessages(stream signaling.SignalingService_SignalingClient, ch chan *signaling.SignalingMessage, errChan chan error) {
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

			c.logger.Verbosef("send data to signaling service, from: %v, to: %v, msgType: %v", msg.From, msg.To, msg.MsgType)
		}
	}
}

func (c *Client) Close() error {
	c.logger.Infof("close signaling client connection")
	return c.conn.Close()
}
