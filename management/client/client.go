package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/linkanyio/ice"
	"github.com/pion/logging"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"io"
	"k8s.io/klog/v2"
	"linkany/internal"
	grpcclient "linkany/management/grpc/client"
	"linkany/management/grpc/mgt"
	"linkany/pkg/config"
	"linkany/pkg/drp"
	"linkany/pkg/probe"
	turnclient "linkany/turn/client"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

type ClientInterface interface {
	// Register will register a device to linkany center
	Register() (*config.DeviceConf, error)

	Login(user *config.User) error

	List() (*config.DeviceConf, error)

	GetUsers() []*config.User
}

var (
	_ ClientInterface = (*Client)(nil)
)

type PeerMap struct {
	lock sync.Mutex
	m    map[string]ice.Candidate
}

// Client is client of linkany, will fetch config from origin server interval
type Client struct {
	km              *internal.KeyManager
	ch              chan *probe.DirectChecker
	pm              *config.PeersManager
	TieBreaker      uint32
	stunUri         string
	ufrag           string
	pwd             string
	ifaceName       string
	conf            *config.LocalConfig
	httpClient      *http.Client
	grpcClient      *grpcclient.GrpcClient
	agent           *ice.Agent
	conn4           net.PacketConn
	udpMux          *ice.UDPMuxDefault
	universalUdpMux *ice.UniversalUDPMuxDefault
	update          func() error
	agentManager    *internal.AgentManager
	drpClient       *drp.Client
	proberManager   *probe.NetProber
	proberMux       sync.Mutex
	turnClient      *turnclient.Client
}

type ClientConfig struct {
	Pm              *config.PeersManager
	Conf            *config.LocalConfig
	PeerCh          chan *probe.DirectChecker
	Agent           *ice.Agent
	UdpMux          *ice.UDPMuxDefault
	UniversalUdpMux *ice.UniversalUDPMuxDefault
	Km              *internal.KeyManager
	AgentManager    *internal.AgentManager
	GrpcClient      *grpcclient.GrpcClient
	Ufrag           string
	Pwd             string
	OfferManager    internal.OfferManager
	ProberManager   *probe.NetProber
	TurnClient      *turnclient.Client
}

func NewClient(config *ClientConfig) ClientInterface {
	client := &Client{
		km:              config.Km,
		TieBreaker:      ice.NewTieBreaker(),
		ch:              config.PeerCh,
		conf:            config.Conf,
		pm:              config.Pm,
		httpClient:      http.DefaultClient,
		udpMux:          config.UdpMux,
		universalUdpMux: config.UniversalUdpMux,
		agentManager:    config.AgentManager,
		ufrag:           config.Ufrag,
		pwd:             config.Pwd,
		proberManager:   config.ProberManager,
		turnClient:      config.TurnClient,
		grpcClient:      config.GrpcClient,
	}

	return client
}

//func (c *Client) SetDrpClient(client *drp.Client) {
//	c.offerManager = client
//}

// Register will register device to linkany center
func (c *Client) Register() (*config.DeviceConf, error) {
	// TODO implement this function
	return nil, nil
}

func (c *Client) Login(user *config.User) error {
	var err error
	ctx := context.Background()
	loginRequest := &mgt.LoginRequest{
		Username: user.Username,
		Password: user.Password,
	}

	body, err := proto.Marshal(loginRequest)
	if err != nil {
		return err
	}
	resp, err := c.grpcClient.Login(ctx, &mgt.ManagementMessage{
		Body: body,
	})

	if err != nil {
		return err
	}

	var loginResponse mgt.LoginResponse
	if err := proto.Unmarshal(resp.Body, &loginResponse); err != nil {
		return err
	}

	homeDir, err := os.UserHomeDir()
	path := filepath.Join(homeDir, ".linkany/config.json")
	_, err = os.Stat(path)
	var file *os.File
	if os.IsNotExist(err) {
		parentDir := filepath.Dir(path)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return err
		}
		file, err = os.Create(path)
		if os.IsExist(err) {
			file, err = os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0755)
		}
	} else {
		file, err = os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0755)
	}
	defer file.Close()
	var local config.LocalConfig
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&local)
	if err != nil && err != io.EOF {
		return err
	}

	appId, err := config.GetAppId()

	ufrag, pwd, err := internal.GenerateUfragPwd()
	if err != nil {
		return err
	}

	b := &config.LocalConfig{
		Auth:  fmt.Sprintf("%s:%s", user.Username, config.StringToBase64(user.Password)),
		AppId: appId,
		Token: loginResponse.Token,
		Ufrag: ufrag,
		Pwd:   pwd,
	}

	err = config.UpdateLocalConfig(b)
	if err != nil {
		return err
	}

	return nil
}

