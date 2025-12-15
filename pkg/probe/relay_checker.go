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
	"net"
	"time"
	"wireflow/internal/core/domain"
	drpgrpc "wireflow/internal/grpc"
	turnclient "wireflow/pkg/turn"
)

var (
	_ domain.Checker = (*relayChecker)(nil)
)

// relayChecker is a wrapper of net.PacketConn
type relayChecker struct {
	startTime       time.Time
	isControlling   bool
	startCh         chan struct{}
	key             string // publicKey of the peer
	dstKey          string // publicKey of the destination peer
	relayConn       net.PacketConn
	outBound        chan RelayMessage
	inBound         chan RelayMessage
	permissionAddrs []net.Addr // Addr will be added to the permission list
	wgConfiger      domain.Configurer
	probe           domain.Probe
	agentManager    domain.AgentManagerFactory
}

type RelayCheckerConfig struct {
	TurnManager  *turnclient.TurnManager
	WgConfiger   domain.Configurer
	AgentManager domain.AgentManagerFactory
	DstKey       string
	SrcKey       string
	Probe        domain.Probe
}

func NewRelayChecker(cfg *RelayCheckerConfig) *relayChecker {
	return &relayChecker{
		agentManager: cfg.AgentManager,
		dstKey:       cfg.DstKey,
		key:          cfg.SrcKey,
		probe:        cfg.Probe,
	}
}

func (c *relayChecker) ProbeSuccess(ctx context.Context, addr string) error {
	return c.probe.ProbeSuccess(ctx, c.dstKey, addr)
}

func (c *relayChecker) ProbeFailure(ctx context.Context, offer domain.Offer) error {
	return c.probe.ProbeFailed(ctx, c, offer)
}

type RelayMessage struct {
	buff      []byte
	relayAddr net.Addr
}

func (c *relayChecker) ProbeConnect(ctx context.Context, isControlling bool, relayOffer domain.Offer) error {
	c.startCh = make(chan struct{})
	c.startTime = time.Now()

	offer := relayOffer.(*domain.RelayOffer)
	switch relayOffer.GetOfferType() {
	case domain.OfferTypeRelayOffer:
		return c.ProbeSuccess(ctx, offer.RelayConn.String())
	case domain.OfferTypeRelayAnswer:
		return c.ProbeSuccess(ctx, offer.MappedAddr.String())
	}

	return c.ProbeFailure(ctx, offer)
}

func (c *relayChecker) HandleOffer(ctx context.Context, offer domain.Offer) error {
	// set the destination permission
	relayOffer := offer.(*domain.RelayOffer)

	switch offer.GetOfferType() {
	case domain.OfferTypeRelayOffer:

		if err := c.probe.SendOffer(ctx, drpgrpc.MessageType_MessageRelayAnswerType, c.key, c.dstKey); err != nil {
			return err
		}
		return c.ProbeSuccess(ctx, relayOffer.RelayConn.String())
	case domain.OfferTypeRelayAnswer:
		return c.ProbeSuccess(ctx, relayOffer.MappedAddr.String())
	}

	return nil
}

func (c *relayChecker) writeTo(buf []byte, addr net.Addr) {
	c.outBound <- RelayMessage{
		buff:      buf,
		relayAddr: addr,
	}
}

//func (c *relayChecker) SetProber(probe *probe) {
//	c.probe = probe
//}
