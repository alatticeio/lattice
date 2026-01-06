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
	"wireflow/internal/core/infra"
	"wireflow/internal/log"
	"wireflow/internal/wferrors"
	ctrclient "wireflow/management/client"
	mgtclient "wireflow/management/client"
	"wireflow/management/nats"
	"wireflow/management/transport"

	wg "golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/ipc"
	"golang.zx2c4.com/wireguard/tun"
)

var (
	_ infra.Client = (*Client)(nil)
)

// Client act as wireflow data plane, wrappers around wireguard device
type Client struct {
	logger      *log.Logger
	Name        string
	iface       *wg.Device
	bind        *DefaultBind
	provisioner infra.Provisioner

	GetNetworkMap func() (*infra.Message, error)
	ctrClient     *ctrclient.Client

	manager struct {
		keyManager  infra.KeyManager
		turnManager *internal.TurnManager
		peerManager *infra.PeerManager
	}

	current *infra.Peer

	callback     func(message *infra.Message) error
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
	client.manager.peerManager = infra.NewPeerManager()
	client.logger = cfg.Logger
	client.manager.turnManager = new(internal.TurnManager)
	client.Name, iface, err = CreateTUN(infra.DefaultMTU, cfg.Logger)
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

	natsSignalService, err := nats.NewNatsService(config.GlobalConfig.SignalUrl)
	if err != nil {
		return nil, err
	}

	factory := transport.NewTransportFactory(natsSignalService, universalUdpMuxDefault)

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
	client.manager.keyManager = infra.NewKeyManager(privateKey)

	factory.Configure(transport.WithPeerManager(client.manager.peerManager), transport.WithKeyManager(client.manager.keyManager))

	localId := client.manager.keyManager.GetPublicKey()
	probeFactory := transport.NewProbeFactory(&transport.ProbeFactoryConfig{
		Factory: factory,
		LocalId: localId,
		Signal:  natsSignalService,
	})

	//subscribe
	natsSignalService.Subscribe(fmt.Sprintf("%s.%s", "wireflow.signals.peers", localId), probeFactory.HandleSignal)

	client.ctrClient.Configure(
		ctrclient.WithSignalHandler(natsSignalService),
		ctrclient.WithKeyManager(client.manager.keyManager),
		ctrclient.WithProbeFactory(probeFactory))

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
		Logger:          cfg.Logger,
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

	client.provisioner = infra.NewProvisioner(infra.NewRouteProvisioner(cfg.Logger),
		infra.NewRuleProvisioner(cfg.Logger), &infra.Params{
			Device:    client.iface,
			IfaceName: client.Name,
		})

	// set configurer
	factory.Configure(transport.WithProvisioner(client.provisioner))

	return client, err
}

// Start will get networkmap
func (c *Client) Start() error {
	ctx := context.Background()
	// init event handler
	c.eventHandler = NewEventHandler(c, log.NewLogger(log.Loglevel, "event-handler"), c.provisioner)
	// start deviceManager, open udp port
	if err := c.iface.Up(); err != nil {
		return err
	}

	if c.current.Address != nil {
		// 设置Device
		if err := c.provisioner.ApplyIP("add", *c.current.Address, c.provisioner.GetIfaceName()); err != nil {
			return err
		}
	}

	if c.manager.keyManager.GetKey() != "" {
		if err := c.provisioner.SetupInterface(&infra.DeviceConfig{
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
func (c *Client) SetConfig(conf *infra.DeviceConf) error {
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

func (c *Client) AddPeer(peer *infra.Peer) error {
	c.manager.peerManager.AddPeer(peer.PublicKey, peer)
	return c.ctrClient.AddPeer(peer)
}

func (c *Client) Configure(peerId string) error {
	//conf *infra.DeviceConfig
	peer := c.manager.peerManager.GetPeer(peerId)
	if peer == nil {
		return errors.New("peer not found")
	}

	conf := &infra.DeviceConfig{
		PrivateKey: peer.PrivateKey,
	}
	return c.provisioner.SetupInterface(conf)
}

func (c *Client) RemovePeer(peer *infra.Peer) error {
	return c.provisioner.RemovePeer(&infra.SetPeer{
		Remove:    true,
		PublicKey: peer.PublicKey,
	})
}

func (c *Client) RemoveAllPeers() {
	c.provisioner.RemoveAllPeers()
}

func (c *Client) GetDeviceName() string {
	return c.Name
}
