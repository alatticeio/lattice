package probe

import (
	"context"
	drpgrpc "linkany/drp/grpc"
	"linkany/internal"
	"linkany/internal/relay"
	turnclient "linkany/turn/client"
	"net"
	"time"
)

var (
	_ internal.Checker = (*relayChecker)(nil)
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
	wgConfiger      internal.ConfigureManager
	probe           internal.Probe
	agentManager    internal.AgentManagerFactory
}

type RelayCheckerConfig struct {
	TurnManager  *turnclient.TurnManager
	WgConfiger   internal.ConfigureManager
	AgentManager internal.AgentManagerFactory
	DstKey       string
	SrcKey       string
	Probe        internal.Probe
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

func (c *relayChecker) ProbeFailure(ctx context.Context, offer internal.Offer) error {
	return c.probe.ProbeFailed(ctx, c, offer)
}

type RelayMessage struct {
	buff      []byte
	relayAddr net.Addr
}

func (c *relayChecker) ProbeConnect(ctx context.Context, isControlling bool, relayOffer internal.Offer) error {
	c.startCh = make(chan struct{})
	c.startTime = time.Now()

	offer := relayOffer.(*relay.RelayOffer)
	switch relayOffer.GetOfferType() {
	case internal.OfferTypeRelayOffer:
		return c.ProbeSuccess(ctx, offer.RelayConn.String())
	case internal.OfferTypeRelayAnswer:
		return c.ProbeSuccess(ctx, offer.MappedAddr.String())
	}

	return c.ProbeFailure(ctx, offer)
}

func (c *relayChecker) HandleOffer(ctx context.Context, offer internal.Offer) error {
	// set the destination permission
	relayOffer := offer.(*relay.RelayOffer)

	switch offer.GetOfferType() {
	case internal.OfferTypeRelayOffer:

		if err := c.probe.SendOffer(ctx, drpgrpc.MessageType_MessageRelayAnswerType, c.key, c.dstKey); err != nil {
			return err
		}
		return c.ProbeSuccess(ctx, relayOffer.RelayConn.String())
	case internal.OfferTypeRelayAnswer:
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
