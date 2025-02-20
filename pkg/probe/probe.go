package probe

import (
	"context"
	"errors"
	"k8s.io/klog/v2"
	"linkany/internal"
	"linkany/internal/direct"
	"linkany/internal/relay"
	"linkany/pkg/iface"
	"linkany/pkg/linkerrors"
	"linkany/signaling/grpc/signaling"
	"linkany/turn/client"
	turnclient "linkany/turn/client"
	"net"
	"sync"
	"sync/atomic"
)

type Probe interface {
	// Start the check process
	Start(srcKey, dstKey string) error

	SendOffer(frameType signaling.MessageType, srcKey, dstKey string) error

	HandleOffer(offer internal.Offer) error

	ProbeConnect(ctx context.Context, offer internal.Offer) error

	ProbeSuccess(publicKey string, conn string) error

	ProbeFailed(checker ConnChecker, offer internal.Offer) error
}

var (
	_ Probe = (*Prober)(nil)
)

// Prober is a wrapper directchecker & relaychecker
type Prober struct {
	closeMux sync.Mutex

	proberClosed atomic.Bool

	ProberDone chan interface{}

	ConnectionState internal.ConnectionState

	isStarted atomic.Bool

	isForceRelay bool

	agentManager *internal.AgentManager

	proberManager *NetProber

	srcKey string
	key    string

	// isController == true, will send a relay offer, otherwise, will wait for the relay offer
	isControlling bool

	isP2P bool

	// directChecker is used to check the direct connection
	directChecker *DirectChecker

	// relayChecker is used to check the relay connection
	relayChecker *RelayChecker

	localKey uint32

	wgConfiger *iface.WGConfigure

	offerManager internal.OfferManager

	turnClient *client.Client

	signalingChannel chan *signaling.EncryptMessage

	ufrag                   string
	pwd                     string
	gatherCh                chan interface{}
	OnConnectionStateChange func(state internal.ConnectionState)
}

func (p *Prober) UpdateConnectionState(state internal.ConnectionState) {
	p.ConnectionState = state
	p.proberManager.AddProber(p.key, p)
	p.OnConnectionStateChange(state)
}

func (p *Prober) GetDirectChecker() *DirectChecker {
	return p.directChecker
}

func (p *Prober) GetRelayChecker() *RelayChecker {
	return p.relayChecker
}

func (p *Prober) HandleOffer(offer internal.Offer) error {
	if _, ok := offer.(*direct.DirectOffer); ok {
		if err := p.directChecker.handleOffer(offer); err != nil {
			return err
		}
	} else {
		o := offer.(*relay.RelayOffer)
		switch o.OfferType {
		case relay.OfferTypeRelayOffer:
			return p.relayChecker.handleOffer(offer)
		case relay.OfferTypeRelayOfferAnswer:
			return p.relayChecker.handleOffer(offer)
		}

	}

	return p.ProbeConnect(context.Background(), offer)
}

type ProberConfig struct {
	IsControlling           bool
	IsForceRelay            bool
	IsP2P                   bool
	DirectChecker           *DirectChecker
	RelayChecker            *RelayChecker
	AgentManager            *internal.AgentManager
	OfferManager            internal.OfferManager
	WGConfiger              *iface.WGConfigure
	ProberManager           *NetProber
	SrcKey                  string
	Key                     string
	TurnClient              *client.Client
	Relayer                 internal.Relay
	SignalingChannel        chan *signaling.EncryptMessage
	Ufrag                   string
	Pwd                     string
	GatherChan              chan interface{}
	OnConnectionStateChange func(state internal.ConnectionState)
	ProberDone              chan interface{}
}

// NewProber creates a new Prober
func NewProber(config *ProberConfig) *Prober {
	prober := &Prober{
		ConnectionState:         internal.ConnectionStateNew,
		isControlling:           config.IsControlling,
		isP2P:                   config.IsP2P,
		directChecker:           config.DirectChecker,
		relayChecker:            config.RelayChecker,
		agentManager:            config.AgentManager,
		offerManager:            config.OfferManager,
		wgConfiger:              config.WGConfiger,
		proberManager:           config.ProberManager,
		isForceRelay:            config.IsForceRelay,
		turnClient:              config.TurnClient,
		signalingChannel:        config.SignalingChannel,
		gatherCh:                config.GatherChan,
		ufrag:                   config.Ufrag,
		pwd:                     config.Pwd,
		key:                     config.Key,
		srcKey:                  config.SrcKey,
		OnConnectionStateChange: config.OnConnectionStateChange,
		ProberDone:              make(chan interface{}),
	}

	prober.localKey = config.AgentManager.GetLocalKey()
	prober.proberClosed.Store(false)
	return prober
}

// ProbeConnect probes the connection, if isForceRelay, will start the relayChecker, otherwise, will start the directChecker
// when direct failed, we will start the relayChecker
func (p *Prober) ProbeConnect(ctx context.Context, offer internal.Offer) error {

	defer func() {
		if p.ConnectionState == internal.ConnectionStateNew {
			p.UpdateConnectionState(internal.ConnectionStateChecking)
		}
	}()

	if p.isForceRelay {
		if _, ok := offer.(*direct.DirectOffer); ok {
			// ignore the direct offer
			return nil
		} else {
			return p.relayChecker.ProbeConnect(ctx, p.isControlling, offer.(*relay.RelayOffer))
		}
	}
	return p.directChecker.ProbeConnect(ctx, p.isControlling, offer)
}

