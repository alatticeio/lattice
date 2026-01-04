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
	"time"
	"wireflow/internal/core/domain"
)

var (
	_ domain.Probe = (*Probe)(nil)
)

// Probe for probe connection from two peers.
type Probe struct {
	localId   string
	peerId    string
	factory   *TransportFactory
	transport domain.Transport
	sender    domain.SignalService
	ctx       context.Context
	cancel    context.CancelFunc

	lastSeen time.Time
	rtt      time.Duration
}

func (p *Probe) Probe(ctx context.Context, peerID string) error {
	// 1. first prepare candidate then send to peerId
	if err := p.transport.Prepare(ctx, peerID, p.sender.Send); err != nil {
		p.OnTransportFail(err)
	}

	// 2. start ping
	return p.Ping(ctx)
}

func (p *Probe) Ping(ctx context.Context) error {
	return nil
}

func (p *Probe) OnTransportFail(err error) {
}

func NewProbe(localId string, remoteId string, signal domain.SignalService, factory *TransportFactory) (*Probe, error) {
	transport, err := factory.GetTransport(remoteId)
	if err != nil {
		return nil, err
	}
	return &Probe{
		localId:   localId,
		peerId:    remoteId,
		factory:   factory,
		sender:    signal,
		transport: transport,
	}, nil
}
