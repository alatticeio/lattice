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

package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/agent/provision"
	"github.com/alatticeio/lattice/internal/grpc"
)

type ProbeFactory struct {
	// localId is the full identity of this node (AppID + PublicKey).
	localId infra.PeerIdentity

	mu     sync.RWMutex
	probes map[string]*Probe // keyed by remote AppID

	signal         infra.SignalService
	getProvisioner func() provision.Provisioner
	getOnMessage   func() func(context.Context, *infra.Message) error
	getWrrp        func() infra.Wrrp

	log *log.Logger

	peerManager *infra.PeerManager
	showLog     bool

	FilteringMux  *infra.FilteringUDPMux
	FilteringMux6 *infra.FilteringUDPMux
}

type ProbeFactoryConfig struct {
	LocalId        infra.PeerIdentity
	Signal         infra.SignalService
	GetOnMessage   func() func(context.Context, *infra.Message) error
	PeerManager    *infra.PeerManager
	GetWrrp        func() infra.Wrrp
	FilteringMux   *infra.FilteringUDPMux
	FilteringMux6  *infra.FilteringUDPMux
	GetProvisioner func() provision.Provisioner
	ShowLog        bool
}

func NewProbeFactory(cfg *ProbeFactoryConfig) *ProbeFactory {
	return &ProbeFactory{
		log:            log.GetLogger("probe-factory"),
		localId:        cfg.LocalId,
		signal:         cfg.Signal,
		probes:         make(map[string]*Probe),
		peerManager:    cfg.PeerManager,
		getWrrp:        cfg.GetWrrp,
		showLog:        cfg.ShowLog,
		FilteringMux:   cfg.FilteringMux,
		FilteringMux6:  cfg.FilteringMux6,
		getProvisioner: cfg.GetProvisioner,
		getOnMessage:   cfg.GetOnMessage,
	}
}

func (f *ProbeFactory) Register(remoteId infra.PeerIdentity, probe *Probe) {
	f.probes[remoteId.AppID] = probe
}

func (f *ProbeFactory) Get(remoteId infra.PeerIdentity) (*Probe, error) {
	// Fast path: probe already exists, read lock is sufficient.
	f.mu.RLock()
	probe := f.probes[remoteId.AppID]
	f.mu.RUnlock()
	if probe != nil {
		return probe, nil
	}

	// Slow path: create a new probe under write lock.
	// Double-check after acquiring the lock in case another goroutine raced here.
	f.mu.Lock()
	defer f.mu.Unlock()
	if probe = f.probes[remoteId.AppID]; probe != nil {
		return probe, nil
	}
	return f.NewProbe(remoteId)
}

func (f *ProbeFactory) Remove(appId string) {
	f.mu.Lock()
	probe := f.probes[appId]
	delete(f.probes, appId)
	f.mu.Unlock()

	// Close outside the lock to avoid deadlock if Close() triggers callbacks
	// that themselves call into ProbeFactory.
	if probe != nil {
		probe.Close()
	}
}

// wgConfigAdapter adapts provision.Provisioner to PeerOps and RouteOps.
type wgConfigAdapter struct {
	getProvisioner func() provision.Provisioner
	getRemotePeer  func() *infra.Peer
}

func (a *wgConfigAdapter) AddPeer(publicKey, allowedIPs string) error {
	pr := a.getProvisioner()
	if pr == nil {
		return nil
	}
	return pr.AddPeer(&provision.SetPeer{
		PublicKey:  publicKey,
		AllowedIPs: allowedIPs,
	})
}

func (a *wgConfigAdapter) SetEndpoint(publicKey, endpoint string, persistentKeepalive int) error {
	pr := a.getProvisioner()
	if pr == nil {
		return nil
	}
	rp := a.getRemotePeer()
	allowedIPs := ""
	if rp != nil {
		allowedIPs = rp.AllowedIPs
		if allowedIPs == "" && rp.Address != nil {
			allowedIPs = fmt.Sprintf("%s/32", *rp.Address)
		}
	}
	return pr.AddPeer(&provision.SetPeer{
		PublicKey:            publicKey,
		Endpoint:             endpoint,
		PersistentKeepalived: persistentKeepalive,
		AllowedIPs:           allowedIPs,
	})
}

func (a *wgConfigAdapter) RemovePeer(publicKey string) error {
	pr := a.getProvisioner()
	if pr == nil {
		return nil
	}
	return pr.RemovePeer(&provision.SetPeer{
		PublicKey: publicKey,
		Remove:    true,
	})
}

