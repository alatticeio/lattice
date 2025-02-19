package drp

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/linkanyio/ice"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"k8s.io/klog/v2"
	"linkany/internal"
	"linkany/internal/direct"
	"linkany/internal/relay"
	"linkany/pkg/iface"
	"linkany/pkg/linkerrors"
	"linkany/pkg/probe"
	signalingclient "linkany/signaling/client"
	"linkany/signaling/grpc/signaling"
	"linkany/turn/client"
	"net"
	"net/netip"
	"sync"
)

var (
	_ internal.OfferManager = (*Client)(nil)
)

type Client struct {
	SrcKey *wgtypes.Key
	DstKey *wgtypes.Key
	client *signalingclient.Client
	node   *Node

	udpMux       *ice.UniversalUDPMuxDefault
	fn           func(key string, addr *net.UDPAddr) error
	agentManager *internal.AgentManager
	wgConfiger   iface.WGConfigure
	probers      *probe.NetProber

	stunClient    *client.Client
	signalChannel chan *signaling.EncryptMessage
}

func (c *Client) SetWgConfiger(wgConfiger iface.WGConfigure) {
	c.wgConfiger = wgConfiger
}

func (c *Client) SendOffer(messageType signaling.MessageType, srcKey, dstKey string, offer internal.Offer) error {
	var err error
	n, bytes, _ := offer.Marshal()
	if n > MAX_PACKET_SIZE {
		return fmt.Errorf("packet too large: %d", n)
	}

	req := &signaling.EncryptMessageReqAndResp{
		SrcPublicKey: srcKey,
		DstPublicKey: dstKey,
		Body:         bytes,
		Type:         messageType,
	}

	body, err := proto.Marshal(req)
	if err != nil {
		return err
	}

	in := &signaling.EncryptMessage{
		Body: body,
	}

	c.signalChannel <- in
	return nil
}

func (c *Client) ReceiveOffer(msg *signaling.EncryptMessage) error {
	var resp signaling.EncryptMessageReqAndResp
	var err error

	if err = proto.Unmarshal(msg.Body, &resp); err != nil {
		return err
	}

	klog.Infof("receive from signaling, pubkey: %v, userId: %v", resp.SrcPublicKey, resp.DstPublicKey)

	switch resp.Type {
	case signaling.MessageType_MessageForwardType:

	case signaling.MessageType_MessageDirectOfferType:
		go func() {
			err := c.handleResponse(&resp)
			if err != nil {
				klog.Errorf("handle response failed: %v", err)
			}
		}()
	case signaling.MessageType_MessageRelayOfferType:
		// handle relay offer
		go func() {
			err := c.handleRelayOffer(&resp)
			if err != nil {
				klog.Errorf("handle relay offer failed: %v", err)
			}
		}()
		//case internal.MessageRelayOfferResponseType:
		//	go c.handleRelayOfferResponse(ft, int(fl+5), b)
	}

	return nil
}

type ClientConfig struct {
	Node          *Node
	UdpMux        *ice.UniversalUDPMuxDefault
	AgentManager  *internal.AgentManager
	OfferManager  internal.OfferManager
	Probers       *probe.NetProber
	SignalChannel chan *signaling.EncryptMessage
}

// NewClient create a new client
func NewClient(config *ClientConfig) *Client {
	return &Client{
		signalChannel: config.SignalChannel,
		node:          config.Node,
		udpMux:        config.UdpMux,
		agentManager:  config.AgentManager,
		probers:       config.Probers,
	}
}

// Clientset remote client which connected to drp
type Clientset struct {
	PubKey wgtypes.Key
	Conn   net.Conn
	Brw    *bufio.ReadWriter
}

// IndexTable  will cache client set
type IndexTable struct {
	sync.RWMutex
	Clients map[string]*Clientset
}

func (c *Client) handleResponse(msg *signaling.EncryptMessageReqAndResp) error {
	var err error
	remoteKey := msg.SrcPublicKey
	dstKey := msg.DstPublicKey

	klog.Infof("remoteKey: %v, dstKey: %v", remoteKey, dstKey)

	offerAnswer, err := direct.UnmarshalOfferAnswer(msg.Body)
	if err != nil {
		klog.Errorf("unmarshal offer answer failed: %v", err)
		return err
	}
	klog.Infof("receive offer answer info, remote wgPort:%d,  remoteUfrag: %s, remotePwd: %s, remote localKey: %v, candidate: %v", offerAnswer.WgPort, offerAnswer.Ufrag, offerAnswer.Pwd, offerAnswer.LocalKey, offerAnswer.Candidate)

	agent, ok := c.agentManager.Get(remoteKey) // agent have created when fetch peers start working
	if !ok {
		klog.Errorf("agent not found")
		return linkerrors.ErrAgentNotFound
	}

	prober := c.probers.GetProber(remoteKey)
	if prober == nil {
		return linkerrors.ErrProberNotFound
	}

	if prober.IsForceRelay() {
		return nil
	}

	if prober.GetDirectChecker() == nil {
		dt := probe.NewDirectChecker(&probe.DirectCheckerConfig{
			Ufrag:      "",
			Agent:      agent,
			WgConfiger: c.wgConfiger,
			Key:        remoteKey,
			LocalKey:   c.agentManager.GetLocalKey(),
		})
		dt.SetProber(prober)
		prober.SetIsControlling(c.agentManager.GetLocalKey() > offerAnswer.LocalKey)
		prober.SetDirectChecker(dt)
		c.probers.AddProber(remoteKey, prober) // update the prober
	}

	return prober.HandleOffer(offerAnswer)
}

func (c *Client) handleRelayOffer(msg *signaling.EncryptMessageReqAndResp) error {
	var err error
	remoteKey := msg.SrcPublicKey
	dstKey := msg.DstPublicKey

	klog.Infof("remoteKey: %v, dstKey: %v", remoteKey, dstKey)

	offerAnswer, err := relay.UnmarshalOffer(msg.Body)
	if err != nil {
		klog.Errorf("unmarshal offer answer failed: %v", err)
		return err
	}

	prober := c.probers.GetProber(remoteKey)
	if prober == nil {
		return linkerrors.ErrProberNotFound
	}
	if prober.GetRelayChecker() == nil {
		rc := probe.NewRelayChecker(&probe.RelayCheckerConfig{
			Client:       c.stunClient,
			AgentManager: c.agentManager,
			DstKey:       remoteKey,
			SrcKey:       dstKey,
		})
		rc.SetProber(prober)
		prober.SetRelayChecker(rc)
	}

	return prober.HandleOffer(offerAnswer)
}

func (c *Client) handleRelayOfferResponse(resp *signaling.EncryptMessageReqAndResp) error {
	var err error
	remoteKey := resp.SrcPublicKey
	srcKey := resp.DstPublicKey

	klog.Infof("handle remoteKey: %v, srcKey: %v", remoteKey, srcKey)

	offerAnswer, err := relay.UnmarshalOffer(resp.Body)
	if err != nil {
		klog.Errorf("unmarshal offer answer failed: %v", err)
		return err
	}

	prober := c.probers.GetProber(remoteKey)
	if prober == nil {
		return errors.New("prober not found")
	}
	if prober.GetRelayChecker() == nil {
		rc := probe.NewRelayChecker(&probe.RelayCheckerConfig{
			Client:       c.stunClient,
			AgentManager: c.agentManager,
			DstKey:       remoteKey,
			SrcKey:       srcKey,
		})
		rc.SetProber(prober)
		prober.SetRelayChecker(rc)
	}

	return prober.HandleOffer(offerAnswer)
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
