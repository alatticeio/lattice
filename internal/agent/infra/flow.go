package infra

import (
	"sync"
)

type FlowController struct {
	// activeTransport map[string]Transport
	activeTransport sync.Map // nolint
}

type TransportType int

const (
	ICE TransportType = iota
	LRP
)

func (t TransportType) String() string {
	switch t {
	case ICE:
		return "ICE"
	case LRP:
		return "LRP"
	default:
		return "Unknown"
	}
}

// Transport priority constants
const (
	PriorityDirect uint8 = 100 // e.g. LAN direct connection
	PriorityICE    uint8 = 80  // P2P NAT traversal (STUN)
	PriorityRelay  uint8 = 50  // LRP relay (NATS/Server)
)

// Transport using from read/write data from/to wire
type Transport interface {
	Write(data []byte) error
	Read(buff []byte) (int, error)
	RemoteAddr() string
	Type() TransportType
	Close() error
	Priority() uint8
}
