package internal

import (
	"errors"
)

// drp is a protocol for relaying packets between two nodes, except stun service, drp just forward.
// as all nodes will join to the drp node，what drp do just is auth check and forward.
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
