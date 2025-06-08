package node

import (
	"context"
	"errors"
	"linkany/internal"
	controlclient "linkany/management/client"
	mgtclient "linkany/management/grpc/client"
	"linkany/management/utils"
	"linkany/management/vo"
	"linkany/pkg/config"
	"linkany/pkg/drp"
	"linkany/pkg/log"
	"linkany/pkg/probe"
	"linkany/pkg/wrapper"
	signalingclient "linkany/signaling/client"
	"linkany/signaling/grpc/signaling"
	turnclient "linkany/turn/client"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	wg "golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var (
	once sync.Once
)

const (
	DefaultMTU = 1420
)

var (
	_ internal.EngineManager = (*Engine)(nil)
)

// Engine is the daemon that manages the WireGuard device
type Engine struct {
	logger          *log.Logger
	keyManager      internal.KeyManager
	Name            string
	device          *wg.Device
	client          *controlclient.Client
	signalingClient *signalingclient.Client
	signalChannel   chan *signaling.SignalingMessage
	bind            *wrapper.NetBind
	GetNetworkMap   func() (*vo.NetworkMap, error)
	updated         atomic.Bool

	group atomic.Value //belong to which group

	nodeManager  *config.NodeManager
	agentManager internal.AgentManagerFactory
	wgConfigure  internal.ConfigureManager
	current      *utils.NodeMessage

	callback func(message *utils.Message) error
}

type EngineConfig struct {
	Logger          *log.Logger
	Conf            *config.LocalConfig
	Port            int
	UdpConn         *net.UDPConn
	InterfaceName   string
	client          *controlclient.Client
	signalingClient *signalingclient.Client
	WgLogger        *wg.Logger
	TurnServerUrl   string
	ForceRelay      bool
	ManagementUrl   string
	SignalingUrl    string
	ShowWgLog       bool
}

func (e *Engine) IpcHandle(conn net.Conn) {
	e.device.IpcHandle(conn)
}

