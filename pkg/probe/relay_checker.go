package probe

import (
	"context"
	"linkany/internal"
	"linkany/internal/relay"
	"linkany/pkg/iface"
	"linkany/signaling/grpc/signaling"
	turnclient "linkany/turn/client"
	"net"
	"time"
)

var (
	_ ConnChecker = (*RelayChecker)(nil)
)

// RelayChecker is a wrapper of net.PacketConn
type RelayChecker struct {
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
	wgConfiger      iface.WGConfigure
	prober          *Prober
	agentManager    *internal.AgentManager
}

type RelayCheckerConfig struct {
	Client       *turnclient.Client
	WgConfiger   iface.WGConfigure
	AgentManager *internal.AgentManager
	DstKey       string
	SrcKey       string
}

func NewRelayChecker(config *RelayCheckerConfig) *RelayChecker {
	return &RelayChecker{
		client:       config.Client,
		agentManager: config.AgentManager,
		dstKey:       config.DstKey,
		key:          config.SrcKey,
	}
}

func (c *RelayChecker) OnSuccess(addr string) error {
	return c.prober.ProbeSuccess(c.dstKey, addr)
}

func (c *RelayChecker) OnFailure(offer internal.Offer) error {
	return c.prober.ProbeFailed(c, offer)
}

type RelayMessage struct {
	buff      []byte
	relayAddr net.Addr
}

func (c *RelayChecker) ProbeConnect(ctx context.Context, isControlling bool, offer *relay.RelayOffer) error {
	c.startCh = make(chan struct{})
	c.startTime = time.Now()

	////send a ping when got pong, success
	//_, err := c.relayConn.WriteTo([]byte("ping"), &offer.MappedAddr)
	//if err != nil {
	//	return err
	//}
	//b := make([]byte, 1024)
	//_, addr, err := c.relayConn.ReadFrom(b)
	//if err != nil {
	//	return err
	//}
	//
	//if string(b) == "pong" {
	//	return c.OnSuccess(addr.String())
	//}

	offerType := offer.OfferType
	switch offerType {
	case relay.OfferTypeRelayOffer:
		return c.OnSuccess(offer.RelayConn.String())
	case relay.OfferTypeRelayOfferAnswer:
		return c.OnSuccess(offer.MappedAddr.String())
	}

	return c.OnFailure(offer)
}

func (c *RelayChecker) handleOffer(offer internal.Offer) error {
	// set the destination permission
	relayOffer := offer.(*relay.RelayOffer)

	switch relayOffer.OfferType {
	case relay.OfferTypeRelayOffer:

		if err := c.prober.SendOffer(signaling.MessageType_MessageRelayAnswerType, c.key, c.dstKey); err != nil {
			return err
		}
		return c.OnSuccess(relayOffer.RelayConn.String())
	case relay.OfferTypeRelayOfferAnswer:
		if err := c.prober.turnClient.CreatePermission(&relayOffer.MappedAddr); err != nil {
			return err
		}

		return c.OnSuccess(relayOffer.MappedAddr.String())
	}

	return nil
}

func (c *RelayChecker) writeTo(buf []byte, addr net.Addr) {
	c.outBound <- RelayMessage{
		buff:      buf,
		relayAddr: addr,
	}
}

func (c *RelayChecker) SetProber(prober *Prober) {
	c.prober = prober
}
