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

package drp

import (
	"encoding/json"
	"wireflow/internal"
)

var (
	_ internal.Offer = (*DrpOffer)(nil)
)

type DrpOffer struct {
	Node *internal.Node `json:"node,omitempty"` // Node information, if needed
}

type DrpOfferConfig struct {
	Node *internal.Node `json:"node,omitempty"` // Node information, if needed
}

func NewDrpOffer(cfg *DrpOfferConfig) *DrpOffer {
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
func (d *DrpOffer) GetOfferType() internal.OfferType {
	return internal.OfferTypeDrpOffer
}

func (d *DrpOffer) TieBreaker() uint64 {
	return 0
}

func (d *DrpOffer) GetNode() *internal.Node {
	return d.Node
}
