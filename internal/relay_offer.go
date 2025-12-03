// Copyright 2025 wireflowio.com, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	Node       *Peer       `json:"node,omitempty"` // Node information, if needed
	LocalKey   uint64      `json:"localKey,omitempty"`
	MappedAddr net.UDPAddr `json:"mappedAddr,omitempty"` // remote addr
	RelayConn  net.UDPAddr `json:"relayConn,omitempty"`
	OfferType  OfferType   `json:"offerType,omitempty"` // OfferTypeRelayOffer
}

type RelayOfferConfig struct {
	OfferType  OfferType
	MappedAddr net.UDPAddr
	RelayConn  net.UDPAddr
	Node       *Peer // Node information, if needed
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

func (r *RelayOffer) GetNode() *Peer {
	return r.Node
}

func (r *RelayOffer) TieBreaker() uint64 {
	return 0
}
