package probe

import (
	"context"
	"errors"
	"fmt"
	"github.com/linkanyio/ice"
	drpgrpc "linkany/drp/grpc"
	"linkany/internal"
	"linkany/internal/direct"
	"linkany/internal/drp"
	"linkany/internal/relay"
	"linkany/pkg/linkerrors"
	"linkany/pkg/log"
	turnclient "linkany/turn/client"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	_ internal.Probe = (*probe)(nil)
)

// probe is a wrapper directchecker & relaychecker
type probe struct {
	logger          *log.Logger
	closeMux        sync.Mutex
	agent           *internal.Agent
	done            chan interface{}
	connectionState internal.ConnectionState
	isStarted       atomic.Bool
	isForceRelay    bool
	probeManager    internal.ProbeManager
	nodeManager     *internal.NodeManager
	agentManager    internal.AgentManagerFactory

	lastCheck time.Time

	from string
	to   string

	drpAddr string

	connectType internal.ConnectType // connectType indicates the type of connection, direct or relay

	// directChecker is used to check the direct connection
	directChecker internal.Checker

	// relayChecker is used to check the relay connection
	relayChecker internal.Checker

	drpChecker internal.Checker

	wgConfiger internal.ConfigureManager

	offerHandler internal.OfferHandler

	turnManager *turnclient.TurnManager

	gatherCh chan interface{}

	udpMux          *ice.UDPMuxDefault // udpMux is used to send and receive packets
	universalUdpMux *ice.UniversalUDPMuxDefault
}

func (p *probe) Restart() error {
	var (
		err error
	)

	originalAgent := p.agent
	defer func() {
		if err = originalAgent.Close(); err != nil {
			p.logger.Errorf("failed to close original agent: %v", err)
		} else {
			p.logger.Infof("original agent closed successfully")
		}
	}()

	p.UpdateConnectionState(internal.ConnectionStateNew)
	// create a new agent
	p.gatherCh = make(chan interface{})
	if p.agent, err = p.probeManager.NewAgent(p.gatherCh, p.OnConnectionStateChange); err != nil {
		return err
	}

	p.agent.OnCandidate(func(candidate ice.Candidate) {
		if candidate == nil {
			p.logger.Verbosef("gathered all candidates")
			close(p.gatherCh)
			return
		}

		p.logger.Verbosef("gathered candidate: %s", candidate.String())
	})

	// when restart should regather candidates
	if err = p.agent.GatherCandidates(); err != nil {
		return err
	}

	// update probe manager
	p.probeManager.AddProbe(p.to, p)
	return nil
}

func (p *probe) GetProbeAgent() *internal.Agent {
	return p.agent
}

func (p *probe) GetConnState() internal.ConnectionState {
	return p.connectionState
}

func (p *probe) ProbeDone() chan interface{} {
	return p.done
}

func (p *probe) GetGatherChan() chan interface{} {
	return p.gatherCh
}

func (p *probe) UpdateConnectionState(state internal.ConnectionState) {
	p.connectionState = state
	p.logger.Verbosef("probe connection state updated to: %v", state)
}

func (p *probe) OnConnectionStateChange(state internal.ConnectionState) error {
	p.connectionState = state
	p.logger.Verbosef("probe connection state updated to: %v", state)
	switch state {
	case internal.ConnectionStateFailed, internal.ConnectionStateDisconnected:
		if err := p.Restart(); err != nil {
			return err
		}
	}

	return nil
}

