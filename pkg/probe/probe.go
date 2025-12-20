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
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"wireflow/drp"
	"wireflow/internal/core/domain"
	"wireflow/internal/core/infra"
	"wireflow/internal/core/manager"
	drpgrpc "wireflow/internal/grpc"
	"wireflow/pkg/log"
	"wireflow/pkg/turn"
	"wireflow/pkg/wferrors"

	"github.com/wireflowio/ice"
)

var (
	_ domain.Prober = (*probe)(nil)
)

// probe is a wrapper directchecker & relaychecker
type probe struct {
	logger          *log.Logger
	closeMux        sync.Mutex
	agent           *manager.Agent
	iceAgent        *ice.Agent
	done            chan interface{}
	connectionState domain.ConnectionState
	isStarted       atomic.Bool
	isForceRelay    bool
	probeManager    domain.ProberManager
	nodeManager     domain.PeerManager

	lastCheck time.Time

	from string
	to   string

	drpAddr string

	connectType domain.ConnType // connectType indicates the type of connection, direct or relay

	// directChecker is used to check the direct connection
	directChecker domain.Checker

	// relayChecker is used to check the relay connection
	relayChecker domain.Checker

	drpChecker domain.Checker

	wgConfiger domain.Configurer

	offerHandler domain.OfferHandler

	turnManager *turn.TurnManager

	gatherCh chan interface{}

	udpMux          *ice.UDPMuxDefault // udpMux is used to send and receive packets
	universalUdpMux *ice.UniversalUDPMuxDefault
}

func (p *probe) Restart() error {
	var (
		err error
	)

	defer func() {
		if err = p.iceAgent.Close(); err != nil {
			p.logger.Errorf("failed to close original agentManager: %v", err)
		} else {
			p.logger.Infof("original agentManager closed successfully")
		}
	}()

	p.UpdateConnectionState(domain.ConnectionStateNew)
	// create a new agentManager
	p.gatherCh = make(chan interface{})
	if p.iceAgent, err = p.agent.NewIceAgent(); err != nil {
		return err
	}

	p.iceAgent.OnCandidate(func(candidate ice.Candidate) {
		if candidate == nil {
			p.logger.Verbosef("gathered all candidates")
			close(p.gatherCh)
			return
		}

		p.logger.Verbosef("gathered candidate: %s", candidate.String())
	})

	// when restart should regather candidates
	if err = p.iceAgent.GatherCandidates(); err != nil {
		return err
	}

	// update probe manager
	p.probeManager.AddProbe(p.to, p)
	return nil
}

func (p *probe) GetConnState() domain.ConnectionState {
	return p.connectionState
}

func (p *probe) ProbeDone() chan interface{} {
	return p.done
}

func (p *probe) GetGatherChan() chan interface{} {
	return p.gatherCh
}

func (p *probe) UpdateConnectionState(state domain.ConnectionState) {
	p.connectionState = state
	p.logger.Verbosef("probe connection state updated to: %v", state)
}

func (p *probe) GetIceAgent() *ice.Agent {
	return p.iceAgent
}

func (p *probe) OnConnectionStateChange(state domain.ConnectionState) error {
	p.connectionState = state
	p.logger.Verbosef("probe connection state updated to: %v", state)
	switch state {
	case domain.ConnectionStateFailed, domain.ConnectionStateDisconnected:
		if err := p.Restart(); err != nil {
			return err
		}
	}

	return nil
}

func (p *probe) HandleOffer(ctx context.Context, offer domain.Offer) error {
	switch offer.GetOfferType() {
	case domain.OfferTypeDirectOffer:
		// later new directChecker
		if p.directChecker == nil {
			p.directChecker = NewDirectChecker(&DirectCheckerConfig{
				Logger:     p.logger,
				Key:        p.to,
				WgConfiger: p.wgConfiger,
				LocalKey:   p.iceAgent.GetTieBreaker(),
				Prober:     p,
			})

			p.probeManager.AddProbe(p.to, p)
		}
		if err := p.handleDirectOffer(ctx, offer.(*domain.DirectOffer)); err != nil {
			return err
		}

	case domain.OfferTypeRelayOffer, domain.OfferTypeRelayAnswer:
		if p.relayChecker == nil {
			p.relayChecker = NewRelayChecker(&RelayCheckerConfig{
				TurnManager: p.turnManager,
				WgConfiger:  p.wgConfiger,
				DstKey:      p.to,
				SrcKey:      p.from,
				Probe:       p,
			})
		}

	case domain.OfferTypeDrpOffer, domain.OfferTypeDrpOfferAnswer:
		if p.drpChecker == nil {
			p.drpChecker = NewDrpChecker(&DrpCheckerConfig{
				Probe:   p,
				From:    p.from,
				To:      p.to,
				DrpAddr: offer.GetNode().DrpAddr,
			})
		}
	}

	p.logger.Infof("probe offer received: %v", offer.GetOfferType())
	return p.ProbeConnect(context.Background(), offer)
}

