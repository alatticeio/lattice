package client

import (
	"context"
	"wireflow/internal"
	"wireflow/internal/grpc"
)

type ValueSetter struct {
	FieldName string
	Value     interface{}
}

type IManagementClient interface {
	GetNetMap() (*internal.Message, error)
	Register(ctx context.Context, appId string) (*internal.Peer, error)
	AddPeer(p *internal.Peer) error
	Watch(ctx context.Context, fn func(message *internal.Message) error) error
	Keepalive(ctx context.Context) error
}

type IDRPClient interface {
	HandleMessage(ctx context.Context, outBoundQueue chan *grpc.DrpMessage, receive func(ctx context.Context, msg *grpc.DrpMessage) error) error
	Close() error
}
