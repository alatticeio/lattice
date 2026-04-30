// Copyright 2026 The Lattice Authors, Inc.
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

//go:build pro

package relay

import (
	"net"

	"github.com/alatticeio/lattice/internal"
	"github.com/alatticeio/lattice/internal/agent/log"

	"github.com/pion/logging"
	"github.com/pion/turn/v4"
)

var (
	_ internal.Client = (*Client)(nil)
)

// Client is a TURN client.
type Client struct {
	logger     *log.Logger
	turnClient *turn.Client
	relayConn  net.PacketConn
	relayInfo  *internal.RelayInfo
}

// ClientConfig is the configuration for a TURN client.
type ClientConfig struct {
	Logger    *log.Logger
	ServerUrl string
	Username  string
	Password  string
}

// NewClient creates a new TURN client and starts listening immediately.
func NewClient(cfg *ClientConfig) (internal.Client, error) {
	conn, err := net.Dial("udp", cfg.ServerUrl)
	if err != nil {
		return nil, err
	}

	turnCfg := &turn.ClientConfig{
		STUNServerAddr: cfg.ServerUrl,
		TURNServerAddr: cfg.ServerUrl,
		Conn:           turn.NewSTUNConn(conn),
		Username:       cfg.Username,
		Password:       cfg.Password,
		Realm:          realm,
		LoggerFactory:  logging.NewDefaultLoggerFactory(),
	}

	client, err := turn.NewClient(turnCfg)
	if err != nil {
		return nil, err
	}

	if err = client.Listen(); err != nil {
		client.Close()
		return nil, err
	}

	return &Client{turnClient: client, logger: cfg.Logger}, nil
}

// GetRelayInfo returns the relay info, allocating a relay socket if allocated is true.
func (c *Client) GetRelayInfo(allocated bool) (*internal.RelayInfo, error) {
	if c.relayInfo != nil {
		return c.relayInfo, nil
	}

	c.relayInfo = &internal.RelayInfo{}

	if allocated {
		relayConn, err := c.turnClient.Allocate()
		if err != nil {
			return nil, err
		}
		c.relayConn = relayConn
		c.relayInfo.RelayConn = relayConn
	}

	mappedAddr, err := c.turnClient.SendBindingRequest()
	if err != nil {
		return nil, err
	}

	c.logger.Info("get from turn", "relayed-info", mappedAddr.String())

	mapAddr, _ := internal.AddrToUdpAddr(mappedAddr)
	c.relayInfo.MappedAddr = *mapAddr

	return c.relayInfo, nil
}

// Close releases the relay connection and the underlying TURN client.
func (c *Client) Close() {
	if c.relayConn != nil {
		c.relayConn.Close() //nolint:errcheck
	}
	c.turnClient.Close()
}

// ReadFrom reads a packet from the relay connection.
func (c *Client) ReadFrom(buf []byte) (int, net.Addr, error) {
	return c.relayConn.ReadFrom(buf)
}

// CreatePermission creates a permission for the given addresses.
func (c *Client) CreatePermission(addr ...net.Addr) error {
	return c.turnClient.CreatePermission(addr...)
}
