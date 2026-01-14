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
	"net"
	"sync"
	"wireflow/internal/grpc"
	"wireflow/internal/infra"
	"wireflow/internal/log"

	"github.com/pion/logging"
	"github.com/wireflowio/ice"
	"google.golang.org/protobuf/proto"
)

var (
	_ infra.Transport = (*PionTransport)(nil)
)

// PionTransport using pion ice for transport
type PionTransport struct {
	su            sync.Mutex
	log           *log.Logger
	localId       string
	sender        func(ctx context.Context, peerId string, data []byte) error
	onClose       func(peerId string)
	provisioner   infra.Provisioner
	remoteId      string
	agent         *AgentWrapper
	state         ice.ConnectionState
	probeAckChan  chan struct{}
	closeOnce     sync.Once
	ackClose      sync.Once
	OfferRecvChan chan struct{}

	universalUdpMuxDefault *ice.UniversalUDPMuxDefault

	peers *infra.PeerManager
	probe infra.Probe
}

type ICETransportConfig struct {
	Sender                 func(ctx context.Context, peerId string, data []byte) error
	RemoteId               string
	LocalId                string
	OnClose                func(peerId string)
	UniversalUdpMuxDefault *ice.UniversalUDPMuxDefault
	Configurer             infra.Provisioner
	PeerManager            *infra.PeerManager
}

func NewPionTransport(cfg *ICETransportConfig) (*PionTransport, error) {
	t := &PionTransport{
		log:                    log.GetLogger("transport"),
		onClose:                cfg.OnClose,
		sender:                 cfg.Sender,
		localId:                cfg.LocalId,
		remoteId:               cfg.RemoteId,
		probeAckChan:           make(chan struct{}),
		OfferRecvChan:          make(chan struct{}),
		universalUdpMuxDefault: cfg.UniversalUdpMuxDefault,
		provisioner:            cfg.Configurer,
		peers:                  cfg.PeerManager,
	}
	var err error
	t.agent, err = t.getAgent(cfg.RemoteId)
	if err != nil {
		return nil, err
	}

	return t, nil
}

func (t *PionTransport) getAgent(remoteId string) (*AgentWrapper, error) {
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
			t.updateTransportState(s)
			if s == ice.ConnectionStateConnected {
				t.log.Info("Setting new connection", "state", "connected")
				pair, err := agent.GetSelectedCandidatePair()
				if err != nil {
					t.log.Error("Get selected candidate pair", err)
					return
				}

				if err := t.AddPeer(remoteId, fmt.Sprintf("%s:%d", pair.Remote.Address(), pair.Remote.Port())); err != nil {
					t.log.Error("Add peer", err)
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

		if err = t.sendCandidate(context.TODO(), agent, remoteId, candidate); err != nil {
			t.log.Error("Send candidate", err)
		}

		t.log.Info("Sending candidate", "candidate", candidate)
	}); err != nil {
		return nil, err
	}

	return agent, err
}

func (t *PionTransport) Prepare(probe infra.Probe) error {
	t.probe = probe
	return t.agent.GatherCandidates()
}

func (t *PionTransport) HandleOffer(ctx context.Context, remoteId string, packet *grpc.SignalPacket) error {
	agent := t.agent
	offer := packet.GetOffer()

	//第一次接收
	if !agent.IsCredentialsInited.Load() {
		agent.RUfrag = offer.Ufrag
		agent.RPwd = offer.Pwd
		agent.RTieBreaker = offer.TieBreaker
		agent.IsCredentialsInited.Store(true)
	}

	candidate, err := ice.UnmarshalCandidate(offer.Candidate)
	if err != nil {
		return err
	}

	currentData := offer.Current
	var remotePeer infra.Peer
	if err = json.Unmarshal(currentData, &remotePeer); err != nil {
		return err
	}

	// cache peer
	t.peers.AddPeer(t.remoteId, &remotePeer)

	if err = agent.AddRemoteCandidate(candidate); err != nil {
		return err
	}

	return nil
}

func (t *PionTransport) OnConnectionStateChange(state ice.ConnectionState) error {
	return nil
}

func (t *PionTransport) Start(ctx context.Context, remoteId string) (err error) {
	if t.agent.GetTieBreaker() > t.agent.RTieBreaker {
		_, err = t.agent.Dial(ctx, t.agent.RUfrag, t.agent.RPwd)
	} else {
		_, err = t.agent.Accept(ctx, t.agent.RUfrag, t.agent.RPwd)
	}

	return err
}

func (t *PionTransport) RawConn() (net.Conn, error) {
	return nil, nil
}

func (t *PionTransport) State() ice.ConnectionState {
	return t.state
}

func (t *PionTransport) Close() error {
	t.log.Info("closing transport", "remoteId", t.remoteId)
	t.closeOnce.Do(func() {
		if err := t.agent.Close(); err != nil {
			t.log.Error("close agent", err)
		}

		if t.onClose != nil {
			t.onClose(t.remoteId)
		}

		//remove peer
		t.Remove(t.remoteId, "")
	})

	return nil
}

func (t *PionTransport) sendCandidate(ctx context.Context, agent *AgentWrapper, remoteId string, candidate ice.Candidate) error {
	//if !t.isShouldSendOffer(t.localId, remoteId) {
	//	return nil
	//}
	current := t.peers.GetPeer(t.localId)
	currentData, err := json.Marshal(current)
	if err != nil {
		return err
	}
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
				Current:    currentData,
			},
		},
	}

	data, err := proto.Marshal(packet)
	if err != nil {
		t.log.Error("Marshal packet", err)
		return err
	}

	if err = t.sender(context.TODO(), remoteId, data); err != nil {
		t.log.Error("send candidate", err)
		return err
	}

	return nil
}

func (t *PionTransport) isShouldSendOffer(localId, remoteId string) bool {
	return localId > remoteId
}

func (t *PionTransport) updateTransportState(newState ice.ConnectionState) {
	t.su.Lock()
	defer t.su.Unlock()
	t.log.Info("Setting new connection state", "remoteId", t.remoteId, "newState", newState)
	t.probe.OnConnectionStateChange(newState)
	t.OnConnectionStateChange(newState)
}

func (t *PionTransport) AddPeer(remoteId, addr string) error {
	var err error
	peer := t.peers.GetPeer(remoteId)
	if err = t.provisioner.AddPeer(&infra.SetPeer{
		Endpoint:             addr,
		PublicKey:            remoteId,
		AllowedIPs:           peer.AllowedIPs,
		PersistentKeepalived: 25,
	}); err != nil {
		return err
	}

	if err = t.provisioner.ApplyRoute("add", *peer.Address, t.provisioner.GetIfaceName()); err != nil {
		return err
	}
	return nil
}

func (t *PionTransport) Remove(remoteId, addr string) error {
	var err error
	peer := t.peers.GetPeer(remoteId)
	if err = t.provisioner.RemovePeer(&infra.SetPeer{
		Endpoint:             addr,
		PublicKey:            remoteId,
		AllowedIPs:           peer.AllowedIPs,
		PersistentKeepalived: 25,
		Remove:               true,
	}); err != nil {
		return err
	}

	if err = t.provisioner.ApplyRoute("delete", *peer.Address, t.provisioner.GetIfaceName()); err != nil {
		return err
	}
	return nil
}