func (a *wgConfigAdapter) ApplyRoute(address, iface string) error {
	pr := a.getProvisioner()
	if pr == nil {
		return nil
	}
	return pr.ApplyRoute("add", address, iface)
}

func (a *wgConfigAdapter) SetupNAT(iface string) error {
	pr := a.getProvisioner()
	if pr == nil {
		return nil
	}
	return pr.SetupNAT(iface)
}

func (p *ProbeFactory) NewProbe(remoteId infra.PeerIdentity) (*Probe, error) {
	getLocalPeer := func() *infra.Peer {
		lp := p.peerManager.GetPeer(p.localId.AppID)
		if lp != nil && lp.AllowedIPs == "" && lp.Address != nil {
			lpCopy := *lp
			lpCopy.AllowedIPs = fmt.Sprintf("%s/32", *lp.Address)
			return &lpCopy
		}
		return lp
	}

	var mu sync.Mutex
	var remotePeer *infra.Peer
	var peerKnownDone atomic.Bool

	getRemotePeer := func() *infra.Peer {
		mu.Lock()
		defer mu.Unlock()
		return remotePeer
	}

	// Configurator: the single channel for all WireGuard configuration.
	configurator := NewWGConfigurator(&wgConfigAdapter{
		getProvisioner: p.getProvisioner,
		getRemotePeer:  getRemotePeer,
	}, &wgConfigAdapter{
		getProvisioner: p.getProvisioner,
		getRemotePeer:  getRemotePeer,
	})

	// onPeerKnown: called once on first SYN/ACK — RegisterPeer + ApplyRoute
	// via the configurator, not direct provisioner calls.
	onPeerKnown := func(peer infra.Peer) {
		if peerKnownDone.Load() {
			return
		}
		if peer.Address == nil {
			return
		}
		if !peerKnownDone.CompareAndSwap(false, true) {
			return
		}
		allowedIPs := peer.AllowedIPs
		if allowedIPs == "" {
			allowedIPs = fmt.Sprintf("%s/32", *peer.Address)
		}
		if err := configurator.RegisterPeer(remoteId.PublicKey.String(), allowedIPs); err != nil {
			p.log.Warn("onPeerKnown: RegisterPeer failed", "remoteId", remoteId.AppID, "err", err)
			peerKnownDone.Store(false)
			return
		}
		iface := ""
		if pr := p.getProvisioner(); pr != nil {
			iface = pr.GetIfaceName()
		}
		if err := configurator.ApplyRoute(*peer.Address, iface); err != nil {
			p.log.Warn("onPeerKnown: ApplyRoute failed", "remoteId", remoteId.AppID, "err", err)
		}
		p.log.Info("peer known, pre-configured WG entry", "remoteId", remoteId.AppID, "allowedIPs", allowedIPs)
	}

	onPeerReceived := func(peer infra.Peer) {
		mu.Lock()
		p.peerManager.AddPeer(peer.AppID, &peer)
		remotePeer = &peer
		mu.Unlock()
		onPeerKnown(peer)
	}

	var probe *Probe

	// State machine with transition callbacks — all WG config goes through
	// the configurator, NOT direct provisioner calls.
	sm := NewStateMachine(StateCreated)

	pubKey := remoteId.PublicKey.String()

	// Only initiator drives PersistentKeepalive.
	persistentKA := 0
	if isInitiator(p.localId, remoteId) {
		persistentKA = provision.PersistentKeepalive
	}

	sm.OnTransition(func(from, to PeerState) {
		p.log.Debug("state transition", "remoteId", remoteId.AppID, "from", from, "to", to)

		switch {
		// First transport ready (ICE or WRRP): set endpoint, route, NAT.
		case from == StateProbing && (to == StateICEReady || to == StateWRRPReady):
			rp := getRemotePeer()
			if rp == nil || rp.Address == nil {
				p.log.Warn("remote peer info not received, cannot set endpoint")
				return
			}

			// Get the active transport to determine endpoint.
			probe.mu.Lock()
			t := probe.currentTransport
			probe.mu.Unlock()
			if t == nil {
				p.log.Warn("no active transport during state transition")
				return
			}

			var endpoint string
			if t.Type() == infra.WRRP {
				endpoint = infra.WrrpFakeAddrPort(remoteId.ID().ToUint64()).String()
			} else {
				endpoint = t.RemoteAddr()
			}

			if err := configurator.SetEndpoint(pubKey, endpoint, persistentKA); err != nil {
				p.log.Error("transition: SetEndpoint failed", err)
				return
			}
			// ApplyRoute is idempotent; re-run in case onPeerKnown was skipped.
			iface := ""
			if pr := p.getProvisioner(); pr != nil {
				iface = pr.GetIfaceName()
			}
			if err := configurator.ApplyRoute(*rp.Address, iface); err != nil {
				p.log.Error("transition: ApplyRoute failed", err)
			}
			if err := configurator.SetupNAT(iface); err != nil {
				p.log.Error("transition: SetupNAT failed", err)
			}

		// ICE upgrade after WRRP: only SetEndpoint — NO duplicate AddPeer,
		// NO route/NAT re-application. This is the P1 bug fix.
		case from == StateWRRPReady && to == StateICEReady:
			probe.mu.Lock()
			t := probe.currentTransport
			probe.mu.Unlock()
			if t == nil {
				p.log.Warn("no active transport during upgrade transition")
				return
			}
			if err := configurator.SetEndpoint(pubKey, t.RemoteAddr(), persistentKA); err != nil {
				p.log.Error("transition: SetEndpoint (upgrade) failed", err)
			}

		// Failed or Closed: clean up WireGuard peer.
		case to == StateFailed || to == StateClosed:
			if err := configurator.RemovePeer(pubKey); err != nil {
				p.log.Warn("transition: RemovePeer failed", "err", err)
			}
		}
	})

	var makeWrrpDialer func() infra.Dialer
	makeWrrpDialer = func() infra.Dialer {
		return NewWrrpDialer(&WrrpDialerConfig{
			LocalId:        p.localId,
			RemoteId:       remoteId,
			Wrrp:           p.getWrrp(),
			Sender:         p.signal.Send,
			GetLocalPeer:   getLocalPeer,
			OnPeerReceived: onPeerReceived,
			OnRestart:      func() { probe.restart() },
		})
	}

	probe = &Probe{
		log:          p.log,
		localId:      p.localId,
		remoteId:     remoteId,
		signal:       p.signal,
		sm:           sm,
		configurator: configurator,
	}

	makeIceDialer := func() infra.Dialer {
		return NewIceDialer(&ICEDialerConfig{
			LocalId:        p.localId,
			RemoteId:       remoteId,
			Sender:         p.signal.Send,
			GetLocalPeer:   getLocalPeer,
			OnPeerReceived: onPeerReceived,
			FilteringMux:   p.FilteringMux,
			ShowLog:        p.showLog,
		})
	}
	probe.newIceDialer = makeIceDialer
	probe.iceDialer = makeIceDialer()
	probe.newWrrpDialer = makeWrrpDialer
	probe.wrrpDialer = makeWrrpDialer()

	// onBeforeRestart resets the peerKnown guard for fresh SYN/ACK exchange.
	probe.onBeforeRestart = func() {
		peerKnownDone.Store(false)
	}

	p.Register(remoteId, probe)
	return probe, nil
}

