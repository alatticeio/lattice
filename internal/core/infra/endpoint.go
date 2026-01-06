// Copyright 2025 The Wireflow Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package infra

import (
	"net/netip"

	"golang.zx2c4.com/wireguard/conn"
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

// MagicEndpoint is a connection endpoint that represents a link layer endpoint
// with a DRP (Direct Routing Protocol) status, a direct address and port,
// and a sticky source address and interface index if supported.
// It implements the conn.Endpoint interface.
// It is used to represent a connection endpoint in the WireGuard context.
var (
	_ conn.Endpoint = (*MagicEndpoint)(nil)
	_ Endpoint      = (*MagicEndpoint)(nil)
)

type MagicEndpoint struct {
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

func (e *MagicEndpoint) From() string {
	return e.Relay.From
}

func (e *MagicEndpoint) To() string {
	return e.Relay.To
}

func (e *MagicEndpoint) FromType() EndpointType {
	if e.Relay != nil {
		return e.Relay.FromType
	}
	return Direct
}

var (
	_ conn.Endpoint = &MagicEndpoint{}
)

func (e *MagicEndpoint) ClearSrc() {
	e.src.ifidx = 0
	e.src.Addr = netip.Addr{}
}

func (e *MagicEndpoint) DstIP() netip.Addr {
	switch e.FromType() {
	case Direct:
		return e.Direct.AddrPort.Addr()
	default:
		return e.Relay.Endpoint.Addr()
	}
}

func (e *MagicEndpoint) SrcIP() netip.Addr {
	return e.src.Addr
}

func (e *MagicEndpoint) SrcIfidx() int32 {
	return e.src.ifidx
}

func (e *MagicEndpoint) DstToBytes() []byte {
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

func (e *MagicEndpoint) DstToString() string {
	switch e.FromType() {
	case Direct:
		return e.Direct.AddrPort.String()
	default:
		return e.Relay.Endpoint.String()
	}
}

func (e *MagicEndpoint) SrcToString() string {
	return e.src.Addr.String()
}
