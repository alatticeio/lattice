package infra

import (
	"context"
	"wireflow/internal/grpc"
)

type Dialer interface {
	// Prepare prepares to send offer to remoteId.
	Prepare(ctx context.Context, remoteId PeerIdentity) error

	// Handle handles incoming signal packets from remoteId.
	Handle(ctx context.Context, remoteId PeerIdentity, packet *grpc.SignalPacket) error

	// Dial dials remoteId when offer is received.
	Dial(ctx context.Context) (Transport, error)

	// Type returns the dialer type.
	Type() DialerType
}

type DialerType string

const (
	ICE_DIALER  DialerType = "ICE_DIALER"
	WRRP_DIALER DialerType = "WRRP_DIALER"
)
