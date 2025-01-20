package probe

import (
	"encoding/json"
	"linkany/internal"
	"net"
)

const (
	defaultRelayOfferSize = 160
)

type OfferType int

const (
	OfferTypeRelayOffer OfferType = iota
	OfferTypeRelayOfferAnswer
)

var (
	_ internal.Offer = (*RelayOffer)(nil)
)

type RelayOffer struct {
	LocalKey   uint32      `json:"localKey,omitempty"`
	OfferType  OfferType   `json:"offerType,omitempty"`  // 0: relay offer 1: relay offer answer
	MappedAddr net.UDPAddr `json:"mappedAddr,omitempty"` // remote addr
	RelayConn  net.UDPAddr `json:"relayConn,omitempty"`
}

func NewOffer(mappedAddr, relayConn net.UDPAddr, localKey uint32, offerType OfferType) *RelayOffer {
	return &RelayOffer{
		LocalKey:   localKey,
		MappedAddr: mappedAddr,
		RelayConn:  relayConn,
		OfferType:  offerType,
	}
}

func (o *RelayOffer) Marshal() (int, []byte, error) {
	b, err := json.Marshal(o)
	if err != nil {
		return 0, nil, err
	}

	return len(b), b[:], nil
}

func UnmarshalOffer(data []byte) (*RelayOffer, error) {
	offer := &RelayOffer{}
	err := json.Unmarshal(data, offer)
	if err != nil {
		return nil, err
	}

	return offer, nil
}
