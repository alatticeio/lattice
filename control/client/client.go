package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/linkanyio/ice"
	"github.com/pion/logging"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"io"
	"k8s.io/klog/v2"
	pb "linkany/control/grpc/peer"
	"linkany/internal"
	"linkany/pkg/config"
	"linkany/pkg/drp"
	"linkany/pkg/linkerrors"
	"linkany/pkg/probe"
	turnclient "linkany/turn/client"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type ClientInterface interface {
	// Register will register a device to linkany center
	Register() (*config.DeviceConf, error)

	Login(user *config.User) (*config.User, error)

	FetchPeers() (*config.DeviceConf, error)

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
	grpcClient      pb.ListWatcherClient
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
	GrpcClient      pb.ListWatcherClient
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
	klog.Infof("start register")
	hostname, err := os.Hostname()
	if err != nil {
		klog.Errorf("get hostname failed: %v", err)
		return nil, err
	}

	info1, err := doRequest(c, "GET", fmt.Sprintf("%s/api/v1/peer/appId/%s", ConsoleDomain, c.conf.AppId), nil, &config.PeerRegisterInfo{}, nil)
	if err != nil {
		return nil, err
	}
	var peer *config.PeerRegisterInfo
	peer = &config.PeerRegisterInfo{
		AppId:               c.conf.AppId,
		Hostname:            hostname,
		PersistentKeepalive: 25,
		Ufrag:               c.conf.Ufrag,
		Pwd:                 c.conf.Pwd,
		TieBreaker:          c.TieBreaker,
	}

	if info1.ID != "" {
		peer.ID = info1.ID
		if info1.PrivateKey != "" {
			privateKey, _ := wgtypes.ParseKey(info1.PrivateKey)
			if privateKey.String() != "" {
				c.km.UpdateKey(privateKey)
			}
		} else {
			privateKey := c.km.GetKey()
			if err != nil {
				klog.Errorf("generate private key failed: %v", err)
				return nil, err
			}
			peer.PrivateKey = privateKey.String()
			peer.PublicKey = privateKey.PublicKey().String()
		}
	}

	jsonStr, err := json.Marshal(peer)
	if err != nil {
		klog.Errorf("marshal peer failed: %v", err)
		return nil, err
	}

	data := bytes.NewBuffer(jsonStr)
	request, err := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/peer/regUpdate", ConsoleDomain), data)
	request.Header.Add("TOKEN", c.conf.Token)
	request.Header.Add("Content-Type", "application/json")
	response, err := c.httpClient.Do(request)

	if response.StatusCode != http.StatusOK {
		klog.Errorf("register failed: %v, code: %v", err, response.StatusCode)
		return nil, err
	}

	defer response.Body.Close()
	bs, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	var resp HttpResponse[config.Peer]
	err = json.Unmarshal(bs, &resp)
	if err != nil {
		return nil, err
	}

	if resp.Code == 403 {
		if err := c.RefreshToken(); err != nil {
			klog.Errorf("refresh token failed: %v", err)
		}
		return nil, fmt.Errorf("register failed: %v, now refreshed, please start again", resp.Message)
	}

	klog.Infof("register success!!!")

	// get conf
	conf, err := doRequest(c, "GET", fmt.Sprintf("%s/api/v1/peer/node/%s", ConsoleDomain, c.conf.AppId), nil, &config.DeviceConf{}, nil)
	if err != nil {
		return nil, err
	}

	return conf, nil
}
func (r HttpResponse[T]) get() T {
	return r.Data
}

