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

package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
	"wireflow/internal/grpc"
	"wireflow/internal/infra"
	"wireflow/internal/log"

	"github.com/pion/ice/v4"
)

type ProbeFactory struct {
	// localId is the full identity of this node (AppID + PublicKey).
	localId infra.PeerIdentity

	mu     sync.RWMutex
	probes map[string]*Probe // keyed by remote AppID

	wrrpProbes map[string]*Probe // nolint

	signal         infra.SignalService
	getProvisioner func() infra.Provisioner
	getOnMessage   func() func(context.Context, *infra.Message) error
	getWrrp        func() infra.Wrrp

	log *log.Logger

	peerManager *infra.PeerManager
	showLog     bool

	FilteringMux  *infra.FilteringUDPMux
	FilteringMux6 *infra.FilteringUDPMux
}

type ProbeFactoryConfig struct {
	LocalId       infra.PeerIdentity
	Signal        infra.SignalService
	GetOnMessage  func() func(context.Context, *infra.Message) error
	PeerManager   *infra.PeerManager
	GetWrrp       func() infra.Wrrp
	FilteringMux  *infra.FilteringUDPMux
	FilteringMux6 *infra.FilteringUDPMux
	GetProvisioner         func() infra.Provisioner
	ShowLog                bool
}

func NewProbeFactory(cfg *ProbeFactoryConfig) *ProbeFactory {
	return &ProbeFactory{
		log:                    log.GetLogger("probe-factory"),
		localId:                cfg.LocalId,
		signal:                 cfg.Signal,
		probes:                 make(map[string]*Probe),
		peerManager:            cfg.PeerManager,
		getWrrp:                cfg.GetWrrp,
		showLog:                cfg.ShowLog,
		FilteringMux:  cfg.FilteringMux,
		FilteringMux6: cfg.FilteringMux6,
		getProvisioner:         cfg.GetProvisioner,
		getOnMessage:           cfg.GetOnMessage,
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

func (p *ProbeFactory) NewProbe(remoteId infra.PeerIdentity) (*Probe, error) {
	// getLocalPeer reads the local peer's current info from the peer manager.
	// Called at dialer construction time (including on each restart) so that a
	// late-arriving Address/AllowedIPs assignment (from ApplyFullConfig) is
	// picked up rather than a stale nil captured at probe creation time.
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
	var firstFailureAt time.Time

	// peerKnownDone guards the one-time onPeerKnown call.  Unlike sync.Once,
	// it allows a retry if the provisioner was nil on the first attempt.
	var peerKnownDone atomic.Bool

	// onPeerKnown pre-configures the WireGuard peer entry (AllowedIPs, no
	// endpoint yet) and installs the kernel route as soon as the peer's
	// identity is known from SYN/ACK — before any transport is established.
	// This decouples route programming from endpoint discovery so that
	// inbound WireGuard handshakes from the peer can be accepted immediately.
	onPeerKnown := func(peer infra.Peer) {
		if peerKnownDone.Load() {
			return
		}
		provisioner := p.getProvisioner()
		if provisioner == nil {
			return // provisioner not ready; onEndpointReady will apply full config
		}
		if peer.Address == nil {
			return // insufficient info; will retry on next onPeerReceived
		}
		if !peerKnownDone.CompareAndSwap(false, true) {
			return // another goroutine won the race
		}
		allowedIPs := peer.AllowedIPs
		if allowedIPs == "" {
			allowedIPs = fmt.Sprintf("%s/32", *peer.Address)
		}
		if err := provisioner.AddPeer(&infra.SetPeer{
			PublicKey:  remoteId.PublicKey.String(),
			AllowedIPs: allowedIPs,
		}); err != nil {
			p.log.Warn("onPeerKnown: AddPeer failed", "remoteId", remoteId.AppID, "err", err)
			peerKnownDone.Store(false) // allow retry
			return
		}
		if err := provisioner.ApplyRoute("add", *peer.Address, provisioner.GetIfaceName()); err != nil {
			p.log.Warn("onPeerKnown: ApplyRoute failed", "remoteId", remoteId.AppID, "err", err)
			// ApplyRoute failure is non-fatal: onEndpointReady will retry (ip route replace is idempotent).
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

	// makeWrrpDialer is the factory used both for the initial dialer and for
	// each restart.  OnRestart captures probe by reference (probe is assigned
	// below) so it is safe to call once the Probe is fully constructed.
	var probe *Probe
	makeWrrpDialer := func() infra.Dialer {
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
		log:      p.log,
		localId:  p.localId,
		remoteId: remoteId,
		signal:   p.signal,
		state:    ice.ConnectionStateNew,
		// onEndpointReady is called once transport (ICE or WRRP) is established.
		// At this point peer identity is already known (onPeerKnown ran on SYN/ACK),
		// so we only need to update the WireGuard endpoint and finish NAT setup.
		onSuccess: func(transport infra.Transport) error {
			mu.Lock()
			firstFailureAt = time.Time{} // reset failure clock on successful connection
			rp := remotePeer
			mu.Unlock()
			if rp == nil {
				return fmt.Errorf("remote peer info not yet received for %s", remoteId.AppID)
			}
			if rp.Address == nil {
				return fmt.Errorf("remote peer %s address not yet received", remoteId.AppID)
			}
			provisioner := p.getProvisioner()
			if provisioner == nil {
				return fmt.Errorf("provisioner not ready for peer %s", remoteId.AppID)
			}
			p.log.Info("connection established", "transportType", transport.Type(), "remoteAddr", transport.RemoteAddr())
			// Only the initiator drives WireGuard keepalives to avoid both ends
			// simultaneously sending Handshake Initiations (causes ~90 s stall).
			persistentKA := 0
			if isInitiator(p.localId, remoteId) {
				persistentKA = infra.PersistentKeepalive
			}
			allowedIPs := rp.AllowedIPs
			if allowedIPs == "" {
				allowedIPs = fmt.Sprintf("%s/32", *rp.Address)
			}
			// Update WireGuard peer entry with the resolved endpoint.
			// AllowedIPs is re-applied (idempotent with replace_allowed_ips=true).
			setPeer := &infra.SetPeer{
				PublicKey:            remoteId.PublicKey.String(),
				PersistentKeepalived: persistentKA,
				AllowedIPs:           allowedIPs,
			}
			if transport.Type() == infra.WRRP {
				setPeer.Endpoint = infra.WrrpFakeAddrPort(remoteId.ID().ToUint64()).String()
			} else {
				setPeer.Endpoint = transport.RemoteAddr()
			}
			if err := provisioner.AddPeer(setPeer); err != nil {
				p.log.Error("onEndpointReady: AddPeer failed", err)
				return err
			}

			// ApplyRoute is idempotent (ip route replace); re-run in case
			// onPeerKnown was skipped because the provisioner was not ready yet.
			if err := provisioner.ApplyRoute("add", *rp.Address, provisioner.GetIfaceName()); err != nil {
				p.log.Error("onEndpointReady: ApplyRoute failed", err)
				return err
			}

			return provisioner.SetupNAT(provisioner.GetIfaceName())
		},
		onFailure: func(err error) error {
			// ErrDialerClosed: the iceDialer was explicitly shut down because
			// ICE reached Failed state, or a SYN arrived on an active agent
			// (remote restarted mid-session).  This is a clean session
			// transition — restart immediately and reset the failure clock so
			// transient ICE failures don't accumulate toward the 60 s limit.
			if errors.Is(err, ErrDialerClosed) {
				mu.Lock()
				firstFailureAt = time.Time{}
				mu.Unlock()
				probe.restart()
				return nil
			}

			// Any other error (e.g. Dial() timed out waiting for an offer)
			// means the remote is genuinely unreachable.  Apply a 10 s backoff
			// and count elapsed time toward the 60 s removal threshold.
			mu.Lock()
			if firstFailureAt.IsZero() {
				firstFailureAt = time.Now()
			}
			elapsed := time.Since(firstFailureAt)
			mu.Unlock()

			// After 60s of timeout failures, give up and let the management
			// server drive the next attempt via PeersRemoved/PeersAdded.
			if elapsed >= 60*time.Second {
				p.log.Info("peer unreachable for 60s, closing probe", "remoteId", remoteId.AppID)
				p.Remove(remoteId.AppID)
				return nil
			}
			p.log.Warn("discover failed, retrying in 10s", "remoteId", remoteId.AppID, "err", err)
			time.AfterFunc(10*time.Second, probe.restart)
			return nil
		},
		wrrpDialer: makeWrrpDialer(),
	}

	// makeIceDialer creates a fresh iceDialer for each connection attempt.
	// Restart is driven entirely by onFailure above — the dialer itself has
	// no restart callback, eliminating the double-restart race condition.
	makeIceDialer := func() infra.Dialer {
		return NewIceDialer(&ICEDialerConfig{
			LocalId:                p.localId,
			RemoteId:               remoteId,
			Sender:                 p.signal.Send,
			GetLocalPeer:           getLocalPeer,
			OnPeerReceived:         onPeerReceived,
			FilteringMux: p.FilteringMux,
			// FilteringMux6: p.FilteringMux6, // IPv6 ICE disabled until e2e tests pass
			ShowLog:                p.showLog,
		})
	}
	probe.newIceDialer = makeIceDialer
	probe.iceDialer = makeIceDialer()
	probe.newWrrpDialer = makeWrrpDialer

	// onBeforeRestart clears stale WireGuard peer state so that the new
	// SYN/ACK exchange starts from a clean baseline.  peerKnownDone is reset
	// so onPeerKnown will re-run when the fresh SYN/ACK arrives.
	probe.onBeforeRestart = func() {
		peerKnownDone.Store(false)
		provisioner := p.getProvisioner()
		if provisioner == nil {
			return
		}
		if err := provisioner.RemovePeer(&infra.SetPeer{
			PublicKey: remoteId.PublicKey.String(),
			Remove:    true,
		}); err != nil {
			p.log.Warn("restart: RemovePeer failed", "remoteId", remoteId.AppID, "err", err)
		}
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
