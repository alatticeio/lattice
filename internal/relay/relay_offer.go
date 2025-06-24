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
	LocalKey   uint64      `json:"localKey,omitempty"`
	MappedAddr net.UDPAddr `json:"mappedAddr,omitempty"` // remote addr
	RelayConn  net.UDPAddr `json:"relayConn,omitempty"`
}

type RelayOfferConfig struct {
	LocalKey   uint64
	OfferType  internal.OfferType
	MappedAddr net.UDPAddr
	RelayConn  net.UDPAddr
}

func NewOffer(cfg *RelayOfferConfig) *RelayOffer {
	return &RelayOffer{
		LocalKey:   cfg.LocalKey,
		MappedAddr: cfg.MappedAddr,
		RelayConn:  cfg.RelayConn,
	}
}

func (o *RelayOffer) Marshal() (int, []byte, error) {
	b, err := json.Marshal(o)
	if err != nil {
		return 0, nil, err
	}

	return len(b), b[:], nil
}

func (o *RelayOffer) OfferType() internal.OfferType {
	return internal.OfferTypeRelayOffer
}

func (o *RelayOffer) GetNode() *internal.NodeMessage {
	return nil
}

func UnmarshalOffer(data []byte) (*RelayOffer, error) {
	offer := &RelayOffer{}
	err := json.Unmarshal(data, offer)
	if err != nil {
		return nil, err
	}

	return offer, nil
}

func (o *RelayOffer) TieBreaker() uint64 {
	return 0
}
