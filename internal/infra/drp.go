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

package infra

import (
	"errors"
)

// drp is a protocol for relaying packets between two peers, except stun service, drp just forward.
// as all peers will join to the drp node，what drp do just is auth check and forward.
// Header: 5byte=1 byte for frame type,4 bytes for frame length

// ProtocolVersion is the version of the protocol
const (
	ProtocolVersion = 1
)

// FrameType represents the type of frame
type FrameType byte

const (
	MessageForwardType            = FrameType(0x01) // frametype(1) + srcPubKey(4) + dstPubkey(4) + framelen(4) + payload
	MessageNodeInfoType           = FrameType(0x02) // frametype(1) + pubkey(4) + framelen(4) + payload
	MessageRegisterType           = FrameType(0x03) // frametype(1) + pubKey(4)
	MessageDirectOfferType        = FrameType(0x04) // frametype(1) + framelen(4) + srcKey + dstKey + payload
	MessageAnswerType             = FrameType(0x05) // frametype(1) + framelen(4) + payload
	MessageRelayOfferType         = FrameType(0x06) // frametype(1) + framelen(4) + srcKey + dstKey + payload
	MessageRelayOfferResponseType = FrameType(0x07) // frametype(1) + framelen(4) + srcKey + dstKey + payload
)

const MAX_PACKET_SIZE = 64 << 10

func (t FrameType) String() string {
	switch t {
	case MessageForwardType:
		return "MessageForward"
	case MessageDirectOfferType:
		return "MessageDirectOfferType"
	case MessageRelayOfferType:
		return "MessageRelayOfferType"
	default:
		return "unknown"
	}
}

var (
	ErrClientExist = errors.New("client exist")
)
