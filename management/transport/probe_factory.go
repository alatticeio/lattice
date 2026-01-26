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
	"fmt"
	"sync"
	"wireflow/internal/grpc"
	"wireflow/internal/infra"
	"wireflow/internal/log"

	"github.com/wireflowio/ice"
)

type ProbeFactory struct {
	localId infra.PeerID

	mu     sync.RWMutex
	probes map[uint64]*Probe

	wrrpMu     sync.RWMutex
	wrrpProbes map[string]*Probe

	signal      infra.SignalService
	provisioner infra.Provisioner

	log *log.Logger

	onMessage   func(context.Context, *infra.Message) error
	peerManager *infra.PeerManager
	wrrp        infra.Wrrp
	peerStore   *infra.PeerStore

	UniversalUdpMuxDefault *ice.UniversalUDPMuxDefault
}

type ProbeFactoryConfig struct {
	LocalId                infra.PeerID
	Signal                 infra.SignalService
	OnMessage              func(context.Context, *infra.Message)
	PeerManager            *infra.PeerManager
	PeerStore              *infra.PeerStore
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
		probes:                 make(map[uint64]*Probe),
		peerManager:            cfg.PeerManager,
		wrrp:                   cfg.Wrrp,
		peerStore:              cfg.PeerStore,
		UniversalUdpMuxDefault: cfg.UniversalUdpMuxDefault,
	}
}

func (f *ProbeFactory) Register(remoteId infra.PeerID, probe *Probe) {
	f.probes[remoteId.ToUint64()] = probe
}

func (f *ProbeFactory) Get(remoteId infra.PeerID) (*Probe, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	var err error
	probe := f.probes[remoteId.ToUint64()]
	if probe == nil {
		probe, err = f.NewProbe(remoteId)
		if err != nil {
			return nil, err
		}
	}
	return probe, err
}

func (f *ProbeFactory) Remove(remoteId uint64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.probes, remoteId)
}

func (p *ProbeFactory) NewProbe(remoteId infra.PeerID) (*Probe, error) {
	wrrpDialer, err := NewWrrpDialer(&WrrpDialerConfig{
		LocalId:     p.localId,
		RemoteId:    remoteId,
		Wrrp:        p.wrrp,
		Sender:      p.signal.Send,
		PeerStore:   p.peerStore,
		PeerManager: p.peerManager,
	})
	if err != nil {
		return nil, err
	}

	remoteKey, b := p.peerStore.GetKeyByID(remoteId)
	if !b {
		return nil, fmt.Errorf("peer not found: %s", remoteId)
	}

	peer := p.peerManager.GetPeer(remoteId.ToUint64())
	probe := &Probe{
		log:      p.log,
		localId:  p.localId,
		remoteId: remoteId,
		signal:   p.signal,
		state:    ice.ConnectionStateNew,
		onSuccess: func(transport infra.Transport) error {
			p.log.Info("connection established", "transportTypoe", transport.Type(), "remoteAddr", transport.RemoteAddr())
			setPeer := &infra.SetPeer{
				//Endpoint:             transport.RemoteAddr(),
				PublicKey:            remoteKey.String(),
				PersistentKeepalived: infra.PersistentKeepalive,
				AllowedIPs:           peer.AllowedIPs,
			}
			if transport.Type() == infra.WRRP {
				setPeer.Endpoint = fmt.Sprintf("wrrp://%d", remoteId.ToUint64())
			} else {
				setPeer.Endpoint = transport.RemoteAddr()
			}
			err := p.provisioner.AddPeer(setPeer)
			if err != nil {
				p.log.Error("probe add peer failed", err)
				return err
			}

			err = p.provisioner.ApplyRoute("add", *peer.Address, p.provisioner.GetIfaceName())
			if err != nil {
				p.log.Error("probe apply route failed", err)
				return err
			}

			return p.provisioner.SetupNAT(peer.InterfaceName)
		},

		iceDialer: NewIceDialer(&ICEDialerConfig{
			LocalId:                p.localId,
			RemoteId:               remoteId,
			Sender:                 p.signal.Send,
			PeerManager:            p.peerManager,
			PeerStore:              p.peerStore,
			UniversalUdpMuxDefault: p.UniversalUdpMuxDefault,
		}),
		wrrpDialer: wrrpDialer,
	}

	p.Register(remoteId, probe)
	return probe, nil
}

func (p *ProbeFactory) Handle(ctx context.Context, remoteId infra.PeerID, packet *grpc.SignalPacket) error {
	p.log.Info("Handle packet", "remoteId", remoteId, "packet", packet)
	probe, err := p.Get(remoteId)
	if err != nil {
		return err
	}
	return probe.Handle(ctx, remoteId, packet)
}

func (p *ProbeFactory) OnReceive(sessionId [28]byte, data []byte) error {
	return nil
}

// TODO
func (p *ProbeFactory) Allows(remoteId string) bool {
	return true
}
