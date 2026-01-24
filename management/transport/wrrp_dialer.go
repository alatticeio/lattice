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
	"net"
	"sync"
	"time"
	"wireflow/internal/grpc"
	"wireflow/internal/infra"
	"wireflow/internal/log"
	"wireflow/pkg/wrrp"

	"github.com/wireflowio/ice"
	"google.golang.org/protobuf/proto"
)

var (
	_ infra.Dialer = (*wrrpDialer)(nil)
)

type wrrpDialer struct {
	mu        sync.Mutex
	log       *log.Logger
	localId   infra.PeerID
	remoteId  infra.PeerID
	wrrp      infra.Wrrp
	readyChan chan struct{}
	closeOnce sync.Once
	cancel    context.CancelFunc
	sender    func(ctx context.Context, peerId infra.PeerID, data []byte) error

	sm *SessionManager
}

type WrrpDialerConfig struct {
	LocalId   infra.PeerID
	RemoteId  infra.PeerID
	Wrrp      infra.Wrrp
	SM        *SessionManager
	SessionId uint64

	Sender func(ctx context.Context, peerId infra.PeerID, data []byte) error
}

func NewWrrpDialer(cfg *WrrpDialerConfig) (infra.Dialer, error) {
	dialer := &wrrpDialer{
		log:       log.GetLogger("wrrp-dialer"),
		localId:   cfg.LocalId,
		remoteId:  cfg.RemoteId,
		wrrp:      cfg.Wrrp,
		readyChan: make(chan struct{}),
		sm:        cfg.SM,
		sender:    cfg.Sender,
	}

	return dialer, nil
}

func (w *wrrpDialer) Prepare(ctx context.Context, remoteId infra.PeerID) error {
	// only send syn when localId > remoteId
	if w.localId.String() < remoteId.String() {
		w.log.Info("localId < remoteId, ignore prepare")
		return nil
	}

	// send syn
	go func() {
		// send syn
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()

		// safe
		w.mu.Lock()
		w.cancel = cancel
		w.mu.Unlock()
		for {
			select {
			case <-ctx.Done():
				w.log.Warn("send syn canceled", "err", ctx.Err())
				return
			case <-ticker.C:
				w.log.Info("send syn")
				err := w.sendPacket(ctx, remoteId, grpc.PacketType_HANDSHAKE_SYN, nil)
				if err != nil {
					w.log.Error("send syn failed", err)
				}
			}
		}
	}()

	return nil
}

func (w *wrrpDialer) sendPacket(ctx context.Context, remoteId infra.PeerID, packetType grpc.PacketType, candidate ice.Candidate) error {
	p := &grpc.SignalPacket{
		Type:     packetType,
		Dialer:   grpc.DialerType_WRRP,
		SenderId: w.localId.ToUint64(),
	}

	switch packetType {
	case grpc.PacketType_HANDSHAKE_SYN, grpc.PacketType_HANDSHAKE_ACK:
		p.Payload = &grpc.SignalPacket_Handshake{
			Handshake: &grpc.Handshake{
				Timestamp: time.Now().Unix(),
				Sid:       w.localId.ToUint64(),
			},
		}
	}

	data, err := proto.Marshal(p)
	if err != nil {
		return err
	}

	w.log.Info("send packet", "localId", w.localId, "remoteId", remoteId, "packetType", packetType)
	return w.sender(ctx, remoteId, data)
}

func (w *wrrpDialer) sendOfferFromWrrp(ctx context.Context) error {
	p := &grpc.SignalPacket{
		Type:     grpc.PacketType_OFFER,
		Dialer:   grpc.DialerType_WRRP,
		SenderId: w.localId.ToUint64(),
		Payload: &grpc.SignalPacket_Offer{
			Offer: &grpc.Offer{
				SrcPeerID: w.localId.String(),
				// TODO add ufrag pwd
			},
		},
	}

	data, err := proto.Marshal(p)
	if err != nil {
		return err
	}
	return w.wrrp.Send(ctx, w.remoteId.ToUint64(), wrrp.Probe, data)
}

func (w *wrrpDialer) Handle(ctx context.Context, remoteId infra.PeerID, packet *grpc.SignalPacket) error {
	if packet.Dialer != grpc.DialerType_WRRP {
		return nil
	}
	switch packet.Type {
	case grpc.PacketType_HANDSHAKE_SYN:
		return w.sendPacket(ctx, remoteId, grpc.PacketType_HANDSHAKE_ACK, nil)
	case grpc.PacketType_HANDSHAKE_ACK:
		w.cancel()
		// send offer
		w.closeOnce.Do(func() {
			close(w.readyChan)
		})
		return w.sendOfferFromWrrp(ctx)
	case grpc.PacketType_OFFER:
		w.closeOnce.Do(func() {
			close(w.readyChan)
		})
	}
	return nil
}

func (w *wrrpDialer) Dial(ctx context.Context) (infra.Transport, error) {
	<-w.readyChan
	return &WrrpTransport{
		conn: &WrrpRawConn{
			wrrp:       w.wrrp,
			remoteAddr: w.wrrp.RemoteAddr(),
		},
	}, nil
}

func (w *wrrpDialer) Type() infra.DialerType {
	return infra.WRRP_DIALER
}

type WrrpTransport struct {
	conn net.Conn
}

func (w WrrpTransport) Priority() uint8 {
	return infra.PriorityRelay
}

func (w WrrpTransport) Close() error {
	return nil
}

func (w WrrpTransport) Write(data []byte) error {
	return nil
}

func (w WrrpTransport) Read(buff []byte) (int, error) {
	return 0, nil
}

func (w WrrpTransport) RemoteAddr() string {
	return w.conn.RemoteAddr().String()
}

func (w WrrpTransport) Type() infra.TransportType {
	return infra.WRRP
}
