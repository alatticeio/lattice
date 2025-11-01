//go:build !windows

package node

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	wg "golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/ipc"
	"golang.zx2c4.com/wireguard/tun"
	"io"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	drp2 "wireflow/drp"
	"wireflow/internal"
	mgtclient "wireflow/management/client"
	"wireflow/management/vo"
	"wireflow/pkg/config"
	lipc "wireflow/pkg/ipc"
	"wireflow/pkg/log"
	"wireflow/pkg/probe"
	turnclient "wireflow/pkg/turn"
	"wireflow/pkg/wrapper"
	"wireflow/turn"
)

var (
	_ internal.EngineManager = (*Engine)(nil)
)

const (
	DefaultMTU = 1420
)

// Engine is the daemon that manages the wireGuard device
type Engine struct {
	ctx           context.Context
	logger        *log.Logger
	keyManager    internal.KeyManager
	Name          string
	device        *wg.Device
	mgtClient     *mgtclient.Client
	drpClient     *drp2.Client
	bind          *wrapper.LinkBind
	GetNetworkMap func() (*vo.NetworkMap, error)
	updated       atomic.Bool

	group atomic.Value //belong to which group

	nodeManager  *internal.NodeManager
	agentManager internal.AgentManagerFactory
	wgConfigure  internal.ConfigureManager
	current      *internal.Node
	turnManager  *turnclient.TurnManager

	callback func(message *internal.Message) error

	keepaliveChan chan struct{} // channel for keepalive
	watchChan     chan struct{} // channel for watch

	eventHandler *EventHandler
}

type EngineConfig struct {
	Logger        *log.Logger
	Conf          *config.LocalConfig
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
			syscall.Kill(os.Getpid(), syscall.SIGTERM)
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

// NewEngine create a new Engine instance
func NewEngine(cfg *EngineConfig) (*Engine, error) {
	var (
		device       tun.Device
		err          error
		engine       *Engine
		probeManager internal.ProbeManager
		proxy        *drp2.Proxy
		turnClient   turnclient.Client
		v4conn       *net.UDPConn
		v6conn       *net.UDPConn
	)
	engine = &Engine{
		ctx:           context.Background(),
		nodeManager:   internal.NewNodeManager(),
		agentManager:  drp2.NewAgentManager(),
		logger:        cfg.Logger,
		keepaliveChan: make(chan struct{}, 1),
		watchChan:     make(chan struct{}, 1),
	}

	engine.turnManager = new(turnclient.TurnManager)
	engine.Name, device, err = internal.CreateTUN(DefaultMTU, cfg.Logger)
	if err != nil {
		return nil, err
	}

	engine.mgtClient = mgtclient.NewClient(&mgtclient.ClientConfig{
		Logger:        log.NewLogger(log.Loglevel, "control-mgtClient"),
		ManagementUrl: cfg.ManagementUrl,
		KeepaliveChan: engine.keepaliveChan,
		WatchChan:     engine.watchChan,
		Conf:          cfg.Conf,
	})

	appId, err := config.GetAppId()
	if err != nil {
		return nil, err
	}
	var privateKey string
	engine.current, err = engine.mgtClient.Register(context.Background(), appId)
	if err != nil {
		return nil, err
	}

	privateKey = engine.current.PrivateKey

	//update key
	engine.keyManager = internal.NewKeyManager(privateKey)
	engine.nodeManager.AddPeer(engine.keyManager.GetPublicKey(), engine.current)

	if v4conn, _, err = wrapper.ListenUDP("udp4", uint16(cfg.Port)); err != nil {
		return nil, err
	}

	if v6conn, _, err = wrapper.ListenUDP("udp6", uint16(cfg.Port)); err != nil {
		return nil, err
	}

	if engine.drpClient, err = drp2.NewClient(&drp2.ClientConfig{Addr: cfg.SignalingUrl, Logger: log.NewLogger(log.Loglevel, "drp-mgtClient")}); err != nil {
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

	var info *turnclient.RelayInfo
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

	engine.bind = wrapper.NewBind(&wrapper.BindConfig{
		Logger:          log.NewLogger(log.Loglevel, "link-bind"),
		UniversalUDPMux: universalUdpMuxDefault,
		V4Conn:          v4conn,
		V6Conn:          v6conn,
		Proxy:           proxy,
		KeyManager:      engine.keyManager,
		RelayConn:       info.RelayConn,
	})

	probeManager = probe.NewManager(cfg.ForceRelay, universalUdpMuxDefault.UDPMuxDefault, universalUdpMuxDefault, engine, cfg.TurnServerUrl)

	offerHandler := drp2.NewOfferHandler(&drp2.OfferHandlerConfig{
		Logger:       log.NewLogger(log.Loglevel, "offer-handler"),
		ProbeManager: probeManager,
		AgentManager: engine.agentManager,
		StunUri:      cfg.TurnServerUrl,
		KeyManager:   engine.keyManager,
		NodeManager:  engine.nodeManager,
		Proxy:        proxy,
		TurnManager:  engine.turnManager,
	})

	proxy = proxy.OfferAndProbe(offerHandler, probeManager)

	engine.device = wg.NewDevice(device, engine.bind, cfg.WgLogger)

	wgConfigure := internal.NewWgConfigure(&internal.WGConfigerParams{
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
	// init event handler
	e.eventHandler = NewEventHandler(e, log.NewLogger(log.Loglevel, "event-handler"), e.mgtClient)
	// start e, open udp port
	if err := e.device.Up(); err != nil {
		return err
	}

	if e.current.Address != "" {
		// 设置Device
		internal.SetDeviceIP()("add", e.current.Address, e.wgConfigure.GetIfaceName())
	}

	if e.keyManager.GetKey() != "" {
		if err := e.DeviceConfigure(&internal.DeviceConfig{
			PrivateKey: e.current.PrivateKey,
		}); err != nil {
			return err
		}
	}
	// watch
	go func() {
		e.watchChan <- struct{}{}
		for {
			select {
			case <-e.watchChan:
				e.logger.Infof("watching chan")
				if err := e.mgtClient.Watch(e.ctx, e.eventHandler.HandleEvent()); err != nil {
					e.logger.Errorf("watch failed: %v", err)
					time.Sleep(10 * time.Second) // retry after 10 seconds
					e.watchChan <- struct{}{}
				}
			case <-e.ctx.Done():
				e.logger.Infof("watching chan closed")
				return
			}
		}
	}()

	go func() {
		e.keepaliveChan <- struct{}{}
		for {
			select {
			case <-e.keepaliveChan:
				e.logger.Infof("keepalive chan")
				if err := e.mgtClient.Keepalive(e.ctx); err != nil {
					e.logger.Errorf("keepalive failed: %v", err)
					time.Sleep(10 * time.Second)
					e.keepaliveChan <- struct{}{}
				}
			case <-e.ctx.Done():
				e.logger.Infof("keepalive chan closed")
				return
			}
		}

	}()

	return nil
}

func (e *Engine) Stop() error {
	e.device.Close()
	return nil
}

// SetConfig updates the configuration of the given interface.
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

func (e *Engine) AddPeer(node internal.Node) error {
	return e.device.IpcSet(node.String())
}

// RemovePeer add remove=true
func (e *Engine) RemovePeer(node internal.Node) error {
	node.Remove = true
	return e.device.IpcSet(node.String())
}

func (e *Engine) close() {
	close(e.keepaliveChan)
	e.drpClient.Close()
	//e.device.Close()
	e.logger.Verbosef("e closed")
}

func (e *Engine) GetWgConfiger() internal.ConfigureManager {
	return e.wgConfigure
}