func (p *probe) handleDirectOffer(ctx context.Context, offer *domain.DirectOffer) error {
	candidates := strings.Split(offer.Candidate, ";")
	for _, candString := range candidates {
		if candString == "" {
			continue
		}
		candidate, err := ice.UnmarshalCandidate(candString)

		if err != nil {
			continue
		}

		if err = p.iceAgent.AddRemoteCandidate(candidate); err != nil {
			p.logger.Errorf("add remote candidate failed: %v", err)
			continue
		}

		p.logger.Infof("add remote candidate success:%v: %v", candidate.Marshal())
	}

	return nil
}

// ProbeConnect probes the connection, if isForceRelay, will start the relayChecker, otherwise, will start the directChecker
// when direct failed, we will start the relayChecker
func (p *probe) ProbeConnect(ctx context.Context, offer domain.Offer) error {
	defer func() {
		if p.connectionState == domain.ConnectionStateNew {
			p.UpdateConnectionState(domain.ConnectionStateChecking)
		}
	}()

	switch offer.GetOfferType() {
	case domain.OfferTypeDirectOffer, domain.OfferTypeDirectOfferAnswer:
		return p.directChecker.ProbeConnect(ctx, p.iceAgent.GetTieBreaker() > offer.TieBreaker(), offer)
	case domain.OfferTypeRelayOffer, domain.OfferTypeRelayAnswer:
		return p.relayChecker.ProbeConnect(ctx, false, offer)
	case domain.OfferTypeDrpOffer, domain.OfferTypeDrpOfferAnswer:
		return p.drpChecker.ProbeConnect(ctx, false, offer)
	default:
		return errors.New("unsupported offer type")
	}
}

func (p *probe) ProbeSuccess(ctx context.Context, publicKey, addr string) error {
	defer func() {
		p.UpdateConnectionState(domain.ConnectionStateConnected)
		p.logger.Infof("probe set to: %v", domain.ConnectionStateConnected)
	}()
	var err error

	peer := p.nodeManager.GetPeer(publicKey)

	p.logger.Infof("peer %v, address: %v, allowIps: %v connected", peer, peer.Address, peer.AllowedIPs)
	switch p.connectType {
	case domain.DrpType:

		addr = fmt.Sprintf("drp:to=%s//%s", publicKey, addr)
	case domain.RelayType:
		addr = fmt.Sprintf("relay:to=%s//%s", publicKey, addr)
	default:

	}

	if peer.AllowedIPs == "" {
		peer.AllowedIPs = fmt.Sprintf("%s/32", peer.Address)
		p.nodeManager.AddPeer(publicKey, peer)
	}

	if err = p.wgConfiger.AddPeer(&domain.SetPeer{
		PublicKey:            publicKey,
		Endpoint:             addr,
		AllowedIPs:           peer.AllowedIPs,
		PersistentKeepalived: 25,
	}); err != nil {
		return err
	}

	if err = infra.NewRouteApplier().ApplyRoute("add", *peer.Address, p.wgConfiger.GetIfaceName()); err != nil {
		return err
	}
	p.logger.Infof("peer connect to %s success", addr)

	return nil
}

func (p *probe) ProbeFailed(ctx context.Context, checker domain.Checker, offer domain.Offer) error {
	defer func() {
		p.UpdateConnectionState(domain.ConnectionStateFailed)
	}()

	return wferrors.ErrProbeFailed
}

func (p *probe) IsForceRelay() bool {
	return p.isForceRelay
}

