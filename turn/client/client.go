package client

import (
	"github.com/pion/logging"
	"github.com/pion/turn/v4"
	"k8s.io/klog/v2"
	"linkany/pkg/config"
	"net"
	"sync"
)

type Client struct {
	lock       sync.Mutex
	realm      string
	conf       *config.LocalConfig
	turnClient *turn.Client
	relayConn  net.PacketConn
	mappedAddr net.Addr
	relayInfo  *RelayInfo
}

type RelayInfo struct {
	MappedAddr net.UDPAddr
	RelayConn  net.PacketConn
}

type ClientConfig struct {
	ServerUrl string // stun.linkany.io:3478
	Realm     string
	Conf      *config.LocalConfig
}

func NewClient(config *ClientConfig) (*Client, error) {
	//Dial TURN Server
	conn, err := net.Dial("udp", config.ServerUrl)
	if err != nil {
		return nil, err
	}
	/*addr, err := net.ResolveUDPAddr("udp", config.ServerUrl)
	if err != nil {
		panic(err)
	}
	conn, err := net.DialUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 51820}, addr)
	if err != nil {
		panic(err)
	}*/

	username := "linkany"
	password := "123456"
	cfg := &turn.ClientConfig{
		STUNServerAddr: config.ServerUrl,
		TURNServerAddr: config.ServerUrl,
		Conn:           turn.NewSTUNConn(conn),
		Username:       username,
		Password:       password,
		Realm:          "linkany.io",
		LoggerFactory:  logging.NewDefaultLoggerFactory(),
	}

	client, err := turn.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	c := &Client{realm: cfg.Realm, conf: config.Conf, turnClient: client}
	return c, nil
}

func (c *Client) GetRelayInfo(allocated bool) (*RelayInfo, error) {

	if c.relayInfo != nil {
		return c.relayInfo, nil
	}
	var err error
	err = c.turnClient.Listen()
	if err != nil {
		return nil, err
	}

	// Allocate a relay socket on the TURN server. On success, it
	// will return a net.PacketConn which represents the remote
	// socket.
	// Send BindingRequest to learn our external IP
	c.relayInfo = &RelayInfo{}
	if allocated {
		relayConn, err := c.turnClient.Allocate()
		if err != nil {
			return nil, err
		}

		c.relayInfo.RelayConn = relayConn
	}

	mappedAddr, err := c.turnClient.SendBindingRequest()
	if err != nil {
		return nil, err
	}

	klog.Infof("get from turn relayed-address=%s", mappedAddr.String())

	mapAddr, _ := AddrToUdpAddr(mappedAddr)
	c.relayInfo.MappedAddr = *mapAddr

	return c.relayInfo, nil
}

func (c *Client) punchHole() error {
	// Send BindingRequest to learn our external IP
	mappedAddr, err := c.turnClient.SendBindingRequest()
	if err != nil {
		return err
	}

	// Punch a UDP hole for the relayConn by sending a data to the mappedAddr.
	// This will trigger a TURN client to generate a permission request to the
	// TURN server. After this, packets from the IP address will be accepted by
	// the TURN server.
	_, err = c.relayConn.WriteTo([]byte("Hello"), mappedAddr)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) Close() {
	c.relayConn.Close()
}

func (c *Client) ReadFrom(buf []byte) (int, net.Addr, error) {
	return c.relayConn.ReadFrom(buf)
}

// CreatePermission creates a permission for the given addresses
func (c *Client) CreatePermission(addr ...net.Addr) error {
	return c.turnClient.CreatePermission(addr...)
}

func AddrToUdpAddr(addr net.Addr) (*net.UDPAddr, error) {
	result, err := net.ResolveUDPAddr("udp", addr.String())
	if err != nil {
		return nil, err
	}

	return result, nil
}
