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
	WRRP
)

func (t TransportType) String() string {
	switch t {
	case ICE:
		return "ICE"
	case WRRP:
		return "WRRP"
	default:
		return "Unknown"
	}
}

// 定义传输层优先级常量
const (
	PriorityDirect uint8 = 100 // 比如 LAN 直连
	PriorityICE    uint8 = 80  // P2P 穿透 (STUN)
	PriorityRelay  uint8 = 50  // WRRP 中转 (NATS/Server)
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
