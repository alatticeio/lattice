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
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"wireflow/internal/grpc"
	"wireflow/internal/infra"
	"wireflow/internal/log"

	"github.com/pion/ice/v4"
	"github.com/pion/logging"
	"github.com/pion/stun/v3"
	"google.golang.org/protobuf/proto"
)

var (
	_ infra.Dialer = (*iceDialer)(nil)
)

// ErrDialerClosed is returned by Dial when the iceDialer is explicitly closed
// (e.g. ICE reached Failed state or a SYN arrived on an active agent).  It
// signals that onClose already triggered probe.restart(), so onFailure should
// NOT schedule a second restart.
var ErrDialerClosed = errors.New("iceDialer explicitly closed")

type iceDialer struct {
	mu             sync.Mutex
	log            *log.Logger
	localId        infra.PeerIdentity
	remoteId       infra.PeerIdentity
	sender         func(ctx context.Context, peerId infra.PeerID, data []byte) error
	provisioner    infra.Provisioner // nolint
	agent               *ice.Agent
	credentialsInited   atomic.Bool
	rUfrag              string
	rPwd                string
	closeOnce           sync.Once
	offerOnce           sync.Once
	closed              atomic.Bool
	showLog             bool
	getLocalPeer        func() *infra.Peer
	onPeerReceived      func(peer infra.Peer)

	// offerReady is closed when the first remote candidate OFFER is received,
	// signalling Dial() that it can call StartDial/StartAccept + AwaitConnect.
	offerReady chan struct{}
	// closeChan is closed when the dialer is closed, unblocking Dial().
	closeChan chan struct{}
	cancel    context.CancelFunc
	ackChan   chan struct{} // nolint

	// filteringMux owns the shared v4 UDP socket and exposes UDPMux/UDPMuxSrflx
	// interfaces for ICE agent construction. filteringMux6 is the v6 counterpart;
	// nil when IPv6 is unavailable.
	filteringMux  *infra.FilteringUDPMux
	filteringMux6 *infra.FilteringUDPMux
}

