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
	"sync"
	"wireflow/internal/core/infra"
	"wireflow/internal/grpc"
	"wireflow/internal/log"
)

type ProbeFactory struct {
	localId string
	mu      sync.RWMutex
	probes  map[string]*Probe
	signal  infra.SignalService
	factory *TransportFactory

	log *log.Logger

	onMessage func(context.Context, *infra.Message)
}

type ProbeFactoryConfig struct {
	LocalId   string
	Signal    infra.SignalService
	Factory   *TransportFactory
	OnMessage func(context.Context, *infra.Message)
}

type ProbeFactoryOptions func(*ProbeFactory)

func WithSignal(signal infra.SignalService) ProbeFactoryOptions {
	return func(p *ProbeFactory) {
		p.signal = signal
	}
}

func (t *ProbeFactory) Configure(opts ...ProbeFactoryOptions) {
	for _, opt := range opts {
		opt(t)
	}
}

func NewProbeFactory(cfg *ProbeFactoryConfig) *ProbeFactory {
	return &ProbeFactory{
		log:     log.NewLogger(log.Loglevel, "wireflow"),
		localId: cfg.LocalId,
		signal:  cfg.Signal,
		factory: cfg.Factory,
		probes:  make(map[string]*Probe),
	}
}

func (f *ProbeFactory) Register(remoteId string, probe *Probe) {
	f.probes[remoteId] = probe
}

func (f *ProbeFactory) Get(remoteId string) (*Probe, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	var err error
	probe := f.probes[remoteId]
	if probe == nil {
		probe, err = f.NewProbe(remoteId)
		if err != nil {
			return nil, err
		}
	}
	return probe, err
}

func (f *ProbeFactory) Remove(remoteId string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.probes, remoteId)
}

func (p *ProbeFactory) NewProbe(remoteId string) (*Probe, error) {
	transport, err := p.factory.GetTransport(remoteId)
	if err != nil {
		return nil, err
	}
	probe := &Probe{
		log:             p.log,
		localId:         p.localId,
		peerId:          remoteId,
		factory:         p.factory,
		signal:          p.signal,
		probeAckChan:    make(chan struct{}),
		remoteOfferChan: make(chan struct{}),
		transport:       transport,
	}

	p.Register(remoteId, probe)
	return probe, nil
}

func (f *ProbeFactory) Probe(ctx context.Context, remoteId string) error {
	var err error
	probe, err := f.Get(remoteId)
	if err != nil {
		return err
	}
	return probe.Probe(ctx, remoteId)
}

func (p *ProbeFactory) HandleSignal(ctx context.Context, remoteId string, packet *grpc.SignalPacket) error {
	var (
		err   error
		probe *Probe
	)

	probe, err = p.Get(remoteId)
	if err != nil {
		return err
	}
	p.log.Infof("handle signal packet from: %s, type: %v", remoteId, packet.Type)
	switch packet.Type {
	case grpc.PacketType_MESSAGE:
		var msg infra.Message
		if err := json.Unmarshal(packet.GetMessage().Content, &msg); err != nil {
			return err
		}
		p.onMessage(ctx, &msg)
	case grpc.PacketType_HANDSHAKE_SYN:
		p.log.Infof("receive syn packet from: %s, will sending ack", remoteId)
		// send ack
		if err = probe.probePacket(ctx, remoteId, grpc.PacketType_HANDSHAKE_ACK); err != nil {
			return err
		}
		// TODO add allows check
		if p.Allows(remoteId) {
			return probe.Probe(ctx, remoteId)
		}
	case grpc.PacketType_HANDSHAKE_ACK:
		return probe.HandleAck(ctx, remoteId, packet)
	case grpc.PacketType_OFFER:
		return probe.HandleOffer(ctx, remoteId, packet)
	}

	return nil
}

// TODO
func (p *ProbeFactory) Allows(remoteId string) bool {
	return true
}
