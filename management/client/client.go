package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
	"wireflow/internal/core/domain"
	"wireflow/internal/grpc"
	"wireflow/management/dto"
	grpclient "wireflow/management/grpc/client"
	"wireflow/pkg/config"
	"wireflow/pkg/log"
	turnclient "wireflow/pkg/turn"
	"wireflow/pkg/wferrors"

	"github.com/golang/protobuf/proto"
	"github.com/wireflowio/ice"
)

type NodeMap struct {
	lock sync.Mutex
	m    map[string]ice.Candidate
}

var (
	_ domain.ManagementClient = (*Client)(nil)
)

// Client is control client of wireflow, will fetch config from origin server interval
type Client struct {
	//as           domain.AgentManagerFactory
	logger       *log.Logger
	keyManager   domain.KeyManager
	nodeManager  domain.PeerManager
	conf         *config.Config
	grpcClient   *grpclient.Client
	conn4        net.PacketConn
	offerHandler domain.OfferHandler
	probeManager domain.ProberManager
	turnManager  *turnclient.TurnManager
	client       domain.Client

	//channel for close for keepalive
	keepaliveChan chan struct{}
	watchChan     chan struct{}
}

type ClientConfig struct {
	Logger        *log.Logger
	ManagementUrl string
	KeepaliveChan chan struct{}
	WatchChan     chan struct{}
	GrpcClient    *grpclient.Client
}

// NewClient will create a new client for connect grpc management server
func NewClient(cfg *ClientConfig) *Client {
	client := &Client{
		logger:        cfg.Logger,
		keepaliveChan: make(chan struct{}),
		watchChan:     make(chan struct{}),
	}

	c, err := grpclient.NewClient(&grpclient.GrpcConfig{
		Addr:          cfg.ManagementUrl,
		Logger:        log.NewLogger(log.Loglevel, "grpc-grpclient"),
		KeepaliveChan: cfg.KeepaliveChan,
		WatchChan:     cfg.WatchChan,
	})

	if err != nil {
		client.logger.Errorf("create grpc client failed: %v", err)
		return nil
	}

	client.grpcClient = c

	return client
}

func (c *Client) SetKeyManager(manager domain.KeyManager) *Client {
	c.keyManager = manager
	return c
}

func (c *Client) SetNodeManager(manager domain.PeerManager) *Client {
	c.nodeManager = manager
	return c
}

type ClientOption func(*Client) error

//func WithAgentManagerFactory(factory domain.AgentManagerFactory) ClientOption {
//	return func(c *Client) error {
//		c.agentManager = factory
//		return nil
//	}
//}

func WithGrpcClient(client *grpclient.Client) ClientOption {
	return func(c *Client) error {
		c.grpcClient = client
		return nil
	}
}

func NewClientWithOption(cfg *ClientConfig, opts ...ClientOption) (*Client, error) {
	client := NewClient(cfg)
	for _, opt := range opts {
		if err := opt(client); err != nil {
			return nil, err
		}
	}
	return client, nil
}

func WithNodeManager(manager domain.PeerManager) ClientOption {
	return func(c *Client) error {
		c.nodeManager = manager
		return nil
	}
}

func WithProbeManager(manager domain.ProberManager) ClientOption {
	return func(c *Client) error {
		c.probeManager = manager
		return nil
	}
}

func WithOfferHandler(handler domain.OfferHandler) ClientOption {
	return func(c *Client) error {
		c.offerHandler = handler
		return nil
	}
}

func WithKeyManager(manager domain.KeyManager) ClientOption {
	return func(c *Client) error {
		c.keyManager = manager
		return nil
	}
}

func WithTurnManager(manager *turnclient.TurnManager) ClientOption {
	return func(c *Client) error {
		c.turnManager = manager
		return nil
	}
}

func WithIClient(iclient domain.Client) ClientOption {
	return func(c *Client) error {
		c.client = iclient
		return nil
	}
}

