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
	"wireflow/internal"
	"wireflow/internal/config"
	"wireflow/internal/core/domain"
	"wireflow/internal/core/infra"
	"wireflow/internal/core/manager"
	"wireflow/internal/log"
	"wireflow/internal/wferrors"
	ctrclient "wireflow/management/client"
	mgtclient "wireflow/management/client"
	"wireflow/management/nats"
	"wireflow/management/probe"

	wg "golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/ipc"
	"golang.zx2c4.com/wireguard/tun"
)

var (
	_ domain.Client = (*Client)(nil)
)

// Client act as wireflow data plane, wrappers around wireguard device
type Client struct {
	logger       *log.Logger
	Name         string
	iface        *wg.Device
	bind         *DefaultBind
	routeApplier infra.RouteApplier

	GetNetworkMap func() (*domain.Message, error)
	ctrClient     *ctrclient.Client

	manager struct {
		keyManager  domain.KeyManager
		turnManager *internal.TurnManager
		peerManager *manager.PeerManager
	}

	wgConfigure domain.Configurer
	current     *domain.Peer

	callback     func(message *domain.Message) error
	eventHandler *EventHandler
}

type ClientConfig struct {
	Logger        *log.Logger
	Port          int
	UdpConn       *net.UDPConn
	InterfaceName string
	client        *mgtclient.Client
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
		iface  tun.Device
		err    error
		client *Client
		//turnClient internal.Client
		v4conn *net.UDPConn
		v6conn *net.UDPConn
	)
	client = new(Client)
	client.manager.peerManager = manager.NewPeerManager()
	client.logger = cfg.Logger
	client.manager.turnManager = new(internal.TurnManager)
	client.Name, iface, err = CreateTUN(domain.DefaultMTU, cfg.Logger)
	if err != nil {
		return nil, err
	}

	if v4conn, _, err = ListenUDP("udp4", uint16(cfg.Port)); err != nil {
		return nil, err
	}

	if v6conn, _, err = ListenUDP("udp6", uint16(cfg.Port)); err != nil {
		return nil, err
	}

	universalUdpMuxDefault := infra.NewUdpMux(v4conn)

	client.routeApplier = infra.NewRouteApplier()

	natsSignalService, err := nats.NewNatsService(config.GlobalConfig.SignalUrl)
	if err != nil {
		return nil, err
	}

	factory := probe.NewTransportFactory(natsSignalService, universalUdpMuxDefault)

	client.ctrClient, err = ctrclient.NewClient(natsSignalService, factory)
	if err != nil {
		return nil, err
	}

	var privateKey string
	client.current, err = client.ctrClient.Register(context.Background(), client.Name)
	if err != nil {
		return nil, err
	}

	privateKey = client.current.PrivateKey
	client.manager.keyManager = manager.NewKeyManager(privateKey)

	factory.Configure(probe.WithPeerManager(client.manager.peerManager), probe.WithKeyManager(client.manager.keyManager))

	//subscribe
	natsSignalService.Subscribe(fmt.Sprintf("%s.%s", "wireflow.signals.peers", client.manager.keyManager.GetPublicKey()), factory.HandleSignal)

	client.ctrClient.Configure(ctrclient.WithSignalHandler(natsSignalService), ctrclient.WithKeyManager(client.manager.keyManager))

	// init stun
	//if turnClient, err = turn.NewClient(&turn.ClientConfig{
	//	ServerUrl: cfg.TurnServerUrl,
	//	Logger:    log.NewLogger(log.Loglevel, "turnclient"),
	//}); err != nil {
	//	return nil, err
	//}

	//var info *internal.RelayInfo
	//if info, err = turnClient.GetRelayInfo(true); err != nil {
	//	return nil, err
	//}
	//
	//client.logger.Verbosef("get relay info, mapped addr: %v, conn addr: %v", info.MappedAddr, info.RelayConn.LocalAddr())

	//client.manager.turnManager.SetInfo(info)

	client.bind = NewBind(&BindConfig{
		Logger:          log.NewLogger(log.Loglevel, "wireflow"),
		UniversalUDPMux: universalUdpMuxDefault,
		V4Conn:          v4conn,
		V6Conn:          v6conn,
		KeyManager:      client.manager.keyManager,
		//RelayConn:       info.RelayConn,
	})

	stunUrl := config.GlobalConfig.StunUrl
	if stunUrl == "" {
		stunUrl = "stun.wireflowio.com"
		config.WriteConfig("stun-url", stunUrl)
	}

	client.iface = wg.NewDevice(iface, client.bind, cfg.WgLogger)

	wgConfigure := manager.NewConfigurer(&manager.Params{
		Device:    client.iface,
		IfaceName: client.Name,
	})

	client.wgConfigure = wgConfigure
	// set configurer
	factory.Configure(probe.WithConfigurer(wgConfigure))

	return client, err
}

// Start will get networkmap
func (c *Client) Start() error {
	ctx := context.Background()
	// init event handler
	c.eventHandler = NewEventHandler(c, log.NewLogger(log.Loglevel, "event-handler"), c.ctrClient)
	// start deviceManager, open udp port
	if err := c.iface.Up(); err != nil {
		return err
	}

	if c.current.Address != nil {
		// 设置Device
		if err := c.routeApplier.ApplyIP("add", *c.current.Address, c.wgConfigure.GetIfaceName()); err != nil {
			return err
		}
	}

	if c.manager.keyManager.GetKey() != "" {
		if err := c.wgConfigure.Configure(&domain.DeviceConfig{
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

func (c *Client) close() {
	c.logger.Verbosef("deviceManager closed")
}

func (c *Client) AddPeer(peer *domain.Peer) error {
	c.manager.peerManager.AddPeer(peer.PublicKey, peer)
	return c.ctrClient.AddPeer(peer)
}

func (c *Client) Configure(peerId string) error {
	//conf *domain.DeviceConfig
	peer := c.manager.peerManager.GetPeer(peerId)
	if peer == nil {
		return errors.New("peer not found")
	}

	conf := &domain.DeviceConfig{
		PrivateKey: peer.PrivateKey,
	}
	return c.wgConfigure.Configure(conf)
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

func (c *Client) GetDeviceName() string {
	return c.Name
}
