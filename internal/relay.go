package internal

import "net"

type Relay interface {
	AddRelayConn(addr net.Addr, relayConn net.PacketConn) error
}
