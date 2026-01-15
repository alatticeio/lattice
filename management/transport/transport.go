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
	"sync"
	"wireflow/internal/infra"
	"wireflow/internal/log"

	"github.com/wireflowio/ice"
)

type TransportFactory struct {
	mu                     sync.Mutex
	transports             map[string]infra.Transport
	sender                 infra.SignalService
	keyManager             infra.KeyManager
	universalUdpMuxDefault *ice.UniversalUDPMuxDefault
	provisioner            infra.Provisioner
	peerManager            *infra.PeerManager
	log                    *log.Logger
	onClose                func(remoteId string) error

	probe   infra.Probe
	showLog bool
}

func NewTransportFactory(sender infra.SignalService, showLog bool, universalUdpMuxDefault *ice.UniversalUDPMuxDefault) *TransportFactory {
	return &TransportFactory{
		transports:             make(map[string]infra.Transport),
		sender:                 sender,
		universalUdpMuxDefault: universalUdpMuxDefault,
		log:                    log.GetLogger("transport-factory"),
		showLog:                showLog,
	}
}

type FactoryOptions func(*TransportFactory)

func WithKeyManager(keyManager infra.KeyManager) FactoryOptions {
	return func(t *TransportFactory) {
		t.keyManager = keyManager
	}
}

func WithProvisioner(provisioner infra.Provisioner) FactoryOptions {
	return func(t *TransportFactory) {
		t.provisioner = provisioner
	}
}

func WithPeerManager(peerManager *infra.PeerManager) FactoryOptions {
	return func(t *TransportFactory) {
		t.peerManager = peerManager
	}
}

func WithOnClose(onClose func(remoteId string) error) FactoryOptions {
	return func(t *TransportFactory) {
		t.onClose = onClose
	}
}

func (t *TransportFactory) Configure(opts ...FactoryOptions) {
	for _, opt := range opts {
		opt(t)
	}
}

func (t *TransportFactory) MakeTransport(localId, remoteId string, onConnectionStateChange func(state ice.ConnectionState)) (infra.Transport, error) {
	transport, err := NewPionTransport(&ICETransportConfig{
		Sender:                  t.sender.Send,
		RemoteId:                remoteId,
		LocalId:                 localId,
		UniversalUdpMuxDefault:  t.universalUdpMuxDefault,
		Configurer:              t.provisioner,
		PeerManager:             t.peerManager,
		ShowLog:                 t.showLog,
		OnConnectionStateChange: onConnectionStateChange,
	})

	if err != nil {
		return nil, err
	}

	orignalClose := t.onClose
	transport.onClose = func(remoteId string) {
		if orignalClose != nil {
			orignalClose(remoteId)
		}
		t.mu.Lock()
		defer t.mu.Unlock()
		delete(t.transports, remoteId)
		t.log.Debug("transport: probe closed and removed from factory", "remoteId", remoteId)
	}
	t.transports[remoteId] = transport
	return transport, nil
}

func (t *TransportFactory) GetTransport(remoteId string, onConnectionStateChange func(state ice.ConnectionState)) (infra.Transport, error) {
	var err error
	t.mu.Lock()
	defer t.mu.Unlock()
	transport, ok := t.transports[remoteId]
	if !ok {
		transport, err = t.MakeTransport(t.keyManager.GetPublicKey(), remoteId, onConnectionStateChange)
		if err != nil {
			return nil, err
		}
	}
	return transport, nil
}
