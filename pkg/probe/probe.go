package probe

import (
	"context"
	"errors"
	"github.com/linkanyio/ice"
	drpgrpc "linkany/drp/grpc"
	"linkany/internal"
	"linkany/internal/direct"
	"linkany/internal/drp"
	"linkany/internal/relay"
	"linkany/pkg/config"
	"linkany/pkg/linkerrors"
	"linkany/pkg/log"
	"linkany/turn/client"
	turnclient "linkany/turn/client"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var (
	_ internal.Probe = (*prober)(nil)
)

// prober is a wrapper directchecker & relaychecker
type prober struct {
	logger          *log.Logger
	closeMux        sync.Mutex
	agent           *internal.Agent
	proberClosed    atomic.Bool
	done            chan interface{}
	connectionState internal.ConnectionState
	isStarted       atomic.Bool
	isForceRelay    bool
	proberManager   internal.ProbeManager
	nodeManager     *config.NodeManager
	agentManager    internal.AgentManagerFactory

	lastCheck time.Time

	from string
	to   string

	drpAddr string

	connectType internal.ConnType // connectType indicates the type of connection, direct or relay

	// directChecker is used to check the direct connection
	directChecker internal.Checker

	// relayChecker is used to check the relay connection
	relayChecker internal.Checker

	drpChecker internal.Checker

	wgConfiger internal.ConfigureManager

	offerHandler internal.OfferHandler

	turnClient *client.Client

	gatherCh chan interface{}

	udpMux          *ice.UDPMuxDefault // udpMux is used to send and receive packets
	universalUdpMux *ice.UniversalUDPMuxDefault
}

// TODO get agent ufrag pwd
func (p *prober) Restart() error {
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
	if p.agent, err = p.proberManager.NewAgent(p.gatherCh, p.OnConnectionStateChange); err != nil {
		return err
	}

	p.agent.OnCandidate(func(candidate ice.Candidate) {
		if candidate == nil {
			p.logger.Verbosef("gathered all candidates")
			close(p.gatherCh)
			return
		}

		p.logger.Verbosef("gathered candidate: %s for %s", candidate.String())
	})

	// when restart should regather candidates
	if err = p.agent.GatherCandidates(); err != nil {
		return err
	}

	// update prober manager
	p.proberManager.AddProbe(p.to, p)
	return nil
}

func (p *prober) GetProbeAgent() *internal.Agent {
	return p.agent
}

func (p *prober) GetConnState() internal.ConnectionState {
	return p.connectionState
}

func (p *prober) ProbeDone() chan interface{} {
	return p.done
}

func (p *prober) GetGatherChan() chan interface{} {
	return p.gatherCh
}

func (p *prober) UpdateConnectionState(state internal.ConnectionState) {
	p.connectionState = state
	p.logger.Verbosef("probe connection state updated to: %v", state)
}

func (p *prober) OnConnectionStateChange(state internal.ConnectionState) error {
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

func (p *prober) HandleOffer(offer internal.Offer) error {
	switch offer.OfferType() {
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

			p.proberManager.AddProbe(p.to, p)
		}

		if err := p.directChecker.HandleOffer(offer); err != nil {
			return err
		}

	case internal.OfferTypeRelayOffer, internal.OfferTypeRelayAnswer:
		return p.relayChecker.HandleOffer(offer)
	case internal.OfferTypeDrpOffer, internal.OfferTypeDrpOfferAnswer:
		if p.drpChecker == nil {
			p.drpChecker = NewDrpChecker(&DrpCheckerConfig{
				Probe:   p,
				From:    p.from,
				To:      p.to,
				DrpAddr: p.drpAddr,
			})
		}

		if err := p.drpChecker.HandleOffer(offer); err != nil {
			return err
		}
	}

	return p.ProbeConnect(context.Background(), offer)
}

// ProbeConnect probes the connection, if isForceRelay, will start the relayChecker, otherwise, will start the directChecker
// when direct failed, we will start the relayChecker
func (p *prober) ProbeConnect(ctx context.Context, offer internal.Offer) error {
	defer func() {
		if p.connectionState == internal.ConnectionStateNew {
			p.UpdateConnectionState(internal.ConnectionStateChecking)
		}
	}()

	switch offer.OfferType() {
	case internal.OfferTypeDirectOffer, internal.OfferTypeDirectOfferAnswer:
		return p.directChecker.ProbeConnect(ctx, p.TieBreaker() > offer.TieBreaker(), offer)
	case internal.OfferTypeRelayOffer, internal.OfferTypeRelayAnswer:
		return p.relayChecker.ProbeConnect(ctx, p.TieBreaker() > offer.TieBreaker(), offer.(*relay.RelayOffer))
	case internal.OfferTypeDrpOffer, internal.OfferTypeDrpOfferAnswer:
		return p.drpChecker.ProbeConnect(ctx, p.TieBreaker() > offer.TieBreaker(), offer)
	}

	return nil
}

func (p *prober) ProbeSuccess(publicKey, addr string) error {
	defer func() {
		p.UpdateConnectionState(internal.ConnectionStateConnected)
		p.logger.Infof("prober set to: %v", internal.ConnectionStateConnected)
	}()
	var err error

	peer := p.nodeManager.GetPeer(publicKey)
	p.logger.Verbosef("peer: %v, key: %v", peer, publicKey)
	p.logger.Infof("peer to: %v, allowIps: %v, remote addr: %v", publicKey, peer.AllowedIPs, addr)
	if err = p.wgConfiger.AddPeer(&internal.SetPeer{
		PublicKey:            publicKey,
		Endpoint:             addr,
		AllowedIPs:           peer.AllowedIPs,
		PersistentKeepalived: 25,
	}); err != nil {
		return err
	}

	p.logger.Infof("peer connect to %s success", addr)
	internal.SetRoute(p.logger)("add", peer.Address, p.wgConfiger.GetIfaceName())

	return nil
}

func (p *prober) ProbeFailed(checker internal.Checker, offer internal.Offer) error {
	defer func() {
		p.UpdateConnectionState(internal.ConnectionStateFailed)
	}()

	return linkerrors.ErrProbeFailed
}

func (p *prober) IsForceRelay() bool {
	return p.isForceRelay
}

func (p *prober) Start(srcKey, dstKey string) error {
	p.lastCheck = time.Now()
	p.logger.Infof("prober start, srcKey: %v, dstKey: %v, isForceRelay: %v,  connection state: %v", srcKey, dstKey, p.isForceRelay, p.connectionState)
	switch p.connectionState {
	case internal.ConnectionStateConnected:
		return nil
	case internal.ConnectionStateNew:
		p.UpdateConnectionState(internal.ConnectionStateChecking)
		if p.isForceRelay {
			return p.SendOffer(drpgrpc.MessageType_MessageRelayOfferType, srcKey, dstKey)
		} else {
			return p.SendOffer(drpgrpc.MessageType_MessageDirectOfferType, srcKey, dstKey)
		}

	default:
	}

	return nil
}

func (p *prober) SendOffer(msgType drpgrpc.MessageType, srcKey, dstKey string) error {
	var err error
	var relayAddr *net.UDPAddr
	var info *client.RelayInfo
	defer func() {
		if err != nil {
			p.UpdateConnectionState(internal.ConnectionStateFailed)
		}
	}()

	var offer internal.Offer
	ufrag, pwd, err := p.GetCredentials()
	switch msgType {
	case drpgrpc.MessageType_MessageDirectOfferType, drpgrpc.MessageType_MessageDirectOfferAnswerType:
		candidates := p.GetCandidates(p.agent)
		offer = direct.NewOffer(&direct.DirectOfferConfig{
			WgPort:     51820,
			Ufrag:      ufrag,
			Pwd:        pwd,
			LocalKey:   p.TieBreaker(),
			Candidates: candidates,
			Node:       p.nodeManager.GetPeer(srcKey),
		})
	case drpgrpc.MessageType_MessageRelayOfferType:
		relayInfo, err := p.turnClient.GetRelayInfo(true)
		if err != nil {
			return errors.New("get relay info failed")
		}

		relayAddr, err = turnclient.AddrToUdpAddr(relayInfo.RelayConn.LocalAddr())
		offer = relay.NewOffer(&relay.RelayOfferConfig{
			MappedAddr: relayInfo.MappedAddr,
			RelayConn:  *relayAddr,
			LocalKey:   p.agent.GetTieBreaker(),
		})
		break
	case drpgrpc.MessageType_MessageRelayAnswerType:
		// write back a response
		info, err = p.turnClient.GetRelayInfo(false)
		if err != nil {
			return err
		}
		p.logger.Infof(">>>>>>relay offer: %v", info.MappedAddr.String())

		offer = relay.NewOffer(&relay.RelayOfferConfig{
			LocalKey:   p.agent.GetTieBreaker(),
			MappedAddr: info.MappedAddr,
			OfferType:  internal.OfferTypeRelayOffer,
		})
	case drpgrpc.MessageType_MessageDrpOfferType, drpgrpc.MessageType_MessageDrpOfferAnswerType:
		offer = drp.NewOffer(&drp.DrpOfferConfig{
			Node: p.nodeManager.GetPeer(srcKey),
		})
	default:
		err = errors.New("unsupported message type")
		return err
	}

	err = p.offerHandler.SendOffer(msgType, srcKey, dstKey, offer)
	return err
}

func (p *prober) TieBreaker() uint64 {
	return p.GetProbeAgent().GetTieBreaker()
}

func (p *prober) Clear(pubKey string) {
	p.closeMux.Lock()
	defer func() {
		p.logger.Infof("prober clearing: %v, remove agent and prober success", pubKey)
		p.proberClosed.Store(true)
		p.closeMux.Unlock()
	}()
	p.agent.Close()
	p.proberManager.Remove(pubKey)
	if !p.proberClosed.Load() {
		close(p.done)
	}
}

func (p *prober) GetCredentials() (string, string, error) {
	return p.GetProbeAgent().GetLocalUserCredentials()
}

func (p *prober) GetLastCheck() time.Time {
	return p.lastCheck
}

func (p *prober) UpdateLastCheck() {
	p.lastCheck = time.Now()
}
