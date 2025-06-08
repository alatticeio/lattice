package internal

import "linkany/signaling/grpc/signaling"

type Offer interface {
	Marshal() (int, []byte, error)
	IsDirectOffer() bool
	TieBreaker() uint64
}

type OfferHandler interface {
	SendOffer(signaling.MessageType, string, string, Offer) error
	ReceiveOffer(message *signaling.SignalingMessage) error
}
