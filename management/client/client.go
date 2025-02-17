package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/linkanyio/ice"
	"github.com/pion/logging"
	"io"
	"k8s.io/klog/v2"
	"linkany/internal"
	"linkany/management/entity"
	grpcclient "linkany/management/grpc/client"
	"linkany/management/grpc/mgt"
	grpcserver "linkany/management/grpc/server"
	"linkany/pkg/config"
	"linkany/pkg/drp"
	"linkany/pkg/probe"
	"linkany/signaling/grpc/signaling"
	turnclient "linkany/turn/client"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type PeerMap struct {
	lock sync.Mutex
	m    map[string]ice.Candidate
}

// Client is client of linkany, will fetch config from origin server interval
type Client struct {
	keyManager      *internal.KeyManager
	signalChannel   chan *signaling.EncryptMessage
	ch              chan *probe.DirectChecker
	peersManager    *config.PeersManager
	TieBreaker      uint32
	stunUri         string
	ufrag           string
	pwd             string
	ifaceName       string
	conf            *config.LocalConfig
	grpcClient      *grpcclient.Client
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
	PeersManager    *config.PeersManager
	Conf            *config.LocalConfig
	PeerCh          chan *probe.DirectChecker
	Agent           *ice.Agent
	UdpMux          *ice.UDPMuxDefault
	UniversalUdpMux *ice.UniversalUDPMuxDefault
	KeyManager      *internal.KeyManager
	AgentManager    *internal.AgentManager
	GrpcClient      *grpcclient.Client
	Ufrag           string
	Pwd             string
	ProberManager   *probe.NetProber
	TurnClient      *turnclient.Client
	SignalChannel   chan *signaling.EncryptMessage
	DrpClient       *drp.Client
}

func NewClient(config *ClientConfig) *Client {
	client := &Client{
		drpClient:       config.DrpClient,
		keyManager:      config.KeyManager,
		TieBreaker:      ice.NewTieBreaker(),
		ch:              config.PeerCh,
		conf:            config.Conf,
		peersManager:    config.PeersManager,
		udpMux:          config.UdpMux,
		universalUdpMux: config.UniversalUdpMux,
		agentManager:    config.AgentManager,
		ufrag:           config.Ufrag,
		pwd:             config.Pwd,
		proberManager:   config.ProberManager,
		turnClient:      config.TurnClient,
		grpcClient:      config.GrpcClient,
		signalChannel:   config.SignalChannel,
	}

	return client
}

