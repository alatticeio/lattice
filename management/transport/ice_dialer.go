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
	"time"
	"wireflow/internal/grpc"
	"wireflow/internal/infra"
	"wireflow/internal/log"

	"github.com/pion/logging"
	"github.com/pion/stun/v3"
	"github.com/wireflowio/ice"
	"google.golang.org/protobuf/proto"
)

var (
	_ infra.Dialer = (*iceDialer)(nil)
)

type iceDialer struct {
	mu          sync.Mutex
	log         *log.Logger
	localId     infra.PeerID
	remoteId    infra.PeerID
	sender      func(ctx context.Context, peerId infra.PeerID, data []byte) error
	onClose     func(peerId infra.PeerID)
	provisioner infra.Provisioner // nolint
	agent       *AgentWrapper
	closeOnce   sync.Once
	showLog     bool
	peerManager *infra.PeerManager

	peerStore *infra.PeerStore

	// offerReady start Dial() after receiving offer
	offerReady chan struct{}
	cancel     context.CancelFunc
	ackChan    chan struct{} // nolint

	universalUdpMuxDefault *ice.UniversalUDPMuxDefault
}

type ICEDialerConfig struct {
	Sender                  func(ctx context.Context, peerId infra.PeerID, data []byte) error
	LocalId                 infra.PeerID
	RemoteId                infra.PeerID
	OnClose                 func(peerId infra.PeerID)
	UniversalUdpMuxDefault  *ice.UniversalUDPMuxDefault
	Configurer              infra.Provisioner
	PeerManager             *infra.PeerManager
	PeerStore               *infra.PeerStore
	ShowLog                 bool
	OnConnectionStateChange func(state ice.ConnectionState)
}

func (i *iceDialer) Handle(ctx context.Context, remoteId infra.PeerID, packet *grpc.SignalPacket) error {
	if packet.Dialer != grpc.DialerType_ICE {
		return nil
	}
	switch packet.Type {
	case grpc.PacketType_HANDSHAKE_ACK:
		// cancel send syn
		i.cancel()
		// start send offer
		return i.agent.GatherCandidates()
	case grpc.PacketType_HANDSHAKE_SYN:
		// send ack to remote
		if err := i.sendPacket(ctx, i.remoteId, grpc.PacketType_HANDSHAKE_ACK, nil); err != nil {
			return err
		}

		// init agent
		if i.agent == nil {
			agent, err := i.getAgent(remoteId)
			if err != nil {
				panic(err)
			}
			i.agent = agent
		}
		// start send offer (locaId < remoteId)
		return i.agent.GatherCandidates()
	case grpc.PacketType_OFFER, grpc.PacketType_ANSWER:
		i.log.Debug("receive offer", "remoteId", remoteId)
		offer := packet.GetOffer() //第一次接收
		if !i.agent.IsCredentialsInited.Load() {
			i.agent.RUfrag = offer.Ufrag
			i.agent.RPwd = offer.Pwd
			i.agent.RTieBreaker = offer.TieBreaker
			i.agent.IsCredentialsInited.Store(true)

			currentData := offer.Current
			var remotePeer infra.Peer
			if err := json.Unmarshal(currentData, &remotePeer); err != nil {
				return err
			}

			// cache peer
			i.peerManager.AddPeer(i.remoteId.ToUint64(), &remotePeer)
		}

		candidate, err := ice.UnmarshalCandidate(offer.Candidate)
		if err != nil {
			return err
		}

		if err = i.agent.AddRemoteCandidate(candidate); err != nil {
			return err
		}

		i.log.Debug("add remote candidate", "candidate", candidate)
		i.closeOnce.Do(func() {
			close(i.offerReady)
		})
	}
	return nil
}

func NewIceDialer(cfg *ICEDialerConfig) infra.Dialer {
	return &iceDialer{
		log:                    log.GetLogger("ice-dialer"),
		sender:                 cfg.Sender,
		onClose:                cfg.OnClose,
		localId:                cfg.LocalId,
		remoteId:               cfg.RemoteId,
		universalUdpMuxDefault: cfg.UniversalUdpMuxDefault,
		showLog:                cfg.ShowLog,
		peerManager:            cfg.PeerManager,
		peerStore:              cfg.PeerStore,
		offerReady:             make(chan struct{}),
	}
}

// Prepare prepare to send offer, send handshake packet first to remote when localId > remoteId.
func (i *iceDialer) Prepare(ctx context.Context, remoteId infra.PeerID) error {
	// init agent
	if i.agent == nil {
		agent, err := i.getAgent(remoteId)
		if err != nil {
			panic(err)
		}
		i.agent = agent
	}
	localIdStr, remoteIdStr := i.localId.String(), remoteId.String()
	i.log.Debug("prepare ice", "localId", i.localId, "remoteId", remoteId, "shouldSync", localIdStr > remoteIdStr)
	// only send syn when localId > remoteId
	if i.localId.String() < remoteId.String() {
		i.log.Debug("localId < remoteId, ignore prepare")
		return nil
	}

	// send syn
	go func() {
		// send syn
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		newCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()

		// safe
		i.mu.Lock()
		if i.cancel != nil {
			i.cancel()
		}
		i.cancel = cancel
		i.mu.Unlock()
		for {
			select {
			case <-newCtx.Done():
				i.log.Warn("send syn canceled", "err", newCtx.Err())
				return
			case <-ticker.C:
				i.log.Debug("send syn")
				err := i.sendPacket(ctx, remoteId, grpc.PacketType_HANDSHAKE_SYN, nil)
				if err != nil {
					i.log.Error("send syn failed", err)
				}
			}
		}
	}()

	return nil
}

