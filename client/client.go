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

//go:build !windows

package client

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"syscall"
	"time"
	"wireflow/drp"
	"wireflow/internal/core/domain"
	"wireflow/internal/core/infra"
	"wireflow/internal/core/manager"
	ctrclient "wireflow/management/client"
	mgtclient "wireflow/management/grpc/client"
	"wireflow/pkg/config"
	"wireflow/pkg/log"
	"wireflow/pkg/probe"
	turnclient "wireflow/pkg/turn"
	"wireflow/pkg/wferrors"
	"wireflow/turn"

	wg "golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/ipc"
	"golang.zx2c4.com/wireguard/tun"
)

var (
	_ domain.IClient = (*Client)(nil)
)

// Client act as wireflow data plane, wrappers around wireguard device
type Client struct {
	ctx    context.Context
	logger *log.Logger
	Name   string
	iface  *wg.Device
	bind   *DefaultBind

	GetNetworkMap func() (*domain.Message, error)

	clients struct {
		ctrClient     *ctrclient.Client
		drpClient     *drp.Client
		keepaliveChan chan struct{} // channel for keepalive
		watchChan     chan struct{} // channel for watch
	}

	managers struct {
		agentManager domain.AgentManagerFactory
		keyManager   domain.IKeyManager
		turnManager  *turnclient.TurnManager
		peerManager  domain.IPeerManager
	}

	wgConfigure domain.Configurer
	current     *domain.Peer

	callback     func(message *domain.Message) error
	eventHandler *EventHandler
}

type ClientConfig struct {
	Logger        *log.Logger
	Conf          *config.LocalConfig
	Port          int
	UdpConn       *net.UDPConn
	InterfaceName string
	client        *mgtclient.Client
	drpClient     *drp.Client
	WgLogger      *wg.Logger
	TurnServerUrl string
	ForceRelay    bool
	ManagementUrl string
	SignalingUrl  string
	ShowWgLog     bool
}

func (c *Client) IpcHandle(socket net.Conn) {
	defer socket.Close()

	buffered := func(s io.ReadWriter) *bufio.ReadWriter {
		reader := bufio.NewReader(s)
		writer := bufio.NewWriter(s)
		return bufio.NewReadWriter(reader, writer)
	}(socket)
	for {
		op, err := buffered.ReadString('\n')
		if err != nil {
			return
		}

		// handle operation
		switch op {
		case "stop\n":
			buffered.Write([]byte("OK\n\n"))
			// send kill signal
			syscall.Kill(os.Getpid(), syscall.SIGTERM)
		case "set=1\n":
			err = c.iface.IpcSetOperation(buffered.Reader)
		case "get=1\n":
			var nextByte byte
			nextByte, err = buffered.ReadByte()
			if err != nil {
				return
			}
			if nextByte != '\n' {
				err = wferrors.IpcErrorf(ipc.IpcErrorInvalid, "trailing character in UAPI get: %q", nextByte)
				break
			}
			err = c.iface.IpcGetOperation(buffered.Writer)
		default:
			c.logger.Errorf("invalid UAPI operation: %v", op)
			return
		}

		// write status
		var status *wferrors.IPCError
		if err != nil && !errors.As(err, &status) {
			// shouldn't happen
			status = wferrors.IpcErrorf(ipc.IpcErrorUnknown, "other UAPI error: %w", err)
		}
		if status != nil {
			c.logger.Errorf("%v", status)
			fmt.Fprintf(buffered, "errno=%d\n\n", status.ErrorCode())
		} else {
			fmt.Fprintf(buffered, "errno=0\n\n")
		}
		buffered.Flush()
	}

}

