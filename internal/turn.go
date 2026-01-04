package internal

import (
	"net"
	"sync"
)

// Client
type Client interface {
	GetRelayInfo(allocated bool) (*RelayInfo, error)
}

type RelayInfo struct {
	MappedAddr net.UDPAddr
	RelayConn  net.PacketConn
}

type TurnManager struct {
	mu        sync.Mutex
	RelayInfo *RelayInfo
}

func (m *TurnManager) GetInfo() *RelayInfo {
	return m.RelayInfo
}

func (m *TurnManager) SetInfo(info *RelayInfo) {
	m.mu.Lock()
	m.RelayInfo = info
	m.mu.Unlock()
}

func AddrToUdpAddr(addr net.Addr) (*net.UDPAddr, error) {
	result, err := net.ResolveUDPAddr("udp", addr.String())
	if err != nil {
		return nil, err
	}

	return result, nil
}
