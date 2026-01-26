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
// +build !windows

// agent for wireflow
package agent

import (
	"context"
	"fmt"
	"net"
	"strings"
	"wireflow/internal"
	"wireflow/internal/config"
	"wireflow/internal/infra"
	"wireflow/internal/log"
	ctrclient "wireflow/management/client"
	"wireflow/management/nats"
	"wireflow/management/transport"
	"wireflow/pkg/utils"
	"wireflow/wrrper"

	wg "golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var (
	_ infra.AgentInterface = (*Agent)(nil)
)

// Agent act as wireflow data plane, wrappers around wireguard device
type Agent struct {
	logger      *log.Logger
	Name        string
	iface       *wg.Device
	bind        *infra.DefaultBind
	provisioner infra.Provisioner

	GetNetworkMap func() (*infra.Message, error)
	ctrClient     *ctrclient.Client

	manager struct {
		keyManager  infra.KeyManager
		turnManager *internal.TurnManager
		peerManager *infra.PeerManager
	}

	peerStore *infra.PeerStore

	current *infra.Peer

	callback       func(message *infra.Message) error
	messageHandler Handler

	DeviceManager *DeviceManager
}

// AgentConfig agent config.
type AgentConfig struct {
	Logger        *log.Logger
	Port          int
	InterfaceName string
	WgLogger      *wg.Logger
	ForceRelay    bool
	ShowLog       bool
	Token         string
	Flags         *config.Flags
}

// NewAgent create a new Agent instance.
func NewAgent(ctx context.Context, cfg *AgentConfig) (*Agent, error) {
	var (
		iface      tun.Device
		err        error
		agent      *Agent
		v4conn     *net.UDPConn
		v6conn     *net.UDPConn
		wrrp       *wrrper.WRRPClient
		privateKey wgtypes.Key
	)
	agent = new(Agent)
	agent.manager.peerManager = infra.NewPeerManager()
	agent.logger = cfg.Logger
	agent.manager.turnManager = new(internal.TurnManager)
	agent.Name, iface, err = infra.CreateTUN(infra.DefaultMTU, cfg.Logger)
	if err != nil {
		return nil, err
	}

	if v4conn, _, err = infra.ListenUDP("udp4", uint16(cfg.Port)); err != nil {
		return nil, err
	}

	if v6conn, _, err = infra.ListenUDP("udp6", uint16(cfg.Port)); err != nil {
		return nil, err
	}

	universalUdpMuxDefault := infra.NewUdpMux(v4conn, cfg.ShowLog)

	natsSignalService, err := nats.NewNatsService(ctx, config.Conf.SignalingURL)
	if err != nil {
		return nil, err
	}

	agent.ctrClient, err = ctrclient.NewClient(natsSignalService)
	if err != nil {
		return nil, err
	}

	agent.current, err = agent.ctrClient.Register(ctx, cfg.Token, agent.Name)
	if err != nil {
		return nil, err
	}

	privateKey, err = utils.ParseKey(agent.current.PrivateKey)
	if err != nil {
		return nil, err
	}
	agent.manager.keyManager = infra.NewKeyManager(privateKey)

	localId := infra.FromKey(agent.manager.keyManager.GetPublicKey())

	// add self to peerManager
	agent.manager.peerManager.AddPeer(localId.ToUint64(), agent.current)
	agent.peerStore = infra.NewPeerStore()
	agent.peerStore.AddPeer(privateKey.PublicKey())

	probeFactory := transport.NewProbeFactory(&transport.ProbeFactoryConfig{
		LocalId:                localId,
		Signal:                 natsSignalService,
		PeerManager:            agent.manager.peerManager,
		PeerStore:              agent.peerStore,
		UniversalUdpMuxDefault: universalUdpMuxDefault,
		Provisioner:            agent.provisioner,
	})

	//subscribe
	if err = natsSignalService.Subscribe(fmt.Sprintf("%s.%s", "wireflow.signals.peers", localId), probeFactory.Handle); err != nil {
		return nil, err
	}

	agent.ctrClient.Configure(
		ctrclient.WithSignalHandler(natsSignalService),
		ctrclient.WithKeyManager(agent.manager.keyManager),
		ctrclient.WithProbeFactory(probeFactory))

	if cfg.Flags.EnableWrrp {
		wrrpUrl := cfg.Flags.WrrperURL
		if wrrpUrl == "" {
			wrrpUrl = agent.current.WrrpUrl
		}

		if wrrpUrl != "" {
			wrrp, err = wrrper.NewWrrpClient(localId, wrrpUrl)
			if err != nil {
				return nil, err
			}

			wrrp.Configure(wrrper.WithOnMessage(probeFactory.Handle))
		}
	}

	agent.bind = infra.NewBind(&infra.BindConfig{
		Logger:          cfg.Logger,
		UniversalUDPMux: universalUdpMuxDefault,
		V4Conn:          v4conn,
		V6Conn:          v6conn,
		WrrpClient:      wrrp,
		KeyManager:      agent.manager.keyManager,
	})

	agent.iface = wg.NewDevice(iface, agent.bind, cfg.WgLogger)

	agent.provisioner = infra.NewProvisioner(infra.NewRouteProvisioner(cfg.Logger),
		infra.NewRuleProvisioner(cfg.Logger), &infra.Params{
			Device:    agent.iface,
			IfaceName: agent.Name,
		})
	// init event handler
	agent.messageHandler = NewMessageHandler(agent, log.GetLogger("event-handler"), agent.provisioner)
	probeFactory.Configure(transport.WithOnMessage(agent.messageHandler.HandleEvent), transport.WithWrrp(wrrp), transport.WithProvisioner(agent.provisioner))

	agent.DeviceManager = NewDeviceManager(log.GetLogger("device-manager"), agent.iface)
	return agent, err
}