// NewEngine create a tun auto
func NewEngine(cfg *EngineConfig) (*Engine, error) {
	var (
		device        tun.Device
		relayer       *wrapper.Relayer
		err           error
		engine        *Engine
		proberManager internal.ProbeManager
	)
	engine = new(Engine)
	engine.logger = cfg.Logger
	engine.signalChannel = make(chan *signaling.SignalingMessage, 1000)

	once.Do(func() {
		engine.Name, device, err = internal.CreateTUN(DefaultMTU, cfg.Logger)
	})

	if err != nil {
		return nil, err
	}

	v4conn, _, err := wrapper.ListenUDP("udp4", uint16(cfg.Port))

	if err != nil {
		return nil, err
	}

	engine.signalingClient, err = signalingclient.NewClient(&signalingclient.ClientConfig{Addr: cfg.SignalingUrl, Logger: log.NewLogger(log.Loglevel, "signalingclient")})
	if err != nil {
		return nil, err
	}

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

	agentManager := drp.NewAgentManager()
	engine.agentManager = agentManager
	engine.nodeManager = config.NewPeersManager()

	universalUdpMuxDefault := agentManager.NewUdpMux(v4conn)

	engine.bind = wrapper.NewBind(&wrapper.BindConfig{
		Logger:          log.NewLogger(log.Loglevel, "net-bind"),
		UniversalUDPMux: universalUdpMuxDefault,
		V4Conn:          v4conn,
		RelayConn:       relayInfo.RelayConn,
		SignalingClient: engine.signalingClient,
	})

	relayer = wrapper.NewRelayer(engine.bind)

	proberManager = probe.NewManager(cfg.ForceRelay, universalUdpMuxDefault.UDPMuxDefault, universalUdpMuxDefault, relayer, engine, cfg.TurnServerUrl)

	// controlclient
	mgtclient, err := mgtclient.NewClient(&mgtclient.GrpcConfig{Addr: cfg.ManagementUrl, Logger: log.NewLogger(log.Loglevel, "mgtclient")})
	if err != nil {
		return nil, err
	}

	// init key manager
	engine.keyManager = internal.NewKeyManager("")

	offerHandler := drp.NewOfferHandler(&drp.OfferHandlerConfig{
		Logger:        log.NewLogger(log.Loglevel, "offerHandler"),
		ProbeManager:  proberManager,
		AgentManager:  engine.agentManager,
		StunUri:       cfg.TurnServerUrl,
		KeyManager:    engine.keyManager,
		SignalChannel: engine.signalChannel,
		NodeManager:   engine.nodeManager,
	})

	go func() {
		for {
			if err = engine.signalingClient.Forward(context.Background(), engine.signalChannel, offerHandler.ReceiveOffer); err != nil {
				engine.logger.Errorf("forward failed: %v, is retrying in 20s", err)
				time.Sleep(20 * time.Second) // retry after 20 seconds
				continue
			}
		}
	}()

	engine.device = wg.NewDevice(device, engine.bind, cfg.WgLogger)

	// start engine, open udp port
	if err := engine.device.Up(); err != nil {
		return nil, err
	}

	wgConfigure := internal.NewWgConfigure(&internal.WGConfigerParams{
		Device:    engine.device,
		IfaceName: engine.Name,
		//Address:      current.Address,
		PeersManager: engine.nodeManager,
	})
	engine.wgConfigure = wgConfigure

	engine.client = controlclient.NewClient(&controlclient.ClientConfig{
		Logger:          log.NewLogger(log.Loglevel, "controlclient"),
		PeersManager:    engine.nodeManager,
		Conf:            cfg.Conf,
		UdpMux:          universalUdpMuxDefault.UDPMuxDefault,
		UniversalUdpMux: universalUdpMuxDefault,
		KeyManager:      engine.keyManager,
		AgentManager:    engine.agentManager,
		ProberManager:   proberManager,
		TurnClient:      turnClient,
		GrpcClient:      mgtclient,
		SignalChannel:   engine.signalChannel,
		OfferHandler:    offerHandler,
		WgConfiger:      wgConfigure,
	})

	// limit node count
	var (
		count int64
	)
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
	engine.keyManager.UpdateKey(privateKey)
	engine.nodeManager.AddPeer(engine.keyManager.GetPublicKey(), engine.current)

	// start heart to signaling
	go func() {
		if err = engine.signalingClient.Heartbeat(context.Background(), engine.signalChannel, engine.keyManager.GetPublicKey()); err != nil {
			engine.logger.Errorf("send heart beat failed: %v", err)
		}
	}()

	return engine, err
}

// Start will get networkmap
func (e *Engine) Start() error {
	// GetNetMap peers from control plane first time, then use watch
	networkMap, err := e.GetNetworkMap()
	if err != nil {
		e.logger.Errorf("sync peers failed: %v", err)
	}

	e.logger.Verbosef("get network map: %s", networkMap)

	// config device
	internal.SetDeviceIP()("add", e.current.Address, e.Name)

	if err = e.DeviceConfigure(&config.DeviceConfig{
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
//	_, err = e.signalingClient.Register(ctx, in)
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
func (e *Engine) SetConfig(conf *config.DeviceConf) error {
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

func (e *Engine) DeviceConfigure(conf *config.DeviceConfig) error {
	return e.device.IpcSet(conf.String())
}

func (e *Engine) AddPeer(peer utils.NodeMessage) error {
	return e.device.IpcSet(peer.NodeString())
}

// RemovePeer add remove=true
func (e *Engine) RemovePeer(peer utils.NodeMessage) error {
	peer.Remove = true
	return e.device.IpcSet(peer.NodeString())
}

func (e *Engine) close() {
	e.signalingClient.Close()
	close(e.signalChannel)
	e.device.Close()
}

func (e *Engine) GetWgConfiger() internal.ConfigureManager {
	return e.wgConfigure
}

func (e *Engine) GetRelayer() internal.Relay {
	return nil
}
