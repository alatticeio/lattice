package infra

import (
	"context"
	"wireflow/internal/grpc"
)

type Dialer interface {
	// Prepare prepare offer will send to remoteId.`
	Prepare(ctx context.Context, remoteId PeerID) error

	// Handle using for handle si
	Handle(ctx context.Context, remoteId PeerID, packet *grpc.SignalPacket) error

	// Dial dial remoteId when receive offer
	Dial(ctx context.Context) (Transport, error)

	// Type return the dialer type
	Type() DialerType
}

type DialerType string

const (
	ICE_DIALER  DialerType = "ICE_DIALER"
	WRRP_DIALER DialerType = "WRRP_DIALER"
)
