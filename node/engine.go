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
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"io"
	drpclient "linkany/drp/client"
	"linkany/internal"
	mgtclient "linkany/management/client"
	grpcclient "linkany/management/grpc/client"
	"linkany/management/vo"
	"linkany/pkg/config"
	"linkany/pkg/drp"
	lipc "linkany/pkg/ipc"
	"linkany/pkg/log"
	"linkany/pkg/probe"
	"linkany/pkg/wrapper"
	turnclient "linkany/turn/client"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

var (
	once sync.Once
	_    internal.EngineManager = (*Engine)(nil)
)

const (
	DefaultMTU = 1420
)

// Engine is the daemon that manages the wireGuard device
type Engine struct {
	logger        *log.Logger
	keyManager    internal.KeyManager
	Name          string
	device        *wg.Device
	mgtClient     *mgtclient.Client
	drpClient     *drpclient.Client
	bind          *wrapper.LinkBind
	GetNetworkMap func() (*vo.NetworkMap, error)
	updated       atomic.Bool

	group atomic.Value //belong to which group

	nodeManager  *internal.NodeManager
	agentManager internal.AgentManagerFactory
	wgConfigure  internal.ConfigureManager
	current      *internal.NodeMessage
	turnManager  *turnclient.TurnManager

	callback func(message *internal.Message) error

	keepaliveChan chan struct{} // channel for keepalive
	watchChan     chan struct{} // channel for watch
}

type EngineConfig struct {
	Logger        *log.Logger
	Conf          *config.LocalConfig
	Port          int
	UdpConn       *net.UDPConn
	InterfaceName string
	client        *mgtclient.Client
	drpClient     *drpclient.Client
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
		count        int64
		probeManager internal.ProbeManager
		proxy        *drpclient.Proxy
		turnClient   *turnclient.Client
		grpcClient   *grpcclient.Client
		v4conn       *net.UDPConn
		v6conn       *net.UDPConn
	)
	engine = new(Engine)
	engine.logger = cfg.Logger

	engine.turnManager = new(turnclient.TurnManager)
	once.Do(func() {
		engine.Name, device, err = internal.CreateTUN(DefaultMTU, cfg.Logger)
		engine.keepaliveChan = make(chan struct{}, 1)
		engine.watchChan = make(chan struct{}, 1)
	})

	if err != nil {
		return nil, err
	}

	// init managers
	engine.nodeManager = internal.NewNodeManager()
	engine.agentManager = drp.NewAgentManager()

	// control-mgtClient
	if grpcClient, err = grpcclient.NewClient(&grpcclient.GrpcConfig{
		Addr:          cfg.ManagementUrl,
		Logger:        log.NewLogger(log.Loglevel, "grpc-mgtclient"),
		KeepaliveChan: engine.keepaliveChan,
		WatchChan:     engine.watchChan}); err != nil {
		return nil, err
	}
	engine.mgtClient = mgtclient.NewClient(&mgtclient.ClientConfig{
		Logger:     log.NewLogger(log.Loglevel, "control-mgtClient"),
		GrpcClient: grpcClient,
		Conf:       cfg.Conf,
	})

	// limit node count
	if engine.current, count, err = engine.mgtClient.Get(context.Background()); err != nil {
		return nil, err
	}

	// TODO
	if count >= 5 {
		return nil, errors.New("your device count has reached the maximum limit")
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
		engine.logger.Infof("register to manager success")
	} else {
		privateKey = engine.current.PrivateKey
	}

	//update key
	engine.keyManager = internal.NewKeyManager(privateKey)
	engine.nodeManager.AddPeer(engine.keyManager.GetPublicKey(), engine.current)

	if v4conn, _, err = wrapper.ListenUDP("udp4", uint16(cfg.Port)); err != nil {
		return nil, err
	}

	if v6conn, _, err = wrapper.ListenUDP("udp6", uint16(cfg.Port)); err != nil {
		return nil, err
	}

	if engine.drpClient, err = drpclient.NewClient(&drpclient.ClientConfig{Addr: cfg.SignalingUrl, Logger: log.NewLogger(log.Loglevel, "drp-mgtClient")}); err != nil {
		return nil, err
	}
	engine.drpClient = engine.drpClient.KeyManager(engine.keyManager)

	// init stun
	if turnClient, err = turnclient.NewClient(&turnclient.ClientConfig{
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

	if proxy, err = drpclient.NewProxy(&drpclient.ProxyConfig{
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

	offerHandler := drp.NewOfferHandler(&drp.OfferHandlerConfig{
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
	// start engine, open udp port
	if err := e.device.Up(); err != nil {
		return err
	}
	// GetNetMap peers from control plane first time, then use watch
	networkMap, err := e.GetNetworkMap()
	if err != nil {
		e.logger.Errorf("sync peers failed: %v", err)
	}

	e.logger.Verbosef("get network map: %s", networkMap)

	// config device
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
			e.logger.Infof("mgt mgtClient keepliving...")
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

func (e *Engine) AddPeer(peer internal.NodeMessage) error {
	return e.device.IpcSet(peer.NodeString())
}

// RemovePeer add remove=true
func (e *Engine) RemovePeer(peer internal.NodeMessage) error {
	peer.Remove = true
	return e.device.IpcSet(peer.NodeString())
}

func (e *Engine) close() {
	close(e.keepaliveChan)
	e.drpClient.Close()
	//e.device.Close()
	e.logger.Verbosef("engine closed")
}

func (e *Engine) GetWgConfiger() internal.ConfigureManager {
	return e.wgConfigure
}
