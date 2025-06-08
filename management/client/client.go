package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"linkany/internal"
	mgtclient "linkany/management/grpc/client"
	"linkany/management/grpc/mgt"
	grpcserver "linkany/management/grpc/server"
	"linkany/management/utils"
	"linkany/management/vo"
	"linkany/pkg/config"
	"linkany/pkg/linkerrors"
	"linkany/pkg/log"
	"linkany/signaling/grpc/signaling"
	turnclient "linkany/turn/client"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/linkanyio/ice"
)

type PeerMap struct {
	lock sync.Mutex
	m    map[string]ice.Candidate
}

// Client is control client of linkany, will fetch config from origin server interval
type Client struct {
	as              internal.AgentManagerFactory
	logger          *log.Logger
	keyManager      internal.KeyManager
	signalChannel   chan *signaling.SignalingMessage
	peersManager    *config.NodeManager
	stunUri         string
	ifaceName       string
	conf            *config.LocalConfig
	grpcClient      *mgtclient.Client
	agent           *ice.Agent
	conn4           net.PacketConn
	udpMux          *ice.UDPMuxDefault
	universalUdpMux *ice.UniversalUDPMuxDefault
	update          func() error
	agentManager    internal.AgentManagerFactory
	offerHandler    internal.OfferHandler
	probeManager    internal.ProbeManager
	proberMux       sync.Mutex
	turnClient      *turnclient.Client
	wgConfigure     internal.ConfigureManager
	ufrag           string
	pwd             string
}

type ClientConfig struct {
	Logger          *log.Logger
	PeersManager    *config.NodeManager
	Conf            *config.LocalConfig
	Agent           *ice.Agent
	UdpMux          *ice.UDPMuxDefault
	UniversalUdpMux *ice.UniversalUDPMuxDefault
	KeyManager      internal.KeyManager
	AgentManager    internal.AgentManagerFactory
	GrpcClient      *mgtclient.Client
	ProberManager   internal.ProbeManager
	TurnClient      *turnclient.Client
	SignalChannel   chan *signaling.SignalingMessage
	OfferHandler    internal.OfferHandler
	WgConfiger      internal.ConfigureManager
}

