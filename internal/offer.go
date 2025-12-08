// Copyright 2025 The Wireflow Authors, Inc.
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
	"context"
	"encoding/json"
	"fmt"
	"wireflow/internal/grpc"
)

type Offer interface {
	Marshal() (int, []byte, error)
	GetOfferType() OfferType
	TieBreaker() uint64
	GetNode() *Peer
}

type OfferHandler interface {
	SendOffer(context.Context, grpc.MessageType, string, string, Offer) error
	ReceiveOffer(ctx context.Context, message *grpc.DrpMessage) error
}

type OfferType int

const (
	OfferTypeDrpOffer OfferType = iota
	OfferTypeDrpOfferAnswer
	OfferTypeDirectOffer
	OfferTypeDirectOfferAnswer
	OfferTypeRelayOffer
	OfferTypeRelayAnswer
)

type ConnType int

const (
	DirectType ConnType = iota
	RelayType
	DrpType
)

func (s ConnType) String() string {
	switch s {
	case DirectType:
		return "direct"
	case RelayType:
		return "Relay"
	case DrpType:
		return "drp"
	default:
		return "unknown"
	}
}

func (s ConnType) MarshalJSON() ([]byte, error) {
	// 将枚举值转换为字符串
	return json.Marshal(s.String())
}

var statusMap = map[string]ConnType{
	"direct": DirectType,
	"drp":    DrpType,
	"Relay":  RelayType,
}

func (s *ConnType) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	// 根据字符串设置Status值
	if status, ok := statusMap[str]; ok {
		*s = status
		return nil
	}

	return fmt.Errorf("invalid status: %s", str)
}

func UnmarshalOffer[T Offer](data []byte, t T) (T, error) {
	err := json.Unmarshal(data, t)
	if err != nil {
		var zero T
		return zero, err
	}
	return t, nil
}
