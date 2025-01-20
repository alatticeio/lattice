package drp

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/linkanyio/ice"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"io"
	"k8s.io/klog/v2"
	"linkany/internal"
	"linkany/pkg/iface"
	"linkany/pkg/probe"
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
	conn   net.Conn
	brw    *bufio.ReadWriter // used to read and write data
	node   *Node

	udpMux       *ice.UniversalUDPMuxDefault
	fn           func(key string, addr *net.UDPAddr) error
	agentManager *internal.AgentManager
	wgConfiger   iface.WGConfigure
	offerManager internal.OfferManager
	probers      *probe.NetProber

	stunClient *client.Client
}

func (c *Client) SetWgConfiger(wgConfiger iface.WGConfigure) {
	c.wgConfiger = wgConfiger
}

func (c *Client) SendOffer(frameType internal.FrameType, srcKey wgtypes.Key, dstKey wgtypes.Key, offer internal.Offer) error {
	var err error
	srcKeyBytes := srcKey[:] //32
	dstKeyBytes := dstKey[:] // 32
	n, bytes, _ := offer.Marshal()
	if n > MAX_PACKET_SIZE {
		return fmt.Errorf("packet too large: %d", n)
	}
	switch frameType {
	case internal.MessageDirectOfferType, internal.MessageRelayOfferType, internal.MessageRelayOfferResponseType:
		if err = writeFrameHeader(c.brw.Writer, frameType, uint32(64+n)); err != nil {
			return err
		}

		if _, err = writeFrame(c.brw.Writer, srcKeyBytes); err != nil {
			return err
		}

		if _, err = writeFrame(c.brw.Writer, dstKeyBytes); err != nil {
			return err
		}

		if _, err = writeFrame(c.brw.Writer, bytes); err != nil {
			return err
		}

		klog.Infof("send offer to drp server success, t: %v, len: %v, srcKey: %v, dstKey: %v, content: %v, contentlen: %v", frameType.String(), 64+uint32(n), srcKeyBytes, dstKeyBytes, bytes, len(bytes))
	}

	return c.brw.Flush()
}

func (c *Client) ReceiveOffer() (internal.Offer, error) {
	//TODO implement me
	panic("implement me")
}

type ClientConfig struct {
	Conn         net.Conn
	Brw          *bufio.ReadWriter
	Node         *Node
	UdpMux       *ice.UniversalUDPMuxDefault
	AgentManager *internal.AgentManager
	OfferManager internal.OfferManager
	Probers      *probe.NetProber
}

// NewClient create a new client
func NewClient(config *ClientConfig) *Client {
	return &Client{
		brw:          config.Brw,
		conn:         config.Conn,
		node:         config.Node,
		udpMux:       config.UdpMux,
		agentManager: config.AgentManager,
		offerManager: config.OfferManager,
		probers:      config.Probers,
	}
}

func (c *Client) ReadWriter() *bufio.ReadWriter {
	return c.brw
}

// Close close the client
func (c *Client) Close() error {
	return c.conn.Close()
}

// SendFrame send frame to drp server
func (c *Client) SendFrame(t internal.FrameType, buf []byte) error {
	var err error
	if err = writeFrameHeader(c.brw.Writer, t, uint32(len(buf))); err != nil {
		return err
	}
	_, err = writeFrame(c.brw.Writer, buf)
	return err
}

