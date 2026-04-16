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

	signal      infra.SignalService
	provisioner infra.Provisioner

	log *log.Logger

	onMessage   func(context.Context, *infra.Message) error
	peerManager *infra.PeerManager
	wrrp        infra.Wrrp

	UniversalUdpMuxDefault *ice.UniversalUDPMuxDefault
}

type ProbeFactoryConfig struct {
	LocalId                infra.PeerIdentity
	Signal                 infra.SignalService
	OnMessage              func(context.Context, *infra.Message)
	PeerManager            *infra.PeerManager
	Wrrp                   infra.Wrrp
	UniversalUdpMuxDefault *ice.UniversalUDPMuxDefault
	Provisioner            infra.Provisioner
}

type ProbeFactoryOptions func(*ProbeFactory)

func WithOnMessage(onMessage func(context.Context, *infra.Message) error) ProbeFactoryOptions {
	return func(p *ProbeFactory) {
		p.onMessage = onMessage
	}
}

func WithProvisioner(provisioner infra.Provisioner) ProbeFactoryOptions {
	return func(p *ProbeFactory) {
		p.provisioner = provisioner
	}
}

func WithWrrp(wrrp infra.Wrrp) ProbeFactoryOptions {
	return func(p *ProbeFactory) {
		p.wrrp = wrrp
	}
}

func (t *ProbeFactory) Configure(opts ...ProbeFactoryOptions) {
	for _, opt := range opts {
		opt(t)
	}
}

func NewProbeFactory(cfg *ProbeFactoryConfig) *ProbeFactory {
	return &ProbeFactory{
		log:                    log.GetLogger("probe-factory"),
		localId:                cfg.LocalId,
		signal:                 cfg.Signal,
		probes:                 make(map[string]*Probe),
		peerManager:            cfg.PeerManager,
		wrrp:                   cfg.Wrrp,
		UniversalUdpMuxDefault: cfg.UniversalUdpMuxDefault,
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

	wrrpDialer, err := NewWrrpDialer(&WrrpDialerConfig{
		LocalId:        p.localId,
		RemoteId:       remoteId,
		Wrrp:           p.wrrp,
		Sender:         p.signal.Send,
		LocalPeer:      localPeer,
		OnPeerReceived: onPeerReceived,
	})
	if err != nil {
		return nil, err
	}

	var probe *Probe
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
			p.log.Info("connection established", "transportType", transport.Type(), "remoteAddr", transport.RemoteAddr())
			setPeer := &infra.SetPeer{
				PublicKey:            remoteId.PublicKey.String(),
				PersistentKeepalived: infra.PersistentKeepalive,
				AllowedIPs:           rp.AllowedIPs,
			}
			if transport.Type() == infra.WRRP {
				setPeer.Endpoint = fmt.Sprintf("wrrp://%d", remoteId.ID().ToUint64())
			} else {
				setPeer.Endpoint = transport.RemoteAddr()
			}
			err := p.provisioner.AddPeer(setPeer)
			if err != nil {
				p.log.Error("probe add peer failed", err)
				return err
			}

			err = p.provisioner.ApplyRoute("add", *rp.Address, p.provisioner.GetIfaceName())
			if err != nil {
				p.log.Error("probe apply route failed", err)
				return err
			}

			return p.provisioner.SetupNAT(rp.InterfaceName)
		},
		onFailure: func(err error) error {
			mu.Lock()
			if firstFailureAt.IsZero() {
				firstFailureAt = time.Now()
			}
			elapsed := time.Since(firstFailureAt)
			mu.Unlock()

			// After 60s of failed reconnection, give up and let the configmap
			// drive the next connection attempt: the management server will push
			// PeersRemoved when it detects the peer offline, and PeersAdded
			// when it comes back, creating a fresh probe at that point.
			if elapsed >= 60*time.Second {
				p.log.Info("peer unreachable for 60s, closing probe", "remoteId", remoteId.AppID)
				p.Remove(remoteId.AppID)
				return nil
			}
			p.log.Warn("discover failed, retrying in 10s", "remoteId", remoteId.AppID, "err", err)
			time.AfterFunc(10*time.Second, probe.restart)
			return nil
		},
		wrrpDialer: wrrpDialer,
	}

	// makeIceDialer is a factory that creates a fresh iceDialer, wired to
	// call probe.restart() on close so reconnection works automatically.
	makeIceDialer := func() infra.Dialer {
		return NewIceDialer(&ICEDialerConfig{
			LocalId:                p.localId,
			RemoteId:               remoteId,
			Sender:                 p.signal.Send,
			LocalPeer:              localPeer,
			OnPeerReceived:         onPeerReceived,
			UniversalUdpMuxDefault: p.UniversalUdpMuxDefault,
			OnClose: func(_ infra.PeerIdentity) {
				probe.restart()
			},
			// After the old session is torn down (remote restarted mid-session),
			// immediately re-dispatch the triggering SYN to the newly created
			// dialer so we don't wait for the remote's next 2-second retry cycle.
			OnSynOnActiveAgent: func(ctx context.Context, rid infra.PeerIdentity, packet *grpc.SignalPacket) {
				if err := probe.Handle(ctx, rid, packet); err != nil {
					p.log.Error("re-dispatch SYN to restarted dialer", err)
				}
			},
		})
	}
	probe.newIceDialer = makeIceDialer
	probe.iceDialer = makeIceDialer()

	p.Register(remoteId, probe)
	return probe, nil
}

// Handle is the NATS SignalHandler boundary: remoteId is PeerID from packet.SenderId.
// It resolves to a full PeerIdentity via PeerManager before passing down.
func (p *ProbeFactory) Handle(ctx context.Context, remoteId infra.PeerID, packet *grpc.SignalPacket) error {
	p.log.Debug("Handle packet", "remoteId", remoteId, "packet", packet)

	// Config messages pushed from the management server (not peer-to-peer ICE packets).
	if packet.Type == grpc.PacketType_MESSAGE {
		if p.onMessage == nil {
			return nil
		}
		var msg infra.Message
		if err := json.Unmarshal(packet.GetMessage().Content, &msg); err != nil {
			return fmt.Errorf("handle MESSAGE: unmarshal: %w", err)
		}
		return p.onMessage(ctx, &msg)
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
