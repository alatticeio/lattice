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

package drp

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"
	"wireflow/internal/core/domain"
	drpgrpc "wireflow/internal/grpc"
	"wireflow/pkg/log"
	turnclient "wireflow/pkg/turn"

	"github.com/wireflowio/ice"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var (
	_ domain.OfferHandler = (*offerHandler)(nil)
)

type offerHandler struct {
	logger *log.Logger
	client *Client
	node   *Node

	keyManager   domain.IKeyManager
	stunUri      string
	fn           func(key string, addr *net.UDPAddr) error
	agentManager domain.AgentManagerFactory
	probeManager domain.ProbeManager
	nodeManager  domain.IPeerManager

	proxy       *Proxy
	relay       bool
	turnManager *turnclient.TurnManager
}

type OfferHandlerConfig struct {
	Logger  *log.Logger
	Node    *Node
	StunUri string

	KeyManager      domain.IKeyManager
	UdpMux          *ice.UDPMuxDefault
	UniversalUdpMux *ice.UniversalUDPMuxDefault
	AgentManager    domain.AgentManagerFactory
	OfferManager    domain.OfferHandler
	ProbeManager    domain.ProbeManager
	Proxy           *Proxy
	NodeManager     domain.IPeerManager
	TurnManager     *turnclient.TurnManager
}

// NewOfferHandler create a new client
func NewOfferHandler(cfg *OfferHandlerConfig) domain.OfferHandler {
	return &offerHandler{
		nodeManager:  cfg.NodeManager,
		logger:       cfg.Logger,
		node:         cfg.Node,
		stunUri:      cfg.StunUri,
		keyManager:   cfg.KeyManager,
		agentManager: cfg.AgentManager,
		probeManager: cfg.ProbeManager,
		proxy:        cfg.Proxy,
		turnManager:  cfg.TurnManager,
	}
}

func (h *offerHandler) SendOffer(ctx context.Context, messageType drpgrpc.MessageType, from, to string, offer domain.Offer) error {
	_, bytes, err := offer.Marshal()
	if err != nil {
		return err
	}
	// write offer to signaling channel
	drpMessage := h.proxy.GetMessageFromPool()
	drpMessage.From = from
	drpMessage.To = to
	drpMessage.Body = bytes
	drpMessage.MsgType = messageType
	return h.proxy.WriteMessage(ctx, drpMessage)
}

func (h *offerHandler) ReceiveOffer(ctx context.Context, msg *drpgrpc.DrpMessage) error {
	if msg.Body == nil {
		return errors.New("body is nil")
	}
	return h.handleOffer(ctx, msg)
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

func (h *offerHandler) handleOffer(ctx context.Context, msg *drpgrpc.DrpMessage) error {
	var (
		offer domain.Offer
		err   error
	)

	var connectType domain.ConnType
	switch msg.MsgType {
	case drpgrpc.MessageType_MessageDirectOfferType, drpgrpc.MessageType_MessageDirectOfferAnswerType:
		offer, err = domain.UnmarshalOffer(msg.Body, &domain.DirectOffer{})
		connectType = domain.DirectType
	case drpgrpc.MessageType_MessageDrpOfferType, drpgrpc.MessageType_MessageDrpOfferAnswerType:
		offer, err = domain.UnmarshalOffer(msg.Body, &DrpOffer{})
		connectType = domain.DrpType
	case drpgrpc.MessageType_MessageRelayOfferType, drpgrpc.MessageType_MessageRelayAnswerType:
		offer, err = domain.UnmarshalOffer(msg.Body, &domain.RelayOffer{})
		connectType = domain.RelayType
	}

	// add peer if not exist in node manager
	if offer.GetNode() != nil && h.nodeManager.GetPeer(msg.From) == nil {
		h.nodeManager.AddPeer(msg.From, offer.GetNode())
	}
	probe := h.probeManager.GetProbe(msg.From)
	if probe == nil {
		cfg := &domain.ProbeConfig{
			Logger:        log.NewLogger(log.Loglevel, "probe"),
			StunUri:       h.stunUri,
			To:            msg.From,
			NodeManager:   h.nodeManager,
			OfferHandler:  h,
			ProberManager: h.probeManager,
			IsForceRelay:  h.relay,
			TurnManager:   h.turnManager,
			LocalKey:      ice.NewTieBreaker(),
			GatherChan:    make(chan interface{}),
			ConnectType:   connectType,
		}

		switch offer.GetOfferType() {
		case domain.OfferTypeDirectOffer, domain.OfferTypeDirectOfferAnswer:
			cfg.ConnType = domain.DirectType
		case domain.OfferTypeRelayOffer, domain.OfferTypeRelayAnswer:
			cfg.ConnType = domain.RelayType
		case domain.OfferTypeDrpOffer, domain.OfferTypeDrpOfferAnswer:
			cfg.ConnType = domain.DrpType
		}

		if probe, err = h.probeManager.NewProbe(cfg); err != nil {
			return err
		}
	}

	switch msg.MsgType {
	case drpgrpc.MessageType_MessageDirectOfferType:
		probe.SendOffer(ctx, drpgrpc.MessageType_MessageDirectOfferAnswerType, msg.To, msg.From)
	case drpgrpc.MessageType_MessageDrpOfferType:
		probe.SendOffer(ctx, drpgrpc.MessageType_MessageDrpOfferAnswerType, msg.To, msg.From)
	case drpgrpc.MessageType_MessageRelayOfferType:
		probe.SendOffer(ctx, drpgrpc.MessageType_MessageRelayAnswerType, msg.To, msg.From)
	}

	return probe.HandleOffer(ctx, offer)
}

func (h *offerHandler) Proxy(proxy *Proxy) domain.OfferHandler {
	h.proxy = proxy
	return h
}
