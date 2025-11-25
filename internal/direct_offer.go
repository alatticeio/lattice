// Copyright 2025 Wireflow.io, Inc.
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
	"encoding/binary"
	"encoding/json"

	"k8s.io/klog/v2"
)

var (
	_ Offer = (*DirectOffer)(nil)
)

type DirectOffer struct {
	WgPort    uint32 `json:"wgPort,omitempty"`     // WireGuard port
	Ufrag     string `json:"ufrag,omitempty"`      // ICE username fragment
	Pwd       string `json:"pwd,omitempty"`        // ICE password
	LocalKey  uint64 `json:"localKey,omitempty"`   // local key for tie breaker
	Candidate string `json:"candidate, omitempty"` // ; separated
	Node      *Peer  `json:"node,omitempty"`       // Node information, if needed
}

type DirectOfferConfig struct {
	WgPort     uint32
	Ufrag      string
	Pwd        string
	LocalKey   uint64
	Candidates string
	Node       *Peer
}

func NewDirectOffer(config *DirectOfferConfig) *DirectOffer {
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

	klog.Infof("send offer is : %v", string(b))
	return len(b), b, nil
}

func (o *DirectOffer) GetOfferType() OfferType {
	return OfferTypeDirectOffer
}

func (o *DirectOffer) TieBreaker() uint64 {
	return o.LocalKey
}

func (o *DirectOffer) len() int {
	return 64 + len(o.Candidate)
}

func (o *DirectOffer) GetNode() *Peer {
	return o.Node
}