// List fetch user's all peer and configuration to linkany instance
func (c *Client) List() (*config.DeviceConf, error) {
	var conf *config.DeviceConf
	var err error
	//appId, err := config.GetAppId()
	//if err != nil {
	//	return nil, err
	//}
	info, err := config.GetLocalUserInfo()
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	loginRequest := &mgt.LoginRequest{
		Username: info.Username,
	}

	body, err := proto.Marshal(loginRequest)
	if err != nil {
		return nil, err
	}

	resp, err := c.grpcClient.List(ctx, &mgt.ManagementMessage{
		Body: body,
	})

	if err != nil {
		return nil, err
	}
	//var peers []*config.Peer
	//if err := json.Unmarshal(resp.Body, peers); err != nil {
	//	return nil, err
	//}

	var networkMap mgt.NetworkMap
	if err := proto.Unmarshal(resp.Body, &networkMap); err != nil {
		return nil, err
	}

	for _, nPeer := range networkMap.Peers {
		peer := &config.Peer{
			PublicKey:           nPeer.PublicKey,
			Endpoint:            nPeer.Endpoint,
			Address:             nPeer.Address,
			AllowedIps:          nPeer.AllowedIps,
			PersistentKeepalive: int(nPeer.PersistentKeepalive),
		}
		mappedPeer := c.pm.GetPeer(peer.PublicKey)
		if mappedPeer == nil {
			mappedPeer = peer
			c.pm.AddPeer(peer.PublicKey, peer)
			klog.Infof("add peer to local cache, key: %s, peer: %v", peer.PublicKey, peer)
		} else if mappedPeer.Connected.Load() {
			continue
		}
		agent, ok := c.agentManager.Get(peer.PublicKey)

		if agent == nil || !ok {
			l := logging.NewDefaultLoggerFactory()
			l.DefaultLogLevel = logging.LogLevelDebug
			agent, err = internal.NewAgent(&internal.AgentParams{
				LoggerFacotry:   l,
				StunUrl:         "stun:81.68.109.143:3478",
				UdpMux:          c.universalUdpMux.UDPMuxDefault,
				UniversalUdpMux: c.universalUdpMux,
				Ufrag:           c.ufrag,
				Pwd:             c.pwd,
				OnCandidate: func(c ice.Candidate) {
					if c != nil {
						klog.Infof("new candidate: %v", c.Marshal())
					}
				},
			})

			klog.Infof("creating agent for peer: %s", peer.PublicKey)
			if err := agent.OnConnectionStateChange(func(connectionState ice.ConnectionState) {
				switch connectionState {
				case ice.ConnectionStateDisconnected:
					peer.Connected.Store(false)
					c.agentManager.Remove(peer.PublicKey)
					klog.Infof("agent disconnected, remove agent")
					break
				case ice.ConnectionStateFailed:
					peer.P2PFlag.Store(true)
					peer.Connected.Store(true)
					peer.Endpoint = "relay"
					c.agentManager.Remove(peer.PublicKey)
					klog.Infof("check connection failed, will use relay, remove agent")
					break
				default:
					c.pm.AddPeer(peer.PublicKey, peer)
				}
			}); err != nil {
				return nil, err
			}

			c.agentManager.Add(peer.PublicKey, agent)
		}

		// start probeConn
		if !peer.ConnectionState.Load() {
			go c.probeConn(agent, peer)
		}

	}

	return conf, nil
}

