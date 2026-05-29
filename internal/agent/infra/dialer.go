package infra

import (
	"context"

	"github.com/alatticeio/lattice/internal/signal"
)

type Dialer interface {
	// Prepare prepares to send offer to remoteId.
	Prepare(ctx context.Context, remoteId PeerIdentity) error

	// Handle handles incoming signal packets from remoteId.
	Handle(ctx context.Context, remoteId PeerIdentity, packet *signal.SignalPacket) error

	// Dial dials remoteId when offer is received.
	Dial(ctx context.Context) (Transport, error)

	// Close tears down the dialer and releases all resources.
	Close() error

	// Type returns the dialer type.
	Type() DialerType
}

type DialerType string

const (
	ICE_DIALER DialerType = "ICE_DIALER"
	LRP_DIALER DialerType = "LRP_DIALER"
)