// RegisterToManagement will register device to linkany center
func (c *Client) RegisterToManagement() (*config.DeviceConf, error) {
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

// List get user's networkmap
func (c *Client) List() (*config.DeviceConf, error) {
	ctx := context.Background()
	var conf *config.DeviceConf
	var err error

	info, err := config.GetLocalConfig()
	if err != nil {
		return nil, err
	}

	request := &mgt.Request{
		AppId:  c.conf.AppId,
		Token:  info.Token,
		PubKey: c.keyManager.GetPublicKey(),
	}

	body, err := proto.Marshal(request)
	if err != nil {
		return nil, err
	}

	resp, err := c.grpcClient.List(ctx, &mgt.ManagementMessage{
		Body: body,
	})

	if err != nil {
		return nil, err
	}

	var networkMap entity.NetworkMap
	if err := json.Unmarshal(resp.Body, &networkMap); err != nil {
		return nil, err
	}

	conf = &config.DeviceConf{}

	for _, p := range networkMap.Peers {
		if err := c.AddPeer(p); err != nil {
			klog.Errorf("add peer failed: %v", err)
		}
	}

	return conf, nil
}

func (c *Client) ToConfigPeer(peer *entity.Peer) *config.Peer {

	return &config.Peer{
		PublicKey:           peer.PublicKey,
		Endpoint:            peer.Endpoint,
		Address:             peer.Address,
		AllowedIps:          peer.AllowedIPs,
		PersistentKeepalive: peer.PersistentKeepalive,
	}
}

func (c *Client) WatchMessage(msg *mgt.WatchMessage) error {
	var err error
	var peers []entity.Peer
	if err = json.Unmarshal(msg.Body, &peers); err != nil {
		return err
	}

	for _, peer := range peers {
		switch msg.Type {
		case mgt.EventType_DELETE:
			//TODO remove peer from local cache & agentManager
			c.peersManager.Remove(peer.PublicKey)
		case mgt.EventType_ADD:
			if err = c.AddPeer(&peer); err != nil {
				klog.Errorf("add peer failed: %v", err)
			}
		}
	}

	return nil

}

func (c *Client) AddPeer(p *entity.Peer) error {
	var err error
	if p.PublicKey == c.keyManager.GetPublicKey() {
		klog.Warningf("self peer, skip")
		err = errors.New("self peer, skip")
		return err
	}
	peer := c.ToConfigPeer(p)
	mappedPeer := c.peersManager.GetPeer(peer.PublicKey)
	if mappedPeer == nil {
		mappedPeer = peer
		c.peersManager.AddPeer(peer.PublicKey, peer)
		klog.Infof("add peer to local cache, key: %s, peer: %v", peer.PublicKey, peer)
	} else if mappedPeer.Connected.Load() {
		return nil
	}

	agent, ok := c.agentManager.Get(peer.PublicKey)
	gatherCh := make(chan interface{})

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
				} else {
					klog.Infof("all candidates has been gathered.")
					close(gatherCh)
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
				c.peersManager.AddPeer(peer.PublicKey, peer)
			}
		}); err != nil {
			return err
		}

		c.agentManager.Add(peer.PublicKey, agent)
	}

	// start probeConn
	go func() {
		err := c.probeConn(agent, peer, gatherCh)
		if err != nil {
			klog.Errorf("probeConn failed: %v", err)
		}
	}()

	return nil
}

func GetCandidates(agent *ice.Agent, gatherCh chan interface{}) string {
	<-gatherCh
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

	klog.Infof("gathered candidates >>>: %v", candString)
	return candString
}

func (c *Client) probeConn(agent *ice.Agent, peer *config.Peer, gatherCh chan interface{}) error {
	peer.ConnectionState.Store(true)
	directContact := func(agent *ice.Agent, peer *config.Peer) error {
		//send NodeInfo
		candidates := GetCandidates(agent, gatherCh)
		directOffer := internal.NewDirectOffer(&internal.DirectOfferConfig{
			WgPort:     51820,
			Ufrag:      c.ufrag,
			Pwd:        c.pwd,
			LocalKey:   c.agentManager.GetLocalKey(),
			Candidates: candidates,
		})

		prober := c.proberManager.GetProber(peer.PublicKey)
		if prober == nil {
			c.proberMux.Lock()
			defer c.proberMux.Unlock()
			prober = probe.NewProber(&probe.ProberConfig{
				DirectOfferManager: c.drpClient,
				RelayOfferManager:  c.drpClient,
				AgentManager:       c.agentManager,
				WGConfiger:         c.proberManager.GetWgConfiger(),
				Key:                peer.PublicKey,
				ProberManager:      c.proberManager,
				IsForceRelay:       c.proberManager.IsForceRelay(),
				TurnClient:         c.turnClient,
				SignalingChannel:   c.signalChannel,
			})
			c.proberManager.AddProber(peer.PublicKey, prober)
		}

		limitRetries := 7
		retries := 0
		timer := time.NewTimer(1 * time.Second)
		for {
			if retries > limitRetries {
				return errors.New("direct check until limit times")
			}

			select {
			case <-timer.C:
				if prober.ConnectionState != internal.ConnectionStateConnected {
					if err := prober.Start(c.keyManager.GetPublicKey(), peer.PublicKey, directOffer); err != nil {
						klog.Errorf("send directOffer failed: %v", err)
					}
				}
				retries++
				timer.Reset(10 * time.Second)
			}

		}

		return nil
	}

	relayContact := func(peer *config.Peer) error {
		//send NodeInfo
		var err error

		relayInfo, err := c.turnClient.GetRelayInfo(true)
		if err != nil {
			return errors.New("get relay info failed")
		}

		relayAddr, err := turnclient.AddrToUdpAddr(relayInfo.RelayConn.LocalAddr())

		relayOffer := probe.NewOffer(relayInfo.MappedAddr, *relayAddr, c.agentManager.GetLocalKey(), probe.OfferTypeRelayOffer)

		dstPubKey := peer.PublicKey

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

		if err := prober.Start(c.keyManager.GetPublicKey(), dstPubKey, relayOffer); err != nil {
			klog.Errorf("send relayOffer failed: %v", err)
			return err
		}

		return nil
	}

	if c.proberManager.IsForceRelay() {
		go func() {
			err := relayContact(peer)
			if err != nil {

			}
		}()
	} else {
		go func() {
			err := directContact(agent, peer)
			if err != nil {
				klog.Errorf("directContact failed: %v", err)
				// use relay
				if err = relayContact(peer); err != nil {
					klog.Errorf("relayContact failed, unavaiable connect to peer: %v, %v", err, peer.PublicKey)
				}
			}
		}()
	}

	return nil
}

