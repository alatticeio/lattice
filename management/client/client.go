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
	"linkany/management/vo"
	"linkany/pkg/config"
	"linkany/pkg/linkerrors"
	"linkany/pkg/log"
	turnclient "linkany/turn/client"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/linkanyio/ice"
)

type NodeMap struct {
	lock sync.Mutex
	m    map[string]ice.Candidate
}

// Client is control client of linkany, will fetch config from origin server interval
type Client struct {
	as           internal.AgentManagerFactory
	logger       *log.Logger
	keyManager   internal.KeyManager
	nodeManager  *internal.NodeManager
	conf         *config.LocalConfig
	grpcClient   *mgtclient.Client
	conn4        net.PacketConn
	agentManager internal.AgentManagerFactory
	offerHandler internal.OfferHandler
	probeManager internal.ProbeManager
	turnManager  *turnclient.TurnManager
	engine       internal.EngineManager

	//channel for close for keepalive
	keepaliveChan chan struct{}
	watchChan     chan struct{}
}

type ClientConfig struct {
	Logger     *log.Logger
	Conf       *config.LocalConfig
	GrpcClient *mgtclient.Client
}

func NewClient(cfg *ClientConfig) *Client {
	client := &Client{
		logger:        cfg.Logger,
		conf:          cfg.Conf,
		grpcClient:    cfg.GrpcClient,
		keepaliveChan: make(chan struct{}),
		watchChan:     make(chan struct{}),
	}

	return client
}

func (c *Client) SetKeyManager(manager internal.KeyManager) *Client {
	c.keyManager = manager
	return c
}

func (c *Client) SetNodeManager(manager *internal.NodeManager) *Client {
	c.nodeManager = manager
	return c
}

func (c *Client) SetProbeManager(manager internal.ProbeManager) *Client {
	c.probeManager = manager
	return c
}

func (c *Client) SetEngine(engine internal.EngineManager) *Client {
	c.engine = engine
	return c
}

func (c *Client) SetOfferHandler(handler internal.OfferHandler) *Client {
	c.offerHandler = handler
	return c
}

func (c *Client) SetTurnManager(turnManager *turnclient.TurnManager) *Client {
	c.turnManager = turnManager
	return c
}

