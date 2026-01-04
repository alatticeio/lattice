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
	"fmt"
	"net"
	"sync"
	"time"
	"wireflow/internal/core/domain"
	"wireflow/internal/core/manager"
	"wireflow/internal/grpc"
	"wireflow/internal/log"

	"github.com/pion/logging"
	"github.com/wireflowio/ice"
	"google.golang.org/protobuf/proto"
)

var (
	_ domain.Transport = (*PionTransport)(nil)
)

// PionTransport using pion ice for transport
type PionTransport struct {
	su            sync.Mutex
	log           *log.Logger
	localId       string
	sender        func(ctx context.Context, peerId string, data []byte) error
	onClose       func(peerId string)
	Configurer    domain.Configurer
	peerId        string
	agent         *AgentWrapper
	state         domain.TransportState
	probeAckChan  chan struct{}
	closeOnce     sync.Once
	ackClose      sync.Once
	OfferRecvChan chan struct{}

	universalUdpMuxDefault *ice.UniversalUDPMuxDefault

	peers *manager.PeerManager
}

type ICETransportConfig struct {
	Sender                 func(ctx context.Context, peerId string, data []byte) error
	PeerId                 string
	LocalId                string
	OnClose                func(peerId string)
	UniversalUdpMuxDefault *ice.UniversalUDPMuxDefault
	Configurer             domain.Configurer
	PeerManager            *manager.PeerManager
}

func NewPionTransport(cfg *ICETransportConfig) (*PionTransport, error) {
	t := &PionTransport{
		log:                    log.NewLogger(log.Loglevel, "transport"),
		onClose:                cfg.OnClose,
		sender:                 cfg.Sender,
		localId:                cfg.LocalId,
		peerId:                 cfg.PeerId,
		probeAckChan:           make(chan struct{}),
		OfferRecvChan:          make(chan struct{}),
		universalUdpMuxDefault: cfg.UniversalUdpMuxDefault,
		Configurer:             cfg.Configurer,
		peers:                  cfg.PeerManager,
	}
	var err error
	t.agent, err = t.getAgent(cfg.PeerId)
	if err != nil {
		return nil, err
	}

	return t, nil
}

func (t *PionTransport) getAgent(peerID string) (*AgentWrapper, error) {
	f := logging.NewDefaultLoggerFactory()
	f.DefaultLogLevel = logging.LogLevelDebug
	// 创建新 Agent
	iceAgent, err := ice.NewAgent(&ice.AgentConfig{
		UDPMux:         t.universalUdpMuxDefault.UDPMuxDefault,
		UDPMuxSrflx:    t.universalUdpMuxDefault,
		NetworkTypes:   []ice.NetworkType{ice.NetworkTypeUDP4},
		Urls:           []*ice.URL{{Scheme: ice.SchemeTypeSTUN, Host: "81.68.109.143", Port: 3478}},
		Tiebreaker:     uint64(ice.NewTieBreaker()),
		LoggerFactory:  f,
		CandidateTypes: []ice.CandidateType{ice.CandidateTypeHost, ice.CandidateTypeServerReflexive},
	})

	var agent *AgentWrapper
	if err == nil {
		agent = &AgentWrapper{
			Agent: iceAgent,
		}
		// 绑定状态监听，成功后更新 WireGuard
		agent.OnConnectionStateChange(func(s ice.ConnectionState) {
			if s == ice.ConnectionStateConnected {
				pair, err := agent.GetSelectedCandidatePair()
				if err != nil {
					t.log.Errorf("Get selected candidate pair error: %v", err)
					return
				}

				if err := t.AddPeer(peerID, fmt.Sprintf("%s:%d", pair.Remote.Address(), pair.Remote.Port())); err != nil {
					t.log.Errorf("Add peer error: %v", err)
				}
			}

			if s == ice.ConnectionStateDisconnected || s == ice.ConnectionStateFailed {
				t.Close()
			}
		})
	}

	if err = agent.OnCandidate(func(candidate ice.Candidate) {
		if candidate == nil {
			return
		}

		if err = t.sendCandidate(context.TODO(), agent, peerID, candidate); err != nil {
			t.log.Errorf("Send candidate error: %v", err)
		}

		t.log.Infof("Sending candidate: %v", candidate)
	}); err != nil {
		return nil, err
	}

	return agent, err
}

func (t *PionTransport) Prepare(ctx context.Context, peerId string, send func(ctx context.Context, peerId string, data []byte) error) error {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	//1. start handshake syn
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				t.log.Errorf("stop send syn packet: %v", ctx.Err())
				return
			case <-ticker.C:
				// send syn
				t.probePacket(ctx, peerId, grpc.PacketType_HANDSHAKE_SYN)
			}
		}

	}()

	//waiting probe ack
	t.log.Infof("waiting for [%s] preProbe ACK...", peerId)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.probeAckChan:
		// send offer
		t.log.Infof("preProbe ACK received, will sending offer to: %s", peerId)
		cancel()
		return t.agent.GatherCandidates()
	}
}

