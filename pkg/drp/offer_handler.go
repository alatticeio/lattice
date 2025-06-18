package drp

import (
	"context"
	"errors"
	"github.com/linkanyio/ice"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	drpclient "linkany/drp/client"
	drpgrpc "linkany/drp/grpc"
	"linkany/internal"
	"linkany/internal/direct"
	"linkany/internal/drp"
	"linkany/internal/relay"
	"linkany/pkg/config"
	"linkany/pkg/log"
	"linkany/turn/client"
	"net"
	"sync"
	"time"
)

var (
	lock sync.Mutex
	_    internal.OfferHandler = (*offerHandler)(nil)
)

type offerHandler struct {
	logger *log.Logger
	client *drpclient.Client
	node   *Node

	keyManager      internal.KeyManager
	stunUri         string
	udpMux          *ice.UDPMuxDefault
	universalUdpMux *ice.UniversalUDPMuxDefault
	fn              func(key string, addr *net.UDPAddr) error
	agentManager    internal.AgentManagerFactory
	probeManager    internal.ProbeManager
	nodeManager     *config.NodeManager

	stunClient *client.Client
	proxy      *drpclient.Proxy
	relay      bool
	turnClient *client.Client
}

type OfferHandlerConfig struct {
	Logger  *log.Logger
	Node    *Node
	StunUri string

	KeyManager      internal.KeyManager
	UdpMux          *ice.UDPMuxDefault
	UniversalUdpMux *ice.UniversalUDPMuxDefault
	AgentManager    internal.AgentManagerFactory
	OfferManager    internal.OfferHandler
	ProbeManager    internal.ProbeManager
	Proxy           *drpclient.Proxy
	NodeManager     *config.NodeManager
}

// NewOfferHandler create a new client
func NewOfferHandler(cfg *OfferHandlerConfig) internal.OfferHandler {
	return &offerHandler{
		nodeManager:     cfg.NodeManager,
		logger:          cfg.Logger,
		node:            cfg.Node,
		stunUri:         cfg.StunUri,
		keyManager:      cfg.KeyManager,
		udpMux:          cfg.UdpMux,
		universalUdpMux: cfg.UniversalUdpMux,
		agentManager:    cfg.AgentManager,
		probeManager:    cfg.ProbeManager,
		proxy:           cfg.Proxy,
	}
}

func (h *offerHandler) SendOffer(messageType drpgrpc.MessageType, srcKey, dstKey string, offer internal.Offer) error {
	_, bytes, err := offer.Marshal()
	if err != nil {
		return err
	}
	// write offer to signaling channel
	h.proxy.WriteMessage(context.Background(), &drpgrpc.DrpMessage{
		From:    srcKey,
		To:      dstKey,
		Body:    bytes,
		MsgType: messageType,
	})

	return nil
}

func (h *offerHandler) ReceiveOffer(msg *drpgrpc.DrpMessage) error {
	if msg.Body == nil {
		return errors.New("body is nil")
	}

	return h.handleOffer(msg)
}

// Clientset remote client which connected to drp
type Clientset struct {
	PubKey        wgtypes.Key
	LastHeartbeat time.Time
	Done          chan struct{}
}

// IndexTable  will cache client set
type IndexTable struct {
	sync.RWMutex
	Clients map[string]*Clientset
}

func (h *offerHandler) handleOffer(msg *drpgrpc.DrpMessage) error {
	var (
		offer internal.Offer
		err   error
	)

	switch msg.MsgType {
	case drpgrpc.MessageType_MessageDirectOfferType, drpgrpc.MessageType_MessageDirectOfferAnswerType:
		offer, err = direct.UnmarshalOffer(msg.Body)
	case drpgrpc.MessageType_MessageDrpOfferType, drpgrpc.MessageType_MessageDrpOfferAnswerType:
		offer, err = drp.UnmarshalOffer(msg.Body)
	case drpgrpc.MessageType_MessageRelayOfferType:
		offer, err = relay.UnmarshalOffer(msg.Body)
	}

	// add peer
	h.nodeManager.AddPeer(msg.From, offer.GetNode())
	probe := h.probeManager.GetProbe(msg.From)
	if probe == nil {
		cfg := &internal.ProberConfig{
			Logger:        log.NewLogger(log.Loglevel, "probe"),
			OfferManager:  h,
			StunUri:       h.stunUri,
			To:            msg.From,
			NodeManager:   h.nodeManager,
			ProberManager: h.probeManager,
			IsForceRelay:  h.relay,
			TurnClient:    h.turnClient,
			LocalKey:      ice.NewTieBreaker(),
			GatherChan:    make(chan interface{}),
		}

		switch offer.OfferType() {
		case internal.OfferTypeDirectOffer, internal.OfferTypeDirectOfferAnswer:
			cfg.ConnType = internal.DirectType
		case internal.OfferTypeRelayOffer, internal.OfferTypeRelayAnswer:
			cfg.ConnType = internal.RelayType
		case internal.OfferTypeDrpOffer, internal.OfferTypeDrpOfferAnswer:
			cfg.ConnType = internal.DrpType
		}

		if probe, err = h.probeManager.NewProbe(cfg); err != nil {
			return err
		}
	}

	switch msg.MsgType {
	case drpgrpc.MessageType_MessageDirectOfferType:
		probe.SendOffer(drpgrpc.MessageType_MessageDirectOfferAnswerType, msg.To, msg.From)
	case drpgrpc.MessageType_MessageDrpOfferType:
		// handle drp offer
		probe.SendOffer(drpgrpc.MessageType_MessageDrpOfferAnswerType, msg.To, msg.From)
	case drpgrpc.MessageType_MessageRelayOfferType:
		probe.SendOffer(drpgrpc.MessageType_MessageRelayAnswerType, msg.To, msg.From)
	}

	return probe.HandleOffer(offer)
}

