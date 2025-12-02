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
	"sync/atomic"
	"syscall"
	"time"
	"wireflow/drp"
	"wireflow/internal"
	mgtclient "wireflow/management/client"
	"wireflow/pkg/config"
	lipc "wireflow/pkg/ipc"
	"wireflow/pkg/log"
	"wireflow/pkg/probe"
	turnclient "wireflow/pkg/turn"
	"wireflow/turn"

	wg "golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/ipc"
	"golang.zx2c4.com/wireguard/tun"
)

var (
	_ internal.DeviceManager = (*Device)(nil)
)

const (
	DefaultMTU = 1420
)

// Device is the daemon that manages the wireGuard iface
type Device struct {
	ctx           context.Context
	logger        *log.Logger
	keyManager    internal.KeyManager
	Name          string
	iface         *wg.Device
	mgtClient     *mgtclient.Client
	drpClient     *drp.Client
	bind          *FlowBind
	GetNetworkMap func() (*internal.Message, error)
	updated       atomic.Bool

	group atomic.Value //belong to which group

	nodeManager  *internal.PeerManager
	agentManager internal.AgentManagerFactory
	wgConfigure  internal.Configurer
	current      *internal.Peer
	turnManager  *turnclient.TurnManager

	callback func(message *internal.Message) error

	keepaliveChan chan struct{} // channel for keepalive
	watchChan     chan struct{} // channel for watch

	eventHandler *EventHandler
}

type DeviceConfig struct {
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

func (device *Device) IpcHandle(socket net.Conn) {
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
			err = device.iface.IpcSetOperation(buffered.Reader)
		case "get=1\n":
			var nextByte byte
			nextByte, err = buffered.ReadByte()
			if err != nil {
				return
			}
			if nextByte != '\n' {
				err = lipc.IpcErrorf(ipc.IpcErrorInvalid, "trailing character in UAPI get: %q", nextByte)
				break
			}
			err = device.iface.IpcGetOperation(buffered.Writer)
		default:
			device.logger.Errorf("invalid UAPI operation: %v", op)
			return
		}

		// write status
		var status *lipc.IPCError
		if err != nil && !errors.As(err, &status) {
			// shouldn't happen
			status = lipc.IpcErrorf(ipc.IpcErrorUnknown, "other UAPI error: %w", err)
		}
		if status != nil {
			device.logger.Errorf("%v", status)
			fmt.Fprintf(buffered, "errno=%d\n\n", status.ErrorCode())
		} else {
			fmt.Fprintf(buffered, "errno=0\n\n")
		}
		buffered.Flush()
	}

}

// NewEngine create a new Device instance
func NewEngine(cfg *DeviceConfig) (*Device, error) {
	var (
		iface        tun.Device
		err          error
		device       *Device
		probeManager internal.ProbeManager
		proxy        *drp.Proxy
		turnClient   turnclient.Client
		v4conn       *net.UDPConn
		v6conn       *net.UDPConn
	)
	device = &Device{
		ctx:           context.Background(),
		nodeManager:   internal.NewPeerManager(),
		agentManager:  drp.NewAgentManager(),
		logger:        cfg.Logger,
		keepaliveChan: make(chan struct{}, 1),
		watchChan:     make(chan struct{}, 1),
	}

	device.turnManager = new(turnclient.TurnManager)
	device.Name, iface, err = CreateTUN(DefaultMTU, cfg.Logger)
	if err != nil {
		return nil, err
	}

	device.mgtClient = mgtclient.NewClient(&mgtclient.ClientConfig{
		Logger:        log.NewLogger(log.Loglevel, "control-mgtClient"),
		ManagementUrl: cfg.ManagementUrl,
		KeepaliveChan: device.keepaliveChan,
		WatchChan:     device.watchChan,
		Conf:          cfg.Conf,
	})

	appId, err := config.GetAppId()
	if err != nil {
		return nil, err
	}
	var privateKey string
	device.current, err = device.mgtClient.Register(context.Background(), appId)
	if err != nil {
		return nil, err
	}

	privateKey = device.current.PrivateKey

	//update key
	device.keyManager = internal.NewKeyManager(privateKey)
	device.nodeManager.AddPeer(device.keyManager.GetPublicKey(), device.current)

	if v4conn, _, err = ListenUDP("udp4", uint16(cfg.Port)); err != nil {
		return nil, err
	}

	if v6conn, _, err = ListenUDP("udp6", uint16(cfg.Port)); err != nil {
		return nil, err
	}

	if device.drpClient, err = drp.NewClient(&drp.ClientConfig{Addr: cfg.SignalingUrl, Logger: log.NewLogger(log.Loglevel, "drp-mgtClient")}); err != nil {
		return nil, err
	}
	device.drpClient = device.drpClient.KeyManager(device.keyManager)

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

	device.logger.Verbosef("get relay info, mapped addr: %v, conn addr: %v", info.MappedAddr, info.RelayConn.LocalAddr())

	device.turnManager.SetInfo(info)

	universalUdpMuxDefault := device.agentManager.NewUdpMux(v4conn)

	if proxy, err = drp.NewProxy(&drp.ProxyConfig{
		DrpClient: device.drpClient,
		DrpAddr:   cfg.SignalingUrl,
	}); err != nil {
		return nil, err
	}

	device.drpClient = device.drpClient.Proxy(proxy)

	device.bind = NewBind(&BindConfig{
		Logger:          log.NewLogger(log.Loglevel, "wireflow-bind"),
		UniversalUDPMux: universalUdpMuxDefault,
		V4Conn:          v4conn,
		V6Conn:          v6conn,
		Proxy:           proxy,
		KeyManager:      device.keyManager,
		RelayConn:       info.RelayConn,
	})

	probeManager = probe.NewManager(cfg.ForceRelay, universalUdpMuxDefault.UDPMuxDefault, universalUdpMuxDefault, device, cfg.TurnServerUrl)

	offerHandler := drp.NewOfferHandler(&drp.OfferHandlerConfig{
		Logger:       log.NewLogger(log.Loglevel, "offer-handler"),
		ProbeManager: probeManager,
		AgentManager: device.agentManager,
		StunUri:      cfg.TurnServerUrl,
		KeyManager:   device.keyManager,
		NodeManager:  device.nodeManager,
		Proxy:        proxy,
		TurnManager:  device.turnManager,
	})

	proxy = proxy.OfferAndProbe(offerHandler, probeManager)

	device.iface = wg.NewDevice(iface, device.bind, cfg.WgLogger)

	wgConfigure := internal.NewConfigurer(&internal.Params{
		Device:       device.iface,
		IfaceName:    device.Name,
		PeersManager: device.nodeManager,
	})
	device.wgConfigure = wgConfigure

	device.mgtClient = device.mgtClient.
		SetNodeManager(device.nodeManager).
		SetProbeManager(probeManager).
		SetKeyManager(device.keyManager).
		SetEngine(device).
		SetOfferHandler(offerHandler).
		SetTurnManager(device.turnManager)
	return device, err
}

