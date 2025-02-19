package internal

import "linkany/signaling/grpc/signaling"

type Offer interface {
	Marshal() (int, []byte, error)
}

type OfferManager interface {
	SendOffer(signaling.MessageType, string, string, Offer) error
	ReceiveOffer(message *signaling.EncryptMessage) error
}