func (c *Client) Login(user *config.User) (*config.User, error) {
	// login to linkany center
	var result = &config.User{}
	var resp *HttpResponse[config.LoginInfo]
	var err error
	resp, err = c.post(fmt.Sprintf("%s/api/v1/user/login", ConsoleDomain), user)
	loginToken := resp.get()
	if err != nil {
		return nil, err
	}

	homeDir, err := os.UserHomeDir()
	path := filepath.Join(homeDir, ".linkany/config.json")
	_, err = os.Stat(path)
	var file *os.File
	if os.IsNotExist(err) {
		parentDir := filepath.Dir(path)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return nil, err
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
		return nil, err
	}

	appId, err := config.GetAppId()

	ufrag, pwd, err := internal.GenerateUfragPwd()
	if err != nil {
		return nil, err
	}

	b := &config.LocalConfig{
		Auth:  fmt.Sprintf("%s:%s", user.Username, config.StringToBase64(user.Password)),
		AppId: appId,
		Token: loginToken.Token,
		Ufrag: ufrag,
		Pwd:   pwd,
	}

	err = config.UpdateLocalConfig(b)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *Client) post(url string, inObject interface{}) (*HttpResponse[config.LoginInfo], error) {
	var err error
	var jsonStr []byte
	jsonStr, err = json.Marshal(inObject)
	if err != nil {
		return nil, err
	}

	data := bytes.NewBuffer(jsonStr)
	response, err := c.httpClient.Post(url, "application/json", data)

	if response.StatusCode != http.StatusOK {
		return nil, err
	} else {
		defer response.Body.Close()
		bs, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}

		var resp HttpResponse[config.LoginInfo]
		err = json.Unmarshal(bs, &resp)
		if err != nil {
			return nil, err
		}

		return &resp, nil
	}

	return nil, nil
}

// FetchPeers fetch user's all peer and configuration to linkany instance
func (c *Client) FetchPeers() (*config.DeviceConf, error) {
	var conf *config.DeviceConf
	var err error
	appId, err := config.GetAppId()
	if err != nil {
		return nil, err
	}

	conf, err = doRequest(c, "GET", fmt.Sprintf("%s/api/v1/peer/node/%s", ConsoleDomain, appId), nil, &config.DeviceConf{}, nil)

	if errors.Is(err, linkerrors.ErrInvalidToken) {
		c.RefreshToken()
		return nil, fmt.Errorf("fetch peers failed: %v, now refreshed", err)
	}

	for _, peer := range conf.Peers {
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

func (c *Client) RefreshToken() error {
	var err error
	var conf *config.LocalConfig
	conf, err = config.GetLocalConfig()
	if err != nil {
		return err
	}
	strs := strings.Split(conf.Auth, ":")
	password, err := config.Base64Decode(strs[1])
	if err != nil {
		return err
	}
	user := config.User{
		Username: strs[0],
		Password: password,
	}

	loginToken, err := doRequest(c, "POST", fmt.Sprintf("%s/api/v1/user/refresh", ConsoleDomain), user, &config.LoginInfo{}, nil)
	if err != nil {
		return err
	}

	b := &config.LocalConfig{
		Auth:  conf.Auth,
		AppId: conf.AppId,
		Token: loginToken.Token,
	}

	err = config.UpdateLocalConfig(b)
	if err != nil {
		return err
	}

	return nil
}

// doRequest user T to doRequest http request, return the actual response
func doRequest[T any](c *Client, method, url string, inObject interface{}, t *T, mapHeader map[string]string) (*T, error) {
	var err error
	var jsonStr []byte
	jsonStr, err = json.Marshal(inObject)
	if err != nil {
		return nil, err
	}

	data := bytes.NewBuffer(jsonStr)
	request, err := http.NewRequest(method, url, data)
	if err != nil {
		return nil, err
	}

	if mapHeader != nil {
		for k, v := range mapHeader {
			request.Header.Add(k, v)
		}
	}

	conf, err := config.GetLocalConfig()
	if err != nil {
		return nil, err
	}

	// default add
	request.Header.Add("TOKEN", conf.Token)
	request.Header.Add("Content-Type", "application/json")
	response, err := c.httpClient.Do(request)

	if err != nil {
		return nil, err
	}

	switch response.StatusCode {
	case http.StatusOK:
		defer response.Body.Close()
		bs, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}

		var resp HttpResponse[T]
		err = json.Unmarshal(bs, &resp)
		if err != nil {
			return nil, err
		}

		*t = resp.get()
		return t, nil
	case http.StatusForbidden:
		return nil, linkerrors.ErrInvalidToken
	default:
		defer response.Body.Close()
		bs, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}
		return nil, errors.New("code:" + string(bs))
	}
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
