package internal

import "context"

// Checker is the interface for checking the connection.
// DirectChecker and RelayChecker are the two implementations.
type Checker interface {

	// HandleOffer will be called when an offer is received
	HandleOffer(offer Offer) error

	// ProbeConnect probes the connection
	ProbeConnect(ctx context.Context, isControlling bool, remoteOffer Offer) error

	// ProbeSuccess will be called when the connection is successful, will add peer to wireguard
	ProbeSuccess(addr string) error

	// ProbeFailure will be called when the connection failed, will remove peer from wireguard
	ProbeFailure(offer Offer) error
}
