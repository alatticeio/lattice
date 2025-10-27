package internal

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"wireflow/management/utils"
	"wireflow/pkg/log"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var lock sync.Mutex
var once sync.Once
var manager *WatchManager

// WatchManager is a singleton that manages the watch channels for connected nodes
// It is used to send messages to all connected nodes
// watcher is a map of networkId to watcher, a watcher is a struct that contains the networkId
// and the channel to send messages to
// m is a map of groupId_nodeId to channel, a channel is used to send messages to the connected peer
// The key is a combination of networkId and publicKey, which is used to identify the connected peer
type WatchManager struct {
	mu       sync.Mutex
	channels map[string]*NodeChannel // key: clientId, value: channel
	logger   *log.Logger
}

type NodeChannel struct {
	nu        sync.Mutex
	networkId []string
	channel   chan *Message // key: clientId, value: channel
}

func (n *NodeChannel) GetChannel() chan *Message {
	n.nu.Lock()
	defer n.nu.Unlock()
	if n.channel == nil {
		n.channel = make(chan *Message, 1000) // buffered channel
	}
	return n.channel
}

// GetChannel get channel by clientID`
func (w *WatchManager) GetChannel(clientId string) *NodeChannel {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.channels == nil {
		return nil
	}
	channel := w.channels[clientId]

	if channel == nil {
		channel = &NodeChannel{
			channel: make(chan *Message, 1000), // buffered channel
		}
	}
	w.channels[clientId] = channel
	return channel
}

func (w *WatchManager) Remove(clientID string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	channel := w.channels[clientID]
	if channel == nil {
		w.logger.Errorf("channel not found for clientID: %s", clientID)
		return
	}

	channel.nu.Lock()
	defer channel.nu.Unlock()
	close(channel.channel)
	delete(w.channels, clientID)
}

// NewWatchManager create a whole manager for connected nodes
func NewWatchManager() *WatchManager {
	lock.Lock()
	defer lock.Unlock()
	if manager != nil {
		return manager
	}
	once.Do(func() {
		manager = &WatchManager{
			channels: make(map[string]*NodeChannel),
			logger:   log.NewLogger(log.Loglevel, "watchmanager"),
		}
	})

	return manager
}

// Message is the message which is sent to connected nodes
type Message struct {
	EventType EventType `json:"eventType"`
	Current   *Node     `json:"node"`
	Network   *Network  `json:"network"`
}

// Node is the node information one client side
type Node struct {
	Name                string           `json:"name,omitempty"`
	Description         string           `json:"description,omitempty"`
	NetworkId           string           `json:"networkId,omitempty"` // belong to which group
	CreatedBy           string           `json:"createdBy,omitempty"` // ownerID
	UserId              uint64           `json:"userId,omitempty"`
	Hostname            string           `json:"hostname,omitempty"`
	AppID               string           `json:"appId,omitempty"`
	Address             string           `json:"address,omitempty"`
	Endpoint            string           `json:"endpoint,omitempty"`
	Remove              bool             `json:"remove,omitempty"` // whether to remove node
	PresharedKey        string           `json:"presharedKey,omitempty"`
	PersistentKeepalive int              `json:"persistentKeepalive,omitempty"`
	PrivateKey          string           `json:"privateKey,omitempty"`
	PublicKey           string           `json:"publicKey,omitempty"`
	AllowedIPs          string           `json:"allowedIps,omitempty"`
	ReplacePeers        bool             `json:"replacePeers,omitempty"` // whether to replace nodes when updating node
	Port                int              `json:"port"`
	Status              utils.NodeStatus `json:"status"`
	GroupName           string           `json:"groupName"`
	Version             uint64           `json:"version"`
	LastUpdatedAt       string           `json:"lastUpdatedAt"`

	//conn type
	DrpAddr     string      `json:"drpAddr,omitempty"`     // drp server address, if is drp node
	ConnectType ConnectType `json:"connectType,omitempty"` // DirectType, RelayType, DrpType
}

// Network is the network information, contains all nodes/policies in the network
type Network struct {
	Address     string    `json:"address"`
	AllowedIps  []string  `json:"allowedIps"`
	Port        int       `json:"port"`
	NetworkId   string    `json:"networkId"`
	NetworkName string    `json:"networkName"`
	Policies    []*Policy `json:"policies"`
	Nodes       []*Node   `json:"nodes"`
}

type Policy struct {
	PolicyName string  `json:"policyName"`
	Rules      []*Rule `json:"rules"`
}

type Rule struct {
	SourceType string `json:"sourceType"`
	TargetType string `json:"targetType"`
	SourceId   string `json:"sourceId"`
	TargetId   string `json:"targetId"`
}

func NewMessage() *Message {
	return &Message{}
}

func (m *Message) WithEventType(eventType EventType) *Message {
	m.EventType = eventType
	return m
}

func (m *Message) WithNode(node *Node) *Message {
	m.Current = node
	return m
}

func (m *Message) WithNetwork(network *Network) *Message {
	m.Network = network
	return m
}

func (n *Network) WithPolicy(policy *Policy) *Network {
	n.Policies = append(n.Policies, policy)
	return n
}

func (w *WatchManager) Send(clientId string, msg *Message) {
	channel := w.GetChannel(clientId)
	channel.channel <- msg
}

type EventType int

const (
	EventTypeJoinNetwork EventType = iota
	EventTypeLeaveNetwork
	EventTypeNodeUpdate
	EventTypeNodeAdd
	EventTypeNodeRemove
	EventTypeIPChange
	EventTypePolicyRuleAdd
	EventTypePolicyRuleChanged
	EventTypePolicyRuleRemove
)

func (e EventType) String() string {
	switch e {
	case EventTypeJoinNetwork:
		return "joinNetwork"
	case EventTypeLeaveNetwork:
		return "leaveNetwork"
	case EventTypeNodeUpdate:
		return "nodeUpdate"
	case EventTypeNodeAdd:
		return "nodeAdd"
	case EventTypeIPChange:
		return "ipChange"
	}
	return "unknown"
}

func (p *Node) String() string {
	keyf := func(value string) string {
		if value == "" {
			return ""
		}
		result, err := wgtypes.ParseKey(value)
		if err != nil {
			return ""
		}

		return hex.EncodeToString(result[:])
	}

	printf := func(sb *strings.Builder, key, value string, keyf func(string) string) {

		if keyf != nil {
			value = keyf(value)
		}

		if value != "" {
			sb.WriteString(fmt.Sprintf("%s=%s\n", key, value))
		}
	}

	var sb strings.Builder
	printf(&sb, "public_key", p.PublicKey, keyf)
	printf(&sb, "preshared_key", p.PresharedKey, keyf)
	printf(&sb, "replace_allowed_ips", strconv.FormatBool(true), nil)
	printf(&sb, "persistent_keepalive_interval", strconv.Itoa(p.PersistentKeepalive), nil)
	printf(&sb, "allowed_ip", p.AllowedIPs, nil)
	printf(&sb, "endpoint", p.Endpoint, nil)

	return sb.String()
}

type Status string

const (
	Active   Status = "Active"
	Inactive Status = "Inactive"
)