// Start will get networkmap
func (c *Agent) Start(ctx context.Context) error {
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

	if err := c.provisioner.SetupInterface(&infra.DeviceConfig{
		PrivateKey: c.current.PrivateKey,
	}); err != nil {
		return err
	}

	// get network map
	remoteCfg, err := c.GetNetworkMap()
	if err != nil {
		return err
	}

	c.messageHandler.ApplyFullConfig(ctx, remoteCfg)

	return nil
}

func (c *Agent) Stop() error {
	c.iface.Close()
	return nil
}

// SetConfig updates the configuration of the given interface.
func (c *Agent) SetConfig(conf *infra.DeviceConf) error {
	nowConf, err := c.iface.IpcGet()
	if err != nil {
		return err
	}

	if conf.String() == nowConf {
		c.logger.Debug("config is same, no need to update", "conf", conf)
		return nil
	}

	reader := strings.NewReader(conf.String())

	return c.iface.IpcSetOperation(reader)
}

func (c *Agent) close() {
	c.logger.Debug("deviceManager closed")
}

func (c *Agent) AddPeer(peer *infra.Peer) error {
	publicKey, err := utils.ParseKey(peer.PublicKey)
	if err != nil {
		return err
	}
	remoteId := infra.FromKey(publicKey)
	c.manager.peerManager.AddPeer(remoteId.ToUint64(), peer)
	c.peerStore.AddPeer(publicKey)
	if peer.PublicKey == c.current.PublicKey {
		return nil
	}
	return c.ctrClient.AddPeer(peer)
}

//func (c *Agent) Configure(peerId string) error {
//	//conf *infra.DeviceConfig
//	peer := c.manager.peerManager.GetPeer(peerId.ToUint64())
//	if peer == nil {
//		return errors.New("peer not found")
//	}
//
//	conf := &infra.DeviceConfig{
//		PrivateKey: peer.PrivateKey,
//	}
//	return c.provisioner.SetupInterface(conf)
//}

func (c *Agent) RemovePeer(peer *infra.Peer) error {
	return c.provisioner.RemovePeer(&infra.SetPeer{
		Remove:    true,
		PublicKey: peer.PublicKey,
	})
}

func (c *Agent) RemoveAllPeers() {
	c.provisioner.RemoveAllPeers()
}

func (c *Agent) GetDeviceName() string {
	return c.Name
}
