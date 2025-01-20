package wrapper

import (
	"linkany/internal"
	"net"
)

var (
	_ internal.Relay = (*Relayer)(nil)
)

type Relayer struct {
	bind *NetBind
}

func NewRelayer(bind *NetBind) *Relayer {
	return &Relayer{
		bind: bind,
	}
}

func (r Relayer) AddRelayConn(addr net.Addr, relayConn net.PacketConn) error {
	return r.bind.SetEndpoint(addr, relayConn)
}
