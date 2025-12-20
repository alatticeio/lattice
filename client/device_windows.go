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

//go:build windows
// +build windows

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
	"sync"
	"sync/atomic"
	"time"
	drp2 "wireflow/drp"
	"wireflow/internal"
	mgtclient "wireflow/management/client"
	grpcclient "wireflow/management/grpc"
	"wireflow/management/vo"
	"wireflow/pkg/config"
	lipc "wireflow/pkg/ipc"
	"wireflow/pkg/log"
	"wireflow/pkg/probe"
	"wireflow/turn"

	wg "golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/ipc"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var (
	once sync.Once
	_    internal.IClient = (*Engine)(nil)
)

const (
	DefaultMTU = 1420
)

// Client is the daemon that manages the wireGuard iface
type Engine struct {
	logger        *log.Logger
	keyManager    internal.KeyManager
	Name          string
	device        *wg.Device
	mgtClient     *mgtclient.Client
	drpClient     *drp2.Client
	bind          *DefaultBind
	GetNetworkMap func() (*vo.NetworkMap, error)
	updated       atomic.Bool

	group atomic.Value //belong to which group

	nodeManager  *internal.PeerManager
	agentManager internal.AgentManagerFactory
	wgConfigure  internal.Configurer
	current      *internal.NodeMessage
	turnManager  *turn.TurnManager

	callback func(message *internal.Message) error

	keepaliveChan chan struct{} // channel for keepalive
	watchChan     chan struct{} // channel for watch
}

type EngineConfig struct {
	Logger        *log.Logger
	Conf          *config.Config
	Port          int
	UdpConn       *net.UDPConn
	InterfaceName string
	client        *mgtclient.Client
	drpClient     *drp2.Client
	WgLogger      *wg.Logger
	TurnServerUrl string
	ForceRelay    bool
	ManagementUrl string
	SignalingUrl  string
	ShowWgLog     bool
}

func (e *Engine) IpcHandle(socket net.Conn) {
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
			os.Exit(0)
		case "set=1\n":
			err = e.device.IpcSetOperation(buffered.Reader)
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
			err = e.device.IpcGetOperation(buffered.Writer)
		default:
			e.logger.Errorf("invalid UAPI operation: %v", op)
			return
		}

		// write status
		var status *lipc.IPCError
		if err != nil && !errors.As(err, &status) {
			// shouldn't happen
			status = lipc.IpcErrorf(ipc.IpcErrorUnknown, "other UAPI error: %w", err)
		}
		if status != nil {
			e.logger.Errorf("%v", status)
			fmt.Fprintf(buffered, "errno=%d\n\n", status.ErrorCode())
		} else {
			fmt.Fprintf(buffered, "errno=0\n\n")
		}
		buffered.Flush()
	}

}

