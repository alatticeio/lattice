package client

import (
	"context"
	"github.com/golang/protobuf/proto"
	"github.com/vishvananda/netlink"
	wg "golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"k8s.io/klog/v2"
	"linkany/internal"
	controlclient "linkany/management/client"
	mgtclient "linkany/management/grpc/client"
	"linkany/management/grpc/mgt"
	"linkany/pkg/config"
	"linkany/pkg/drp"
	"linkany/pkg/iface"
	"linkany/pkg/probe"
	"linkany/pkg/wrapper"
	signalingclient "linkany/signaling/client"
	"linkany/signaling/grpc/signaling"
	turnclient "linkany/turn/client"
	"net"
	"strings"
	"sync"
	"sync/atomic"
)

var (
	once sync.Once
)

const (
	DefaultMTU = 1420
)

// Engine is the daemon that manages the WireGuard device
type Engine struct {
	keyManager      *internal.KeyManager
	Name            string
	device          *wg.Device
	client          *controlclient.Client
	signalingClient *signalingclient.Client
	signalChannel   chan *signaling.EncryptMessage
	bind            *wrapper.NetBind
	GetNetworkMap   func() (*config.DeviceConf, error)
	updated         atomic.Bool

	peersManager *config.PeersManager
	agentManager *internal.AgentManager
	wgConfigure  iface.WGConfigureInterface

	callback func(message *mgt.WatchMessage) error
}

type EngineParams struct {
	Conf            *config.LocalConfig
	Port            int
	UdpConn         *net.UDPConn
	InterfaceName   string
	client          *controlclient.Client
	signalingClient *signalingclient.Client
	Logger          *wg.Logger
	StunUri         string
	ForceRelay      bool
	ManagementAddr  string
	SignalingAddr   string
}

func (e *Engine) IpcHandle(conn net.Conn) {
	e.device.IpcHandle(conn)
}

// NewEngine create a tun auto
func NewEngine(cfg *EngineParams) (*Engine, error) {
	var device tun.Device
	var err error
	engine := new(Engine)
	engine.signalChannel = make(chan *signaling.EncryptMessage, 1000)

	once.Do(func() {
		engine.Name, device, err = iface.CreateTUN(DefaultMTU)
	})

	if err != nil {
		return nil, err
	}

	v4conn, _, err := wrapper.ListenUDP("udp4", uint16(cfg.Port))

	if err != nil {
		return nil, err
	}

	engine.signalingClient, err = signalingclient.NewClient(&signalingclient.ClientConfig{Addr: cfg.SignalingAddr})
	if err != nil {
		return nil, err
	}

	// init stun
	turnClient, err := turnclient.NewClient(&turnclient.ClientConfig{
		ServerUrl: "stun.linkany.io:3478",
		Conf:      cfg.Conf,
	})

	if err != nil {
		return nil, err
	}

	relayInfo, err := turnClient.GetRelayInfo(true)
	klog.Infof("relay conn addr: %s", relayInfo.RelayConn.LocalAddr().String())

	if err != nil {
		return nil, err
	}

	engine.agentManager = internal.NewAgentManager()
	engine.peersManager = config.NewPeersManager()

	universalUdpMuxDefault := internal.NewUdpMux(v4conn)

	engine.bind = wrapper.NewBind(&wrapper.BindConfig{
		UniversalUDPMux: universalUdpMuxDefault,
		V4Conn:          v4conn,
		RelayConn:       relayInfo.RelayConn,
		SignalingClient: engine.signalingClient,
	})

	relayer := wrapper.NewRelayer(engine.bind)

	proberManager := probe.NewProberManager(cfg.ForceRelay, relayer)

	// controlclient
	grpcClient, err := mgtclient.NewClient(&mgtclient.GrpcConfig{Addr: cfg.ManagementAddr})
	if err != nil {
		return nil, err
	}

	// init key manager
	engine.keyManager = internal.NewKeyManager("")

	//callback := func(msg *signaling.EncryptMessage) error {
	//	fmt.Println(msg)
	//	return nil
	//}

	drpclient := drp.NewClient(&drp.ClientConfig{
		Probers:       proberManager,
		AgentManager:  engine.agentManager,
		UdpMux:        universalUdpMuxDefault,
		SignalChannel: engine.signalChannel,
	})

	go func() {
		if err = engine.signalingClient.Forward(context.Background(), engine.signalChannel, drpclient.ReceiveOffer); err != nil {
			klog.Errorf("forward failed: %v", err)
		}
	}()

	ufrag, pwd := probe.GenerateRandomUfragPwd()

	engine.client = controlclient.NewClient(&controlclient.ClientConfig{
		PeersManager:    engine.peersManager,
		Conf:            cfg.Conf,
		UdpMux:          universalUdpMuxDefault.UDPMuxDefault,
		UniversalUdpMux: universalUdpMuxDefault,
		KeyManager:      engine.keyManager,
		AgentManager:    engine.agentManager,
		Ufrag:           ufrag,
		Pwd:             pwd,
		ProberManager:   proberManager,
		TurnClient:      turnClient,
		GrpcClient:      grpcClient,
		SignalChannel:   engine.signalChannel,
		DrpClient:       drpclient,
	})

	//fetconf
	var current *config.Peer
	current, err = engine.client.Get(context.Background())
	if err != nil {
		return nil, err
	}

	var privateKey string
	var publicKey string
	if current.AppID != cfg.Conf.AppId {
		key, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return nil, err
		}
		privateKey = key.String()
		publicKey = key.PublicKey().String()
		_, err = engine.client.Register(privateKey, publicKey, cfg.Conf.Token)
		if err != nil {
			klog.Errorf("register failed, with err: %s\n", err.Error())
			return nil, err
		}
		klog.Infof("register to manager success")
	} else {
		privateKey = current.PrivateKey
	}
	//update key
	engine.keyManager.UpdateKey(privateKey)

	// register to signaling server
	if err = engine.registerToSignaling(context.Background(), cfg.Conf); err != nil {
		return nil, err
	}
	klog.Infof("register to signaling success")

	engine.device = wg.NewDevice(device, engine.bind, cfg.Logger)

	// start engine, open udp port
	if err := engine.device.Up(); err != nil {
		return nil, err
	}

	wgConfigure := iface.NewWgConfigure(&iface.WGConfigerParams{
		Device:       engine.device,
		IfaceName:    engine.Name,
		Address:      current.Address,
		PeersManager: engine.peersManager,
	})
	engine.wgConfigure = wgConfigure

	proberManager.SetWgConfiger(wgConfigure)

	return engine, err
}