func (i *iceDialer) Dial(ctx context.Context) (infra.Transport, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-i.offerReady:
		i.log.Debug("start dial")
		if i.agent.GetTieBreaker() > i.agent.RTieBreaker {
			conn, err := i.agent.Dial(ctx, i.agent.RUfrag, i.agent.RPwd)
			if err != nil {
				return nil, err
			}
			return &ICETransport{Conn: conn}, nil
		} else {
			conn, err := i.agent.Accept(ctx, i.agent.RUfrag, i.agent.RPwd)
			if err != nil {
				return nil, err
			}
			return &ICETransport{Conn: conn}, nil
		}
	}
}

func (i *iceDialer) Type() infra.DialerType {
	return infra.ICE_DIALER
}

func (i *iceDialer) getAgent(remoteId infra.PeerID) (*AgentWrapper, error) {
	f := logging.NewDefaultLoggerFactory()
	f.DefaultLogLevel = logging.LogLevelDebug
	if i.showLog {
	} else {
		f.DefaultLogLevel = logging.LogLevelError
	}
	//创建新 Agent
	iceAgent, err := ice.NewAgent(&ice.AgentConfig{
		UDPMux:         i.universalUdpMuxDefault.UDPMuxDefault,
		UDPMuxSrflx:    i.universalUdpMuxDefault,
		NetworkTypes:   []ice.NetworkType{ice.NetworkTypeUDP4},
		Urls:           []*stun.URI{{Scheme: stun.SchemeTypeSTUN, Host: "stun.wireflow.run", Port: 3478}},
		Tiebreaker:     uint64(ice.NewTieBreaker()),
		LoggerFactory:  f,
		CandidateTypes: []ice.CandidateType{ice.CandidateTypeHost, ice.CandidateTypeServerReflexive},
	})

	//iceAgent, err := ice.NewAgent(&ice.AgentConfig{
	//	NetworkTypes:  []ice.NetworkType{ice.NetworkTypeUDP4},
	//	LoggerFactory: f,
	//	Tiebreaker:    uint64(ice.NewTieBreaker()),
	//})

	var agent *AgentWrapper
	if err == nil {
		agent = &AgentWrapper{
			Agent: iceAgent,
		}
		// 绑定状态监听，成功后更新 WireGuard
		err = agent.OnConnectionStateChange(func(s ice.ConnectionState) {
			i.log.Debug("ice state changed", "state", s)
			if s == ice.ConnectionStateConnected {
				i.log.Debug("ice connected")
			}

			if s == ice.ConnectionStateDisconnected || s == ice.ConnectionStateFailed {
				i.close()
			}
		})

		if err != nil {
			return nil, err
		}
	}

	if err = agent.OnCandidate(func(candidate ice.Candidate) {
		if candidate == nil {
			return
		}

		if err = i.sendPacket(context.TODO(), remoteId, grpc.PacketType_OFFER, candidate); err != nil {
			i.log.Error("Send candidate", err)
		}

		i.log.Debug("Sending candidate", "remoteId", remoteId, "candidate", candidate)
	}); err != nil {
		return nil, err
	}

	return agent, err
}

func (i *iceDialer) sendPacket(ctx context.Context, remoteId infra.PeerID, packetType grpc.PacketType, candidate ice.Candidate) error {
	p := &grpc.SignalPacket{
		Type:     packetType,
		SenderId: i.localId.ToUint64(),
	}

	switch packetType {
	case grpc.PacketType_HANDSHAKE_SYN, grpc.PacketType_HANDSHAKE_ACK:
		p.Payload = &grpc.SignalPacket_Handshake{
			Handshake: &grpc.Handshake{
				Timestamp: time.Now().Unix(),
			},
		}
	case grpc.PacketType_OFFER:
		agent := i.agent
		current := i.peerManager.GetPeer(i.localId.ToUint64())
		currentData, err := json.Marshal(current)
		if err != nil {
			return err
		}
		ufrag, pwd, err := agent.GetLocalUserCredentials()
		if err != nil {
			return err
		}

		currentKey, b := i.peerStore.GetKeyByID(i.localId)
		if !b {
			return fmt.Errorf("peer not found: %s", i.localId)
		}

		p.Payload = &grpc.SignalPacket_Offer{
			Offer: &grpc.Offer{
				Ufrag:      ufrag,
				Pwd:        pwd,
				TieBreaker: agent.GetTieBreaker(),
				Candidate:  candidate.Marshal(),
				Current:    currentData,
				PublicKey:  currentKey.String(),
			},
		}
	}
	data, err := proto.Marshal(p)
	if err != nil {
		return err
	}
	return i.sender(ctx, remoteId, data)

}

func (i *iceDialer) close() error {
	i.log.Debug("closing ice", "remoteId", i.remoteId)
	i.closeOnce.Do(func() {
		if err := i.agent.Close(); err != nil {
			i.log.Error("close agent", err)
		}

		if i.onClose != nil {
			i.onClose(i.remoteId)
		}

		//remove peer
		//i.Remove(t.remoteId, "")
	})

	return nil
}

var (
	_ infra.Transport = (*ICETransport)(nil)
)

type ICETransport struct {
	Conn net.Conn
}

func (i *ICETransport) Priority() uint8 {
	return infra.PriorityDirect
}

func (i *ICETransport) Close() error {
	//TODO implement me
	panic("implement me")
}

func (i *ICETransport) Write(data []byte) error {
	return nil
}

func (i *ICETransport) Read(buff []byte) (int, error) {
	return 0, nil
}

func (i *ICETransport) RemoteAddr() string {
	return i.Conn.RemoteAddr().String()
}

func (i *ICETransport) Type() infra.TransportType {
	return infra.ICE
}
