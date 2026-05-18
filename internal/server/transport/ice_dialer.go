// Copyright 2026 The Lattice Authors, Inc.
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

	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/agent/provision"
	"github.com/alatticeio/lattice/internal/grpc"

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
	mu                sync.Mutex
	log               *log.Logger
	localId           infra.PeerIdentity
	remoteId          infra.PeerIdentity
	sender            func(ctx context.Context, peerId infra.PeerID, data []byte) error
	provisioner       provision.Provisioner // nolint
	agent             *ice.Agent
	credentialsInited atomic.Bool
	rUfrag            string
	rPwd              string
	closeOnce         sync.Once
	offerOnce         sync.Once
	gatherOnce        sync.Once // guards the single GatherCandidates() call
	closed            atomic.Bool
	showLog           bool
	getLocalPeer      func() *infra.Peer
	onPeerReceived    func(peer infra.Peer)
	onRestart         func() // called when a SYN arrives on a closed dialer (remote peer restarted)

	// offerReady is closed when the first remote candidate OFFER is received,
	// signalling Dial() that it can call StartDial/StartAccept + AwaitConnect.
	offerReady chan struct{}
	// closeChan is closed when the dialer is closed, unblocking Dial().
	closeChan chan struct{}
	cancel    context.CancelFunc
	ackChan   chan struct{} // nolint

	// pendingCandidates stores remote ICE candidates that arrived before the local
	// ICE agent was initialized (race between fast responder and slower getAgent()).
	// They are replayed into the agent once it becomes available in Prepare().
	// Protected by mu.
	pendingCandidates []ice.Candidate

	// gatheredCandidates caches local ICE candidates emitted by OnCandidate so
	// they can be resent to the remote peer when a late or retransmitted ACK/SYN
	// arrives after GatherCandidates() has already completed.  Without this, the
	// remote side never receives our OFFER and ICE cannot make progress.
	// Protected by mu.
	gatheredCandidates []ice.Candidate

	// filteringMux owns the shared v4 UDP socket and exposes UDPMux/UDPMuxSrflx
	// interfaces for ICE agent construction. filteringMux6 is the v6 counterpart;
	// nil when IPv6 is unavailable.
	filteringMux  *infra.FilteringUDPMux
	filteringMux6 *infra.FilteringUDPMux
}

type ICEDialerConfig struct {
	Sender        func(ctx context.Context, peerId infra.PeerID, data []byte) error
	LocalId       infra.PeerIdentity
	RemoteId      infra.PeerIdentity
	FilteringMux  *infra.FilteringUDPMux
	FilteringMux6 *infra.FilteringUDPMux // nil when IPv6 unavailable
	Configurer    provision.Provisioner
	// GetLocalPeer is called at send time so late-arriving ApplyFullConfig
	// updates (Address, AllowedIPs) are always reflected in SYN/ACK peer info.
	GetLocalPeer   func() *infra.Peer
	OnPeerReceived func(peer infra.Peer)
	ShowLog        bool
	// OnRestart is called when a SYN arrives on an already-closed dialer,
	// indicating the remote peer restarted. Mirrors the LRP dialer pattern.
	OnRestart func()
}

