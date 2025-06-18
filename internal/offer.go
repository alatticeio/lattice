package internal

import (
	"encoding/json"
	drpgrpc "linkany/drp/grpc"
	"linkany/management/utils"
)

type Offer interface {
	Marshal() (int, []byte, error)
	OfferType() OfferType
	TieBreaker() uint64
	GetNode() *utils.NodeMessage
}

type OfferHandler interface {
	SendOffer(drpgrpc.MessageType, string, string, Offer) error
	ReceiveOffer(message *drpgrpc.DrpMessage) error
}

type OfferType int

const (
	OfferTypeDrpOffer OfferType = iota
	OfferTypeDrpOfferAnswer
	OfferTypeDirectOffer
	OfferTypeDirectOfferAnswer
	OfferTypeRelayOffer
	OfferTypeRelayAnswer
)

type ConnType int

const (
	DirectType ConnType = iota
	RelayType
	DrpType
)

func UnmarshalOffer[T Offer](data []byte, t T) (T, error) {
	err := json.Unmarshal(data, t)
	if err != nil {
		var zero T
		return zero, err
	}
	return t, nil
}