func (p *Prober) ProbeSuccess(publicKey, addr string) error {
	defer func() {
		p.UpdateConnectionState(internal.ConnectionStateConnected)
		klog.Infof("prober set to: %v", internal.ConnectionStateConnected)
	}()
	var err error

	peer := p.wgConfiger.GetPeersManager().GetPeer(publicKey)
	klog.Infof("peer remoteKey: %v, allowIps: %v, remote addr: %v", publicKey, peer.AllowedIps, addr)
	if err = p.wgConfiger.AddPeer(&iface.SetPeer{
		PublicKey:            publicKey,
		Endpoint:             addr,
		AllowedIPs:           peer.AllowedIps,
		PersistentKeepalived: 25,
	}); err != nil {
		return err
	}

	klog.Infof("peer connection to %s success", addr)
	iface.SetRoute()("add", p.wgConfiger.GetAddress(), p.wgConfiger.GetIfaceName())

	if p.isForceRelay {
		endpoint, err := net.ResolveUDPAddr("udp", addr)
		if err != nil {
			return err
		}

		if relayInfo, err := p.turnClient.GetRelayInfo(true); err != nil {
			return err
		} else {
			err := p.proberManager.relayer.AddRelayConn(endpoint, relayInfo.RelayConn)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (p *Prober) ProbeFailed(checker ConnChecker, offer internal.Offer) error {
	defer func() {
		p.UpdateConnectionState(internal.ConnectionStateFailed)
	}()
	//if checker.(*DirectChecker) == p.directChecker {
	//	p.isForceRelay = true
	//	return p.Start(p.srcKey, p.key)
	//}

	return linkerrors.ErrProbeFailed
}

func (p *Prober) IsForceRelay() bool {
	return p.isForceRelay
}

func (p *Prober) Start(srcKey, dstKey string) error {

	klog.Infof("prober start, srcKey: %v, dstKey: %v, isForceRelay: %v,  connection state: %v", srcKey, dstKey, p.isForceRelay, p.ConnectionState)
	switch p.ConnectionState {
	case internal.ConnectionStateConnected:
		return nil
	case internal.ConnectionStateNew:
		p.UpdateConnectionState(internal.ConnectionStateChecking)

	default:
		if p.isForceRelay {
			return p.SendOffer(signaling.MessageType_MessageRelayOfferType, srcKey, dstKey)
		} else {
			return p.SendOffer(signaling.MessageType_MessageDirectOfferType, srcKey, dstKey)
		}
	}

	return nil
}

func (p *Prober) SendOffer(msgType signaling.MessageType, srcKey, dstKey string) error {
	var err error
	var relayAddr *net.UDPAddr
	var info *client.RelayInfo
	defer func() {
		if err != nil {
			p.UpdateConnectionState(internal.ConnectionStateFailed)
		}
	}()

	var offer internal.Offer
	switch msgType {
	case signaling.MessageType_MessageDirectOfferType:
		agent, b := p.agentManager.Get(dstKey)
		if !b {
			return linkerrors.ErrAgentNotFound
		}
		candidates := GetCandidates(agent, p.gatherCh)
		offer = direct.NewOffer(&direct.DirectOfferConfig{
			WgPort:     51820,
			Ufrag:      p.ufrag,
			Pwd:        p.pwd,
			LocalKey:   p.agentManager.GetLocalKey(),
			Candidates: candidates,
		})
		break
	case signaling.MessageType_MessageRelayOfferType:
		relayInfo, err := p.turnClient.GetRelayInfo(true)
		if err != nil {
			return errors.New("get relay info failed")
		}

		relayAddr, err = turnclient.AddrToUdpAddr(relayInfo.RelayConn.LocalAddr())
		offer = relay.NewOffer(&relay.RelayOfferConfig{
			MappedAddr: relayInfo.MappedAddr,
			RelayConn:  *relayAddr,
			LocalKey:   p.agentManager.GetLocalKey(),
			OfferType:  relay.OfferTypeRelayOffer,
		})
		break
	case signaling.MessageType_MessageRelayAnswerType:
		// write back a response
		info, err = p.turnClient.GetRelayInfo(false)
		if err != nil {
			return err
		}
		klog.Infof(">>>>>>relay offer: %v", info.MappedAddr.String())

		offer = relay.NewOffer(&relay.RelayOfferConfig{
			LocalKey:   p.agentManager.GetLocalKey(),
			MappedAddr: info.MappedAddr,
			OfferType:  relay.OfferTypeRelayOfferAnswer,
		})
	default:
		err = errors.New("unsupported message type")
		return err
	}

	err = p.offerManager.SendOffer(msgType, srcKey, dstKey, offer)
	return err
}

func (p *Prober) SetDirectChecker(dt *DirectChecker) {
	p.directChecker = dt
}

func (p *Prober) SetRelayChecker(rc *RelayChecker) {
	p.relayChecker = rc
}

func (p *Prober) SetIsControlling(b bool) {
	p.isControlling = b
}

func (p *Prober) Clear(pubKey string) {
	p.closeMux.Lock()
	defer func() {
		klog.Infof("prober clearing: %v, remove agent and prober success", pubKey)
		p.proberClosed.Store(true)
		p.closeMux.Unlock()
	}()
	p.agentManager.Remove(pubKey)
	p.proberManager.Remove(pubKey)
	if !p.proberClosed.Load() {
		close(p.ProberDone)
	}
}