func (p *probe) Start(ctx context.Context, srcKey, dstKey string) error {
	p.lastCheck = time.Now()
	p.logger.Infof("probe start, srcKey: %v, dstKey: %v, connection type: %v,  connection state: %v", srcKey, dstKey, p.connectType, p.connectionState)
	switch p.connectionState {
	case domain.ConnectionStateConnected:
		return nil
	case domain.ConnectionStateNew:
		p.UpdateConnectionState(domain.ConnectionStateChecking)
		switch p.connectType {
		case domain.DrpType:
			return p.SendOffer(ctx, drpgrpc.MessageType_MessageDrpOfferType, srcKey, dstKey)
		case domain.DirectType:
			return p.SendOffer(ctx, drpgrpc.MessageType_MessageDirectOfferType, srcKey, dstKey)
		case domain.RelayType:
			return p.SendOffer(ctx, drpgrpc.MessageType_MessageRelayOfferType, srcKey, dstKey)
		}

	default:
	}

	return nil
}

func (p *probe) SetConnectType(connType domain.ConnType) {
	p.connectType = connType
	p.logger.Infof("set connect type: %v", connType)
}

func (p *probe) SendOffer(ctx context.Context, msgType drpgrpc.MessageType, from, to string) error {
	var err error
	var relayAddr *net.UDPAddr
	defer func() {
		if err != nil {
			p.UpdateConnectionState(domain.ConnectionStateFailed)
		}
	}()

	var offer domain.Offer
	switch msgType {
	case drpgrpc.MessageType_MessageDirectOfferType, drpgrpc.MessageType_MessageDirectOfferAnswerType:
		ufrag, pwd, err := p.GetCredentials()
		if err != nil {
			p.UpdateConnectionState(domain.ConnectionStateFailed)
			return err
		}
		candidates := p.GetCandidates()
		offer = domain.NewDirectOffer(&domain.DirectOfferConfig{
			WgPort:     51820,
			Ufrag:      ufrag,
			Pwd:        pwd,
			LocalKey:   p.iceAgent.GetTieBreaker(),
			Candidates: candidates,
			Node:       p.nodeManager.GetPeer(from),
		})
	case drpgrpc.MessageType_MessageRelayOfferType, drpgrpc.MessageType_MessageRelayAnswerType:
		relayInfo := p.turnManager.GetInfo()
		relayAddr, err = turn.AddrToUdpAddr(relayInfo.RelayConn.LocalAddr())
		offer = domain.NewRelayOffer(&domain.RelayOfferConfig{
			MappedAddr: relayInfo.MappedAddr,
			RelayConn:  *relayAddr,
			Node:       p.nodeManager.GetPeer(from),
		})

	case drpgrpc.MessageType_MessageDrpOfferType, drpgrpc.MessageType_MessageDrpOfferAnswerType:
		offer = drp.NewDrpOffer(&drp.DrpOfferConfig{
			Node: p.nodeManager.GetPeer(from),
		})
	default:
		err = errors.New("unsupported message type")
		return err
	}

	err = p.offerHandler.SendOffer(ctx, msgType, from, to, offer)
	return err
}

//func (p *probe) TieBreaker() uint64 {
//	return p.GetProbeAgent().GetTieBreaker()
//}

func (p *probe) Clear(pubKey string) {
	p.closeMux.Lock()
	defer func() {
		p.logger.Infof("probe clearing: %v, remove agentManager and probe success", pubKey)
		p.closeMux.Unlock()
	}()
	p.iceAgent.Close()
	p.probeManager.RemoveProbe(pubKey)
}

func (p *probe) GetCredentials() (string, string, error) {
	return p.iceAgent.GetLocalUserCredentials()
}

func (p *probe) GetLastCheck() time.Time {
	return p.lastCheck
}

func (p *probe) UpdateLastCheck() {
	p.lastCheck = time.Now()
}

func (p *probe) GetCandidates() string {
	var (
		err        error
		candidates []ice.Candidate
		candString string
	)
	select {
	case <-p.gatherCh:
		candidates, err = p.iceAgent.GetLocalCandidates()
		if err != nil {
			return ""
		}
		for i, candidate := range candidates {
			candString = candidate.Marshal()
			if i != len(candidates)-1 {
				candString += ";"
			}
		}
		p.logger.Verbosef("gathered candidates >>>: %v", candString)
		return candString
	}
}