func GetCandidates(agent *ice.Agent) string {
	var err error
	var ch = make(chan struct{})
	var candidates []ice.Candidate
	go func() {
		for {
			candidates, err = agent.GetLocalCandidates()
			if err != nil || len(candidates) == 0 {
				continue
			}

			close(ch)
			break
		}
	}()

	select {
	case <-ch:
	}

	var candString string
	for i, candidate := range candidates {
		candString = candidate.Marshal()
		if i != len(candidates)-1 {
			candString += ";"
		}
	}

	return candString
}

func (c *Client) probeConn(agent *ice.Agent, peer *config.Peer) error {
	peer.ConnectionState.Store(true)
	directContact := func(agent *ice.Agent, peer *config.Peer) error {
		//send NodeInfo
		var err error
		candidates := GetCandidates(agent)
		directOffer := internal.NewDirectOffer(&internal.DirectOfferConfig{
			WgPort:     51820,
			Ufrag:      c.ufrag,
			Pwd:        c.pwd,
			LocalKey:   c.agentManager.GetLocalKey(),
			Candidates: candidates,
		})
		dstPubKey, err := wgtypes.ParseKey(peer.PublicKey)
		if err != nil {
			klog.Errorf("parse public key failed: %v", err)
			return err
		}

		prober := c.proberManager.GetProber(dstPubKey)
		if prober == nil {
			c.proberMux.Lock()
			defer c.proberMux.Unlock()
			prober = probe.NewProber(&probe.ProberConfig{
				DirectOfferManager: c.drpClient,
				RelayOfferManager:  c.drpClient,
				AgentManager:       c.agentManager,
				WGConfiger:         c.proberManager.GetWgConfiger(),
				Key:                dstPubKey,
				ProberManager:      c.proberManager,
				IsForceRelay:       c.proberManager.IsForceRelay(),
				TurnClient:         c.turnClient,
			})
			c.proberManager.AddProber(dstPubKey, prober)
		}

		if err := prober.Start(c.km.GetPublicKey(), dstPubKey, directOffer); err != nil {
			klog.Errorf("send directOffer failed: %v", err)
			return err
		}

		return nil
	}

	relayConact := func(peer *config.Peer) error {
		//send NodeInfo
		var err error

		relayInfo, err := c.turnClient.GetRelayInfo(true)
		if err != nil {
			return errors.New("get relay info failed")
		}

		relayAddr, err := turnclient.AddrToUdpAddr(relayInfo.RelayConn.LocalAddr())

		relayOffer := probe.NewOffer(relayInfo.MappedAddr, *relayAddr, c.agentManager.GetLocalKey(), probe.OfferTypeRelayOffer)

		dstPubKey, err := wgtypes.ParseKey(peer.PublicKey)
		if err != nil {
			klog.Errorf("parse public key failed: %v", err)
			return err
		}

		prober := c.proberManager.GetProber(dstPubKey)
		if prober == nil {
			c.proberMux.Lock()
			defer c.proberMux.Unlock()
			prober = probe.NewProber(&probe.ProberConfig{
				DirectOfferManager: c.drpClient,
				RelayOfferManager:  c.drpClient,
				AgentManager:       c.agentManager,
				WGConfiger:         c.proberManager.GetWgConfiger(),
				Key:                dstPubKey,
				ProberManager:      c.proberManager,
				IsForceRelay:       c.proberManager.IsForceRelay(),
				TurnClient:         c.turnClient,
			})
			c.proberManager.AddProber(dstPubKey, prober)
		}

		if err := prober.Start(c.km.GetPublicKey(), dstPubKey, relayOffer); err != nil {
			klog.Errorf("send relayOffer failed: %v", err)
			return err
		}

		return nil
	}

	if c.proberManager.IsForceRelay() {
		go relayConact(peer)
	} else {
		go directContact(agent, peer)
	}

	return nil
}

// TODO implement this function
func (c *Client) GetUsers() []*config.User {
	var users []*config.User
	users = append(users, config.NewUser("linkany", "123456"))
	return users
}

func (c *Client) SetDrpClient(client *drp.Client) {
	c.drpClient = client
}
