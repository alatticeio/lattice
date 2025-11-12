// Copyright 2025 Wireflow.io, Inc.
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

package turn

import (
	"net"
	"sync"
	configlocal "wireflow/pkg/config"
	"wireflow/pkg/log"
	turnclient "wireflow/pkg/turn"

	"github.com/pion/logging"
	"github.com/pion/turn/v4"
)

var (
	_ turnclient.Client = (*Client)(nil)
)

type Client struct {
	logger     *log.Logger
	lock       sync.Mutex
	realm      string
	conf       *configlocal.LocalConfig
	turnClient *turn.Client
	relayConn  net.PacketConn
	mappedAddr net.Addr
	relayInfo  *turnclient.RelayInfo
}

type ClientConfig struct {
	Logger    *log.Logger
	ServerUrl string // stun.wireflow.io:3478
	Realm     string
	Conf      *configlocal.LocalConfig
}

func NewClient(cfg *ClientConfig) (turnclient.Client, error) {
	//Dial TURN Server
	conn, err := net.Dial("udp", cfg.ServerUrl)
	if err != nil {
		return nil, err
	}
	var username, password string
	username, password, err = configlocal.DecodeAuth(cfg.Conf.Auth)
	if err != nil {
		return nil, err
	}

	turnCfg := &turn.ClientConfig{
		STUNServerAddr: cfg.ServerUrl,
		TURNServerAddr: cfg.ServerUrl,
		Conn:           turn.NewSTUNConn(conn),
		Username:       username,
		Password:       password,
		Realm:          "wireflow.io",
		LoggerFactory:  logging.NewDefaultLoggerFactory(),
	}

	client, err := turn.NewClient(turnCfg)
	if err != nil {
		return nil, err
	}

	c := &Client{realm: turnCfg.Realm, conf: cfg.Conf, turnClient: client, logger: cfg.Logger}
	return c, nil
}

func (c *Client) GetRelayInfo(allocated bool) (*turnclient.RelayInfo, error) {

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
	// Push BindingRequest to learn our external IP
	c.relayInfo = &turnclient.RelayInfo{}
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

	c.logger.Verbosef("get from turn relayed-address=%s", mappedAddr.String())

	mapAddr, _ := turnclient.AddrToUdpAddr(mappedAddr)
	c.relayInfo.MappedAddr = *mapAddr

	return c.relayInfo, nil
}

func (c *Client) punchHole() error {
	// Push BindingRequest to learn our external IP
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
