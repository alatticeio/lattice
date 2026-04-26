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
	"net"
	"sync"
	"time"
)

// injectedMsg holds a single packet injected into a ChanPacketConn.
type injectedMsg struct {
	data []byte
	addr net.Addr
}

// ChanPacketConn is a channel-backed net.PacketConn.
//
// It is given to UDPMuxDefault (and UniversalUDPMuxDefault) instead of the
// real UDP socket. The mux's internal connWorker goroutine blocks on
// ReadFrom, which returns only when FilteringUDPMux.readLoop calls inject()
// after classifying an incoming packet as STUN. This makes FilteringUDPMux
// the sole reader of the real socket, eliminating the race between the mux's
// connWorker and WireGuard's makeReceiveIPv4.
//
// WriteTo is proxied to the real socket so that the mux can send STUN
// binding responses/requests to the remote peer.
type ChanPacketConn struct {
	recvCh    chan injectedMsg
	realConn  net.PacketConn // proxied for WriteTo (ICE sends STUN responses here)
	local     net.Addr
	done      chan struct{}
	closeOnce sync.Once
}

func newChanPacketConn(realConn net.PacketConn) *ChanPacketConn {
	return &ChanPacketConn{
		recvCh:   make(chan injectedMsg, 256),
		realConn: realConn,
		local:    realConn.LocalAddr(),
		done:     make(chan struct{}),
	}
}

// inject is called by FilteringUDPMux.readLoop for every STUN packet it reads
// from the real socket. The mux's connWorker will receive it via ReadFrom.
func (c *ChanPacketConn) inject(data []byte, addr net.Addr) {
	buf := make([]byte, len(data))
	copy(buf, data)
	select {
	case c.recvCh <- injectedMsg{data: buf, addr: addr}:
	case <-c.done:
	}
}

// ReadFrom blocks until a STUN packet is injected or the conn is closed.
// Called exclusively by the mux's internal connWorker goroutine.
func (c *ChanPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	select {
	case msg := <-c.recvCh:
		n = copy(p, msg.data)
		return n, msg.addr, nil
	case <-c.done:
		return 0, nil, net.ErrClosed
	}
}

// WriteTo proxies to the real socket so ICE can send STUN responses/requests.
func (c *ChanPacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	return c.realConn.WriteTo(p, addr)
}

func (c *ChanPacketConn) LocalAddr() net.Addr             { return c.local }
func (c *ChanPacketConn) SetDeadline(_ time.Time) error    { return nil }
func (c *ChanPacketConn) SetReadDeadline(_ time.Time) error { return nil }
func (c *ChanPacketConn) SetWriteDeadline(_ time.Time) error { return nil }

// Close signals ReadFrom to return net.ErrClosed, allowing the mux's
// connWorker to exit cleanly.
func (c *ChanPacketConn) Close() error {
	c.closeOnce.Do(func() { close(c.done) })
	return nil
}
