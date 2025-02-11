package probe

import (
	"context"
	"errors"
	"k8s.io/klog/v2"
	"linkany/internal"
	"linkany/pkg/iface"
	"linkany/signaling/grpc/signaling"
	"linkany/turn/client"
	"net"
	"sync/atomic"
)

type Probe interface {
	// Start the check process
	Start(srcKey, dstKey string, offer internal.Offer) error

	SendOffer(frameType signaling.MessageType, srcKey, dstKey string, offer internal.Offer) error

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
	ConnectionState internal.ConnectionState

	isStarted atomic.Bool

	isForceRelay bool

	agentManager *internal.AgentManager

	proberManager *NetProber

	key string

	// isController == true, will send a relay offer, otherwise, will wait for the relay offer
	isControlling bool

	isP2P bool

	// directChecker is used to check the direct connection
	directChecker *DirectChecker

	// relayChecker is used to check the relay connection
	relayChecker *RelayChecker

	localKey uint32

	wgConfiger *iface.WGConfigure

	directOfferManager internal.OfferManager
	relayOfferManager  internal.OfferManager

	turnClient *client.Client

	signalingChannel chan *signaling.EncryptMessage
}

func (p *Prober) UpdateConnectionState(state internal.ConnectionState) {
	p.ConnectionState = state
	p.proberManager.AddProber(p.key, p)
}

func (p *Prober) GetDirectChecker() *DirectChecker {
	return p.directChecker
}

func (p *Prober) GetRelayChecker() *RelayChecker {
	return p.relayChecker
}

func (p *Prober) HandleOffer(offer internal.Offer) error {
	if _, ok := offer.(*internal.DirectOffer); ok {
		if err := p.directChecker.handleOffer(offer); err != nil {
			return err
		}
	} else {
		o := offer.(*RelayOffer)
		switch o.OfferType {
		case OfferTypeRelayOffer:
			return p.relayChecker.handleOffer(offer)
		case OfferTypeRelayOfferAnswer:
			return p.relayChecker.handleOffer(offer)
		}

	}

	return p.ProbeConnect(context.Background(), offer)
}

type ProberConfig struct {
	IsControlling      bool
	IsForceRelay       bool
	IsP2P              bool
	DirectChecker      *DirectChecker
	RelayChecker       *RelayChecker
	AgentManager       *internal.AgentManager
	DirectOfferManager internal.OfferManager
	RelayOfferManager  internal.OfferManager
	WGConfiger         *iface.WGConfigure
	ProberManager      *NetProber
	Key                string
	TurnClient         *client.Client
	Relayer            internal.Relay
	SignalingChannel   chan *signaling.EncryptMessage
}

// NewProber creates a new Prober
func NewProber(config *ProberConfig) *Prober {
	prober := &Prober{
		ConnectionState:    internal.ConnectionStateNew,
		isControlling:      config.IsControlling,
		isP2P:              config.IsP2P,
		directChecker:      config.DirectChecker,
		relayChecker:       config.RelayChecker,
		agentManager:       config.AgentManager,
		directOfferManager: config.DirectOfferManager,
		relayOfferManager:  config.RelayOfferManager,
		wgConfiger:         config.WGConfiger,
		proberManager:      config.ProberManager,
		isForceRelay:       config.IsForceRelay,
		turnClient:         config.TurnClient,
		signalingChannel:   config.SignalingChannel,
	}

	prober.localKey = config.AgentManager.GetLocalKey()
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
		if _, ok := offer.(*internal.DirectOffer); ok {
			// ignore the direct offer
			return nil
		} else {
			return p.relayChecker.ProbeConnect(ctx, p.isControlling, offer.(*RelayOffer))
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
	klog.Infof("peer remoteKey: %v, remote addr: %v", publicKey, addr)

	peer := p.wgConfiger.GetPeersManager().GetPeer(publicKey)
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
			p.proberManager.relayer.AddRelayConn(endpoint, relayInfo.RelayConn)
		}
	}

	return nil
}

func (p *Prober) ProbeFailed(checker ConnChecker, offer internal.Offer) error {
	defer p.UpdateConnectionState(internal.ConnectionStateFailed)
	if checker.(*DirectChecker) == p.directChecker {
		return p.relayChecker.ProbeConnect(context.Background(), p.isControlling, offer.(*RelayOffer))
	}

	return errors.New("probe connect failed, need check the network you are in")
}

func (p *Prober) IsForceRelay() bool {
	return p.isForceRelay
}

func (p *Prober) Start(srcKey, dstKey string, offer internal.Offer) error {
	klog.Infof("prober start, srcKey: %v, dstKey: %v, offer: %v, isForceRelay: %v,  connection state: %v", srcKey, dstKey, offer, p.isForceRelay, p.ConnectionState)
	switch p.ConnectionState {
	case internal.ConnectionStateConnected, internal.ConnectionStateChecking:
		return nil
	case internal.ConnectionStateNew:
		if p.isForceRelay {
			return p.SendOffer(signaling.MessageType_MessageRelayOfferType, srcKey, dstKey, offer)
		} else {
			return p.SendOffer(signaling.MessageType_MessageDirectOfferType, srcKey, dstKey, offer)
		}
	default:

	}

	return nil
}

func (p *Prober) SendOffer(msgType signaling.MessageType, srcKey, dstKey string, offer internal.Offer) error {
	switch msgType {
	case signaling.MessageType_MessageDirectOfferType:
		return p.directOfferManager.SendOffer(msgType, srcKey, dstKey, offer)
	case signaling.MessageType_MessageRelayOfferType:
		return p.relayOfferManager.SendOffer(msgType, srcKey, dstKey, offer)
	}

	return nil
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