func (t *PionTransport) probePacket(ctx context.Context, peerId string, packetType grpc.PacketType) error {
	packet := &grpc.SignalPacket{
		SenderId: t.localId,
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

	return t.sender(ctx, peerId, data)
}

func (t *PionTransport) HandleSignal(ctx context.Context, peerId string, packet *grpc.SignalPacket) error {
	var err error
	switch packet.Type {
	case grpc.PacketType_HANDSHAKE_SYN:
		// send ack
		if err = t.probePacket(ctx, peerId, grpc.PacketType_HANDSHAKE_ACK); err != nil {
			return err
		}
	case grpc.PacketType_HANDSHAKE_ACK:
		// ack chan close, will send or waiting offer
		t.log.Infof("probe ACK received from [%s]", peerId)
		t.ackClose.Do(func() {
			close(t.probeAckChan)
		})
	default:
		agent := t.agent
		offer := packet.GetOffer()

		//第一次接收
		if !agent.IsCredentialsInited.Load() {
			agent.RUfrag = offer.Ufrag
			agent.RPwd = offer.Pwd
			agent.RTieBreaker = offer.TieBreaker
			agent.IsCredentialsInited.Store(true)
			close(t.OfferRecvChan)

			//start
			go func() {
				t.Start(ctx, peerId)
			}()
		}

		candidate, err := ice.UnmarshalCandidate(offer.Candidate)
		if err != nil {
			return err
		}

		if err = agent.AddRemoteCandidate(candidate); err != nil {
			return err
		}

		return nil
	}

	return nil
}

func (t *PionTransport) OnConnectionStateChange(state domain.TransportState) error {
	return nil
}

func (t *PionTransport) Start(ctx context.Context, peerId string) (err error) {
	agent := t.agent
	go func() {
		ctx, timeout := context.WithTimeout(ctx, 60*time.Second)
		defer timeout()
		select {
		case <-ctx.Done():
			t.log.Errorf("close peer %s connection", peerId)
			return
		case <-t.OfferRecvChan:
			if agent.GetTieBreaker() > agent.RTieBreaker {
				_, err = agent.Dial(ctx, agent.RUfrag, agent.RPwd)
			} else {
				_, err = agent.Accept(ctx, agent.RUfrag, agent.RPwd)
			}

			if err != nil {
				t.log.Errorf("err: %v", err)
			}
		}

	}()

	return nil

}

func (t *PionTransport) RawConn() (net.Conn, error) {
	return nil, nil
}

func (t *PionTransport) State() domain.TransportState {
	return t.state
}

func (t *PionTransport) Close() error {
	t.log.Infof("closing transport for : %s", t.peerId)
	t.closeOnce.Do(func() {
		if err := t.agent.Close(); err != nil {
			t.log.Errorf("close agent error: %v", err)
		}

		if t.onClose != nil {
			t.onClose(t.peerId)
		}
	})

	return nil
}

func (t *PionTransport) sendCandidate(ctx context.Context, agent *AgentWrapper, peerId string, candidate ice.Candidate) error {
	//if !t.isShouldSendOffer(t.localId, peerId) {
	//	return nil
	//}
	ufrag, pwd, err := agent.GetLocalUserCredentials()
	if err != nil {
		return err
	}
	packet := &grpc.SignalPacket{
		Type:     grpc.PacketType_OFFER,
		SenderId: t.localId,
		Payload: &grpc.SignalPacket_Offer{
			Offer: &grpc.Offer{
				Ufrag:      ufrag,
				Pwd:        pwd,
				TieBreaker: agent.GetTieBreaker(),
				Candidate:  candidate.Marshal(),
			},
		},
	}

	data, err := proto.Marshal(packet)
	if err != nil {
		t.log.Errorf("Marshal packet error: %v", err)
		return err
	}

	if err = t.sender(context.TODO(), peerId, data); err != nil {
		t.log.Errorf("send candidate: %v", err)
		return err
	}

	return nil
}

func (t *PionTransport) isShouldSendOffer(localId, peerId string) bool {
	return localId > peerId
}

func (t *PionTransport) updateTransportState(newState domain.TransportState) {
	t.su.Lock()
	defer t.su.Unlock()
	oldState := t.state
	t.state = newState
	if oldState != newState {
		t.log.Infof("Transport State changed: %v -> %v", t.peerId, oldState, newState)
		// 这里可以触发回调通知 Probe 层或 WireGuard 层
		t.OnConnectionStateChange(newState)
	}
}

func (t *PionTransport) AddPeer(peerId, addr string) error {
	peer := t.peers.GetPeer(peerId)
	return t.Configurer.AddPeer(&domain.SetPeer{
		Endpoint:             addr,
		PublicKey:            peerId,
		AllowedIPs:           peer.AllowedIPs,
		PersistentKeepalived: 25,
	})
}
