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
	"time"
	"wireflow/internal/grpc"
	"wireflow/internal/infra"
	"wireflow/internal/log"

	"github.com/wireflowio/ice"
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

	UniversalUdpMuxDefault *ice.UniversalUDPMuxDefault
}

type ProbeFactoryConfig struct {
	LocalId                infra.PeerIdentity
	Signal                 infra.SignalService
	GetOnMessage           func() func(context.Context, *infra.Message) error
	PeerManager            *infra.PeerManager
	GetWrrp                func() infra.Wrrp
	UniversalUdpMuxDefault *ice.UniversalUDPMuxDefault
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
		UniversalUdpMuxDefault: cfg.UniversalUdpMuxDefault,
		getProvisioner:         cfg.GetProvisioner,
		getOnMessage:           cfg.GetOnMessage,
	}
}

func (f *ProbeFactory) Register(remoteId infra.PeerIdentity, probe *Probe) {
	f.probes[remoteId.AppID] = probe
}

func (f *ProbeFactory) Get(remoteId infra.PeerIdentity) (*Probe, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	var err error
	probe := f.probes[remoteId.AppID]
	if probe == nil {
		probe, err = f.NewProbe(remoteId)
		if err != nil {
			return nil, err
		}
	}
	return probe, err
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
	localPeer := p.peerManager.GetPeer(p.localId.AppID)
	if localPeer != nil && localPeer.AllowedIPs == "" && localPeer.Address != nil {
		peerCopy := *localPeer
		peerCopy.AllowedIPs = fmt.Sprintf("%s/32", *localPeer.Address)
		localPeer = &peerCopy
	}

	var mu sync.Mutex
	var remotePeer *infra.Peer
	var firstFailureAt time.Time
	onPeerReceived := func(peer infra.Peer) {
		mu.Lock()
		p.peerManager.AddPeer(peer.AppID, &peer)
		remotePeer = &peer
		mu.Unlock()
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
			LocalPeer:      localPeer,
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
		onSuccess: func(transport infra.Transport) error {
			mu.Lock()
			firstFailureAt = time.Time{} // reset failure clock on successful connection
			rp := remotePeer
			mu.Unlock()
			if rp == nil {
				return fmt.Errorf("remote peer info not yet received for %s", remoteId.AppID)
			}
			provisioner := p.getProvisioner()
			if provisioner == nil {
				return fmt.Errorf("provisioner not ready for peer %s", remoteId.AppID)
			}
			p.log.Info("connection established", "transportType", transport.Type(), "remoteAddr", transport.RemoteAddr())
			// Only the ICE initiator (localId > remoteId, i.e. the SYN sender)
			// drives WireGuard keepalives.  If both ends set PersistentKeepalive
			// they simultaneously send Handshake Initiations, continuously
			// overwriting each other's session state and causing all Responses
			// to be rejected (~90 s before one side gives up and the other can
			// finally complete the handshake).
			persistentKA := 0
			if p.localId.String() > remoteId.String() {
				persistentKA = infra.PersistentKeepalive
			}
			setPeer := &infra.SetPeer{
				PublicKey:            remoteId.PublicKey.String(),
				PersistentKeepalived: persistentKA,
				AllowedIPs:           rp.AllowedIPs,
			}
			if transport.Type() == infra.WRRP {
				setPeer.Endpoint = infra.WrrpFakeAddrPort(remoteId.ID().ToUint64()).String()
			} else {
				setPeer.Endpoint = transport.RemoteAddr()
			}
			err := provisioner.AddPeer(setPeer)
			if err != nil {
				p.log.Error("probe add peer failed", err)
				return err
			}

			err = provisioner.ApplyRoute("add", *rp.Address, provisioner.GetIfaceName())
			if err != nil {
				p.log.Error("probe apply route failed", err)
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
			LocalPeer:              localPeer,
			OnPeerReceived:         onPeerReceived,
			UniversalUdpMuxDefault: p.UniversalUdpMuxDefault,
			ShowLog:                p.showLog,
		})
	}
	probe.newIceDialer = makeIceDialer
	probe.iceDialer = makeIceDialer()
	probe.newWrrpDialer = makeWrrpDialer

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
		return fmt.Errorf("unknown peer: %s", remoteId)
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
