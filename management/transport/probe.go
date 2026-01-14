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
	"sync"
	"time"
	"wireflow/internal/grpc"
	"wireflow/internal/infra"
	"wireflow/internal/log"

	"github.com/wireflowio/ice"
	"google.golang.org/protobuf/proto"
)

var (
	_ infra.Probe = (*Probe)(nil)
)

// Probe for probe connection from two peers.
type Probe struct {
	localId         string
	peerId          string
	factory         *TransportFactory
	transport       infra.Transport
	state           ice.ConnectionState
	signal          infra.SignalService
	ctx             context.Context
	cancel          context.CancelFunc
	closeAckOnce    sync.Once
	closeOnce       sync.Once
	probeAckChan    chan struct{}
	remoteOfferChan chan struct{}
	log             *log.Logger
	lastSeen        time.Time
	rtt             time.Duration
}

func (p *Probe) OnConnectionStateChange(state ice.ConnectionState) {
	p.updateState(state)
}

func (p *Probe) Probe(ctx context.Context, remoteId string) error {
	if p.state != ice.ConnectionStateNew {
		return nil
	}
	p.updateState(ice.ConnectionStateChecking)
	// 1. first prepare candidate then send to remoteId
	go func() {
		if err := p.Prepare(ctx, remoteId, p.signal.Send); err != nil {
			p.OnTransportFail(err)
		}
	}()

	return nil
}

func (p *Probe) Prepare(ctx context.Context, remoteId string, send func(ctx context.Context, remoteId string, data []byte) error) error {
	p.log.Info("Prepare probe peer", "remoteId", remoteId)
	probeCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	//1. start handshake syn
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-probeCtx.Done():
				p.log.Error("stop send syn packet", ctx.Err())
				return
			case <-ticker.C:
				// send syn
				p.probePacket(ctx, remoteId, grpc.PacketType_HANDSHAKE_SYN)
			}
		}

	}()

	// start to connect
	if err := p.Start(ctx, remoteId); err != nil {
		p.OnTransportFail(err)
	}

	//waiting probe ack
	p.log.Info("waiting for preProbe ACK...", "remoteId", remoteId)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.probeAckChan:
		cancel()
		// send offer
		p.log.Info("preProbe ACK received, will sending offer", "remoteId", remoteId)
		return p.transport.Prepare(p)
	}
}

func (p *Probe) probePacket(ctx context.Context, remoteId string, packetType grpc.PacketType) error {
	packet := &grpc.SignalPacket{
		SenderId: p.localId,
		Type:     packetType,
		Payload: &grpc.SignalPacket_Handshake{
			Handshake: &grpc.Handshake{
				Timestamp: time.Now().Unix(),
			},
		},
	}

	data, err := proto.Marshal(packet)
	if err != nil {
		return err
	}

	return p.signal.Send(ctx, remoteId, data)
}

func (p *Probe) HandleAck(ctx context.Context, remoteId string, packet *grpc.SignalPacket) error {
	defer func() {
		p.closeAckOnce.Do(func() {
			close(p.probeAckChan)
		})
	}()
	p.updateState(ice.ConnectionStateChecking)
	return nil
}

func (p *Probe) HandleOffer(ctx context.Context, remoteId string, packet *grpc.SignalPacket) error {
	defer func() {
		p.closeOnce.Do(func() {
			close(p.remoteOfferChan)
		})
	}()

	return p.transport.HandleOffer(ctx, remoteId, packet)
}

func (p *Probe) Start(ctx context.Context, remoteId string) error {
	p.log.Info("Start probe pee", "remoteId", remoteId)
	sendReady, recvReady := false, false
	ctx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	go func() {
		for {
			select {
			case <-ctx.Done():
				p.log.Error("stop send ready ack", ctx.Err())
				return
			case <-p.probeAckChan:
				sendReady = true

			case <-p.remoteOfferChan:
				recvReady = true
			}

			//
			if sendReady && recvReady {
				p.log.Info("send ready and recv ready, will dial or accept connection")
				break
			}
		}

		if err := p.transport.Start(ctx, remoteId); err != nil {
			p.OnTransportFail(err)
		}
	}()

	return nil
}

func (p *Probe) Ping(ctx context.Context) error {
	return nil
}

func (p *Probe) OnTransportFail(err error) {
	p.log.Error("OnTransportFail", err)
}

func (p *Probe) updateState(state ice.ConnectionState) {
	p.state = state
}
