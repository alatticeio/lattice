package internal

import (
	"golang.zx2c4.com/wireguard/conn"
	"net/netip"
)

type EndpointType int

const (
	DRP EndpointType = iota
	Relay
	Direct
)

type RelayEndpoint interface {
	FromType() EndpointType
	From() string
	To() string
}

type Endpoint interface {
	FromType() EndpointType
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
	_ Endpoint      = (*LinkEndpoint)(nil)
)

type LinkEndpoint struct {
	Relay *struct {
		// FromType indicates the type of the endpoint.
		FromType EndpointType
		Status   bool
		// From is the source address of the Relay.
		From string
		// To is the destination address of the Relay.
		To string
		// Endpoint is the Relay endpoint address.
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
	return e.Relay.From
}

func (e *LinkEndpoint) To() string {
	return e.Relay.To
}

func (e *LinkEndpoint) FromType() EndpointType {
	if e.Relay != nil {
		return e.Relay.FromType
	}
	return Direct
}

var (
	_ conn.Endpoint = &LinkEndpoint{}
)

func (e *LinkEndpoint) ClearSrc() {
	e.src.ifidx = 0
	e.src.Addr = netip.Addr{}
}

func (e *LinkEndpoint) DstIP() netip.Addr {
	switch e.FromType() {
	case Direct:
		return e.Direct.AddrPort.Addr()
	default:
		return e.Relay.Endpoint.Addr()
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

	switch e.FromType() {
	case Direct:
		b, _ = e.Direct.AddrPort.MarshalBinary()
	default:
		b, _ = e.Relay.Endpoint.MarshalBinary()
	}

	return b
}

func (e *LinkEndpoint) DstToString() string {
	switch e.FromType() {
	case Direct:
		return e.Direct.AddrPort.String()
	default:
		return e.Relay.Endpoint.String()
	}
}

func (e *LinkEndpoint) SrcToString() string {
	return e.src.Addr.String()
}
