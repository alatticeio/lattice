// Copyright 2026 The Lattice Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gvisor

import (
	"fmt"
	"log"
	"net"
	"net/netip"
	"sync/atomic"

	"github.com/alatticeio/lattice-shim/shim"
	"golang.zx2c4.com/wireguard/conn"
)

// udpBind is a lightweight WireGuardBind backed by a simple UDP socket.
// It implements shim.WireGuardBind so the gVisor sandbox can send and
// receive encrypted WireGuard packets through a plain UDP :port socket.
type udpBind struct {
	conn   *net.UDPConn
	closed atomic.Bool
}

// NewUDPBind opens a UDP socket on the given address and returns a
// shim.WireGuardBind suitable for attaching wireguard-go to the gVisor
// netstack.
//
// addr must be of the form ":51820". Use "0.0.0.0:51820" to listen on all
// IPv4 interfaces, "[::]:51820" for all IPv6.
func NewUDPBind(addr string) (shim.WireGuardBind, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("gvisor: resolve udp addr %s: %w", addr, err)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, fmt.Errorf("gvisor: listen udp %s: %w", addr, err)
	}

	log.Printf("[gvisor] udp bind listening on %s", conn.LocalAddr())
	return &udpBind{conn: conn}, nil
}

// Write sends an encrypted WireGuard packet via the UDP socket.
func (b *udpBind) Write(packet []byte) error {
	if b.closed.Load() {
		return net.ErrClosed
	}
	_, err := b.conn.Write(packet)
	return err
}

// Read receives an encrypted WireGuard packet from the UDP socket.
// The returned slice is owned by the caller.
func (b *udpBind) Read() ([]byte, error) {
	if b.closed.Load() {
		return nil, net.ErrClosed
	}
	buf := make([]byte, 65535) // max UDP datagram payload
	n, err := b.conn.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

// Compile-time check.
var _ shim.WireGuardBind = (*udpBind)(nil)

// Close closes the UDP socket.
func (b *udpBind) Close() error {
	b.closed.Store(true)
	return b.conn.Close()
}

// wgBindAdapter wraps a udpBind as wireguard-go's conn.Bind. It is used by the
// sandbox to attach wireguard-go directly to the UDP socket without going
// through the shim.WireGuardBind path (pumpOutbound). The real socket cleanup
// is handled by the caller (sandboxCloser); adapter.Close() is deliberately a
// no-op.
type wgBindAdapter struct {
	ub *udpBind
}

// ToWireGuardBind converts this udpBind to a wireguard-go compatible conn.Bind.
func (b *udpBind) ToWireGuardBind() conn.Bind {
	return &wgBindAdapter{ub: b}
}

// NewUDPBindWithBind creates a UDP socket and returns both a shim.WireGuardBind
// (for the pumpOutbound path) and a wireguard-go conn.Bind (for the full
// wireguard-go path). At most one of the two should be active at any time.
//
// When wireguard-go is used, pass the conn.Bind to device.NewDevice and leave
// the shim.WireGuardBind unused (or nil in sandbox Config) — the conn.Bind's
// Open/Close methods manage socket lifecycle compatibly with wireguard-go's
// BindUpdate protocol.
func NewUDPBindWithBind(addr string) (shim.WireGuardBind, conn.Bind, error) {
	wgb, err := NewUDPBind(addr)
	if err != nil {
		return nil, nil, err
	}
	ub, ok := wgb.(*udpBind)
	if !ok {
		return nil, nil, fmt.Errorf("gvisor: unexpected WireGuardBind type %T", wgb)
	}
	return ub, ub.ToWireGuardBind(), nil
}

// Open returns the existing UDP socket's port and a receive function. It does
// not open a new socket — the socket was opened by NewUDPBind.
func (a *wgBindAdapter) Open(port uint16) ([]conn.ReceiveFunc, uint16, error) {
	addr := a.ub.conn.LocalAddr().(*net.UDPAddr)
	actualPort := uint16(addr.Port)
	recvFn := a.makeReceiveFunc()
	return []conn.ReceiveFunc{recvFn}, actualPort, nil
}

// Close is a no-op to prevent wireguard-go's BindUpdate from closing the
// underlying UDP socket. The socket is closed by the sandboxCloser via
// shim.WireGuardBind.Close().
func (a *wgBindAdapter) Close() error {
	return nil
}

// SetMark is not supported in the sandbox (no CAP_NET_ADMIN).
func (a *wgBindAdapter) SetMark(mark uint32) error {
	return nil
}

// Send writes encrypted WireGuard packets to the destination endpoint via the
// UDP socket.
func (a *wgBindAdapter) Send(bufs [][]byte, endpoint conn.Endpoint) error {
	if a.ub.closed.Load() {
		return net.ErrClosed
	}
	dstIP := endpoint.DstIP()
	dstPort := endpoint.(*conn.StdNetEndpoint).Port()
	udpAddr := net.UDPAddrFromAddrPort(netip.AddrPortFrom(dstIP, dstPort))
	for _, buf := range bufs {
		if _, err := a.ub.conn.WriteTo(buf, udpAddr); err != nil {
			return err
		}
	}
	return nil
}

// ParseEndpoint parses an endpoint string in "ip:port" format.
func (a *wgBindAdapter) ParseEndpoint(s string) (conn.Endpoint, error) {
	addrPort, err := netip.ParseAddrPort(s)
	if err != nil {
		return nil, err
	}
	return &conn.StdNetEndpoint{AddrPort: addrPort}, nil
}

// BatchSize returns 1 — we handle one packet at a time.
func (a *wgBindAdapter) BatchSize() int {
	return 1
}

func (a *wgBindAdapter) makeReceiveFunc() conn.ReceiveFunc {
	return func(bufs [][]byte, sizes []int, eps []conn.Endpoint) (int, error) {
		if a.ub.closed.Load() {
			return 0, net.ErrClosed
		}
		n, srcAddr, err := a.ub.conn.ReadFromUDPAddrPort(bufs[0])
		if err != nil {
			return 0, err
		}
		sizes[0] = n
		eps[0] = &conn.StdNetEndpoint{AddrPort: srcAddr}
		return 1, nil
	}
}