func (p *probe) HandleOffer(ctx context.Context, offer internal.Offer) error {
	switch offer.GetOfferType() {
	case internal.OfferTypeDirectOffer:
		// later new directChecker
		if p.directChecker == nil {
			p.directChecker = NewDirectChecker(&DirectCheckerConfig{
				Logger:     p.logger,
				Agent:      p.agent,
				Key:        p.to,
				WgConfiger: p.wgConfiger,
				LocalKey:   p.TieBreaker(),
				Prober:     p,
			})

			p.probeManager.AddProbe(p.to, p)
		}
		return p.handleDirectOffer(ctx, offer.(*direct.DirectOffer))

	case internal.OfferTypeRelayOffer, internal.OfferTypeRelayAnswer:
		if p.relayChecker == nil {
			p.relayChecker = NewRelayChecker(&RelayCheckerConfig{
				TurnManager:  p.turnManager,
				WgConfiger:   p.wgConfiger,
				AgentManager: p.agentManager,
				DstKey:       p.to,
				SrcKey:       p.from,
				Probe:        p,
			})
		}

	case internal.OfferTypeDrpOffer, internal.OfferTypeDrpOfferAnswer:
		if p.drpChecker == nil {
			p.drpChecker = NewDrpChecker(&DrpCheckerConfig{
				Probe:   p,
				From:    p.from,
				To:      p.to,
				DrpAddr: offer.GetNode().DrpAddr,
			})
		}
	}

	return p.ProbeConnect(context.Background(), offer)
}

func (p *probe) handleDirectOffer(ctx context.Context, offer *direct.DirectOffer) error {
	candidates := strings.Split(offer.Candidate, ";")
	for _, candString := range candidates {
		if candString == "" {
			continue
		}
		candidate, err := ice.UnmarshalCandidate(candString)

		if err != nil {
			continue
		}

		if err = p.agent.AddRemoteCandidate(candidate); err != nil {
			p.logger.Errorf("add remote candidate failed: %v", err)
			continue
		}

		p.logger.Infof("add remote candidate success:%v, agent: %v", candidate.Marshal(), p.agent)
	}

	return nil
}

// ProbeConnect probes the connection, if isForceRelay, will start the relayChecker, otherwise, will start the directChecker
// when direct failed, we will start the relayChecker
func (p *probe) ProbeConnect(ctx context.Context, offer internal.Offer) error {
	defer func() {
		if p.connectionState == internal.ConnectionStateNew {
			p.UpdateConnectionState(internal.ConnectionStateChecking)
		}
	}()

	switch offer.GetOfferType() {
	case internal.OfferTypeDirectOffer, internal.OfferTypeDirectOfferAnswer:
		return p.directChecker.ProbeConnect(ctx, p.TieBreaker() > offer.TieBreaker(), offer)
	case internal.OfferTypeRelayOffer, internal.OfferTypeRelayAnswer:
		return p.relayChecker.ProbeConnect(ctx, false, offer)
	case internal.OfferTypeDrpOffer, internal.OfferTypeDrpOfferAnswer:
		return p.drpChecker.ProbeConnect(ctx, false, offer)
	default:
		return errors.New("unsupported offer type")
	}
}

func (p *probe) ProbeSuccess(ctx context.Context, publicKey, addr string) error {
	defer func() {
		p.UpdateConnectionState(internal.ConnectionStateConnected)
		p.logger.Infof("probe set to: %v", internal.ConnectionStateConnected)
	}()
	var err error

	peer := p.nodeManager.GetPeer(publicKey)

	switch p.connectType {
	case internal.DrpType:

		addr = fmt.Sprintf("drp:to=%s//%s", publicKey, addr)
	case internal.RelayType:
		addr = fmt.Sprintf("relay:to=%s//%s", publicKey, addr)
	default:

	}

	if err = p.wgConfiger.AddPeer(&internal.SetPeer{
		PublicKey:            publicKey,
		Endpoint:             addr,
		AllowedIPs:           peer.AllowedIPs,
		PersistentKeepalived: 25,
	}); err != nil {
		return err
	}

	internal.SetRoute(p.logger)("add", peer.Address, p.wgConfiger.GetIfaceName())
	p.logger.Infof("peer connect to %s success", addr)

	return nil
}

func (p *probe) ProbeFailed(ctx context.Context, checker internal.Checker, offer internal.Offer) error {
	defer func() {
		p.UpdateConnectionState(internal.ConnectionStateFailed)
	}()

	return linkerrors.ErrProbeFailed
}

