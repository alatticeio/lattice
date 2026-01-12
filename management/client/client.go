// Copyright 2025 The Wireflow Authors, Inc.
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

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"wireflow/internal/config"
	"wireflow/internal/core/infra"
	"wireflow/internal/log"
	"wireflow/management/dto"
	"wireflow/management/transport"
)

var (
	_ infra.ManagementClient = (*Client)(nil)
)

type Client struct {
	logger       *log.Logger
	natsUrl      string
	nats         infra.SignalService
	keyManager   infra.KeyManager
	factory      *transport.TransportFactory
	probeFactory *transport.ProbeFactory
}

func NewClient(nats infra.SignalService, factory *transport.TransportFactory) (*Client, error) {
	client := &Client{
		logger:  log.GetLogger("ctrl-client"),
		factory: factory,
		nats:    nats,
	}

	return client, nil
}

type ClientOptions func(*Client)

func WithKeyManager(keyManager infra.KeyManager) func(*Client) {
	return func(c *Client) {
		c.keyManager = keyManager
	}
}

func WithSignalHandler(nats infra.SignalService) func(*Client) {
	return func(c *Client) {
		c.nats = nats
	}
}

func WithProbeFactory(probeFactory *transport.ProbeFactory) func(*Client) {
	return func(c *Client) {
		c.probeFactory = probeFactory
	}
}

func (c *Client) Configure(opts ...ClientOptions) {
	for _, opt := range opts {
		opt(c)
	}
}

func (c *Client) GetNetMap(token string) (*infra.Message, error) {
	ctx := context.Background()
	var err error

	if token == "" {
		token = config.GlobalConfig.Token
	}
	request := &dto.PeerDto{
		AppID:     config.GlobalConfig.AppId,
		PublicKey: c.keyManager.GetPublicKey(),
		Token:     token,
	}

	data, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	data, err = c.RequestNats(ctx, "wireflow.signals.peer", "GetNetMap", data)
	if err != nil {
		return nil, err
	}

	var msg infra.Message
	if err = json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}

	return &msg, nil
}

// Register will register device to wireflow center
func (c *Client) Register(ctx context.Context, token, interfaceName string) (*infra.Peer, error) {
	var err error

	hostname, err := os.Hostname()
	if err != nil {
		c.logger.Error("get hostname failed", err)
		return nil, err
	}

	if token == "" {
		token = config.GlobalConfig.Token
	}

	registryRequest := &dto.PeerDto{
		Name:                config.GlobalConfig.AppId,
		Hostname:            hostname,
		InterfaceName:       interfaceName,
		Platform:            runtime.GOOS,
		AppID:               config.GlobalConfig.AppId,
		PersistentKeepalive: 25,
		Port:                51820,
		Token:               token,
	}

	data, err := json.Marshal(registryRequest)
	if err != nil {
		return nil, err
	}

	data, err = c.RequestNats(ctx, "wireflow.signals.peer", "register", data)

	if err != nil {
		return nil, fmt.Errorf("register failed. %v", err)
	}

	var node infra.Peer
	if err = json.Unmarshal(data, &node); err != nil {
		return nil, err
	}

	return &node, nil
}

func (c *Client) RequestNats(ctx context.Context, subject, method string, data []byte) ([]byte, error) {
	data, err := c.nats.Request(ctx, subject, method, data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (c *Client) AddPeer(p *infra.Peer) error {
	var (
		err   error
		probe *transport.Probe
	)

	remoteId := p.PublicKey

	onClose := func(remoteId string) error {
		c.probeFactory.Remove(remoteId)
		c.logger.Info("remote prober for peer", "peerId", remoteId)
		return nil
	}

	c.factory.Configure(transport.WithOnClose(onClose))

	probe, err = c.probeFactory.Get(remoteId)
	if err != nil {
		return err
	}
	return probe.Probe(context.Background(), remoteId)
}
