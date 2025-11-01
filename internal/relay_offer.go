package internal

import (
	"encoding/json"
	"net"
)

const (
	defaultRelayOfferSize = 160
)

var (
	_ Offer = (*RelayOffer)(nil)
)

type RelayOffer struct {
	Node       *Node       `json:"node,omitempty"` // Node information, if needed
	LocalKey   uint64      `json:"localKey,omitempty"`
	MappedAddr net.UDPAddr `json:"mappedAddr,omitempty"` // remote addr
	RelayConn  net.UDPAddr `json:"relayConn,omitempty"`
	OfferType  OfferType   `json:"offerType,omitempty"` // OfferTypeRelayOffer
}

type RelayOfferConfig struct {
	OfferType  OfferType
	MappedAddr net.UDPAddr
	RelayConn  net.UDPAddr
	Node       *Node // Node information, if needed
}

func NewRelayOffer(cfg *RelayOfferConfig) *RelayOffer {
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

func (r *RelayOffer) GetOfferType() OfferType {
	return OfferTypeRelayOffer
}

func (r *RelayOffer) GetNode() *Node {
	return r.Node
}

func (r *RelayOffer) TieBreaker() uint64 {
	return 0
}
