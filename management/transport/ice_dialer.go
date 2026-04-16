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
	"sync/atomic"
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
	mu             sync.Mutex
	log            *log.Logger
	localId        infra.PeerIdentity
	remoteId       infra.PeerIdentity
	sender         func(ctx context.Context, peerId infra.PeerID, data []byte) error
	onClose        func(peerId infra.PeerIdentity)
	provisioner    infra.Provisioner // nolint
	agent          *AgentWrapper
	closeOnce      sync.Once
	offerOnce      sync.Once
	closed         atomic.Bool
	showLog        bool
	localPeer      *infra.Peer
	onPeerReceived func(peer infra.Peer)

	// onSynOnActiveAgent is called after the old ICE session is torn down when a
	// SYN arrives while an agent is already active (remote restarted mid-session).
	// The probe uses this to immediately re-dispatch the SYN to the new dialer
	// instead of waiting for the remote's next retry (up to 2 s later).
	onSynOnActiveAgent func(ctx context.Context, remoteId infra.PeerIdentity, packet *grpc.SignalPacket)

	// offerReady start Dial() after receiving offer
	offerReady chan struct{}
	// closeChan is closed when the dialer is closed, unblocking any pending Dial().
	closeChan chan struct{}
	cancel    context.CancelFunc
	ackChan   chan struct{} // nolint

	universalUdpMuxDefault *ice.UniversalUDPMuxDefault
}

type ICEDialerConfig struct {
	Sender                  func(ctx context.Context, peerId infra.PeerID, data []byte) error
	LocalId                 infra.PeerIdentity
	RemoteId                infra.PeerIdentity
	OnClose                 func(peerId infra.PeerIdentity)
	UniversalUdpMuxDefault  *ice.UniversalUDPMuxDefault
	Configurer              infra.Provisioner
	LocalPeer               *infra.Peer
	OnPeerReceived          func(peer infra.Peer)
	ShowLog                 bool
	OnConnectionStateChange func(state ice.ConnectionState)
	// OnSynOnActiveAgent is called (after the old session is closed) when a SYN
	// arrives while an ICE agent is already running, indicating the remote restarted.
	OnSynOnActiveAgent func(ctx context.Context, remoteId infra.PeerIdentity, packet *grpc.SignalPacket)
}