// NewClient create a new Client instance
func NewClient(cfg *ClientConfig) (*Client, error) {
	var (
		iface        tun.Device
		err          error
		client       *Client
		probeManager domain.ProbeManager
		proxy        *drp.Proxy
		turnClient   turnclient.Client
		v4conn       *net.UDPConn
		v6conn       *net.UDPConn
	)
	client = &Client{
		ctx:    context.Background(),
		logger: cfg.Logger,
	}
	client.clients.keepaliveChan = make(chan struct{}, 1)
	client.clients.watchChan = make(chan struct{}, 1)
	client.managers.turnManager = new(turnclient.TurnManager)
	client.managers.peerManager = manager.NewPeerManager()
	client.managers.agentManager = manager.NewAgentManagerFactory()
	client.Name, iface, err = CreateTUN(domain.DefaultMTU, cfg.Logger)
	if err != nil {
		return nil, err
	}

	client.clients.ctrClient = ctrclient.NewClient(&ctrclient.ClientConfig{
		Logger:        log.NewLogger(log.Loglevel, "control-ctrClient"),
		ManagementUrl: cfg.ManagementUrl,
		Conf:          cfg.Conf,
	})

	appId, err := config.GetAppId()
	if err != nil {
		return nil, err
	}
	var privateKey string
	client.current, err = client.clients.ctrClient.Register(context.Background(), appId)
	if err != nil {
		return nil, err
	}

	privateKey = client.current.PrivateKey

	//update key
	client.managers.keyManager = manager.NewKeyManager(privateKey)
	client.managers.peerManager.AddPeer(client.managers.keyManager.GetPublicKey(), client.current)

	if v4conn, _, err = ListenUDP("udp4", uint16(cfg.Port)); err != nil {
		return nil, err
	}

	if v6conn, _, err = ListenUDP("udp6", uint16(cfg.Port)); err != nil {
		return nil, err
	}

	// init drp clients
	if client.clients.drpClient, err = drp.NewClient(&drp.ClientConfig{
		Addr:       cfg.SignalingUrl,
		Logger:     log.NewLogger(log.Loglevel, "drp-ctrClient"),
		KeyManager: client.managers.keyManager,
	}); err != nil {
		return nil, err
	}

	// init stun
	if turnClient, err = turn.NewClient(&turn.ClientConfig{
		ServerUrl: cfg.TurnServerUrl,
		Conf:      cfg.Conf,
		Logger:    log.NewLogger(log.Loglevel, "turnclient"),
	}); err != nil {
		return nil, err
	}

	var info *turnclient.RelayInfo
	if info, err = turnClient.GetRelayInfo(true); err != nil {
		return nil, err
	}

	client.logger.Verbosef("get relay info, mapped addr: %v, conn addr: %v", info.MappedAddr, info.RelayConn.LocalAddr())

	client.managers.turnManager.SetInfo(info)

	universalUdpMuxDefault := client.managers.agentManager.NewUdpMux(v4conn)

	client.bind = NewBind(&BindConfig{
		Logger:          log.NewLogger(log.Loglevel, "wireflow-bind"),
		UniversalUDPMux: universalUdpMuxDefault,
		V4Conn:          v4conn,
		V6Conn:          v6conn,
		Proxy:           proxy,
		KeyManager:      client.managers.keyManager,
		RelayConn:       info.RelayConn,
	})

	probeManager = probe.NewManager(cfg.ForceRelay, universalUdpMuxDefault.UDPMuxDefault, universalUdpMuxDefault, client, cfg.TurnServerUrl)

	offerHandler := drp.NewOfferHandler(&drp.OfferHandlerConfig{
		Logger:       log.NewLogger(log.Loglevel, "offer-handler"),
		ProbeManager: probeManager,
		AgentManager: client.managers.agentManager,
		StunUri:      cfg.TurnServerUrl,
		KeyManager:   client.managers.keyManager,
		NodeManager:  client.managers.peerManager,
		Proxy:        proxy,
		TurnManager:  client.managers.turnManager,
	})

	// init proxy in drp clients
	client.clients.drpClient.Configure(
		drp.WithProbeManager(probeManager),
		drp.WithOfferHandler(offerHandler))

	client.iface = wg.NewDevice(iface, client.bind, cfg.WgLogger)

	wgConfigure := manager.NewConfigurer(&manager.Params{
		Device:       client.iface,
		IfaceName:    client.Name,
		PeersManager: client.managers.peerManager,
	})
	client.wgConfigure = wgConfigure

	// init control clients
	client.clients.ctrClient.Configure(
		ctrclient.WithNodeManager(client.managers.peerManager),
		ctrclient.WithProbeManager(probeManager),
		ctrclient.WithTurnManager(client.managers.turnManager),
		ctrclient.WithIClient(client),
		ctrclient.WithOfferHandler(offerHandler),
		ctrclient.WithKeyManager(client.managers.keyManager))
	return client, err
}

