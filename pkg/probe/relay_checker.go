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
	key             string // publickey of the peer
	dstKey          string // publickey of the destination peer
	relayConn       net.PacketConn
	client          *turnclient.Client
	outBound        chan RelayMessage
	inBound         chan RelayMessage
	permissionAddrs []net.Addr // addrs will be added to the permission list
	wgConfiger      internal.ConfigureManager
	prober          internal.Probe
	agentManager    *internal.AgentManagerFactory
}

type RelayCheckerConfig struct {
	Client       *turnclient.Client
	WgConfiger   internal.ConfigureManager
	AgentManager *internal.AgentManagerFactory
	DstKey       string
	SrcKey       string
}

func NewRelayChecker(config *RelayCheckerConfig) *relayChecker {
	return &relayChecker{
		client:       config.Client,
		agentManager: config.AgentManager,
		dstKey:       config.DstKey,
		key:          config.SrcKey,
	}
}

func (c *relayChecker) ProbeSuccess(ctx context.Context, addr string) error {
	return c.prober.ProbeSuccess(ctx, c.dstKey, addr)
}

func (c *relayChecker) ProbeFailure(ctx context.Context, offer internal.Offer) error {
	return c.prober.ProbeFailed(ctx, c, offer)
}

type RelayMessage struct {
	buff      []byte
	relayAddr net.Addr
}

func (c *relayChecker) ProbeConnect(ctx context.Context, isControlling bool, relayOffer internal.Offer) error {
	c.startCh = make(chan struct{})
	c.startTime = time.Now()

	offer := relayOffer.(*relay.RelayOffer)
	switch relayOffer.OfferType() {
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

	switch offer.OfferType() {
	case internal.OfferTypeRelayOffer:

		if err := c.prober.SendOffer(ctx, drpgrpc.MessageType_MessageRelayAnswerType, c.key, c.dstKey); err != nil {
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
