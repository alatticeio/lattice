package internal

import (
	"golang.zx2c4.com/wireguard/conn"
	"net/netip"
)

type DrpEndpoint interface {
	// FromDrp if drp, return true, address, nil
	FromDrp() bool
	From() string
	To() string
}

// LinkEndpoint is a connection endpoint that represents a link layer endpoint
// with a DRP (Direct Routing Protocol) status, a direct address and port,
// and a sticky source address and interface index if supported.
// It implements the conn.Endpoint interface.
// It is used to represent a connection endpoint in the WireGuard context.
var (
	_ conn.Endpoint = (*LinkEndpoint)(nil)
	_ DrpEndpoint   = (*LinkEndpoint)(nil)
)

type LinkEndpoint struct {
	Drp *struct {
		Status   bool
		From     string
		To       string
		Endpoint netip.AddrPort
	}

	// AddrPort is the endpoint destination.
	Direct struct {
		AddrPort netip.AddrPort
	}

	// src is the current sticky source address and interface index, if supported.
	src struct {
		netip.Addr
		ifidx int32
	}
}

func (e *LinkEndpoint) From() string {
	return e.Drp.From
}

func (e *LinkEndpoint) To() string {
	return e.Drp.To
}

func (e *LinkEndpoint) FromDrp() bool {
	if e.Drp != nil {
		return true
	}

	return false
}

var (
	_ conn.Endpoint = &LinkEndpoint{}
)

func (e *LinkEndpoint) ClearSrc() {
	e.src.ifidx = 0
	e.src.Addr = netip.Addr{}
}

func (e *LinkEndpoint) DstIP() netip.Addr {
	if !e.FromDrp() {
		return e.Direct.AddrPort.Addr()
	} else {
		return e.Drp.Endpoint.Addr()

	}
}

func (e *LinkEndpoint) SrcIP() netip.Addr {
	return e.src.Addr
}

func (e *LinkEndpoint) SrcIfidx() int32 {
	return e.src.ifidx
}

func (e *LinkEndpoint) DstToBytes() []byte {
	var (
		b []byte
	)
	if !e.FromDrp() {
		b, _ = e.Direct.AddrPort.MarshalBinary()
	} else {
		b, _ = e.Drp.Endpoint.MarshalBinary()
	}
	return b
}

func (e *LinkEndpoint) DstToString() string {
	if !e.FromDrp() {
		return e.Direct.AddrPort.String()
	}
	return e.Drp.Endpoint.String()
}

func (e *LinkEndpoint) SrcToString() string {
	return e.src.Addr.String()
}
