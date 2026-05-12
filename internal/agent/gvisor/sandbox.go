//go:build pro

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

// Package gvisor provides a zero-privilege agent sandbox backed by gVisor's
// user-space network stack (via lattice-shim). It integrates with the Lattice
// policy engine and audit pipeline, and exposes a WireGuard endpoint bridge
// for attaching wireguard-go without a kernel TUN device.
package gvisor

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/alatticeio/lattice-shim/shim"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

// Sandbox wraps a shim.Netstack with Lattice-specific policy and audit
// adapters, plus a bridge for wireguard-go attachment.
type Sandbox struct {
	id      string
	ns      *shim.Netstack
	localIP string
	ep      *shim.WireGuardEndpoint

	ch     *channel.Endpoint
	stopCh chan struct{}
	wg     sync.WaitGroup

	mu     sync.Mutex
	closed bool
}

// Config holds the parameters needed to create a Sandbox.
type Config struct {
	// ID is a caller-chosen identifier for this sandbox (e.g. agent name).
	ID string

	// LocalIP is the VPN IP address assigned to this sandbox by Lattice IPAM.
	LocalIP string

	// PolicyChecker is injected by the caller. When nil, all traffic is allowed.
	PolicyChecker shim.PolicyChecker

	// AuditWriter is injected by the caller. When nil, audit is silent.
	AuditWriter shim.AuditWriter

	// WireGuardBind is the caller's wireguard-go bind that handles encrypted
	// packet I/O. Outbound raw IP packets read from the gVisor channel are
	// written to this bind for encryption + UDP delivery. Decrypted packets
	// received by the bind are injected back into the channel via the
	// WireGuardEndpoint.Inbound callback.
	//
	// When nil, the sandbox cannot reach remote peers — only local (loopback)
	// communication within the netstack works.
	WireGuardBind shim.WireGuardBind
}

// New creates a gVisor-backed Sandbox.
func New(cfg Config) (*Sandbox, error) {
	if cfg.LocalIP == "" {
		return nil, fmt.Errorf("gvisor: LocalIP is required")
	}

	opts := []shim.NetstackOption{
		shim.WithIdentity(cfg.ID),
	}
	if cfg.PolicyChecker != nil {
		opts = append(opts, shim.WithPolicy(cfg.PolicyChecker))
	}
	if cfg.AuditWriter != nil {
		opts = append(opts, shim.WithAudit(cfg.AuditWriter))
	}

	ns, err := shim.NewNetstack(cfg.LocalIP, opts...)
	if err != nil {
		return nil, fmt.Errorf("gvisor: NewNetstack: %w", err)
	}

	ch := ns.Channel()

	s := &Sandbox{
		id:      cfg.ID,
		ns:      ns,
		localIP: cfg.LocalIP,
		ch:      ch,
		stopCh:  make(chan struct{}),
	}

	// WireGuard bridge: pumps packets between gVisor channel endpoint and
	// wireguard-go via the caller-supplied bind.
	ep := &shim.WireGuardEndpoint{
		// Outbound: called when wireguard-go has a decrypted IP packet for us.
		// Inject it into the gVisor netstack so it reaches the agent process.
		Outbound: nil, // filled below

		// Inbound: called by gVisor netstack when it has a raw IP packet to send.
		// This is wired through the channel reader goroutine below.
		Inbound: nil, // not used directly; reader goroutine pumps via bind.Write

		Close: func() error {
			close(s.stopCh)
			s.wg.Wait()
			return ns.Close()
		},
	}

	if cfg.WireGuardBind != nil {
		// Outbound: gVisor channel → wireguard-go (encrypt + UDP send).
		// This runs as a background goroutine reading from the channel.
		s.wg.Add(1)
		go s.pumpOutbound(cfg.WireGuardBind)

		// Inbound: wireguard-go decrypt → gVisor channel.
		ep.Outbound = func(packet []byte) error {
			view := buffer.NewViewWithData(packet)
			var buf buffer.Buffer
			buf.Append(view)
			pkt := stack.NewPacketBuffer(stack.PacketBufferOptions{
				Payload: buf,
			})
			ch.InjectInbound(header.IPv4ProtocolNumber, pkt)
			pkt.DecRef()
			return nil
		}
	} else {
		ep.Outbound = func(packet []byte) error {
			log.Printf("[gvisor] sandbox %q: no WG bind configured, dropping outbound packet (%d bytes)", s.id, len(packet))
			return nil
		}
	}

	s.ep = ep

	log.Printf("[gvisor] sandbox %q created, localIP=%s, wgBind=%v", cfg.ID, cfg.LocalIP, cfg.WireGuardBind != nil)
	return s, nil
}

// pumpOutbound reads raw IP packets from the gVisor channel endpoint and
// writes them to wireguard-go's bind for encryption + UDP delivery.
func (s *Sandbox) pumpOutbound(bind shim.WireGuardBind) {
	defer s.wg.Done()
	for {
		select {
		case <-s.stopCh:
			return
		default:
		}

		pkt := s.ch.Read()
		if pkt == nil {
			// No packet available; yield to avoid busy loop.
			// In production, ReadContext with a short timeout is preferred.
			continue
		}

		// Convert *stack.PacketBuffer to []byte.
		data := pkt.ToView()
		if data == nil || data.Size() == 0 {
			pkt.DecRef()
			continue
		}

		if err := bind.Write(data.AsSlice()); err != nil {
			log.Printf("[gvisor] sandbox %q: bind.Write: %v", s.id, err)
		}
		pkt.DecRef()
	}
}

// DialContext dials a remote address through the gVisor netstack. Policy and
// audit hooks are applied automatically.
func (s *Sandbox) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, fmt.Errorf("gvisor: sandbox closed")
	}
	s.mu.Unlock()
	return s.ns.DialContext(ctx, network, addr)
}

// ListenTCP creates a TCP listener on the gVisor netstack.
func (s *Sandbox) ListenTCP(addr string) (net.Listener, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, fmt.Errorf("gvisor: sandbox closed")
	}
	s.mu.Unlock()
	return s.ns.ListenTCP(addr)
}

// WireGuardEndpoint returns the bridge that the caller should connect to
// wireguard-go.
func (s *Sandbox) WireGuardEndpoint() *shim.WireGuardEndpoint {
	return s.ep
}

// ID returns the sandbox identifier.
func (s *Sandbox) ID() string { return s.id }

// LocalIP returns the VPN IP address.
func (s *Sandbox) LocalIP() string { return s.localIP }

// Close shuts down the sandbox, its netstack, and the WireGuard bridge.
func (s *Sandbox) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	log.Printf("[gvisor] sandbox %q closing", s.id)
	if s.ep != nil && s.ep.Close != nil {
		return s.ep.Close()
	}
	return s.ns.Close()
}
