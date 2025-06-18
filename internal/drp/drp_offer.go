package drp

import (
	"encoding/json"
	"linkany/internal"
	"linkany/management/utils"
)

var (
	_ internal.Offer = (*DrpOffer)(nil)
)

type DrpOffer struct {
	Node *utils.NodeMessage `json:"node,omitempty"` // Node information, if needed
}

type DrpOfferConfig struct {
	Node *utils.NodeMessage `json:"node,omitempty"` // Node information, if needed
}

func NewOffer(cfg *DrpOfferConfig) *DrpOffer {
	return &DrpOffer{
		Node: cfg.Node,
	}
}

func (d *DrpOffer) Marshal() (int, []byte, error) {
	b, err := json.Marshal(d)
	if err != nil {
		return 0, nil, err
	}
	return len(b), b, nil
}
func (d *DrpOffer) OfferType() internal.OfferType {
	return internal.OfferTypeDrpOffer
}

func (d *DrpOffer) TieBreaker() uint64 {
	return 0
}

func (d *DrpOffer) GetNode() *utils.NodeMessage {
	return d.Node
}

func UnmarshalOffer(data []byte) (*DrpOffer, error) {
	offer := &DrpOffer{}
	err := json.Unmarshal(data, &offer)
	if err != nil {
		return nil, err
	}
	return offer, nil
}
