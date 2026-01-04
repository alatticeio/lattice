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

package probe

import (
	"context"
	"sync"
	"wireflow/internal/core/domain"
	"wireflow/internal/core/manager"
	"wireflow/internal/grpc"
	"wireflow/internal/log"

	"github.com/wireflowio/ice"
)

type TransportFactory struct {
	mu                     sync.Mutex
	transports             map[string]domain.Transport
	sender                 domain.SignalService
	keyManager             domain.KeyManager
	universalUdpMuxDefault *ice.UniversalUDPMuxDefault
	configurer             domain.Configurer
	peerManager            *manager.PeerManager
	log                    *log.Logger
}

func NewTransportFactory(sender domain.SignalService, universalUdpMuxDefault *ice.UniversalUDPMuxDefault) *TransportFactory {
	return &TransportFactory{
		transports:             make(map[string]domain.Transport),
		sender:                 sender,
		universalUdpMuxDefault: universalUdpMuxDefault,
		log:                    log.NewLogger(log.Loglevel, "wireflow"),
	}
}

type FactoryOptions func(*TransportFactory)

func WithKeyManager(keyManager domain.KeyManager) FactoryOptions {
	return func(t *TransportFactory) {
		t.keyManager = keyManager
	}
}

func WithConfigurer(configure domain.Configurer) FactoryOptions {
	return func(t *TransportFactory) {
		t.configurer = configure
	}
}

func WithPeerManager(peerManager *manager.PeerManager) FactoryOptions {
	return func(t *TransportFactory) {
		t.peerManager = peerManager
	}
}

func (t *TransportFactory) Configure(opts ...FactoryOptions) {
	for _, opt := range opts {
		opt(t)
	}
}

func (t *TransportFactory) MakeTransport(localId, peerId string) (domain.Transport, error) {
	transport, err := NewPionTransport(&ICETransportConfig{
		Sender:                 t.sender.Send,
		PeerId:                 peerId,
		LocalId:                localId,
		UniversalUdpMuxDefault: t.universalUdpMuxDefault,
		Configurer:             t.configurer,
		PeerManager:            t.peerManager,
	})

	if err != nil {
		return nil, err
	}

	transport.onClose = func(peerId string) {
		t.mu.Lock()
		defer t.mu.Unlock()
		delete(t.transports, peerId)
		t.log.Infof("transport: peer %s closed and removed from factory", peerId)
	}
	t.transports[peerId] = transport
	return transport, nil
}

func (t *TransportFactory) GetTransport(peerId string) (domain.Transport, error) {
	var err error
	t.mu.Lock()
	defer t.mu.Unlock()
	transport, ok := t.transports[peerId]
	if !ok {
		transport, err = t.MakeTransport(t.keyManager.GetPublicKey(), peerId)
		if err != nil {
			return nil, err
		}
	}
	return transport, nil
}

func (t *TransportFactory) HandleSignal(ctx context.Context, peerId string, packet *grpc.SignalPacket) error {
	transport := t.transports[peerId]
	var err error
	if transport == nil {
		transport, err = t.GetTransport(peerId)
		if err != nil {
			return err
		}
	}

	return transport.HandleSignal(ctx, peerId, packet)
}