func (c *Client) Configure(opts ...ClientOption) error {
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) Login(user *config.User) error {
	var err error
	ctx := context.Background()
	loginRequest := &grpc.LoginRequest{
		Username: user.Username,
		Password: user.Password,
	}

	body, err := proto.Marshal(loginRequest)
	if err != nil {
		return err
	}
	resp, err := c.grpcClient.Login(ctx, &grpc.ManagementMessage{
		Body: body,
	})

	if err != nil {
		return err
	}

	var loginResponse grpc.LoginResponse
	if err := proto.Unmarshal(resp.Body, &loginResponse); err != nil {
		return err
	}

	homeDir, err := os.UserHomeDir()
	path := filepath.Join(homeDir, ".wireflow/config.json")
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
	var local config.Config
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&local)
	if err != nil && err != io.EOF {
		return err
	}

	appId := config.GlobalConfig.AppId

	//b := &config.Config{
	//	Auth:  fmt.Sprintf("%s:%s", user.Username, config.StringToBase64(user.Password)),
	//	AppId: appId,
	//	Token: loginResponse.Token,
	//}

	fmt.Printf("appId: %s\n", appId)
	return nil
}

// GetNetMap get current node network map
func (c *Client) GetNetMap() (*domain.Message, error) {
	ctx := context.Background()
	var err error

	request := &grpc.Request{
		AppId:  c.conf.AppId,
		PubKey: c.keyManager.GetPublicKey(),
	}

	body, err := proto.Marshal(request)
	if err != nil {
		return nil, err
	}

	resp, err := c.grpcClient.GetNetMap(ctx, &grpc.ManagementMessage{
		Body: body,
	})

	if err != nil {
		return nil, err
	}

	var msg domain.Message
	if err = json.Unmarshal(resp.Body, &msg); err != nil {
		return nil, err
	}

	return &msg, nil
}

// TODO implement this function
func (c *Client) GetUsers() []*config.User {
	var users []*config.User
	users = append(users, config.NewUser("wireflow", "123456"))
	return users
}

func (c *Client) ToConfigPeer(peer *domain.Peer) *domain.Peer {

	return &domain.Peer{
		PublicKey:           peer.PublicKey,
		Endpoint:            peer.Endpoint,
		Address:             peer.Address,
		AllowedIPs:          peer.AllowedIPs,
		PersistentKeepalive: peer.PersistentKeepalive,
		ConnectType:         peer.ConnectType,
	}
}