type ICEDialerConfig struct {
	Sender       func(ctx context.Context, peerId infra.PeerID, data []byte) error
	LocalId      infra.PeerIdentity
	RemoteId     infra.PeerIdentity
	FilteringMux  *infra.FilteringUDPMux
	FilteringMux6 *infra.FilteringUDPMux // nil when IPv6 unavailable
	Configurer    infra.Provisioner
	// GetLocalPeer is called at send time so late-arriving ApplyFullConfig
	// updates (Address, AllowedIPs) are always reflected in SYN/ACK peer info.
	GetLocalPeer   func() *infra.Peer
	OnPeerReceived func(peer infra.Peer)
	ShowLog        bool
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
		// Extract peer info from ACK payload (new design: peer info in SYN/ACK).
		if hs := packet.GetHandshake(); hs != nil && len(hs.PeerInfo) > 0 {
			var remotePeer infra.Peer
			if err := json.Unmarshal(hs.PeerInfo, &remotePeer); err == nil {
				i.onPeerReceived(remotePeer)
			}
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

		// Extract peer info from SYN payload (new design: peer info in SYN/ACK).
		if hs := packet.GetHandshake(); hs != nil && len(hs.PeerInfo) > 0 {
			var remotePeer infra.Peer
			if err := json.Unmarshal(hs.PeerInfo, &remotePeer); err == nil {
				i.onPeerReceived(remotePeer)
			}
		}

		i.mu.Lock()
		existingAgent := i.agent
		i.mu.Unlock()

		// If an agent already exists the remote restarted before we detected the
		// disconnect (fast restart, keepalive not yet timed out).  Close this
		// dialer; Dial() will return ErrDialerClosed which causes onFailure to
		// call probe.restart() immediately.  The remote's next SYN retry (≤2 s)
		// will be handled by the fresh dialer.
		if existingAgent != nil {
			i.log.Debug("SYN on active agent — remote restarted, forcing close", "remoteId", remoteId)
			i.Close() //nolint:errcheck
			return nil
		}

		// send ack to remote (includes our own peer info)
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
		// responder also gathers candidates
		return agent.GatherCandidates()
	case grpc.PacketType_OFFER, grpc.PacketType_ANSWER:
		i.log.Debug("receive offer", "remoteId", remoteId)
		offer := packet.GetOffer()
		// Use double-checked locking so that IsCredentialsInited.Store(true)
		// only happens AFTER onPeerReceived sets remotePeer.  Without this,
		// a concurrent OFFER handler that sees IsCredentialsInited=true can
		// skip the block and fire offerOnce.Do(close(offerReady)) while
		// remotePeer is still nil, causing onSuccess to fail.
		if !i.credentialsInited.Load() {
			i.mu.Lock()
			if !i.credentialsInited.Load() {
				i.rUfrag = offer.Ufrag
				i.rPwd = offer.Pwd

				var remotePeer infra.Peer
				if err := json.Unmarshal(offer.Current, &remotePeer); err != nil {
					i.mu.Unlock()
					return err
				}
				i.onPeerReceived(remotePeer)
				i.credentialsInited.Store(true)
			}
			i.mu.Unlock()
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
		localId:                cfg.LocalId,
		remoteId:               cfg.RemoteId,
		filteringMux:  cfg.FilteringMux,
		filteringMux6: cfg.FilteringMux6,
		showLog:                cfg.ShowLog,
		getLocalPeer:           cfg.GetLocalPeer,
		onPeerReceived:         cfg.OnPeerReceived,
		offerReady:             make(chan struct{}),
		closeChan:              make(chan struct{}),
		cancel:                 func() {}, // no-op until Prepare sets a real one
	}
}

// Prepare sends handshake SYN when local is the initiator (localId > remoteId numerically).
func (i *iceDialer) Prepare(ctx context.Context, remoteId infra.PeerIdentity) error {
	i.log.Debug("prepare ice", "localId", i.localId, "remoteId", remoteId, "isInitiator", isInitiator(i.localId, remoteId))
	// Only the initiator sends SYN.  The responder waits for a SYN and creates
	// its agent inside Handle() to avoid pre-creating unnecessary ICE agents.
	if !isInitiator(i.localId, remoteId) {
		i.log.Debug("not initiator, waiting for SYN")
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
	// Use a timeout slightly longer than the SYN window (60 s) so that if no
	// offer arrives before the initiator gives up sending SYNs, Dial returns
	// an error.  This causes discover() to fail, which triggers onFailure →
	// probe.restart() → a fresh SYN cycle, preventing a permanent deadlock
	// when the passive side stays offline for more than ~71 s.
	dialCtx, cancel := context.WithTimeout(ctx, 65*time.Second)
	defer cancel()
	select {
	case <-dialCtx.Done():
		return nil, fmt.Errorf("iceDialer: timed out waiting for offer: %w", dialCtx.Err())
	case <-i.closeChan:
		return nil, ErrDialerClosed
	case <-i.offerReady:
		i.log.Debug("start dial")
		var iceConn *ice.Conn
		var err error
		if isInitiator(i.localId, i.remoteId) {
			iceConn, err = i.agent.StartDial(i.rUfrag, i.rPwd)
		} else {
			iceConn, err = i.agent.StartAccept(i.rUfrag, i.rPwd)
		}
		if err != nil {
			return nil, err
		}
		if err = i.agent.AwaitConnect(dialCtx); err != nil {
			return nil, err
		}
		remoteAddr := iceConn.RemoteAddr().String()
		// Close the ICE conn and dialer after a brief delay to let final STUN
		// checks complete.  Calling i.Close() sets closed=true and clears i.agent,
		// so any late SYN retries from the remote's ticker are dropped rather than
		// being misread as "remote restarted" and triggering a spurious restart.
		go func() {
			time.Sleep(500 * time.Millisecond)
			iceConn.Close() //nolint:errcheck
			i.Close()       //nolint:errcheck
		}()
		return &ICETransport{remoteAddr: remoteAddr}, nil
	}
}

func (i *iceDialer) Type() infra.DialerType {
	return infra.ICE_DIALER
}

// udpMux returns a combined UDPMux for all available network interfaces.
// When IPv6 is available, MultiUDPMuxDefault aggregates v4 and v6 host candidates
// so the ICE agent can gather candidates from both stacks via a single option.
func (i *iceDialer) udpMux() ice.UDPMux {
	if i.filteringMux6 != nil {
		return ice.NewMultiUDPMuxDefault(i.filteringMux.UDPMux(), i.filteringMux6.UDPMux())
	}
	return i.filteringMux.UDPMux()
}

// networkTypes returns the ICE network types enabled for this dialer.
// UDP6 is only included when a v6 FilteringUDPMux is present.
func (i *iceDialer) networkTypes() []ice.NetworkType {
	types := []ice.NetworkType{ice.NetworkTypeUDP4}
	if i.filteringMux6 != nil {
		types = append(types, ice.NetworkTypeUDP6)
	}
	return types
}

func (i *iceDialer) getAgent(remoteId infra.PeerIdentity) (*ice.Agent, error) {
	f := logging.NewDefaultLoggerFactory()
	if i.showLog {
		f.DefaultLogLevel = logging.LogLevelDebug
	} else {
		f.DefaultLogLevel = logging.LogLevelError
	}
	// DisconnectedTimeout: how long without a keepalive response before ICE
	// moves Connected→Disconnected and starts aggressive re-checks.
	// 10s gives the agent enough headroom to survive transient network blips
	// without triggering a full restart.
	//
	// FailedTimeout: how long ICE retries while Disconnected before giving up
	// and moving to Failed.  15s is the practical upper bound — beyond this
	// WireGuard's own PersistentKeepalive will have already re-established the
	// path if the remote is reachable at all.
	disconnectedTimeout := 10 * time.Second
	failedTimeout := 15 * time.Second
	iceAgent, err := ice.NewAgentWithOptions(
		ice.WithInterfaceFilter(func(name string) bool {
			name = strings.ToLower(name)
			// 过滤掉所有虚拟网卡以及 WireGuard TUN 接口（wf0）。
			// wf0 不能作为 ICE candidate：若选中，WireGuard 会把对端 endpoint
			// 配置为 wf0 地址，导致加密包再次经过 wf0 形成路由环路。
			if strings.Contains(name, "docker") ||
				strings.Contains(name, "veth") ||
				strings.Contains(name, "br-") ||
				strings.HasPrefix(name, "wf") {
				return false
			}
			return true
		}),
		ice.WithUDPMux(i.udpMux()),
		ice.WithUDPMuxSrflx(i.filteringMux.UDPMuxSrflx()),
		ice.WithNetworkTypes(i.networkTypes()),
		ice.WithUrls([]*stun.URI{
			{Scheme: stun.SchemeTypeSTUN, Host: "stun.wireflow.run", Port: 3478},
		}),
		ice.WithLoggerFactory(f),
		ice.WithCandidateTypes([]ice.CandidateType{ice.CandidateTypeHost, ice.CandidateTypeServerReflexive}),
		ice.WithDisconnectedTimeout(disconnectedTimeout),
		ice.WithFailedTimeout(failedTimeout),
	)

	if err != nil {
		return nil, err
	}
	if err = iceAgent.OnConnectionStateChange(func(s ice.ConnectionState) {
		i.log.Debug("ice state changed", "state", s)
		// Only close on Failed, not Disconnected.
		// When ICE enters Disconnected it retries keepalives aggressively for
		// FailedTimeout (15s) and can recover to Connected without any
		// application intervention.  Closing on Disconnected short-circuits
		// that built-in recovery and triggers a full SYN restart cycle which
		// cascades to the remote side as well, causing the connect/disconnect loop.
		if s == ice.ConnectionStateFailed {
			i.Close() //nolint:errcheck
		}
	}); err != nil {
		return nil, err
	}

	if err = iceAgent.OnCandidate(func(candidate ice.Candidate) {
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

	return iceAgent, nil
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
		// Include local peer info so the remote side learns our WG config
		// (Address, AllowedIPs) at SYN/ACK time — before any ICE candidate
		// exchange begins.  getLocalPeer() is called here (not at construction)
		// to pick up Address/AllowedIPs that may have arrived via ApplyFullConfig
		// after the dialer was created.
		hs := &grpc.Handshake{Timestamp: time.Now().Unix()}
		if lp := i.getLocalPeer(); lp != nil {
			if data, err := json.Marshal(lp); err == nil {
				hs.PeerInfo = data
			}
		}
		p.Payload = &grpc.SignalPacket_Handshake{Handshake: hs}
	case grpc.PacketType_OFFER:
		agent := i.agent
		// Keep Current in OFFER for backward compatibility with older nodes that
		// have not yet adopted the SYN/ACK peer-info exchange.
		currentData, _ := json.Marshal(i.getLocalPeer())

		ufrag, pwd, err := agent.GetLocalUserCredentials()
		if err != nil {
			return err
		}
		if candidate == nil {
			return fmt.Errorf("candidate is nil for OFFER")
		}
		p.Payload = &grpc.SignalPacket_Offer{
			Offer: &grpc.Offer{
				Ufrag:     ufrag,
				Pwd:       pwd,
				Candidate: candidate.Marshal(),
				Current:   currentData,
				PublicKey: i.localId.PublicKey.String(),
			},
		}
	}
	data, err := proto.Marshal(p)
	if err != nil {
		return err
	}
	return i.sender(ctx, remoteId.ID(), data)
}

func (i *iceDialer) Close() error {
	i.log.Debug("closing ice", "remoteId", i.remoteId)
	i.closeOnce.Do(func() {
		i.closed.Store(true)
		i.mu.Lock()
		agent := i.agent
		i.agent = nil
		i.mu.Unlock()

		// Unblock Dial(): it will return ErrDialerClosed, discover() will fail,
		// and onFailure will call probe.restart() — the single restart path.
		close(i.closeChan)

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
	remoteAddr string
}

func (i *ICETransport) Priority() uint8 {
	return infra.PriorityDirect
}

func (i *ICETransport) Close() error {
	return nil
}

func (i *ICETransport) Write(data []byte) error {
	return nil
}

func (i *ICETransport) Read(buff []byte) (int, error) {
	return 0, nil
}

func (i *ICETransport) RemoteAddr() string {
	return i.remoteAddr
}

func (i *ICETransport) Type() infra.TransportType {
	return infra.ICE
}
