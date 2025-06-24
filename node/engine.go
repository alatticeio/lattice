package node

import (
	"context"
	"errors"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	drpclient "linkany/drp/client"
	"linkany/internal"
	controlclient "linkany/management/client"
	grpcclient "linkany/management/grpc/client"
	"linkany/management/vo"
	"linkany/pkg/config"
	"linkany/pkg/drp"
	"linkany/pkg/log"
	"linkany/pkg/probe"
	"linkany/pkg/wrapper"
	turnclient "linkany/turn/client"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	wg "golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
)

var (
	once sync.Once
	_    internal.EngineManager = (*Engine)(nil)
)

const (
	DefaultMTU = 1420
)

// Engine is the daemon that manages the WireGuard device
type Engine struct {
	logger        *log.Logger
	keyManager    internal.KeyManager
	Name          string
	device        *wg.Device
	client        *controlclient.Client
	drpClient     *drpclient.Client
	bind          *wrapper.NetBind
	GetNetworkMap func() (*vo.NetworkMap, error)
	updated       atomic.Bool

	group atomic.Value //belong to which group

	nodeManager  *internal.NodeManager
	agentManager internal.AgentManagerFactory
	wgConfigure  internal.ConfigureManager
	current      *internal.NodeMessage

	callback func(message *internal.Message) error
}

type EngineConfig struct {
	Logger        *log.Logger
	Conf          *config.LocalConfig
	Port          int
	UdpConn       *net.UDPConn
	InterfaceName string
	client        *controlclient.Client
	drpClient     *drpclient.Client
	WgLogger      *wg.Logger
	TurnServerUrl string
	ForceRelay    bool
	ManagementUrl string
	SignalingUrl  string
	ShowWgLog     bool
}

func (e *Engine) IpcHandle(conn net.Conn) {
	e.device.IpcHandle(conn)
}

// NewEngine create a tun auto
func NewEngine(cfg *EngineConfig) (*Engine, error) {
	var (
		device        tun.Device
		err           error
		engine        *Engine
		count         int64
		proberManager internal.ProbeManager
		proxy         *drpclient.Proxy
	)
	engine = new(Engine)
	engine.logger = cfg.Logger

	once.Do(func() {
		engine.Name, device, err = internal.CreateTUN(DefaultMTU, cfg.Logger)
	})

	if err != nil {
		return nil, err
	}

	// init managers
	engine.nodeManager = internal.NewNodeManager()
	engine.agentManager = drp.NewAgentManager()

	// control-client
	grpcClient, err := grpcclient.NewClient(&grpcclient.GrpcConfig{Addr: cfg.ManagementUrl, Logger: log.NewLogger(log.Loglevel, "grpc-client")})
	if err != nil {
		return nil, err
	}

	engine.client = controlclient.NewClient(&controlclient.ClientConfig{
		Logger:     log.NewLogger(log.Loglevel, "control-client"),
		GrpcClient: grpcClient,
		Conf:       cfg.Conf,
	})

	// limit node count
	engine.current, count, err = engine.client.Get(context.Background())
	if err != nil {
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
		_, err = engine.client.Register(privateKey, publicKey, cfg.Conf.Token)
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

	v4conn, _, err := wrapper.ListenUDP("udp4", uint16(cfg.Port))

	if err != nil {
		return nil, err
	}

	engine.drpClient, err = drpclient.NewClient(&drpclient.ClientConfig{Addr: cfg.SignalingUrl, Logger: log.NewLogger(log.Loglevel, "drp-client")})
	if err != nil {
		return nil, err
	}

	engine.drpClient = engine.drpClient.KeyManager(engine.keyManager)

	// init stun
	turnClient, err := turnclient.NewClient(&turnclient.ClientConfig{
		ServerUrl: cfg.TurnServerUrl,
		Conf:      cfg.Conf,
		Logger:    log.NewLogger(log.Loglevel, "turnclient"),
	})

	if err != nil {
		return nil, err
	}

	relayInfo, err := turnClient.GetRelayInfo(true)

	if err != nil {
		return nil, err
	}
	engine.logger.Infof("relay conn addr: %s", relayInfo.RelayConn.LocalAddr().String())

	universalUdpMuxDefault := engine.agentManager.NewUdpMux(v4conn)

	proxy, err = drpclient.NewProxy(&drpclient.ProxyConfig{
		DrpClient: engine.drpClient,
		DrpAddr:   cfg.SignalingUrl,
	})

	if err != nil {
		return nil, err
	}

	engine.drpClient = engine.drpClient.Proxy(proxy)

	engine.bind = wrapper.NewBind(&wrapper.BindConfig{
		Logger:          log.NewLogger(log.Loglevel, "net-bind"),
		UniversalUDPMux: universalUdpMuxDefault,
		V4Conn:          v4conn,
		Proxy:           proxy,
		KeyManager:      engine.keyManager,
	})

	proberManager = probe.NewManager(cfg.ForceRelay, universalUdpMuxDefault.UDPMuxDefault, universalUdpMuxDefault, engine, cfg.TurnServerUrl)

	offerHandler := drp.NewOfferHandler(&drp.OfferHandlerConfig{
		Logger:       log.NewLogger(log.Loglevel, "offer-handler"),
		ProbeManager: proberManager,
		AgentManager: engine.agentManager,
		StunUri:      cfg.TurnServerUrl,
		KeyManager:   engine.keyManager,
		NodeManager:  engine.nodeManager,
		Proxy:        proxy,
	})

	proxy = proxy.OfferAndProbe(offerHandler, proberManager)

	engine.device = wg.NewDevice(device, engine.bind, cfg.WgLogger)

	wgConfigure := internal.NewWgConfigure(&internal.WGConfigerParams{
		Device:       engine.device,
		IfaceName:    engine.Name,
		PeersManager: engine.nodeManager,
	})
	engine.wgConfigure = wgConfigure

	engine.client = engine.client.SetNodeManager(engine.nodeManager).SetProbeManager(proberManager).SetKeyManager(engine.keyManager).SetEngine(engine).SetOfferHandler(offerHandler)
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
		for {
			if err := e.client.Watch(context.Background(), e.client.HandleWatchMessage); err != nil {
				e.logger.Errorf("watch failed: %v", err)
				time.Sleep(10 * time.Second) // retry after 10 seconds
				continue
			}
		}
	}()

	go func() {
		if err := e.client.Keepalive(context.Background()); err != nil {
			e.logger.Errorf("keepalive failed: %v", err)
		} else {
			e.logger.Infof("mgt client keepliving...")
		}
	}()

	return nil
}

//func (e *Engine) registerToSignaling(ctx context.Context, cfg *config.LocalConfig) error {
//
//	publicKey := e.keyManager.GetPublicKey()
//	var req = &signaling.EncryptMessageReqAndResp{
//		SrcPublicKey: publicKey,
//		Token:        cfg.Token,
//	}
//
//	bs, err := proto.Marshal(req)
//	if err != nil {
//		return err
//	}
//
//	in := &signaling.EncryptMessage{
//		PublicKey: publicKey,
//		Body:      bs,
//	}
//
//	_, err = e.drpClient.Register(ctx, in)
//
//	if err != nil {
//		e.logger.Errorf("register to signaling failed: %v", err)
//		return err
//	}
//	return nil
//}

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
	e.drpClient.Close()
	e.device.Close()
}

func (e *Engine) GetWgConfiger() internal.ConfigureManager {
	return e.wgConfigure
}

func (e *Engine) GetRelayer() internal.Relay {
	return nil
}
