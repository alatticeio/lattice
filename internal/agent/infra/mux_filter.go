// Copyright 2026 The Lattice Authors, Inc.
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
	"sync/atomic"

	"github.com/pion/ice/v4"
	"github.com/pion/logging"
	"github.com/pion/stun/v3"
)

// PassThroughPacket carries a non-STUN UDP packet forwarded from
// FilteringUDPMux to WireGuard's receive path.
type PassThroughPacket struct {
	Data []byte
	Addr *net.UDPAddr
}

// FilteringUDPMux wraps UniversalUDPMuxDefault and becomes the sole reader of
// the shared UDP socket. It classifies every incoming packet:
//
//   - STUN packet  → injected into ChanPacketConn → mux's connWorker dispatches
//     it to the correct ICE muxedConn by ufrag.
//   - Non-STUN packet → sent to passThroughCh → WireGuard's DefaultBind reads it.
//
// This eliminates the race between UDPMuxDefault.connWorker and
// WireGuard's makeReceiveIPv4, which previously both read the same socket.
type FilteringUDPMux struct {
	// inner is the real UniversalUDPMuxDefault, exposed to ice.Agent via
	// WithUDPMux / WithUDPMuxSrflx. It holds chanConn as its UDPConn so
	// its connWorker only consumes packets we explicitly inject.
	inner    *ice.UniversalUDPMuxDefault
	chanConn *ChanPacketConn

	realConn      net.PacketConn // true socket; FilteringUDPMux is the sole reader
	passThroughCh chan<- PassThroughPacket

	stopCh       chan struct{}
	wg           sync.WaitGroup
	droppedCount atomic.Uint64 // count of dropped pass-through packets
}

// NewFilteringUDPMux constructs the wrapper. realConn is the shared UDP socket.
// logger may be nil (ICE log disabled).
// Call SetPassThrough then Start before creating any ICE agents.
func NewFilteringUDPMux(realConn net.PacketConn, logger logging.LeveledLogger) *FilteringUDPMux {
	chanConn := newChanPacketConn(realConn)

	// Give the mux our fake conn, not the real socket.
	// The mux's connWorker will block on chanConn.ReadFrom, receiving only
	// packets we inject — it never races with our readLoop.
	inner := ice.NewUniversalUDPMuxDefault(ice.UniversalUDPMuxParams{
		Logger:  logger,
		UDPConn: chanConn,
	})

	return &FilteringUDPMux{
		inner:    inner,
		chanConn: chanConn,
		realConn: realConn,
		stopCh:   make(chan struct{}),
	}
}

// SetPassThrough registers the channel that receives non-STUN (WireGuard)
// packets. Must be called before Start.
func (f *FilteringUDPMux) SetPassThrough(ch chan<- PassThroughPacket) {
	f.passThroughCh = ch
}

// UDPMux returns the UDPMux interface for ice.WithUDPMux (host candidates).
func (f *FilteringUDPMux) UDPMux() ice.UDPMux {
	return f.inner.UDPMuxDefault
}

// UDPMuxSrflx returns the UniversalUDPMux interface for ice.WithUDPMuxSrflx
// (server-reflexive candidates).
func (f *FilteringUDPMux) UDPMuxSrflx() ice.UniversalUDPMux {
	return f.inner
}

// Start launches the sole-reader goroutine. Must be called after SetPassThrough
// and before any ICE agent is created.
func (f *FilteringUDPMux) Start() {
	f.wg.Add(1)
	go f.readLoop()
}

// readLoop is the only goroutine that reads from realConn.
// STUN packets are injected into chanConn for the mux's connWorker.
// All other packets are forwarded to passThroughCh for WireGuard.
func (f *FilteringUDPMux) readLoop() {
	defer f.wg.Done()

	buf := make([]byte, 2048)
	for {
		n, addr, err := f.realConn.ReadFrom(buf)
		if err != nil {
			if err == net.ErrClosed {
				return
			}
			// transient error (e.g. EAGAIN); keep running
			continue
		}

		pkt := buf[:n]
		udpAddr, _ := addr.(*net.UDPAddr)

		if stun.IsMessage(pkt) {
			// STUN: inject into the mux so connWorker can dispatch by ufrag.
			f.chanConn.inject(pkt, addr)
		} else if f.passThroughCh != nil {
			// Non-STUN (WireGuard encrypted): forward to DefaultBind.
			// Allocate a fresh buffer; buf is reused on the next iteration.
			data := make([]byte, n)
			copy(data, pkt)
			select {
			case f.passThroughCh <- PassThroughPacket{Data: data, Addr: udpAddr}:
			default:
				// Channel full: drop rather than block the sole reader.
				f.droppedCount.Add(1)
			}
		}
	}
}

// Close stops the readLoop, drains the mux, and shuts down chanConn.
func (f *FilteringUDPMux) Close() error {
	close(f.stopCh)
	f.wg.Wait()
	// Closing chanConn unblocks the mux's connWorker so it can exit.
	_ = f.chanConn.Close()
	return f.inner.Close()
}

// DroppedCount returns the number of pass-through packets dropped due to
// a full channel. Useful for monitoring NAT overload conditions.
func (f *FilteringUDPMux) DroppedCount() uint64 {
	return f.droppedCount.Load()
}