func (h *offerHandler) handleDirectOffer(msg *drpgrpc.DrpMessage, isAnswer bool) error {
	var (
		err   error
		probe internal.Probe
		offer internal.Offer
	)
	// remote src public key
	offer, err = internal.UnmarshalOffer(msg.Body, offer)
	if err != nil {
		h.logger.Errorf("unmarshal offer answer failed: %v", err)
		return err
	}

	// add peer
	h.nodeManager.AddPeer(msg.From, offer.GetNode())

	probe = h.probeManager.GetProbe(msg.From)
	if probe == nil {
		probe, err = h.probeManager.NewProbe(&internal.ProberConfig{
			Logger:        log.NewLogger(log.Loglevel, "probe"),
			OfferManager:  h,
			StunUri:       h.stunUri,
			To:            msg.From,
			NodeManager:   h.nodeManager,
			ProberManager: h.probeManager,
			IsForceRelay:  h.relay,
			TurnClient:    h.turnClient,
			LocalKey:      ice.NewTieBreaker(),
			GatherChan:    make(chan interface{}),
		})

		if err != nil {
			return err
		}
	}

	if !isAnswer {
		// send a direct offer to the remote client
		if err = probe.SendOffer(drpgrpc.MessageType_MessageDirectOfferAnswerType, h.keyManager.GetPublicKey(), msg.From); err != nil {
			return err
		}
	}
	return probe.HandleOffer(offer)
}

func (h *offerHandler) handleRelayOffer(msg *drpgrpc.DrpMessage) error {
	//var err error
	//srcPublicKey := msg.SrcPublicKey
	//dstKey := msg.DstPublicKey
	//
	//h.logger.Verbosef("srcPublicKey: %v, dstKey: %v", srcPublicKey, dstKey)
	//
	//offerAnswer, err := relay.UnmarshalOffer(msg.Body)
	//if err != nil {
	//	h.logger.Errorf("unmarshal offer answer failed: %v", err)
	//	return err
	//}
	//
	//prober := h.ProbeManager.GetProbe(srcPublicKey)
	//if prober == nil {
	//	h.createProbe()
	//	return linkerrors.ErrProberNotFound
	//}
	//
	//rc := probe.NewRelayChecker(&probe.RelayCheckerConfig{
	//	Client:       h.stunClient,
	//	AgentManagerFactory: h.agentManager,
	//	DstPublicKey:       srcPublicKey,
	//	SrcPublicKey:       dstKey,
	//})
	//rc.SetProbe(prober)
	//prober.SetRelayChecker(rc)

	//return prober.HandleOffer(offerAnswer)
	return nil
}

//
//func (h *offerHandler) handleDrpOffer(msg *drpgrpc.DrpMessage, isAnswer bool) error {
//	var (
//		err   error
//		probe internal.Probe
//	)
//	// remote src public key
//	offer, err := drp.UnmarshalOffer(msg.Body)
//	if err != nil {
//		h.logger.Errorf("unmarshal offer answer failed: %v", err)
//		return err
//	}
//
//	h.nodeManager.AddPeer(msg.From, offer.Node)
//
//	probe = h.probeManager.GetProbe(msg.From)
//	if probe == nil {
//		probe, err = h.probeManager.NewProbe(&internal.ProberConfig{
//			Logger:           log.NewLogger(log.Loglevel, "probe"),
//			OfferManager:     h,
//			StunUri:          h.stunUri,
//			IsDrp:            true,
//			WGConfiger:       h.probeManager.GetWgConfiger(),
//			To:               msg.From,
//			NodeManager:      h.nodeManager,
//			ProberManager:    h.probeManager,
//			IsForceRelay:     h.relay,
//			TurnClient:       h.turnClient,
//			SignalingChannel: h.outBoundQueue,
//			LocalKey:         ice.NewTieBreaker(),
//			GatherChan:       make(chan interface{}),
//		})
//
//		if err != nil {
//			return err
//		}
//	}
//
//	if !isAnswer {
//		probe.SendOffer(drpgrpc.MessageType_MessageDrpOfferAnswerType, h.keyManager.GetPublicKey(), msg.From)
//	}
//
//	return probe.HandleOffer(offer)
//
//}

func (h *offerHandler) handleRelayOfferResponse(resp *drpgrpc.DrpMessage) error {
	//var err error
	//remoteKey := resp.SrcPublicKey
	//srcKey := resp.DstPublicKey
	//
	//h.logger.Verbosef("handle remoteKey: %v, srcKey: %v", remoteKey, srcKey)
	//
	//offerAnswer, err := relay.UnmarshalOffer(resp.Body)
	//if err != nil {
	//	h.logger.Errorf("unmarshal offer answer failed: %v", err)
	//	return err
	//}
	//
	//prober := h.ProbeManager.GetProbe(remoteKey)
	//if prober == nil {
	//
	//	return errors.New("prober not found")
	//}
	//if prober.GetRelayChecker() == nil {
	//	rc := probe.NewRelayChecker(&probe.RelayCheckerConfig{
	//		Client:       h.stunClient,
	//		AgentManagerFactory: h.agentManager,
	//		DstPublicKey:       remoteKey,
	//		SrcPublicKey:       srcKey,
	//	})
	//	rc.SetProbe(prober)
	//	prober.SetRelayChecker(rc)
	//}
	//
	//return prober.HandleOffer(offerAnswer)
	return nil
}

func (h *offerHandler) SetProxy(proxy *drpclient.Proxy) {
	h.proxy = proxy
}