// Start will get networkmap
func (e *Engine) Start() error {
	// List peers from control plane first time, then use watch
	conf, err := e.GetNetworkMap()
	if err != nil {
		klog.Errorf("sync peers failed: %v", err)
	}

	//TODO set device config
	klog.Infof("networkmap: %v", conf)
	// set device config
	deviceConfig := &config.DeviceConfig{
		PrivateKey: e.keyManager.GetKey(),
	}

	if err = e.DeviceConfigure(deviceConfig); err != nil {
		return err
	}

	// watch
	go func() {
		if err := e.client.Watch(context.Background(), e.client.WatchMessage); err != nil {
			klog.Errorf("watch failed: %v", err)
		}
	}()

	go func() {
		if err := e.client.Keepalive(context.Background()); err != nil {
			klog.Errorf("keepalive failed: %v", err)
		}
	}()

	return nil
}

func (e *Engine) registerToSignaling(ctx context.Context, cfg *config.LocalConfig) error {

	publicKey := e.keyManager.GetPublicKey()
	var req = &signaling.EncryptMessageReqAndResp{
		SrcPublicKey: publicKey,
		Token:        cfg.Token,
	}

	bs, err := proto.Marshal(req)
	if err != nil {
		return err
	}

	in := &signaling.EncryptMessage{
		PublicKey: publicKey,
		Body:      bs,
	}

	_, err = e.signalingClient.Register(ctx, in)

	return err
}

func (e *Engine) Stop() error {
	e.device.Close()
	return nil
}

// GetLink returns the link with the given interface name.
func GetLink(interfaceName string) (netlink.Link, error) {
	return netlink.LinkByName(interfaceName)
}

// SetConfig updates the configuration of the given interface.
func (e *Engine) SetConfig(conf *config.DeviceConf) error {
	nowConf, err := e.device.IpcGet()
	if err != nil {
		return err
	}

	if conf.String() == nowConf {
		klog.Infof("config is same, no need to update")
		return nil
	}

	reader := strings.NewReader(conf.String())

	return e.device.IpcSetOperation(reader)
}

func (e *Engine) DeviceConfigure(conf *config.DeviceConfig) error {
	return e.device.IpcSet(conf.String())
}

func (e *Engine) AddPeer(peer config.Peer) error {
	return e.device.IpcSet(peer.String())
}

// RemovePeer add remove=true
func (e *Engine) RemovePeer(peer config.Peer) error {
	peer.Remove = true
	return e.device.IpcSet(peer.String())
}

func (e *Engine) close() {
	e.signalingClient.Close()
	close(e.signalChannel)
	e.device.Close()
}