// Start will get networkmap
func (c *Client) Start() error {
	ctx := context.Background()
	// init event handler
	c.eventHandler = NewEventHandler(c, log.NewLogger(log.Loglevel, "event-handler"), c.clients.ctrClient)
	// start deviceManager, open udp port
	if err := c.iface.Up(); err != nil {
		return err
	}

	if c.current.Address != "" {
		// 设置Device
		infra.SetDeviceIP()("add", c.current.Address, c.wgConfigure.GetIfaceName())
	}

	if c.managers.keyManager.GetKey() != "" {
		if err := c.Configure(&domain.DeviceConfig{
			PrivateKey: c.current.PrivateKey,
		}); err != nil {
			return err
		}
	}

	// get network map
	remoteCfg, err := c.GetNetworkMap()
	if err != nil {
		return err
	}

	c.eventHandler.ApplyFullConfig(ctx, remoteCfg)

	// watch
	go func() {
		c.clients.watchChan <- struct{}{}
		for {
			select {
			case <-c.clients.watchChan:
				if err = c.clients.ctrClient.Watch(c.ctx, c.eventHandler.HandleEvent()); err != nil {
					c.logger.Errorf("watch failed: %v", err)
					time.Sleep(10 * time.Second) // retry after 10 seconds
					c.clients.watchChan <- struct{}{}
				}
			case <-c.ctx.Done():
				c.logger.Infof("watching chan closed")
				return
			}
		}
	}()

	go func() {
		c.clients.keepaliveChan <- struct{}{}
		for {
			select {
			case <-c.clients.keepaliveChan:
				if err = c.clients.ctrClient.Keepalive(c.ctx); err != nil {
					c.logger.Errorf("keepalive failed: %v", err)
					time.Sleep(10 * time.Second)
					c.clients.keepaliveChan <- struct{}{}
				}
			case <-c.ctx.Done():
				return
			}
		}

	}()

	return nil
}

func (c *Client) Stop() error {
	c.iface.Close()
	return nil
}

// SetConfig updates the configuration of the given interface.
func (c *Client) SetConfig(conf *domain.DeviceConf) error {
	nowConf, err := c.iface.IpcGet()
	if err != nil {
		return err
	}

	if conf.String() == nowConf {
		c.logger.Infof("config is same, no need to update")
		return nil
	}

	reader := strings.NewReader(conf.String())

	return c.iface.IpcSetOperation(reader)
}

func (c *Client) Configure(conf *domain.DeviceConfig) error {
	return c.iface.IpcSet(conf.String())
}

func (c *Client) close() {
	close(c.clients.keepaliveChan)
	c.clients.drpClient.Close()
	//deviceManager.iface.Close()
	c.logger.Verbosef("deviceManager closed")
}

func (c *Client) GetDeviceConfiger() domain.Configurer {
	return c.wgConfigure
}

func (c *Client) AddPeer(peer *domain.Peer) error {
	return c.clients.ctrClient.AddPeer(peer)
}

func (c *Client) RemovePeer(peer *domain.Peer) error {
	return c.wgConfigure.RemovePeer(&domain.SetPeer{
		Remove:    true,
		PublicKey: peer.PublicKey,
	})
}

func (c *Client) RemoveAllPeers() {
	c.wgConfigure.RemoveAllPeers()
}