func NewClient(cfg *ClientConfig) *Client {
	client := &Client{
		logger:          cfg.Logger,
		offerHandler:    cfg.OfferHandler,
		keyManager:      cfg.KeyManager,
		conf:            cfg.Conf,
		peersManager:    cfg.PeersManager,
		udpMux:          cfg.UdpMux,
		universalUdpMux: cfg.UniversalUdpMux,
		agentManager:    cfg.AgentManager,
		probeManager:    cfg.ProberManager,
		turnClient:      cfg.TurnClient,
		grpcClient:      cfg.GrpcClient,
		signalChannel:   cfg.SignalChannel,
		wgConfigure:     cfg.WgConfiger,
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

	b := &config.LocalConfig{
		Auth:  fmt.Sprintf("%s:%s", user.Username, config.StringToBase64(user.Password)),
		AppId: appId,
		Token: loginResponse.Token,
	}

	err = config.UpdateLocalConfig(b)
	if err != nil {
		return err
	}

	return nil
}

// GetNetMap get current node network map
func (c *Client) GetNetMap() (*vo.NetworkMap, error) {
	ctx := context.Background()
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

	resp, err := c.grpcClient.GetNetMap(ctx, &mgt.ManagementMessage{
		Body: body,
	})

	if err != nil {
		return nil, err
	}

	var networkMap vo.NetworkMap
	if err := json.Unmarshal(resp.Body, &networkMap); err != nil {
		return nil, err
	}

	for _, p := range networkMap.Nodes {
		if err := c.AddPeer(p); err != nil {
			c.logger.Errorf("add peer failed: %v", err)
		}
	}

	return &networkMap, nil
}

func (c *Client) ToConfigPeer(peer *utils.NodeMessage) *utils.NodeMessage {

	return &utils.NodeMessage{
		PublicKey:           peer.PublicKey,
		Endpoint:            peer.Endpoint,
		Address:             peer.Address,
		AllowedIPs:          peer.AllowedIPs,
		PersistentKeepalive: peer.PersistentKeepalive,
	}
}

func (c *Client) HandleWatchMessage(msg *utils.Message) error {
	var err error

	switch msg.EventType {
	case utils.EventTypeGroupNodeRemove:
		for _, node := range msg.GroupMessage.Nodes {
			c.logger.Infof("watch received event type: %v, node: %v", utils.EventTypeGroupNodeRemove, node.String())
			err := c.RemovePeer(node)
			if err != nil {
				c.logger.Errorf("remove node failed: %v", err)
			}
		}
	case utils.EventTypeGroupNodeAdd:
		for _, node := range msg.GroupMessage.Nodes {
			c.logger.Infof("watch received event type: %v, node: %v", utils.EventTypeGroupNodeAdd, node.String())
			if err = c.AddPeer(node); err != nil {
				c.logger.Errorf("add node failed: %v", err)
			}
		}
	case utils.EventTypeGroupAdd:
		c.logger.Verbosef("watching received event type: %v >>> add group: %v", utils.EventTypeGroupAdd, msg.GroupMessage.GroupName)
	}

	return nil

}

func (c *Client) AddPeer(p *utils.NodeMessage) error {
	var (
		err   error
		probe internal.Probe
	)
	if p.PublicKey == c.keyManager.GetPublicKey() {
		c.logger.Verbosef("current node, skipping...")
		return nil
	}

	probe = c.GetProber(p.PublicKey)
	if probe != nil {
		switch probe.GetConnState() {
		case internal.ConnectionStateConnected:
			return nil
		case internal.ConnectionStateChecking:
			return nil
		}
	} else {
		if probe, err = c.probeManager.NewProbe(&internal.ProberConfig{
			Logger:        c.logger,
			StunUri:       c.stunUri,
			ProberManager: c.probeManager,
			GatherChan:    make(chan interface{}),
			OfferManager:  c.offerHandler,
			WGConfiger:    c.wgConfigure,
			NodeManager:   c.peersManager,
			Ufrag:         c.ufrag,
			Pwd:           c.pwd,
			To:            p.PublicKey,
		}); err != nil {
			return err
		}
	}

	peer := c.ToConfigPeer(p)
	mappedPeer := c.peersManager.GetPeer(peer.PublicKey)
	if mappedPeer == nil {
		mappedPeer = peer
		c.peersManager.AddPeer(peer.PublicKey, peer)
		c.logger.Verbosef("add peer to local cache, key: %s, peer: %v", peer.PublicKey, peer)
	}
	// start probe when gather candidates finished
	go c.doProbe(probe, peer)
	return nil
}

func (c *Client) doProbe(prober internal.Probe, peer *utils.NodeMessage) {
	errChan := make(chan error, 1)
	limitRetries := 7
	retries := 0
	timer := time.NewTimer(1 * time.Second)
	for {
		if retries > limitRetries {
			c.logger.Errorf("direct check until limit times")
			errChan <- linkerrors.ErrProbeFailed
			return
		}

		select {
		case <-timer.C:
			switch prober.GetConnState() {
			case internal.ConnectionStateConnected, internal.ConnectionStateFailed:
				return
			default:
				switch prober.GetConnState() {
				case internal.ConnectionStateChecking:
					if time.Since(prober.GetLastCheck()) > 30*time.Second {
						c.logger.Verbosef("peer %s is checking over 30s, retry direct check", peer.PublicKey)
						retries++
						prober.UpdateLastCheck()
						timer.Reset(1 * time.Second)
						continue
					} else {
						c.logger.Verbosef("peer %s is checking in 30s", peer.PublicKey)
					}
					c.logger.Verbosef("peer %s is checking, skip direct check", peer.PublicKey)
				case internal.ConnectionStateFailed, internal.ConnectionStateNew:
					if err := prober.Start(c.keyManager.GetPublicKey(), peer.PublicKey); err != nil {
						c.logger.Errorf("send directOffer failed: %v", err)
						err = linkerrors.ErrProbeFailed
						return
					} else if prober.GetConnState() != internal.ConnectionStateConnected {
						retries++
						timer.Reset(30 * time.Second)
					}
				case internal.ConnectionStateConnected:
					c.logger.Verbosef("peer %s is already connected, skip direct check", peer.PublicKey)
				}
			}
		case <-prober.ProbeDone():
			errChan <- linkerrors.ErrProbeFailed
			return
		}
	}

	if err := <-errChan; err != nil {
		c.logger.Errorf("probe failed: %v", err)
		return
	}
}

// TODO implement this function
func (c *Client) GetUsers() []*config.User {
	var users []*config.User
	users = append(users, config.NewUser("linkany", "123456"))
	return users
}

func (c *Client) Get(ctx context.Context) (*utils.NodeMessage, int64, error) {
	req := &mgt.Request{
		AppId: c.conf.AppId,
		Token: c.conf.Token,
	}

	body, err := proto.Marshal(req)
	if err != nil {
		return nil, -1, err
	}

	msg, err := c.grpcClient.Get(ctx, &mgt.ManagementMessage{Body: body})
	if err != nil {
		return nil, -1, err
	}

	type Result struct {
		Peer  utils.NodeMessage
		Count int64
	}
	var result Result
	if err := json.Unmarshal(msg.Body, &result); err != nil {
		return nil, -1, err
	}
	return &result.Peer, result.Count, nil
}

func (c *Client) Watch(ctx context.Context, callback func(msg *utils.Message) error) error {
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
		c.logger.Errorf("get hostname failed: %v", err)
		return nil, err
	}

	local, err := config.GetLocalConfig()
	if err != nil && err != io.EOF {
		return nil, err
	}
	registryRequest := &grpcserver.RegRequest{
		Token:               token,
		Hostname:            hostname,
		AppID:               local.AppId,
		PersistentKeepalive: 25,
		PrivateKey:          privateKey,
		PublicKey:           publicKey,
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

func (c *Client) RemovePeer(node *utils.NodeMessage) error {
	wgConfigure := c.probeManager.GetWgConfiger()
	if err := wgConfigure.RemovePeer(&internal.SetPeer{
		PublicKey: node.PublicKey,
		Remove:    true,
	}); err != nil {
		return err
	}

	//TODO add check when no same network peers exists, then delete the route.
	internal.SetRoute(c.logger)("delete", wgConfigure.GetAddress(), wgConfigure.GetIfaceName())
	return nil
}

func (c *Client) GetProber(pubKey string) internal.Probe {
	return c.probeManager.GetProbe(pubKey)
}
