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
	"errors"
	"os"
	"sync/atomic"
	"time"

	"golang.zx2c4.com/wireguard/tun"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

// tunAdapter bridges the gVisor channel endpoint to wireguard-go's tun.Device
// interface. Raw IP packets flow between the gVisor netstack and wireguard-go
// through this adapter.
//
// Read path: gVisor channel -> wireguard-go (encrypt -> UDP)
// Write path: wireguard-go (decrypt <- UDP) -> gVisor channel
type tunAdapter struct {
	ch     *channel.Endpoint
	inject func(packet []byte) error
	mtu    int32
	closed atomic.Bool
	events chan tun.Event
}

// NewTUNAdapter creates a tun.Device backed by the gVisor channel endpoint.
// The inject callback is invoked by tunAdapter.Write with decrypted IP packets
// from wireguard-go; it should inject them into the gVisor netstack (e.g. via
// channel.Endpoint.InjectInbound).
//
// events is pre-populated with EventUp and then closed so that wireguard-go's
// RoutineTUNEventReader goroutine exits cleanly instead of blocking forever on
// a nil channel.
func NewTUNAdapter(ch *channel.Endpoint, inject func(packet []byte) error) tun.Device {
	events := make(chan tun.Event, 1)
	events <- tun.EventUp
	close(events)
	return &tunAdapter{ch: ch, inject: inject, mtu: 1500, events: events}
}

// Read reads one packet from the gVisor channel endpoint and places it into
// bufs[0] at the given offset. Returns the count of packets read (0 or 1).
//
// gVisor's channel.Endpoint.Read is non-blocking (returns nil when empty).
// We loop with a 1ms sleep to avoid a 100% CPU busy-spin in wireguard-go's
// TUN reader goroutine. 1ms is acceptable latency for an encrypted overlay.
func (t *tunAdapter) Read(bufs [][]byte, sizes []int, offset int) (int, error) {
	for {
		if t.closed.Load() {
			return 0, errors.New("tun: closed")
		}

		pkt := t.ch.Read()
		if pkt == nil {
			time.Sleep(time.Millisecond)
			continue
		}
		defer pkt.DecRef()

		data := pkt.ToView()
		if data == nil || data.Size() == 0 {
			continue
		}

		n := data.Size()
		if n+offset > len(bufs[0]) {
			n = len(bufs[0]) - offset
		}
		copy(bufs[0][offset:], data.AsSlice()[:n])
		sizes[0] = n
		return 1, nil
	}
}

// Write injects decrypted IP packets from wireguard-go into the gVisor
// netstack via the inject callback.
func (t *tunAdapter) Write(bufs [][]byte, offset int) (int, error) {
	if t.closed.Load() {
		return 0, errors.New("tun: closed")
	}

	var total int
	for _, buf := range bufs {
		data := buf[offset:]
		if len(data) == 0 {
			continue
		}
		if err := t.inject(data); err != nil {
			return total, err
		}
		total++
	}
	return total, nil
}

// InjectIntoChannel creates an inject callback that inserts a raw IP packet
// into the gVisor channel endpoint as an inbound packet.
func InjectIntoChannel(ch *channel.Endpoint) func([]byte) error {
	return func(packet []byte) error {
		view := buffer.NewViewWithData(packet)
		var buf buffer.Buffer
		_ = buf.Append(view)
		pkt := stack.NewPacketBuffer(stack.PacketBufferOptions{
			Payload: buf,
		})
		ch.InjectInbound(header.IPv4ProtocolNumber, pkt)
		pkt.DecRef()
		return nil
	}
}

// File returns nil — wireguard-go uses goroutine-based I/O without epoll.
func (t *tunAdapter) File() *os.File { return nil }

// MTU returns the configured MTU.
func (t *tunAdapter) MTU() (int, error) { return int(t.mtu), nil }

// Name returns a human-readable name.
func (t *tunAdapter) Name() (string, error) { return "gvisor-tun", nil }

// Events returns the pre-populated event channel. wireguard-go's
// RoutineTUNEventReader reads EventUp from it and exits when the channel
// closes, preventing a goroutine leak.
func (t *tunAdapter) Events() <-chan tun.Event { return t.events }

// BatchSize returns 1 — we do not batch.
func (t *tunAdapter) BatchSize() int { return 1 }

// Close marks the adapter as closed. Subsequent Read/Write calls will return
// an error.
func (t *tunAdapter) Close() error {
	t.closed.Store(true)
	return nil
}
