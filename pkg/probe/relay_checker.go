package probe

import (
	"context"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"k8s.io/klog/v2"
	internal2 "linkany/internal"
	"linkany/pkg/iface"
	client2 "linkany/turn/client"
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
	key             wgtypes.Key // publickey of the peer
	dstKey          wgtypes.Key // publickey of the destination peer
	relayConn       net.PacketConn
	client          *client2.Client
	outBound        chan RelayMessage
	inBound         chan RelayMessage
	permissionAddrs []net.Addr // addrs will be added to the permission list
	wgConfiger      iface.WGConfigure
	prober          *Prober
	agentManager    *internal2.AgentManager
}

type RelayCheckerConfig struct {
	Client       *client2.Client
	WgConfiger   iface.WGConfigure
	AgentManager *internal2.AgentManager
	DstKey       wgtypes.Key
	SrcKey       wgtypes.Key
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

func (c *RelayChecker) OnFailure(offer internal2.Offer) error {
	return c.prober.ProbeFailed(c, offer)
}

type RelayMessage struct {
	buff      []byte
	relayAddr net.Addr
}

func (c *RelayChecker) ProbeConnect(ctx context.Context, isControlling bool, offer *RelayOffer) error {
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
	case OfferTypeRelayOffer:
		return c.OnSuccess(offer.RelayConn.String())
	case OfferTypeRelayOfferAnswer:
		return c.OnSuccess(offer.MappedAddr.String())
	}

	return c.OnFailure(offer)
}

func (c *RelayChecker) handleOffer(offer internal2.Offer) error {
	// set the destination permission
	relayOffer := offer.(*RelayOffer)

	switch relayOffer.OfferType {
	case OfferTypeRelayOffer:
		// write back a response
		info, err := c.prober.turnClient.GetRelayInfo(false)
		if err != nil {
			return err
		}
		klog.Infof(">>>>>>relay offer: %v", info.MappedAddr.String())

		newOffer := &RelayOffer{
			LocalKey:   c.agentManager.GetLocalKey(),
			MappedAddr: info.MappedAddr,
			OfferType:  OfferTypeRelayOfferAnswer,
		}

		if err = c.prober.SendOffer(internal2.MessageRelayOfferResponseType, c.key, c.dstKey, newOffer); err != nil {
			return err
		}
		return c.OnSuccess(relayOffer.RelayConn.String())
	case OfferTypeRelayOfferAnswer:
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