// RegisterToManagement will register device to linkany center
func (c *Client) RegisterToManagement() (*internal.DeviceConf, error) {
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

func (c *Client) ToConfigPeer(peer *internal.NodeMessage) *internal.NodeMessage {

	return &internal.NodeMessage{
		PublicKey:           peer.PublicKey,
		Endpoint:            peer.Endpoint,
		Address:             peer.Address,
		AllowedIPs:          peer.AllowedIPs,
		PersistentKeepalive: peer.PersistentKeepalive,
		ConnectType:         peer.ConnectType,
	}
}

func (c *Client) HandleWatchMessage(msg *internal.Message) error {
	var err error

	switch msg.EventType {
	case internal.EventTypeGroupNodeRemove:
		for _, node := range msg.GroupMessage.Nodes {
			c.logger.Infof("watch received event type: %v, node: %v", internal.EventTypeGroupNodeRemove, node.String())
			err := c.RemovePeer(node)
			if err != nil {
				c.logger.Errorf("remove node failed: %v", err)
			}
		}
	case internal.EventTypeGroupNodeAdd:
		for _, node := range msg.GroupMessage.Nodes {
			c.logger.Infof("watch received event type: %v, node: %v", internal.EventTypeGroupNodeAdd, node.String())
			if err = c.AddPeer(node); err != nil {
				c.logger.Errorf("add node failed: %v", err)
			}
		}
	case internal.EventTypeGroupAdd:
		c.logger.Verbosef("watching received event type: %v >>> add group: %v", internal.EventTypeGroupAdd, msg.GroupMessage.GroupName)
	}

	return nil

}

func (c *Client) AddPeer(p *internal.NodeMessage) error {
	var (
		err   error
		probe internal.Probe
	)
	if p.PublicKey == c.keyManager.GetPublicKey() {
		c.logger.Verbosef("current node, skipping...")
		return nil
	}

	node := c.ToConfigPeer(p)
	// start probe when gather candidates finished
	var connectType internal.ConnectType
	current := c.nodeManager.GetPeer(c.keyManager.GetPublicKey())
	if current.ConnectType == internal.DrpType || node.ConnectType == internal.DrpType {
		connectType = internal.DrpType
	} else if current.ConnectType == internal.RelayType || node.ConnectType == internal.RelayType {
		connectType = internal.RelayType
	} else {
		connectType = internal.DirectType
	}

	probe = c.probeManager.GetProbe(p.PublicKey)
	if probe != nil {
		switch probe.GetConnState() {
		case internal.ConnectionStateConnected:
			return nil
		case internal.ConnectionStateChecking:
			return nil
		}
	} else {
		if probe, err = c.probeManager.NewProbe(&internal.ProbeConfig{
			Logger:        c.logger,
			ProberManager: c.probeManager,
			GatherChan:    make(chan interface{}),
			WGConfiger:    c.engine.GetWgConfiger(),
			NodeManager:   c.nodeManager,
			To:            p.PublicKey,
			OfferHandler:  c.offerHandler,
			TurnManager:   c.turnManager,
			ConnectType:   connectType,
		}); err != nil {
			return err
		}
	}

	mappedPeer := c.nodeManager.GetPeer(node.PublicKey)
	if mappedPeer == nil {
		mappedPeer = node
		c.nodeManager.AddPeer(node.PublicKey, node)
		c.logger.Verbosef("add node to local cache, key: %s, node: %v", node.PublicKey, node)
	}

	go c.doProbe(probe, node)
	return nil
}

// doProbe will start a direct check to the node, if the peer is not connected, it will send drp offer to remote
func (c *Client) doProbe(probe internal.Probe, node *internal.NodeMessage) {
	errChan := make(chan error, 10)
	limitRetries := 7
	retries := 0
	timer := time.NewTimer(1 * time.Second)

	check := func() {
		for {
			if retries > limitRetries {
				c.logger.Errorf("direct check until limit times")
				errChan <- linkerrors.ErrProbeFailed
				return
			}

			select {
			case <-timer.C:
				switch probe.GetConnState() {
				case internal.ConnectionStateConnected, internal.ConnectionStateFailed:
					return
				default:
					switch probe.GetConnState() {
					case internal.ConnectionStateChecking:
						c.logger.Verbosef("node %s is checking, skip direct check", node.PublicKey)
					case internal.ConnectionStateNew:
						if err := probe.Start(context.Background(), c.keyManager.GetPublicKey(), node.PublicKey); err != nil {
							c.logger.Errorf("send directOffer failed: %v", err)
							err = linkerrors.ErrProbeFailed
							return
						} else if probe.GetConnState() != internal.ConnectionStateConnected {
							retries++
							timer.Reset(30 * time.Second)
						}

					case internal.ConnectionStateDisconnected:
						c.logger.Verbosef("node %s is disconnected, retry direct check", node.PublicKey)
						retries++
						timer.Reset(30 * time.Second)
					case internal.ConnectionStateConnected:
						c.logger.Verbosef("node %s is already connected, skip direct check", node.PublicKey)
					}
				}
			case <-probe.ProbeDone():
				errChan <- linkerrors.ErrProbeFailed
				return
			}
		}
	}

	// do check
	check()

	if err := <-errChan; err != nil {
		c.logger.Errorf("probe direct failed: %v", err)
		probe.SetConnectType(internal.DrpType)
		check()

		if err := <-errChan; err != nil {
			c.logger.Errorf("probe drp failed: %v", err)
		}
		return
	}
}

// TODO implement this function
func (c *Client) GetUsers() []*config.User {
	var users []*config.User
	users = append(users, config.NewUser("linkany", "123456"))
	return users
}

func (c *Client) Get(ctx context.Context) (*internal.NodeMessage, int64, error) {
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
		Peer  internal.NodeMessage
		Count int64
	}
	var result Result
	if err := json.Unmarshal(msg.Body, &result); err != nil {
		return nil, -1, err
	}
	return &result.Peer, result.Count, nil
}

func (c *Client) Watch(ctx context.Context, callback func(msg *internal.Message) error) error {
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
func (c *Client) Register(privateKey, publicKey, token string) (*internal.DeviceConf, error) {
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
	return &internal.DeviceConf{}, nil
}

func (c *Client) RemovePeer(node *internal.NodeMessage) error {
	wgConfigure := c.engine.GetWgConfiger()
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