func (p *probe) IsForceRelay() bool {
	return p.isForceRelay
}

func (p *probe) Start(ctx context.Context, srcKey, dstKey string) error {
	p.lastCheck = time.Now()
	p.logger.Infof("probe start, srcKey: %v, dstKey: %v, connection type: %v,  connection state: %v", srcKey, dstKey, p.connectType, p.connectionState)
	switch p.connectionState {
	case internal.ConnectionStateConnected:
		return nil
	case internal.ConnectionStateNew:
		p.UpdateConnectionState(internal.ConnectionStateChecking)
		switch p.connectType {
		case internal.DrpType:
			return p.SendOffer(ctx, drpgrpc.MessageType_MessageDrpOfferType, srcKey, dstKey)
		case internal.DirectType:
			return p.SendOffer(ctx, drpgrpc.MessageType_MessageDirectOfferType, srcKey, dstKey)
		case internal.RelayType:
			return p.SendOffer(ctx, drpgrpc.MessageType_MessageRelayOfferType, srcKey, dstKey)
		}

	default:
	}

	return nil
}

func (p *probe) SetConnectType(connType internal.ConnectType) {
	p.connectType = connType
	p.logger.Infof("set connect type: %v", connType)
}

func (p *probe) SendOffer(ctx context.Context, msgType drpgrpc.MessageType, from, to string) error {
	var err error
	var relayAddr *net.UDPAddr
	defer func() {
		if err != nil {
			p.UpdateConnectionState(internal.ConnectionStateFailed)
		}
	}()

	var offer internal.Offer
	switch msgType {
	case drpgrpc.MessageType_MessageDirectOfferType, drpgrpc.MessageType_MessageDirectOfferAnswerType:
		ufrag, pwd, err := p.GetCredentials()
		if err != nil {
			p.UpdateConnectionState(internal.ConnectionStateFailed)
			return err
		}
		candidates := p.GetCandidates(p.agent)
		offer = direct.NewOffer(&direct.DirectOfferConfig{
			WgPort:     51820,
			Ufrag:      ufrag,
			Pwd:        pwd,
			LocalKey:   p.TieBreaker(),
			Candidates: candidates,
			Node:       p.nodeManager.GetPeer(from),
		})
	case drpgrpc.MessageType_MessageRelayOfferType, drpgrpc.MessageType_MessageRelayAnswerType:
		relayInfo := p.turnManager.GetInfo()
		relayAddr, err = turnclient.AddrToUdpAddr(relayInfo.RelayConn.LocalAddr())
		offer = relay.NewOffer(&relay.RelayOfferConfig{
			MappedAddr: relayInfo.MappedAddr,
			RelayConn:  *relayAddr,
			Node:       p.nodeManager.GetPeer(from),
		})

	case drpgrpc.MessageType_MessageDrpOfferType, drpgrpc.MessageType_MessageDrpOfferAnswerType:
		offer = drp.NewOffer(&drp.DrpOfferConfig{
			Node: p.nodeManager.GetPeer(from),
		})
	default:
		err = errors.New("unsupported message type")
		return err
	}

	err = p.offerHandler.SendOffer(ctx, msgType, from, to, offer)
	return err
}

func (p *probe) TieBreaker() uint64 {
	return p.GetProbeAgent().GetTieBreaker()
}

func (p *probe) Clear(pubKey string) {
	p.closeMux.Lock()
	defer func() {
		p.logger.Infof("probe clearing: %v, remove agent and probe success", pubKey)
		p.closeMux.Unlock()
	}()
	p.agent.Close()
	p.probeManager.RemoveProbe(pubKey)
}

func (p *probe) GetCredentials() (string, string, error) {
	return p.GetProbeAgent().GetLocalUserCredentials()
}

func (p *probe) GetLastCheck() time.Time {
	return p.lastCheck
}

func (p *probe) UpdateLastCheck() {
	p.lastCheck = time.Now()
}
