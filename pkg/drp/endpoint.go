package drp

import (
	"golang.zx2c4.com/wireguard/conn"
	"net/netip"
)

type AnyEndpoint struct {
	IsRelay bool // is use relay transport?
	// AddrPort is the endpoint destination.
	netip.AddrPort
	// src is the current sticky source address and interface index, if supported.
	src struct {
		netip.Addr
		ifidx int32
	}
}

var (
	_ conn.Endpoint = &AnyEndpoint{}
)

func (e *AnyEndpoint) ClearSrc() {
	e.src.ifidx = 0
	e.src.Addr = netip.Addr{}
}

func (e *AnyEndpoint) DstIP() netip.Addr {
	return e.AddrPort.Addr()
}

func (e *AnyEndpoint) SrcIP() netip.Addr {
	return e.src.Addr
}

func (e *AnyEndpoint) SrcIfidx() int32 {
	return e.src.ifidx
}

func (e *AnyEndpoint) DstToBytes() []byte {
	b, _ := e.AddrPort.MarshalBinary()
	return b
}

func (e *AnyEndpoint) DstToString() string {
	return e.AddrPort.String()
}

func (e *AnyEndpoint) SrcToString() string {
	return e.src.Addr.String()
}
