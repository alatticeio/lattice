package drp

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/linkanyio/ice"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"linkany/internal"
	"linkany/internal/direct"
	"linkany/pkg/config"
	"linkany/pkg/log"
	signalingclient "linkany/signaling/client"

	"linkany/signaling/grpc/signaling"
	"linkany/turn/client"
	"net"
	"net/netip"
	"sync"
	"time"
)

var (
	lock sync.Mutex
	_    internal.OfferHandler = (*offerHandler)(nil)
)

type offerHandler struct {
	logger *log.Logger
	client *signalingclient.Client
	node   *Node

	keyManager      internal.KeyManager
	stunUri         string
	udpMux          *ice.UDPMuxDefault
	universalUdpMux *ice.UniversalUDPMuxDefault
	fn              func(key string, addr *net.UDPAddr) error
	agentManager    internal.AgentManagerFactory
	probeManager    internal.ProbeManager
	nodeManager     *config.NodeManager

	stunClient    *client.Client
	signalChannel chan *signaling.SignalingMessage

	relay      bool
	turnClient *client.Client
}

type OfferHandlerConfig struct {
	Logger  *log.Logger
	Node    *Node
	StunUri string

	KeyManager      internal.KeyManager
	Ufrag           string
	Pwd             string
	UdpMux          *ice.UDPMuxDefault
	UniversalUdpMux *ice.UniversalUDPMuxDefault
	AgentManager    internal.AgentManagerFactory
	OfferManager    internal.OfferHandler
	ProbeManager    internal.ProbeManager
	SignalChannel   chan *signaling.SignalingMessage
	NodeManager     *config.NodeManager
}

// NewOfferHandler create a new client
func NewOfferHandler(cfg *OfferHandlerConfig) internal.OfferHandler {
	return &offerHandler{
		nodeManager:     cfg.NodeManager,
		logger:          cfg.Logger,
		signalChannel:   cfg.SignalChannel,
		node:            cfg.Node,
		stunUri:         cfg.StunUri,
		keyManager:      cfg.KeyManager,
		udpMux:          cfg.UdpMux,
		universalUdpMux: cfg.UniversalUdpMux,
		agentManager:    cfg.AgentManager,
		probeManager:    cfg.ProbeManager,
	}
}

func (h *offerHandler) SendOffer(messageType signaling.MessageType, srcKey, dstKey string, offer internal.Offer) error {
	n, bytes, err := offer.Marshal()
	if err != nil {
		return err
	}
	if n > MAX_PACKET_SIZE {
		return fmt.Errorf("packet too large: %d", n)
	}

	// write offer to signaling channel
	h.signalChannel <- &signaling.SignalingMessage{
		From:    srcKey,
		To:      dstKey,
		Body:    bytes,
		MsgType: messageType,
	}

	return nil
}

func (h *offerHandler) ReceiveOffer(msg *signaling.SignalingMessage) error {
	var err error
	if msg.Body == nil {
		return errors.New("body is nil")
	}
	if err = json.Unmarshal(msg.Body, msg); err != nil {
		return err
	}

	h.logger.Verbosef("receive from signaling service, srcPubKey: %v, dstPubKey: %v", msg.From, msg.To)

	switch msg.MsgType {
	case signaling.MessageType_MessageForwardType:

	case signaling.MessageType_MessageDirectOfferType:
		go func() {
			if err := h.handleDirectOffer(msg, false); err != nil {
				h.logger.Errorf("handle response failed: %v", err)
			}
		}()

	case signaling.MessageType_MessageDirectOfferAnswerType:
		// handle direct offer answer
		go func() {
			if err := h.handleDirectOffer(msg, true); err != nil {
				h.logger.Errorf("handle response failed: %v", err)
			}
		}()
	case signaling.MessageType_MessageRelayOfferType:
		// handle relay offer
		go func() {
			err := h.handleRelayOffer(msg)
			if err != nil {
				h.logger.Errorf("handle relay offer failed: %v", err)
			}
		}()
		//case internal.MessageRelayOfferResponseType:
		//	go h.handleRelayOfferResponse(ft, int(fl+5), b)
	}

	return nil
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

func (h *offerHandler) handleDirectOffer(msg *signaling.SignalingMessage, isAnswer bool) error {
	var (
		err   error
		probe internal.Probe
	)
	// remote src public key
	offer, err := direct.UnmarshalOffer(msg.Body)
	if err != nil {
		h.logger.Errorf("unmarshal offer answer failed: %v", err)
		return err
	}

	// add peer
	h.nodeManager.AddPeer(msg.From, offer.Node)

	probe = h.probeManager.GetProbe(msg.From)
	if probe == nil {
		probe, err = h.probeManager.NewProbe(&internal.ProberConfig{
			Logger:           log.NewLogger(log.Loglevel, "probe"),
			OfferManager:     h,
			StunUri:          h.stunUri,
			WGConfiger:       h.probeManager.GetWgConfiger(),
			To:               msg.From,
			NodeManager:      h.nodeManager,
			ProberManager:    h.probeManager,
			IsForceRelay:     h.relay,
			TurnClient:       h.turnClient,
			SignalingChannel: h.signalChannel,
			LocalKey:         ice.NewTieBreaker(),
			GatherChan:       make(chan interface{}),
		})

		if err != nil {
			return err
		}
	}

	if !isAnswer {
		// send a direct offer to the remote client
		if err = probe.SendOffer(signaling.MessageType_MessageDirectOfferAnswerType, h.keyManager.GetPublicKey(), msg.From); err != nil {
			return err
		}
	}
	return probe.HandleOffer(offer)
}

//func (h *offerHandler) createProbe(from, to string, agent *internal.Agent, err error, offer *direct.DirectOffer) (internal.Probe, error) {
//	h.logger.Verbosef("newProbe not found for to: %v, will create a new one", to)
//	//create a new newProbe
//	cfg := &internal.ProberConfig{
//		Logger:           log.NewLogger(log.Loglevel, "probe"),
//		OfferManager:     h,
//		StunUri:          h.stunUri,
//		WGConfiger:       h.probeManager.GetWgConfiger(),
//		To:               to,
//		ProberManager:    h.probeManager,
//		IsForceRelay:     h.relay,
//		TurnClient:       h.turnClient,
//		SignalingChannel: h.signalChannel,
//		LocalKey:         ice.NewTieBreaker(),
//		GatherChan:       make(chan interface{}),
//		OnConnectionStateChange: func(state internal.ConnectionState, srcKey string) {
//			switch state {
//			case internal.ConnectionStateFailed:
//				probe := h.probeManager.GetProbe(srcKey)
//				if err = probe.Restart(); err != nil {
//					h.logger.Errorf("probe restart failed: %v, srcKey :%v", err, srcKey)
//				} else {
//					h.logger.Infof("probe restarted for peer: %s", srcKey)
//				}
//			case internal.ConnectionStateConnected:
//			case internal.ConnectionStateChecking:
//			default:
//
//			}
//		},
//	}
//
//	newProbe, err := h.probeManager.NewProbe(cfg)
//	if err != nil {
//		return nil, err
//	}
//	return newProbe, nil
//}

func (h *offerHandler) handleRelayOffer(msg *signaling.SignalingMessage) error {
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

func (h *offerHandler) handleRelayOfferResponse(resp *signaling.SignalingMessage) error {
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

func parse(addr string) (conn.Endpoint, error) {
	addrPort, err := netip.ParseAddrPort(addr)
	if err != nil {
		return nil, err
	}

	return &AnyEndpoint{
		AddrPort: addrPort,
		src: struct {
			netip.Addr
			ifidx int32
		}{},
	}, nil
}