// TODO implement this function
func (c *Client) GetUsers() []*config.User {
	var users []*config.User
	users = append(users, config.NewUser("linkany", "123456"))
	return users
}

func (c *Client) Get(ctx context.Context) (*config.Peer, error) {
	req := &mgt.Request{
		AppId: c.conf.AppId,
	}

	body, err := proto.Marshal(req)
	if err != nil {
		return nil, err
	}

	msg, err := c.grpcClient.Get(ctx, &mgt.ManagementMessage{Body: body})
	if err != nil {
		return nil, err
	}

	var peer config.Peer
	if err := json.Unmarshal(msg.Body, &peer); err != nil {
		return nil, err
	}
	return &peer, nil
}

func (c *Client) Watch(ctx context.Context, callback func(msg *mgt.WatchMessage) error) error {

	req := &mgt.Request{
		PubKey: c.keyManager.GetPublicKey(),
	}

	body, err := proto.Marshal(req)
	if err != nil {
		return err
	}

	return c.grpcClient.Watch(ctx, &mgt.ManagementMessage{Body: body}, callback)
}

func (c *Client) Keepalive(ctx context.Context) error {
	req := &mgt.Request{
		PubKey: c.keyManager.GetPublicKey(),
		Token:  c.conf.Token,
	}

	body, err := proto.Marshal(req)
	if err != nil {
		return err
	}

	return c.grpcClient.Keepalive(ctx, &mgt.ManagementMessage{Body: body})
}

// Register will register device to linkany center
func (c *Client) Register(privateKey, publicKey, token string) (*config.DeviceConf, error) {
	var err error
	ctx := context.Background()

	hostname, err := os.Hostname()
	if err != nil {
		klog.Errorf("get hostname failed: %v", err)
		return nil, err
	}

	local, err := config.GetLocalConfig()
	if err != nil && err != io.EOF {
		return nil, err
	}
	registryRequest := &grpcserver.RegistryRequest{
		Token:               token,
		Hostname:            hostname,
		AppID:               local.AppId,
		PersistentKeepalive: 25,
		PrivateKey:          privateKey,
		PublicKey:           publicKey,
		Ufrag:               c.ufrag,
		Pwd:                 c.pwd,
		Port:                51820,
		Status:              1,
	}
	body, err := json.Marshal(registryRequest)
	if err != nil {
		return nil, err
	}
	_, err = c.grpcClient.Registry(ctx, &mgt.ManagementMessage{
		Body: body,
	})

	if err != nil {
		return nil, err
	}
	return &config.DeviceConf{}, nil
}