// NewClient create a new Client instance
func NewEngine(cfg *EngineConfig) (*Engine, error) {
	var (
		device       tun.Device
		err          error
		engine       *Engine
		count        int64
		probeManager internal.ProbeManager
		proxy        *drp2.Proxy
		turnClient   *turn.Client
		grpcClient   *grpcclient.Client
	)
	engine = new(Engine)
	engine.logger = cfg.Logger

	engine.turnManager = new(turn.TurnManager)
	once.Do(func() {
		engine.Name, device, err = CreateTUN(DefaultMTU, cfg.Logger)
		engine.keepaliveChan = make(chan struct{}, 1)
		engine.watchChan = make(chan struct{}, 1)
	})

	if err != nil {
		return nil, err
	}

	// init manager
	engine.nodeManager = internal.NewPeerManager()
	engine.agentManager = drp2.NewAgentManager()

	// control-ctrClient
	if grpcClient, err = grpcclient.NewClient(&grpcclient.GrpcConfig{
		Addr:          cfg.ManagementUrl,
		Logger:        log.NewLogger(log.Loglevel, "grpc-mgtclient"),
		KeepaliveChan: engine.keepaliveChan,
		WatchChan:     engine.watchChan}); err != nil {
		return nil, err
	}
	engine.mgtClient = mgtclient.NewClient(&mgtclient.ClientConfig{
		Logger:     log.NewLogger(log.Loglevel, "control-ctrClient"),
		GrpcClient: grpcClient,
		Conf:       cfg.Conf,
	})

	// limit node count
	if engine.current, count, err = engine.mgtClient.Get(context.Background()); err != nil {
		return nil, err
	}

	// TODO
	if count >= 5 {
		return nil, errors.New("your iface count has reached the maximum limit")
	}
	var privateKey string
	var publicKey string
	if engine.current.AppID != cfg.Conf.AppId {
		key, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return nil, err
		}
		privateKey = key.String()
		publicKey = key.PublicKey().String()
		_, err = engine.mgtClient.Register(privateKey, publicKey, cfg.Conf.Token)
		if err != nil {
			engine.logger.Errorf("register failed, with err: %s\n", err.Error())
			return nil, err
		}
		engine.logger.Infof("register to deviceManager success")
	} else {
		privateKey = engine.current.PrivateKey
	}

	//update key
	engine.keyManager = internal.NewKeyManager(privateKey)
	engine.nodeManager.AddPeer(engine.keyManager.GetPublicKey(), engine.current)

	v4conn, _, err := ListenUDP("udp4", uint16(cfg.Port))

	if err != nil {
		return nil, err
	}

	if engine.drpClient, err = drp2.NewClient(&drp2.ClientConfig{Addr: cfg.SignalingUrl, Logger: log.NewLogger(log.Loglevel, "drp-ctrClient")}); err != nil {
		return nil, err
	}
	engine.drpClient = engine.drpClient.KeyManager(engine.keyManager)

	// init stun
	if turnClient, err = turn.NewClient(&turn.ClientConfig{
		ServerUrl: cfg.TurnServerUrl,
		Conf:      cfg.Conf,
		Logger:    log.NewLogger(log.Loglevel, "turnclient"),
	}); err != nil {
		return nil, err
	}

	var info *turn.RelayInfo
	if info, err = turnClient.GetRelayInfo(true); err != nil {
		return nil, err
	}

	engine.logger.Verbosef("get relay info, mapped addr: %v, conn addr: %v", info.MappedAddr, info.RelayConn.LocalAddr())

	engine.turnManager.SetInfo(info)

	universalUdpMuxDefault := engine.agentManager.NewUdpMux(v4conn)

	if proxy, err = drp2.NewProxy(&drp2.ProxyConfig{
		DrpClient: engine.drpClient,
		DrpAddr:   cfg.SignalingUrl,
	}); err != nil {
		return nil, err
	}

	engine.drpClient = engine.drpClient.Proxy(proxy)

	engine.bind = NewBind(&BindConfig{
		Logger:          log.NewLogger(log.Loglevel, "net-bind"),
		UniversalUDPMux: universalUdpMuxDefault,
		V4Conn:          v4conn,
		Proxy:           proxy,
		KeyManager:      engine.keyManager,
		RelayConn:       info.RelayConn,
	})

	probeManager = probe.NewProberManager(cfg.ForceRelay, universalUdpMuxDefault.UDPMuxDefault, universalUdpMuxDefault, engine, cfg.TurnServerUrl)

	offerHandler := drp2.NewOfferHandler(&drp2.OfferHandlerConfig{
		Logger:       log.NewLogger(log.Loglevel, "offer-handler"),
		ProbeManager: probeManager,
		StunUri:      cfg.TurnServerUrl,
		KeyManager:   engine.keyManager,
		NodeManager:  engine.nodeManager,
		Proxy:        proxy,
		TurnManager:  engine.turnManager,
	})

	proxy = proxy.OfferAndProbe(offerHandler, probeManager)

	engine.device = wg.NewDevice(device, engine.bind, cfg.WgLogger)

	wgConfigure := internal.NewConfigurer(&internal.Params{
		Device:       engine.device,
		IfaceName:    engine.Name,
		PeersManager: engine.nodeManager,
	})
	engine.wgConfigure = wgConfigure

	engine.mgtClient = engine.mgtClient.
		SetNodeManager(engine.nodeManager).
		SetProbeManager(probeManager).
		SetKeyManager(engine.keyManager).
		SetEngine(engine).
		SetOfferHandler(offerHandler).
		SetTurnManager(engine.turnManager)
	return engine, err
}

// Start will get networkmap
func (e *Engine) Start() error {
	// start deviceManager, open udp port
	if err := e.device.Up(); err != nil {
		return err
	}
	// GetNetMap peers from control plane first time, then use watch
	networkMap, err := e.GetNetworkMap()
	if err != nil {
		e.logger.Errorf("sync peers failed: %v", err)
	}

	e.logger.Verbosef("get network map: %s", networkMap)

	// config iface
	internal.SetDeviceIP()("add", e.current.Address, e.Name)

	if err = e.DeviceConfigure(&internal.DeviceConfig{
		PrivateKey: e.keyManager.GetKey(),
	}); err != nil {
		return err
	}

	for _, node := range networkMap.Nodes {
		e.nodeManager.AddPeer(node.PublicKey, node)
	}
	// watch
	go func() {
		if err := e.mgtClient.Watch(context.Background(), e.mgtClient.HandleWatchMessage); err != nil {
			e.logger.Errorf("watch failed: %v", err)
			time.Sleep(10 * time.Second) // retry after 10 seconds
		}
	}()

	go func() {
		if err := e.mgtClient.Keepalive(context.Background()); err != nil {
			e.logger.Errorf("keepalive failed: %v", err)
		} else {
			e.logger.Infof("mgt ctrClient keepliving...")
		}
	}()

	return nil
}

func (e *Engine) Stop() error {
	e.device.Close()
	return nil
}

// ConfigSet updates the configuration of the given interface.
func (e *Engine) SetConfig(conf *internal.DeviceConf) error {
	nowConf, err := e.device.IpcGet()
	if err != nil {
		return err
	}

	if conf.String() == nowConf {
		e.logger.Infof("config is same, no need to update")
		return nil
	}

	reader := strings.NewReader(conf.String())

	return e.device.IpcSetOperation(reader)
}

func (e *Engine) DeviceConfigure(conf *internal.DeviceConfig) error {
	return e.device.IpcSet(conf.String())
}

func (e *Engine) AddPeer(peer internal.NodeMessage) error {
	return e.device.IpcSet(peer.Node())
}

// RemovePeer add remove=true
func (e *Engine) RemovePeer(peer internal.NodeMessage) error {
	peer.Remove = true
	return e.device.IpcSet(peer.Node())
}

func (e *Engine) close() {
	close(e.keepaliveChan)
	e.drpClient.Close()
	//deviceManager.iface.Close()
	e.logger.Verbosef("deviceManager closed")
}

func (e *Engine) GetWgConfiger() internal.Configurer {
	return e.wgConfigure
}