// Handle is the NATS SignalHandler boundary: remoteId is PeerID from packet.SenderId.
// It resolves to a full PeerIdentity via PeerManager before passing down.
func (p *ProbeFactory) Handle(ctx context.Context, remoteId infra.PeerID, packet *grpc.SignalPacket) error {
	p.log.Debug("Handle packet", "remoteId", remoteId, "packet", packet)

	// Config messages pushed from the management server (not peer-to-peer ICE packets).
	if packet.Type == grpc.PacketType_MESSAGE {
		onMessage := p.getOnMessage()
		if onMessage == nil {
			return nil
		}
		var msg infra.Message
		if err := json.Unmarshal(packet.GetMessage().Content, &msg); err != nil {
			return fmt.Errorf("handle MESSAGE: unmarshal: %w", err)
		}
		return onMessage(ctx, &msg)
	}

	remoteIdentity, ok := p.peerManager.GetIdentity(remoteId)
	if !ok {
		// Peer not yet registered (config message hasn't arrived yet). Drop
		// the packet — the sender will retry the handshake after backoff.
		p.log.Warn("dropping signal packet from unknown peer, config not yet applied", "remoteId", remoteId)
		return nil
	}
	probe, err := p.Get(remoteIdentity)
	if err != nil {
		return err
	}
	return probe.Handle(ctx, remoteIdentity, packet)
}

func (p *ProbeFactory) OnReceive(sessionId [28]byte, data []byte) error {
	return nil
}

// TODO
func (p *ProbeFactory) Allows(remoteId string) bool {
	return true
}
