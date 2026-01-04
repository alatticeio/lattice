package probe

import (
	"context"
	"encoding/json"
	"wireflow/internal/core/domain"
	"wireflow/internal/grpc"
)

type PacketHandler struct {
	factory    *TransportFactory
	handleFunc func(context.Context, *domain.Message)
}

func NewPacketHandler(factory *TransportFactory, handleFunc func(ctx context.Context, msg *domain.Message)) *PacketHandler {
	return &PacketHandler{
		factory:    factory,
		handleFunc: handleFunc,
	}
}

func (p *PacketHandler) HandleSignal(ctx context.Context, peerId string, packet *grpc.SignalPacket) error {
	switch packet.Type {
	case grpc.PacketType_MESSAGE:
		var msg domain.Message
		if err := json.Unmarshal(packet.GetMessage().Content, &msg); err != nil {
			return err
		}
		p.handleFunc(ctx, &msg)
	default:
		if err := p.factory.HandleSignal(ctx, peerId, packet); err != nil {
			return err
		}
	}

	return nil
}
