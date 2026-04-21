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
	"time"
	"wireflow/internal/config"
	"wireflow/internal/infra"
	"wireflow/internal/log"
	"wireflow/management/dto"
	"wireflow/management/transport"
	"wireflow/pkg/utils"
)

var (
	_ infra.ManagementClient = (*Client)(nil)
)

type Client struct {
	logger          *log.Logger
	nats            infra.SignalService
	getKeyManager   func() infra.KeyManager
	getProbeFactory func() *transport.ProbeFactory
}

// ClientConfig holds the dependencies for NewClient. GetKeyManager and
// GetProbeFactory are closures resolved lazily at call time, allowing them
// to be constructed after the Client itself without a two-phase Configure().
type ClientConfig struct {
	Nats            infra.SignalService
	GetKeyManager   func() infra.KeyManager
	GetProbeFactory func() *transport.ProbeFactory
}

func NewClient(cfg *ClientConfig) (*Client, error) {
	return &Client{
		logger:          log.GetLogger("ctrl-client"),
		nats:            cfg.Nats,
		getKeyManager:   cfg.GetKeyManager,
		getProbeFactory: cfg.GetProbeFactory,
	}, nil
}

func (c *Client) GetNetMap(token string) (*infra.Message, error) {
	ctx := context.Background()
	var err error

	if token == "" {
		token = config.Conf.Token
	}
	request := &dto.PeerDto{
		AppID:     config.Conf.AppId,
		PublicKey: c.getKeyManager().GetPublicKey().String(),
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
	if token == "" {
		return nil, fmt.Errorf("token is empty")
	}
	var err error

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	hostname, err := os.Hostname()
	if err != nil {
		c.logger.Error("get hostname failed", err)
		return nil, err
	}

	registryRequest := &dto.PeerDto{
		Name:                config.Conf.AppId,
		Hostname:            hostname,
		InterfaceName:       interfaceName,
		Platform:            runtime.GOOS,
		AppID:               config.Conf.AppId,
		PersistentKeepalive: 25,
		Port:                config.Conf.WgPort,
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

	//remoteId := p.PublicKey
	//
	//onClose := func(remoteId string) error {
	//	c.probeFactory.Remove(remoteId)
	//	c.logger.Info("remote prober for peer", "peerId", remoteId)
	//	return nil
	//}

	key, err := utils.ParseKey(p.PublicKey)
	if err != nil {
		return err
	}
	peerIdentity := infra.NewPeerIdentity(p.AppID, key)

	probe, err = c.getProbeFactory().Get(peerIdentity)
	if err != nil {
		return err
	}
	return probe.Start(context.Background(), peerIdentity)
}