// Send will send wireguard data to drp server, just add header to the data
func (c *Client) Send(bufs [][]byte) error {
	var err error
	if len(bufs) == 0 {
		return fmt.Errorf("empty buffer")
	}
	if c.SrcKey == nil || c.DstKey == nil {
		return fmt.Errorf("src key or dst key is nil")
	}
	//writeFrameHeader(c.brw.Writer, t, uint32(len(bufs))+8) //add src key and dst key length
	if _, err = writeFrame(c.brw.Writer, c.SrcKey[:]); err != nil {
		return err
	}
	if _, err = writeFrame(c.brw.Writer, c.DstKey[:]); err != nil {
		return err
	}
	if _, err = writeFrame(c.brw.Writer, bufs[0]); err != nil {
		return err
	}

	return nil
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

// ReceiveDetail receive data from drp server
func (c *Client) ReceiveDetail() conn.ReceiveFunc {
	return func(bufs [][]byte, sizes []int, eps []conn.Endpoint) (n int, err error) {
		br := c.brw.Reader
		b := make([]byte, 1024) // not need allocate
		ft, fl, err := ReadFrameHeader(br, b)
		if err != nil {
			if err == io.EOF {
				return 0, nil
			}
			klog.Errorf("read from remote failed: %v", err)
			return 0, nil
		}

		n, err = ReadFrame(br, 5, int(fl+5), b)
		if err != nil {
			return 0, err
		}

		if n != int(fl) {
			return 0, errors.New("read frame failed")
		}

		switch ft {
		case internal.MessageForwardType:
			srcKey, dstKey, content, err := ReadKey(br, fl)
			if err != nil {
				return 0, err
			}

			if dstKey != c.SrcKey {
				return 0, errors.New("data not send to this node")
			}

			fmt.Println(srcKey, dstKey)
			// copy original remote wireguard data to bufs.
			copy(b[:], content)
			eps[0], err = parse(c.conn.RemoteAddr().String())

		case internal.MessageDirectOfferType:
			klog.Infof("handle node info got frame type: %v, frame len: %v, content: %v", ft, fl, b[:fl+5])
			go c.handleNodeInfo(ft, int(fl+5), b)
		case internal.MessageRelayOfferType:
			// handle relay offer
			go c.handleRelayOffer(ft, int(fl+5), b)
		case internal.MessageRelayOfferResponseType:
			go c.handleRelayOfferResponse(ft, int(fl+5), b)
		}

		return 0, nil
	}
}

func (c *Client) handleNodeInfo(t internal.FrameType, length int, buf []byte) error {
	var err error
	remoteKey := wgtypes.Key(buf[5:37])
	dstKey := wgtypes.Key(buf[37:69])

	klog.Infof("remoteKey: %v, dstKey: %v", remoteKey.String(), dstKey.String())
	content := buf[69:length]

	offerAnswer, err := internal.UnmarshalOfferAnswer(content)
	if err != nil {
		klog.Errorf("unmarshal offer answer failed: %v", err)
		return err
	}
	klog.Infof("got offer answer info, remote wgPort:%d,  remoteUfrag: %s, remotePwd: %s, remote localKey: %v, candidate: %v", offerAnswer.WgPort, offerAnswer.Ufrag, offerAnswer.Pwd, offerAnswer.LocalKey, offerAnswer.Candidate)

	agent, ok := c.agentManager.Get(remoteKey.String()) // agent have created when fetch peers start working
	if !ok {
		klog.Errorf("agent not found")
		return errors.New("agent not found")
	}

	prober := c.probers.GetProber(remoteKey)
	if prober == nil {
		return errors.New("prober not found")
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

func (c *Client) handleRelayOffer(t internal.FrameType, length int, buf []byte) error {
	var err error
	remoteKey := wgtypes.Key(buf[5:37])
	dstKey := wgtypes.Key(buf[37:69])

	klog.Infof("remoteKey: %v, dstKey: %v", remoteKey.String(), dstKey.String())
	content := buf[69:length]

	offerAnswer, err := probe.UnmarshalOffer(content)
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
			SrcKey:       dstKey,
		})
		rc.SetProber(prober)
		prober.SetRelayChecker(rc)
	}

	return prober.HandleOffer(offerAnswer)
}

func (c *Client) handleRelayOfferResponse(ft internal.FrameType, length int, buf []byte) error {
	var err error
	remoteKey := wgtypes.Key(buf[5:37])
	srcKey := wgtypes.Key(buf[37:69])

	klog.Infof("handle remoteKey: %v, srcKey: %v", remoteKey.String(), srcKey.String())
	content := buf[69:length]

	offerAnswer, err := probe.UnmarshalOffer(content)
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