func (i *iceDialer) Handle(ctx context.Context, remoteId infra.PeerIdentity, packet *grpc.SignalPacket) error {
	if packet.Dialer != grpc.DialerType_ICE {
		return nil
	}
	switch packet.Type {
	case grpc.PacketType_RESTART_NOTIFY:
		// The remote non-initiator just started fresh. If our ICE dialer is already
		// closed (we were previously connected), trigger probe.restart() so we
		// re-initiate the handshake with the restarted peer's new endpoint.
		// If our dialer is still open (normal startup race), safely ignore.
		if i.closed.Load() {
			if i.onRestart != nil {
				go i.onRestart()
			}
		}
		return nil
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
		// Guard with credentialsInited to avoid redundant calls on ACK retransmissions.
		if hs := packet.GetHandshake(); hs != nil && len(hs.PeerInfo) > 0 {
			if !i.credentialsInited.Load() {
				i.mu.Lock()
				if !i.credentialsInited.Load() {
					var remotePeer infra.Peer
					if err := json.Unmarshal(hs.PeerInfo, &remotePeer); err == nil {
						i.onPeerReceived(remotePeer)
					}
					i.credentialsInited.Store(true)
				}
				i.mu.Unlock()
			}
		}
		// cancel send syn
		i.cancel()
		// start send offer — guard with gatherOnce so that retransmitted ACKs
		// (from the responder resending ACK on SYN retransmit) don't trigger a
		// second GatherCandidates call, which pion returns as an error
		// ("attempting to gather candidates during gathering state").
		var gatherErr error
		var gatherRanNow bool
		i.gatherOnce.Do(func() {
			gatherRanNow = true
			gatherErr = agent.GatherCandidates()
		})
		if !gatherRanNow {
			// GatherCandidates already ran (Prepare() fired it before ACK arrived).
			// The remote peer may have missed our OFFERs if its subscription was not
			// yet ready.  Resend cached candidates so it can complete ICE setup.
			go i.resendCandidates(ctx)
		}
		return gatherErr
	case grpc.PacketType_HANDSHAKE_SYN:
		// If already fully closed, the remote peer restarted after our ICE cleanup.
		// Trigger probe.restart() so a fresh dialer is created to handle the peer's
		// next SYN retry (sent every 2 s). Without this the probe stays stuck in
		// ICEReady with a dead dialer and the WG peer entry is never re-established.
		if i.closed.Load() {
			if i.onRestart != nil {
				go i.onRestart()
			}
			return nil
		}

		// Extract peer info from SYN payload (new design: peer info in SYN/ACK).
		// Guard with credentialsInited to avoid redundant calls on SYN retransmissions.
		if hs := packet.GetHandshake(); hs != nil && len(hs.PeerInfo) > 0 {
			if !i.credentialsInited.Load() {
				i.mu.Lock()
				if !i.credentialsInited.Load() {
					var remotePeer infra.Peer
					if err := json.Unmarshal(hs.PeerInfo, &remotePeer); err == nil {
						i.onPeerReceived(remotePeer)
					}
					i.credentialsInited.Store(true)
				}
				i.mu.Unlock()
			}
		}

		i.mu.Lock()
		existingAgent := i.agent
		i.mu.Unlock()

		// If an agent already exists AND the dialer is still open, this is a
		// SYN retransmit while ICE setup is in progress — NOT a remote restart.
		// (A genuine remote-restart SYN arrives only after the successful-ICE
		// close path sets i.closed=true, so it takes the i.closed branch above.)
		// Resend ACK so the initiator can cancel its 2-second SYN ticker and
		// proceed with candidate exchange.  Closing here would destroy the
		// ongoing ICE setup and cause the initiator's OFFERs to be dropped by
		// the replacement dialer's nil-agent guard.
		if existingAgent != nil {
			i.log.Debug("SYN retransmit during ICE setup, resending ACK", "remoteId", remoteId)
			_ = i.sendPacket(ctx, i.remoteId, grpc.PacketType_HANDSHAKE_ACK, nil)
			// Also resend our cached candidates: the initiator may have restarted
			// its dialer (new gatherOnce) and is waiting for our OFFER to unblock
			// its Dial().  Without this, only one side runs ICE and checks fail.
			go i.resendCandidates(ctx)
			return nil
		}

		// init agent BEFORE sending ACK.
		// ACK signals the initiator to start gathering candidates and sending OFFERs
		// immediately. If we send ACK first and then call getAgent(), the first OFFER
		// can arrive before i.agent is set and gets dropped with "agent nil, dropping".
		agent, err := i.getAgent(remoteId)
		if err != nil {
			return err
		}
		// Drain any candidates that arrived concurrently while getAgent() was
		// running (initiator racing to send OFFERs before we finish setup).
		var pending []ice.Candidate
		i.mu.Lock()
		i.agent = agent
		pending = i.pendingCandidates
		i.pendingCandidates = nil
		i.mu.Unlock()

		// send ack to remote (includes our own peer info)
		if err := i.sendPacket(ctx, i.remoteId, grpc.PacketType_HANDSHAKE_ACK, nil); err != nil {
			return err
		}
		// responder also gathers candidates (gatherOnce ensures idempotency)
		var gatherErr error
		i.gatherOnce.Do(func() {
			gatherErr = agent.GatherCandidates()
		})
		if gatherErr != nil {
			return gatherErr
		}
		// Replay any candidates buffered before the agent was ready.
		for _, c := range pending {
			if err := agent.AddRemoteCandidate(c); err != nil {
				i.log.Warn("replay pending candidate failed", "err", err)
			}
		}
		if len(pending) > 0 {
			i.log.Debug("replayed pending candidates", "count", len(pending))
			i.offerOnce.Do(func() {
				close(i.offerReady)
			})
		}
		return nil
	case grpc.PacketType_OFFER, grpc.PacketType_ANSWER:
		if i.closed.Load() {
			i.log.Debug("receive offer: dialer closed, dropping", "remoteId", remoteId)
			return nil
		}

		offer := packet.GetOffer()

		// Always ensure remote ICE credentials are set — credentialsInited may
		// already be true from an earlier ACK/SYN (which don't carry ICE creds).
		// Without this check a late OFFER could skip the credential assignment,
		// leaving rUfrag empty and causing StartDial("") to fail.
		// Do this before the agent-nil check so credentials are captured even
		// when the agent is not yet ready.
		if i.rUfrag == "" {
			i.mu.Lock()
			if i.rUfrag == "" {
				i.rUfrag = offer.Ufrag
				i.rPwd = offer.Pwd
			}
			i.mu.Unlock()
		}

		// Extract peer info from OFFER (backward compatibility with
		// older nodes that don't send peer_info in SYN/ACK).
		if !i.credentialsInited.Load() {
			i.mu.Lock()
			if !i.credentialsInited.Load() {
				if len(offer.Current) > 0 {
					var remotePeer infra.Peer
					if err := json.Unmarshal(offer.Current, &remotePeer); err == nil {
						i.onPeerReceived(remotePeer)
					}
				}
				i.credentialsInited.Store(true)
			}
			i.mu.Unlock()
		}

		candidate, err := ice.UnmarshalCandidate(offer.Candidate)
		if err != nil {
			return err
		}

		// Check agent under the same lock used by Prepare() to avoid a TOCTOU
		// race: if the agent is not ready yet, buffer the candidate for replay
		// once Prepare() sets i.agent, rather than dropping it.
		i.mu.Lock()
		agent := i.agent
		if agent == nil {
			i.pendingCandidates = append(i.pendingCandidates, candidate)
			i.mu.Unlock()
			i.log.Debug("receive offer: agent not ready, buffering candidate", "remoteId", remoteId)
			return nil
		}
		i.mu.Unlock()

		i.log.Debug("receive offer", "remoteId", remoteId)
		if err = agent.AddRemoteCandidate(candidate); err != nil {
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
		log:            log.GetLogger("ice-dialer"),
		sender:         cfg.Sender,
		localId:        cfg.LocalId,
		remoteId:       cfg.RemoteId,
		filteringMux:   cfg.FilteringMux,
		filteringMux6:  cfg.FilteringMux6,
		showLog:        cfg.ShowLog,
		getLocalPeer:   cfg.GetLocalPeer,
		onPeerReceived: cfg.OnPeerReceived,
		onRestart:      cfg.OnRestart,
		offerReady:     make(chan struct{}),
		closeChan:      make(chan struct{}),
		cancel:         func() {}, // no-op until Prepare sets a real one
	}
}

// Prepare sends handshake SYN when local is the initiator (localId > remoteId numerically).
func (i *iceDialer) Prepare(ctx context.Context, remoteId infra.PeerIdentity) error {
	i.log.Debug("prepare ice", "localId", i.localId, "remoteId", remoteId, "isInitiator", isInitiator(i.localId, remoteId))
	// Only the initiator sends SYN.  The responder waits for a SYN and creates
	// its agent inside Handle() to avoid pre-creating unnecessary ICE agents.
	if !isInitiator(i.localId, remoteId) {
		i.log.Debug("not initiator, waiting for SYN")
		// Notify the remote that we are starting fresh. If the remote's ICE dialer
		// is already closed (it was previously connected to us), receiving this
		// triggers probe.restart() on the remote so it re-initiates the ICE
		// handshake. If the remote is still probing (normal startup), the
		// notification is ignored.
		go func() {
			if err := i.sendPacket(ctx, remoteId, grpc.PacketType_RESTART_NOTIFY, nil); err != nil {
				i.log.Debug("restart notify send failed", "remoteId", remoteId, "err", err)
			}
		}()
		return nil
	}
	// init agent (initiator side only)
	i.mu.Lock()
	needInit := i.agent == nil
	i.mu.Unlock()
	if needInit {
		agent, err := i.getAgent(remoteId)
		if err != nil {
			return fmt.Errorf("getAgent failed: %w", err)
		}
		var pending []ice.Candidate
		i.mu.Lock()
		if i.agent == nil {
			i.agent = agent
			// Drain any candidates that arrived before the agent was ready.
			pending = i.pendingCandidates
			i.pendingCandidates = nil
		} else {
			// Another goroutine beat us; discard the redundant agent.
			_ = agent.Close()
		}
		i.mu.Unlock()

		// Replay buffered candidates outside the lock (AddRemoteCandidate may block).
		for _, c := range pending {
			if err := agent.AddRemoteCandidate(c); err != nil {
				i.log.Warn("replay pending candidate failed", "err", err)
			}
		}
		if len(pending) > 0 {
			i.log.Debug("replayed pending candidates", "count", len(pending))
			i.offerOnce.Do(func() {
				close(i.offerReady)
			})
		}

		// Always start gathering candidates once the agent is ready.
		// If Handle(ACK) arrived while getAgent() was still running it returned
		// early (agent was nil) and skipped GatherCandidates.  gatherOnce
		// ensures it is called exactly once even if Handle(ACK) races here.
		var gatherErr error
		i.gatherOnce.Do(func() { gatherErr = agent.GatherCandidates() })
		if gatherErr != nil {
			return fmt.Errorf("GatherCandidates (prepare): %w", gatherErr)
		}
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
		i.mu.Lock()
		agent := i.agent
		i.mu.Unlock()
		if agent == nil {
			return nil, ErrDialerClosed
		}
		var iceConn *ice.Conn
		var err error
		if isInitiator(i.localId, i.remoteId) {
			iceConn, err = agent.StartDial(i.rUfrag, i.rPwd)
		} else {
			iceConn, err = agent.StartAccept(i.rUfrag, i.rPwd)
		}
		if err != nil {
			return nil, err
		}
		if err = agent.AwaitConnect(dialCtx); err != nil {
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
			// Filter out all virtual network interfaces and WireGuard TUN interfaces (wf0).
			// wf0 cannot be used as an ICE candidate: if selected, WireGuard would configure
			// the peer endpoint to the wf0 address, causing encrypted packets to pass through wf0 again, forming a routing loop.
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
			{Scheme: stun.SchemeTypeSTUN, Host: "stun.alattice.io", Port: 3478},
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
		i.mu.Lock()
		i.gatheredCandidates = append(i.gatheredCandidates, candidate)
		i.mu.Unlock()
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
		i.mu.Lock()
		agent := i.agent
		i.mu.Unlock()
		if agent == nil {
			return fmt.Errorf("agent is nil, cannot send OFFER")
		}
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

// resendCandidates retransmits all locally gathered ICE candidates to the
// remote peer.  Called when a late ACK or a SYN-retransmit arrives after
// GatherCandidates() has already completed, ensuring the remote side receives
// our OFFER even if it missed the original transmission.
func (i *iceDialer) resendCandidates(ctx context.Context) {
	i.mu.Lock()
	cached := make([]ice.Candidate, len(i.gatheredCandidates))
	copy(cached, i.gatheredCandidates)
	i.mu.Unlock()
	for _, c := range cached {
		if err := i.sendPacket(ctx, i.remoteId, grpc.PacketType_OFFER, c); err != nil {
			i.log.Warn("resend candidate failed", "err", err)
		}
	}
	if len(cached) > 0 {
		i.log.Debug("resent cached candidates", "remoteId", i.remoteId, "count", len(cached))
	}
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
