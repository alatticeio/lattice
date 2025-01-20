package internal

import (
	"encoding/binary"
)

var (
	_ Offer = (*DirectOffer)(nil)
)

type DirectOffer struct {
	WgPort    uint32
	Ufrag     string
	Pwd       string
	LocalKey  uint32
	Candidate string // ; separated
}

type DirectOfferConfig struct {
	WgPort     uint32
	Ufrag      string
	Pwd        string
	LocalKey   uint32
	Candidates string
}

func NewDirectOffer(config *DirectOfferConfig) *DirectOffer {
	return &DirectOffer{
		WgPort:    config.WgPort,
		Candidate: config.Candidates,
		Ufrag:     config.Ufrag,
		Pwd:       config.Pwd,
		LocalKey:  config.LocalKey,
	}
}

var bin = binary.BigEndian

func (offer *DirectOffer) Marshal() (int, []byte, error) {
	b := make([]byte, offer.len())
	bin.PutUint32(b[0:4], offer.WgPort)     //4
	copy(b[4:28], offer.Ufrag[:])           //24
	copy(b[28:60], offer.Pwd[:])            //32
	bin.PutUint32(b[60:64], offer.LocalKey) //4
	copy(b[64:], offer.Candidate[:])        //1024

	return len(b), b, nil
}

func (offer *DirectOffer) len() int {
	return 64 + len(offer.Candidate)
}

func UnmarshalOfferAnswer(data []byte) (*DirectOffer, error) {
	offer := &DirectOffer{}
	offer.WgPort = binary.BigEndian.Uint32(data[0:4])
	offer.Ufrag = string(data[4:28])
	offer.Pwd = string(data[28:60])
	offer.LocalKey = binary.BigEndian.Uint32(data[60:64])
	offer.Candidate = string(data[64:])
	return offer, nil
}
