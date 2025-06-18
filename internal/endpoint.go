package internal

import (
	"golang.zx2c4.com/wireguard/conn"
	"net/netip"
)

type RemoteEndpoint struct {
	IsDrp bool // is use drp transport server
	// AddrPort is the endpoint destination.
	netip.AddrPort
	// src is the current sticky source address and interface index, if supported.
	src struct {
		netip.Addr
		ifidx int32
	}
}

var (
	_ conn.Endpoint = &RemoteEndpoint{}
)

func (e *RemoteEndpoint) ClearSrc() {
	e.src.ifidx = 0
	e.src.Addr = netip.Addr{}
}

func (e *RemoteEndpoint) DstIP() netip.Addr {
	return e.AddrPort.Addr()
}

func (e *RemoteEndpoint) SrcIP() netip.Addr {
	return e.src.Addr
}

func (e *RemoteEndpoint) SrcIfidx() int32 {
	return e.src.ifidx
}

func (e *RemoteEndpoint) DstToBytes() []byte {
	b, _ := e.AddrPort.MarshalBinary()
	return b
}

func (e *RemoteEndpoint) DstToString() string {
	return e.AddrPort.String()
}

func (e *RemoteEndpoint) SrcToString() string {
	return e.src.Addr.String()
}
