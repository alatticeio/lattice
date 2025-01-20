package internal

import "github.com/linkanyio/ice"

type SignalServer interface {
	SignalOffer(offer DirectOffer, remoteKey string) error
	SignalAnswer(offer DirectOffer, remoteKey string) error
	SignalICECandidate(candidate ice.Candidate, remoteKey string) error
	Ready() bool
}

type Server struct {
}
