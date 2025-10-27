package direct

import (
	"encoding/binary"
	"encoding/json"
	"wireflow/internal"
)

var (
	_ internal.Offer = (*DirectOffer)(nil)
)

type DirectOffer struct {
	WgPort    uint32         `json:"wgPort,omitempty"`     // WireGuard port
	Ufrag     string         `json:"ufrag,omitempty"`      // ICE username fragment
	Pwd       string         `json:"pwd,omitempty"`        // ICE password
	LocalKey  uint64         `json:"localKey,omitempty"`   // local key for tie breaker
	Candidate string         `json:"candidate, omitempty"` // ; separated
	Node      *internal.Node `json:"node,omitempty"`       // Node information, if needed
}

type DirectOfferConfig struct {
	WgPort     uint32
	Ufrag      string
	Pwd        string
	LocalKey   uint64
	Candidates string
	Node       *internal.Node
}

func NewOffer(config *DirectOfferConfig) *DirectOffer {
	return &DirectOffer{
		WgPort:    config.WgPort,
		Candidate: config.Candidates,
		Ufrag:     config.Ufrag,
		Pwd:       config.Pwd,
		LocalKey:  config.LocalKey,
		Node:      config.Node,
	}
}

var bin = binary.BigEndian

func (o *DirectOffer) Marshal() (int, []byte, error) {
	b, err := json.Marshal(o)
	if err != nil {
		return 0, nil, err
	}
	return len(b), b, nil
}

func (o *DirectOffer) GetOfferType() internal.OfferType {
	return internal.OfferTypeDirectOffer
}

func (o *DirectOffer) TieBreaker() uint64 {
	return o.LocalKey
}

func (o *DirectOffer) len() int {
	return 64 + len(o.Candidate)
}

func (o *DirectOffer) GetNode() *internal.Node {
	return o.Node
}

func UnmarshalOffer(data []byte) (*DirectOffer, error) {
	offer := &DirectOffer{}
	err := json.Unmarshal(data, &offer)
	if err != nil {
		return nil, err
	}
	return offer, nil
}