func (i *iceDialer) Handle(ctx context.Context, remoteId infra.PeerIdentity, packet *grpc.SignalPacket) error {
	if packet.Dialer != grpc.DialerType_ICE {
		return nil
	}
	switch packet.Type {
	case grpc.PacketType_HANDSHAKE_ACK:
		if i.closed.Load() {
			return nil
		}
		i.mu.Lock()
		agent := i.agent
		i.mu.Unlock()
		if agent == nil {
			return nil
		}
		// cancel send syn
		i.cancel()
		// start send offer
		return agent.GatherCandidates()
	case grpc.PacketType_HANDSHAKE_SYN:
		// If already fully closed, the remote may have restarted after our cleanup.
		// Drop the SYN — probe.restart() already created a new iceDialer that will
		// handle the next retry (Node A resends SYN every 2 s).
		if i.closed.Load() {
			return nil
		}

		i.mu.Lock()
		existingAgent := i.agent
		i.mu.Unlock()

		// If an agent already exists the remote restarted before we detected the
		// disconnect (fast restart, keepalive not yet timed out).  Force-close the
		// current dialer so probe.restart() creates a fresh one, then immediately
		// re-dispatch this SYN to the new dialer to avoid waiting for the remote's
		// next 2-second retry cycle.
		if existingAgent != nil {
			i.log.Debug("SYN on active agent — remote restarted, forcing close", "remoteId", remoteId)
			i.close() //nolint:errcheck
			if i.onSynOnActiveAgent != nil {
				i.onSynOnActiveAgent(ctx, remoteId, packet)
			}
			return nil
		}

		// send ack to remote
		if err := i.sendPacket(ctx, i.remoteId, grpc.PacketType_HANDSHAKE_ACK, nil); err != nil {
			return err
		}

		// init agent
		agent, err := i.getAgent(remoteId)
		if err != nil {
			return err
		}
		i.mu.Lock()
		i.agent = agent
		i.mu.Unlock()
		// start send offer (localId < remoteId)
		return agent.GatherCandidates()
	case grpc.PacketType_OFFER, grpc.PacketType_ANSWER:
		i.log.Debug("receive offer", "remoteId", remoteId)
		offer := packet.GetOffer()
		if !i.agent.IsCredentialsInited.Load() {
			i.agent.RUfrag = offer.Ufrag
			i.agent.RPwd = offer.Pwd
			i.agent.RTieBreaker = offer.TieBreaker
			i.agent.IsCredentialsInited.Store(true)

			var remotePeer infra.Peer
			if err := json.Unmarshal(offer.Current, &remotePeer); err != nil {
				return err
			}

			i.onPeerReceived(remotePeer)
		}

		candidate, err := ice.UnmarshalCandidate(offer.Candidate)
		if err != nil {
			return err
		}

		if err = i.agent.AddRemoteCandidate(candidate); err != nil {
			return err
		}

		i.log.Debug("add remote candidate", "candidate", candidate)
		i.offerOnce.Do(func() {
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
		localPeer:              cfg.LocalPeer,
		onPeerReceived:         cfg.OnPeerReceived,
		onSynOnActiveAgent:     cfg.OnSynOnActiveAgent,
		offerReady:             make(chan struct{}),
		closeChan:              make(chan struct{}),
		cancel:                 func() {}, // no-op until Prepare sets a real one
	}
}

// Prepare sends handshake SYN when localId > remoteId (lexicographic on PeerID string).
func (i *iceDialer) Prepare(ctx context.Context, remoteId infra.PeerIdentity) error {
	i.log.Debug("prepare ice", "localId", i.localId, "remoteId", remoteId, "shouldSync", i.localId.String() > remoteId.String())
	// only send syn when localId > remoteId
	// passive side returns early without pre-creating the agent;
	// the agent will be created in Handle() upon receiving SYN.
	if i.localId.String() < remoteId.String() {
		i.log.Debug("localId < remoteId, ignore prepare")
		return nil
	}
	// init agent (initiator side only)
	if i.agent == nil {
		agent, err := i.getAgent(remoteId)
		if err != nil {
			panic(err)
		}
		i.agent = agent
	}

	// send syn
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		newCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()

		i.mu.Lock()
		if i.cancel != nil {
			i.cancel()
		}
		i.cancel = cancel
		i.mu.Unlock()

		// Send the first SYN immediately instead of waiting for the first tick.
		i.log.Debug("send syn")
		if err := i.sendPacket(ctx, remoteId, grpc.PacketType_HANDSHAKE_SYN, nil); err != nil {
			i.log.Error("send syn failed", err)
		}

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
	case <-i.closeChan:
		return nil, fmt.Errorf("iceDialer closed before offer received")
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

func (i *iceDialer) getAgent(remoteId infra.PeerIdentity) (*AgentWrapper, error) {
	f := logging.NewDefaultLoggerFactory()
	if i.showLog {
		f.DefaultLogLevel = logging.LogLevelDebug
	} else {
		f.DefaultLogLevel = logging.LogLevelError
	}
	disconnectedTimeout := 3 * time.Second
	failedTimeout := 8 * time.Second
	iceAgent, err := ice.NewAgent(&ice.AgentConfig{
		UDPMux:              i.universalUdpMuxDefault.UDPMuxDefault,
		UDPMuxSrflx:         i.universalUdpMuxDefault,
		NetworkTypes:        []ice.NetworkType{ice.NetworkTypeUDP4},
		Urls: []*stun.URI{
				{Scheme: stun.SchemeTypeSTUN, Host: "stun.l.google.com", Port: 19302},
				{Scheme: stun.SchemeTypeSTUN, Host: "stun1.l.google.com", Port: 19302},
			},
		Tiebreaker:          uint64(ice.NewTieBreaker()),
		LoggerFactory:       f,
		CandidateTypes:      []ice.CandidateType{ice.CandidateTypeHost, ice.CandidateTypeServerReflexive},
		DisconnectedTimeout: &disconnectedTimeout,
		FailedTimeout:       &failedTimeout,
	})

	var agent *AgentWrapper
	if err == nil {
		agent = &AgentWrapper{Agent: iceAgent}
		err = agent.OnConnectionStateChange(func(s ice.ConnectionState) {
			i.log.Debug("ice state changed", "state", s)
			if s == ice.ConnectionStateDisconnected || s == ice.ConnectionStateFailed {
				i.close() //nolint:errcheck
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

// sendPacket sends a signal packet to remoteId.
// PeerIdentity.ID() is used for NATS routing; PublicKey is used in OFFER payload.
func (i *iceDialer) sendPacket(ctx context.Context, remoteId infra.PeerIdentity, packetType grpc.PacketType, candidate ice.Candidate) error {
	if i.closed.Load() {
		return nil
	}
	p := &grpc.SignalPacket{
		Type:     packetType,
		SenderId: i.localId.ID().ToUint64(),
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
		current := i.localPeer
		currentData, err := json.Marshal(current)
		if err != nil {
			return err
		}

		ufrag, pwd, err := agent.GetLocalUserCredentials()
		if err != nil {
			return err
		}
		if candidate == nil {
			return fmt.Errorf("candidate is nil for OFFER")
		}
		p.Payload = &grpc.SignalPacket_Offer{
			Offer: &grpc.Offer{
				Ufrag:      ufrag,
				Pwd:        pwd,
				TieBreaker: agent.GetTieBreaker(),
				Candidate:  candidate.Marshal(),
				Current:    currentData,
				PublicKey:  i.localId.PublicKey.String(), // directly from PeerIdentity
			},
		}
	}
	data, err := proto.Marshal(p)
	if err != nil {
		return err
	}
	return i.sender(ctx, remoteId.ID(), data)
}

func (i *iceDialer) close() error {
	i.log.Debug("closing ice", "remoteId", i.remoteId)
	i.closeOnce.Do(func() {
		i.closed.Store(true)
		i.mu.Lock()
		agent := i.agent
		i.agent = nil
		i.mu.Unlock()

		// Unblock any Dial() waiting on offerReady or closeChan.
		close(i.closeChan)

		// Notify the probe before closing the ICE agent so probe.restart() can
		// create a fresh dialer immediately, without waiting for agent.Close() to
		// finish (which can block briefly waiting for ICE goroutines to stop).
		if i.onClose != nil {
			i.onClose(i.remoteId)
		}

		if agent != nil {
			if err := agent.Close(); err != nil {
				i.log.Error("close agent", err)
			}
		}
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
	return i.Conn.Close()
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
