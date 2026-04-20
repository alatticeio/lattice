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
	natsService infra.SignalService

	GetNetworkMap func() (*infra.Message, error)
	ctrClient    *ctrclient.Client
	probeFactory *transport.ProbeFactory

	manager struct {
		keyManager  infra.KeyManager
		turnManager *internal.TurnManager
		peerManager *infra.PeerManager
	}

	current *infra.Peer

	token          string
	callback       func(message *infra.Message) error // nolint
	messageHandler Handler

	DeviceManager *DeviceManager
}

// AgentConfig agent config.
type AgentConfig struct {
	Logger        *log.Logger
	Port          int
	InterfaceName string
	ForceRelay    bool
	ShowLog       bool
	Token         string
	Flags         *config.Config
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

	natsSignalService, err := nats.NewNatsService(ctx, config.Conf.AppId, "client", config.Conf.SignalingURL)
	if err != nil {
		return nil, err
	}
	agent.natsService = natsSignalService

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

	localIdentity := infra.NewPeerIdentity(agent.current.AppID, privateKey.PublicKey())

	// add self to peerManager
	agent.manager.peerManager.AddPeer(agent.current.AppID, agent.current)

	agent.probeFactory = transport.NewProbeFactory(&transport.ProbeFactoryConfig{
		LocalId:                localIdentity,
		Signal:                 natsSignalService,
		PeerManager:            agent.manager.peerManager,
		UniversalUdpMuxDefault: universalUdpMuxDefault,
		Provisioner:            agent.provisioner,
		ShowLog:                cfg.ShowLog,
	})

	//subscribe
	if err = natsSignalService.Subscribe(fmt.Sprintf("%s.%s", "wireflow.signals.peers", localIdentity), agent.probeFactory.Handle); err != nil {
		return nil, err
	}

	agent.ctrClient.Configure(
		ctrclient.WithSignalHandler(natsSignalService),
		ctrclient.WithKeyManager(agent.manager.keyManager),
		ctrclient.WithProbeFactory(agent.probeFactory))

	if cfg.Flags.EnableWrrp {
		wrrpUrl := cfg.Flags.WrrperURL
		if wrrpUrl == "" {
			wrrpUrl = agent.current.WrrpUrl
		}

		if wrrpUrl != "" {
			wrrp, err = wrrper.NewWrrpClient(localIdentity.ID(), wrrpUrl)
			if err != nil {
				return nil, err
			}

			wrrp.Configure(wrrper.WithOnMessage(agent.probeFactory.Handle))
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

	wgLogLevel := wg.LogLevelError
	if cfg.ShowLog {
		wgLogLevel = wg.LogLevelVerbose
	}
	agent.iface = wg.NewDevice(iface, agent.bind, wg.NewLogger(wgLogLevel, fmt.Sprintf("(%s) ", cfg.InterfaceName)))

	agent.provisioner = infra.NewProvisioner(infra.NewRouteProvisioner(cfg.Logger),
		infra.NewRuleProvisioner(cfg.Logger, agent.Name), &infra.Params{
			Device:    agent.iface,
			IfaceName: agent.Name,
		})
	// init event handler
	agent.messageHandler = NewMessageHandler(agent, log.GetLogger("event-handler"), agent.provisioner)
	agent.probeFactory.Configure(transport.WithOnMessage(agent.messageHandler.HandleEvent), transport.WithWrrp(wrrp), transport.WithProvisioner(agent.provisioner))

	agent.DeviceManager = NewDeviceManager(log.GetLogger("device-manager"), agent.iface, make(chan struct{}))
	agent.token = cfg.Token

	// Re-register and re-apply the network map whenever NATS reconnects.
	// This covers the case where wireflow-aio restarts and loses all agent state.
	// The handler reads GetNetworkMap at call time (not at setup time), so it
	// works even though GetNetworkMap is assigned externally after NewAgent returns.
	natsSignalService.SetReconnectedHandler(func() {
		ctx := context.Background()
		peer, err := agent.ctrClient.Register(ctx, agent.token, agent.Name)
		if err != nil {
			agent.logger.Error("NATS reconnect: re-register failed", err)
			return
		}
		agent.current = peer

		if agent.GetNetworkMap == nil {
			return
		}
		remoteCfg, err := agent.GetNetworkMap()
		if err != nil {
			agent.logger.Error("NATS reconnect: re-fetch network map failed", err)
			return
		}
		if err = agent.messageHandler.ApplyFullConfig(ctx, remoteCfg); err != nil {
			agent.logger.Error("NATS reconnect: re-apply config failed", err)
		}
	})

	return agent, err
}

// Start will get networkmap
func (c *Agent) Start(ctx context.Context) error {
	// start deviceManager, open udp port
	if err := c.iface.Up(); err != nil {
		return err
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

	return c.messageHandler.ApplyFullConfig(ctx, remoteCfg)
}

func (c *Agent) Stop() error {
	// Drain NATS first so the server immediately removes this client's subscriptions,
	// preventing "no responders" on the next restart.
	if c.natsService != nil {
		if err := c.natsService.Close(); err != nil {
			c.logger.Warn("nats drain failed", "err", err)
		}
	}
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

// nolint:unused
func (c *Agent) close() {
	c.logger.Debug("deviceManager closed")
}

func (c *Agent) AddPeer(peer *infra.Peer) error {
	c.manager.peerManager.AddPeer(peer.AppID, peer)
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
	// Close and evict the probe so we stop trying to reconnect to an offline peer.
	// A fresh probe will be created when the management server pushes PeersAdded
	// after the peer comes back online.
	c.probeFactory.Remove(peer.AppID)
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

func (c *Agent) GetPeerManager() *infra.PeerManager {
	return c.manager.peerManager
}