// Start will get networkmap
func (device *Device) Start() error {
	ctx := context.Background()
	// init event handler
	device.eventHandler = NewEventHandler(device, log.NewLogger(log.Loglevel, "event-handler"), device.mgtClient)
	// start deviceManager, open udp port
	if err := device.iface.Up(); err != nil {
		return err
	}

	if device.current.Address != "" {
		// 设置Device
		internal.SetDeviceIP()("add", device.current.Address, device.wgConfigure.GetIfaceName())
	}

	if device.keyManager.GetKey() != "" {
		if err := device.Configure(&internal.DeviceConfig{
			PrivateKey: device.current.PrivateKey,
		}); err != nil {
			return err
		}
	}

	// get network map
	remoteCfg, err := device.GetNetworkMap()
	if err != nil {
		return err
	}

	device.eventHandler.ApplyFullConfig(ctx, remoteCfg)

	// watch
	go func() {
		device.watchChan <- struct{}{}
		for {
			select {
			case <-device.watchChan:
				if err = device.mgtClient.Watch(device.ctx, device.eventHandler.HandleEvent()); err != nil {
					device.logger.Errorf("watch failed: %v", err)
					time.Sleep(10 * time.Second) // retry after 10 seconds
					device.watchChan <- struct{}{}
				}
			case <-device.ctx.Done():
				device.logger.Infof("watching chan closed")
				return
			}
		}
	}()

	go func() {
		device.keepaliveChan <- struct{}{}
		for {
			select {
			case <-device.keepaliveChan:
				if err = device.mgtClient.Keepalive(device.ctx); err != nil {
					device.logger.Errorf("keepalive failed: %v", err)
					time.Sleep(10 * time.Second)
					device.keepaliveChan <- struct{}{}
				}
			case <-device.ctx.Done():
				return
			}
		}

	}()

	return nil
}

func (device *Device) Stop() error {
	device.iface.Close()
	return nil
}

// SetConfig updates the configuration of the given interface.
func (device *Device) SetConfig(conf *internal.DeviceConf) error {
	nowConf, err := device.iface.IpcGet()
	if err != nil {
		return err
	}

	if conf.String() == nowConf {
		device.logger.Infof("config is same, no need to update")
		return nil
	}

	reader := strings.NewReader(conf.String())

	return device.iface.IpcSetOperation(reader)
}

func (device *Device) Configure(conf *internal.DeviceConfig) error {
	return device.iface.IpcSet(conf.String())
}

func (device *Device) close() {
	close(device.keepaliveChan)
	device.drpClient.Close()
	//deviceManager.iface.Close()
	device.logger.Verbosef("deviceManager closed")
}

func (device *Device) GetDeviceConfiger() internal.Configurer {
	return device.wgConfigure
}

func (device *Device) AddPeer(peer *internal.Peer) error {
	return device.mgtClient.AddPeer(peer)
}

func (device *Device) RemovePeer(peer *internal.Peer) error {
	return device.wgConfigure.RemovePeer(&internal.SetPeer{
		Remove:    true,
		PublicKey: peer.PublicKey,
	})
}

func (device *Device) RemoveAllPeers() {
	device.wgConfigure.RemoveAllPeers()
}
