package client

import (
	"fmt"
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/conn"
	wg "golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"k8s.io/klog/v2"
	controlclient "linkany/control/client"
	client2 "linkany/control/grpc/client"
	"linkany/internal"
	"linkany/pkg/config"
	"linkany/pkg/drp"
	"linkany/pkg/iface"
	"linkany/pkg/probe"
	"linkany/pkg/wrapper"
	"linkany/signaling/client"
	turnclient "linkany/turn/client"
	"net"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	once sync.Once
)

const (
	DefaultMTU = 1420
)

type Engine struct {
	km     *internal.KeyManager
	Name   string
	device *wg.Device
	//agent         *ice.Agent
	tieBreaker uint64
	client     controlclient.ClientInterface
	bind       conn.Bind
	drpClient  *drp.Client
	OnSync     func(client controlclient.ClientInterface) (*config.DeviceConf, error)
	updated    atomic.Bool

	pm           *config.PeersManager
	agentManager *internal.AgentManager
	wgConfiger   iface.WGConfigure
}

type EngineParams struct {
	Conf          *config.LocalConfig
	Port          int
	UdpConn       *net.UDPConn
	InterfaceName string
	client        *controlclient.Client
	Logger        *wg.Logger
	StunUri       string
	ForceRelay    bool
	GrpcAddr      string
}

func (e *Engine) IpcHandle(conn net.Conn) {
	e.device.IpcHandle(conn)
}

// NewEngine create a tun auto
func NewEngine(cfg *EngineParams) (*Engine, error) {
	var tdevice tun.Device
	var err error
	var ifaceName string
	once.Do(func() {
		ifaceName, tdevice, err = iface.CreateTUN(DefaultMTU)
	})
	if err != nil {
		return nil, err
	}

	v4conn, _, err := wrapper.ListenUDP("udp4", 51820)
	// init stun
	turnClient, err := turnclient.NewClient(&turnclient.ClientConfig{
		ServerUrl: "stun.linkany.io:3478",
		Conf:      cfg.Conf,
	})

	relayInfo, err := turnClient.GetRelayInfo(true)
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}

	privateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return nil, err
	}
	km := internal.NewKeyManager(privateKey)
	agentManager := internal.NewAgentManager()

	peersManager := config.NewPeersManager()
	universalUdpMuxDefault := internal.NewUdpMux(v4conn)
	if err != nil {
		return nil, err
	}
	proberManager := probe.NewProberManager(cfg.ForceRelay)

	grpcClient, err := client2.NewGrpcClient(&client2.GrpcConfig{Addr: cfg.GrpcAddr})
	if err != nil {
		return nil, err
	}

	ufrag, pwd := probe.GenerateRandomUfragPwd()
	client := controlclient.NewClient(&controlclient.ClientConfig{
		Pm:              peersManager,
		Conf:            cfg.Conf,
		UdpMux:          universalUdpMuxDefault.UDPMuxDefault,
		UniversalUdpMux: universalUdpMuxDefault,
		Km:              km,
		AgentManager:    agentManager,
		Ufrag:           ufrag,
		Pwd:             pwd,
		ProberManager:   proberManager,
		TurnClient:      turnClient,
		GrpcClient:      grpcClient,
	})
	//fetconf
	deviceConf, err := client.Register()
	if err != nil {
		return nil, err
	}

	drpClient, err := NewDrpClient(deviceConf.DrpUrl, agentManager, proberManager, turnClient)
	if err != nil {
		return nil, err
	}

	bind := wrapper.NewBind(&wrapper.BindConfig{
		DrpClient:       drpClient,
		UniversalUDPMux: universalUdpMuxDefault,
		V4Conn:          v4conn,
		RelayConn:       relayInfo.RelayConn,
	})

	device := wg.NewDevice(tdevice, bind, cfg.Logger)

	relayer := wrapper.NewRelayer(bind)
	proberManager.SetRelayer(relayer)

	// set device config
	e := &Engine{device: device, Name: ifaceName, bind: bind, km: km, pm: peersManager}
	deviceConfig := &config.DeviceConfig{
		PrivateKey: km.GetKey().String(),
		ListenPort: 51820,
	}
	e.DeviceConfigure(deviceConfig)
	e.agentManager = agentManager

	// start engine, open udp port
	if err := e.device.Up(); err != nil {
		return nil, err
	}

	wgConfiger := iface.NewWgConfiger(&iface.WGConfigerParams{
		Device:       device,
		IfaceName:    ifaceName,
		Address:      deviceConf.Device.Address,
		PeersManager: peersManager,
	})

	//proberManager.SetWgConfiger(wgConfiger)

	client.(*controlclient.Client).SetDrpClient(drpClient) // set offer manager
	e.client = client
	e.wgConfiger = wgConfiger
	drpClient.SetWgConfiger(wgConfiger)
	return e, err
}

// Start open a ticker to sync peers
func (e *Engine) Start(ticker *time.Ticker, quit chan struct{}) error {
	go func() {
		for {
			select {
			case <-ticker.C:
				// do stuff
				conf, err := e.OnSync(e.client)
				if err != nil {
					klog.Errorf("sync peers failed: %v", err)
					break
				}

				// this should be done after ipset
				if conf.DrpUrl != "" {
					if !e.updated.Load() {
						e.updated.Store(true)
					}
				}

			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()

	return nil
}

func NewDrpClient(drpUrl string, manager *internal.AgentManager, probers *probe.NetProber, turnClient *turnclient.Client) (*drp.Client, error) {
	u, err := url.Parse(drpUrl)
	if err != nil {
		klog.Errorf("parse drp url failed: %v", err)
		return nil, err
	}
	if !strings.Contains(u.Host, ":") {
		u.Host = fmt.Sprintf("%s:80", u.Host)
	}
	addr, err := net.ResolveTCPAddr("tcp", u.Host)
	if err != nil {
		klog.Errorf("resolve tcp addr failed: %v", err)
		return nil, err
	}

	node := drp.NewNode("", addr, nil)
	drpClient, err := client.NewClient(node, manager, probers, turnClient).Connect(drpUrl)
	if err != nil {
		klog.Errorf("connect to drp server failed: %v", err)
		return nil, err
	}
	klog.Infof("connect to drp server success")

	return drpClient, nil

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
