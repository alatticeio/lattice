package relay

import (
	"encoding/json"
	"linkany/internal"
	"net"
)

const (
	defaultRelayOfferSize = 160
)

var (
	_ internal.Offer = (*RelayOffer)(nil)
)

type RelayOffer struct {
	Node       *internal.NodeMessage `json:"node,omitempty"` // Node information, if needed
	LocalKey   uint64                `json:"localKey,omitempty"`
	MappedAddr net.UDPAddr           `json:"mappedAddr,omitempty"` // remote addr
	RelayConn  net.UDPAddr           `json:"relayConn,omitempty"`
	OfferType  internal.OfferType    `json:"offerType,omitempty"` // OfferTypeRelayOffer
}

type RelayOfferConfig struct {
	OfferType  internal.OfferType
	MappedAddr net.UDPAddr
	RelayConn  net.UDPAddr
	Node       *internal.NodeMessage // Node information, if needed
}

func NewOffer(cfg *RelayOfferConfig) *RelayOffer {
	return &RelayOffer{
		MappedAddr: cfg.MappedAddr,
		RelayConn:  cfg.RelayConn,
		OfferType:  cfg.OfferType,
		Node:       cfg.Node,
	}
}

func (r *RelayOffer) Marshal() (int, []byte, error) {
	b, err := json.Marshal(r)
	if err != nil {
		return 0, nil, err
	}

	return len(b), b[:], nil
}

func (r *RelayOffer) GetOfferType() internal.OfferType {
	return internal.OfferTypeRelayOffer
}

func (r *RelayOffer) GetNode() *internal.NodeMessage {
	return r.Node
}

func UnmarshalOffer(data []byte) (*RelayOffer, error) {
	offer := &RelayOffer{}
	err := json.Unmarshal(data, offer)
	if err != nil {
		return nil, err
	}

	return offer, nil
}

func (r *RelayOffer) TieBreaker() uint64 {
	return 0
}
