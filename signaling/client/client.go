package client

import (
	"bufio"
	"fmt"
	"k8s.io/klog/v2"
	"linkany/internal"
	"linkany/pkg/drp"
	"linkany/pkg/probe"
	"linkany/turn/client"
	"net"
	"net/http"
)

// Client http client will use to connect drp server, Upgrade drp protocol
type Client struct {
	node       *drp.Node // drp server
	manager    *internal.AgentManager
	probers    *probe.NetProber
	turnClient *client.Client
}

func NewClient(node *drp.Node, manager *internal.AgentManager, probers *probe.NetProber, turnClient *client.Client) *Client {
	return &Client{
		node:       node,
		manager:    manager,
		probers:    probers,
		turnClient: turnClient,
	}
}

func (c *Client) Connect(url string) (*drp.Client, error) {
	return c.connect(url)
}

// connect connect use http,then upgrade drp protocol
func (c *Client) connect(url string) (*drp.Client, error) {
	var err error
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Upgrade", "drp")
	req.Header.Set("Connection", "Upgrade")

	//TODO impl ipv6 logic
	var conn net.Conn
	conn, err = net.Dial("tcp", c.node.IpV4Addr.String())
	if err != nil {
		return nil, fmt.Errorf("dial to drp %s failed: %v", c.node.IpV4Addr.String(), err)
	}
	// upgrade drp

	brw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	//use brw write
	if err = req.Write(brw); err != nil {
		return nil, err
	}

	if err = brw.Flush(); err != nil {
		return nil, err
	}

	resp, err := http.ReadResponse(brw.Reader, req)
	if err != nil {
		return nil, err
	}

	klog.Infof("statusCode: %d", resp.StatusCode)

	if resp.StatusCode != http.StatusSwitchingProtocols {
		return nil, fmt.Errorf("unexpected code, can not switch drp protocol: %d", resp.StatusCode)
	}

	// upgrade success
	klog.Infof("drp protocol upgrade success")

	client := drp.NewClient(&drp.ClientConfig{
		Brw:          brw,
		Conn:         conn,
		Node:         c.node,
		AgentManager: c.manager,
		Probers:      c.probers,
	})
	return client, err
}