func (c *Client) AddPeer(p *domain.Peer) error {
	var (
		err   error
		probe domain.Prober
	)
	if p.PublicKey == c.keyManager.GetPublicKey() {
		c.logger.Verbosef("current node, skipping...")
		return nil
	}

	node := c.ToConfigPeer(p)
	// start probe when gather candidates finished
	var connectType domain.ConnType
	current := c.nodeManager.GetPeer(c.keyManager.GetPublicKey())
	if current.ConnectType == domain.DrpType || node.ConnectType == domain.DrpType {
		connectType = domain.DrpType
	} else if current.ConnectType == domain.RelayType || node.ConnectType == domain.RelayType {
		connectType = domain.RelayType
	} else {
		connectType = domain.DirectType
	}

	probe = c.probeManager.GetProbe(p.PublicKey)
	if probe != nil {
		switch probe.GetConnState() {
		case domain.ConnectionStateConnected:
			return nil
		case domain.ConnectionStateChecking:
			return nil
		}
	} else {
		if probe, err = c.probeManager.NewProbe(&domain.ProbeConfig{
			Logger:        c.logger,
			ProberManager: c.probeManager,
			GatherChan:    make(chan interface{}),
			WGConfiger:    c.client.GetDeviceConfiger(),
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
func (c *Client) doProbe(probe domain.Prober, node *domain.Peer) {
	errChan := make(chan error, 10)
	limitRetries := 7
	retries := 0
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	check := func() {
		for {
			if retries > limitRetries {
				c.logger.Errorf("direct check until limit times")
				errChan <- wferrors.ErrProbeFailed
				return
			}

			select {
			case <-ticker.C:
				switch probe.GetConnState() {
				case domain.ConnectionStateConnected, domain.ConnectionStateFailed:
					return
				default:
					switch probe.GetConnState() {
					case domain.ConnectionStateChecking:
						c.logger.Verbosef("node %s is checking, skip direct check", node.PublicKey)
					case domain.ConnectionStateNew:
						if err := probe.Start(context.Background(), c.keyManager.GetPublicKey(), node.PublicKey); err != nil {
							c.logger.Errorf("send directOffer failed: %v", err)
							err = wferrors.ErrProbeFailed
							return
						} else if probe.GetConnState() != domain.ConnectionStateConnected {
							retries++
							ticker.Reset(30 * time.Second)
						}

					case domain.ConnectionStateDisconnected:
						c.logger.Verbosef("node %s is disconnected, retry direct check", node.PublicKey)
						retries++
						ticker.Reset(30 * time.Second)
					case domain.ConnectionStateConnected:
						c.logger.Verbosef("node %s is already connected, skip direct check", node.PublicKey)
					}
				}
			case <-probe.ProbeDone():
				errChan <- wferrors.ErrProbeFailed
				return
			}
		}
	}

	// do check
	check()

	if err := <-errChan; err != nil {
		c.logger.Errorf("probe direct failed: %v", err)
		probe.SetConnectType(domain.DrpType)
		check()

		if err := <-errChan; err != nil {
			c.logger.Errorf("probe drp failed: %v", err)
		}
		return
	}
}

func (c *Client) Get(ctx context.Context) (*domain.Peer, int64, error) {
	req := &grpc.Request{
		AppId: c.conf.AppId,
		Token: c.conf.Token,
	}

	body, err := proto.Marshal(req)
	if err != nil {
		return nil, -1, err
	}

	msg, err := c.grpcClient.Get(ctx, &grpc.ManagementMessage{Body: body})
	if err != nil {
		return nil, -1, err
	}

	type Result struct {
		Peer  domain.Peer
		Count int64
	}
	var result Result
	if err := json.Unmarshal(msg.Body, &result); err != nil {
		return nil, -1, err
	}
	return &result.Peer, result.Count, nil
}

func (c *Client) Watch(ctx context.Context, fn func(message *domain.Message) error) error {
	req := &grpc.Request{
		PubKey: c.keyManager.GetPublicKey(),
		AppId:  c.conf.AppId,
	}

	body, err := proto.Marshal(req)
	if err != nil {
		return err
	}

	return c.grpcClient.Watch(ctx, &grpc.ManagementMessage{Body: body}, fn)
}

func (c *Client) Keepalive(ctx context.Context) error {
	req := &grpc.Request{
		PubKey: c.keyManager.GetPublicKey(),
		AppId:  c.conf.AppId,
		Token:  c.conf.Token,
	}

	body, err := proto.Marshal(req)
	if err != nil {
		return err
	}

	return c.grpcClient.Keepalive(ctx, &grpc.ManagementMessage{Body: body})
}

// Register will register device to wireflow center
func (c *Client) Register(ctx context.Context, interfaceName string) (*domain.Peer, error) {
	var err error

	hostname, err := os.Hostname()
	if err != nil {
		c.logger.Errorf("get hostname failed: %v", err)
		return nil, err
	}

	registryRequest := &dto.NodeDto{
		Hostname:            hostname,
		InterfaceName:       interfaceName,
		Platform:            runtime.GOOS,
		AppID:               config.GlobalConfig.AppId,
		PersistentKeepalive: 25,
		Port:                51820,
		Status:              1,
	}
	body, err := json.Marshal(registryRequest)
	if err != nil {
		return nil, err
	}
	resp, err := c.grpcClient.Registry(ctx, &grpc.ManagementMessage{
		Body: body,
	})

	if err != nil {
		return nil, fmt.Errorf("register failed. %v", err)
	}

	var node domain.Peer
	if err = json.Unmarshal(resp.Body, &node); err != nil {
		return nil, err
	}

	return &node, nil
}

func (c *Client) Request(ctx context.Context, message *grpc.ManagementMessage) (*grpc.ManagementMessage, error) {
	return c.grpcClient.Request(ctx, message)
}
