// Copyright 2025 wireflowio.com, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"wireflow/pkg/log"
	"wireflow/pkg/utils"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

var lock sync.Mutex
var once sync.Once
var manager *WatchManager

// WatchManager is a singleton that manages the watch channels for connected peers
// It is used to send messages to all connected peers
// watcher is a map of networkId to watcher, a watcher is a struct that contains the networkId
// and the channel to send messages to
// m is a map of groupId_nodeId to channel, a channel is used to send messages to the connected peer
// The key is a combination of networkId and publicKey, which is used to identify the connected peer
type WatchManager struct {
	mu sync.Mutex
	// push channel
	channels     map[string]*NodeChannel // key: clientId, value: channel
	recvChannels map[string]*NodeChannel
	logger       *log.Logger
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

// NewWatchManager create a whole manager for connected peers
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

// Message is the message which is sent to connected peers
type Message struct {
	EventType     EventType      `json:"eventType"`     //主事件类型
	ConfigVersion string         `json:"configVersion"` //版本号
	Timestamp     int64          `json:"timestamp"`     //时间戳
	Changes       *ChangeDetails `json:"changes"`       // 配置变化详情
	Current       *Peer          `json:"peer"`          //当前节点信息
	Network       *Network       `json:"network"`       //网络信息
}

type ChangeDetails struct {
	//节点信息变化
	AddressChanged  bool `json:"addressChanged,omitempty"`  //IP地址变化
	KeyChanged      bool `json:"keyChanged,omitempty"`      //密钥变化
	EndpointChanged bool `json:"endpointChanged,omitempty"` //远程地址变化

	//网络拓扑变化
	PeersAdded   []*Peer  `json:"peersAdded,omitempty"`   //节点添加的列表
	PeersRemoved []*Peer  `json:"peersRemoved,omitempty"` //节点移除列表
	PeersUpdated []string `json:"peersUpdated,omitempty"` // 节点更新列表

	//策略变化
	PoliciesAdded   []string `json:"policiesAdded,omitempty"`
	PoliciesRemoved []string `json:"policiesRemoved,omitempty"`
	PoliciesUpdated []string `json:"policiesUpdated,omitempty"`

	//网络配置变化
	NetworkJoined        []string `json:"networkJoined,omitempty"`
	NetworkLeft          []string `json:"networkLeft,omitempty"`
	NetworkConfigChanged bool     `json:"networkConfigChanged,omitempty"`

	Reason       string `json:"reason,omitempty"`       //变更原因描述
	TotalChanges int    `json:"totalChanges,omitempty"` // 变更总数
}

func (c *ChangeDetails) HasChanges() bool {
	return c.TotalChanges > 0
}

func (c *ChangeDetails) String() string {
	data, _ := json.Marshal(c)
	return string(data)
}

// Summary returns a summary of the changes
func (c *ChangeDetails) Summary() string {
	parts := make([]string, 0)

	if c.AddressChanged {
		parts = append(parts, "address")
	}
	if c.KeyChanged {
		parts = append(parts, "key")
	}
	if len(c.PeersAdded) > 0 {
		parts = append(parts, fmt.Sprintf("+%d peers", len(c.PeersAdded)))
	}
	if len(c.PeersRemoved) > 0 {
		parts = append(parts, fmt.Sprintf("-%d peers", len(c.PeersRemoved)))
	}
	if len(c.PoliciesAdded) > 0 {
		parts = append(parts, fmt.Sprintf("+%d policies", len(c.PoliciesAdded)))
	}
	if len(c.PoliciesUpdated) > 0 {
		parts = append(parts, fmt.Sprintf("~%d policies", len(c.PoliciesUpdated)))
	}

	if len(parts) == 0 {
		return "no changes"
	}

	return strings.Join(parts, ", ")
}

// Peer is the information of a wireflow peer, contains all the information of a peer
type Peer struct {
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
	ReplacePeers        bool             `json:"replacePeers,omitempty"` // whether to replace peers when updating node
	Port                int              `json:"port"`
	Status              utils.NodeStatus `json:"status"`
	GroupName           string           `json:"groupName"`
	Version             uint64           `json:"version"`
	LastUpdatedAt       string           `json:"lastUpdatedAt"`

	//conn type
	DrpAddr     string   `json:"drpAddr,omitempty"`     // drp server address, if is drp node
	ConnectType ConnType `json:"connectType,omitempty"` // DirectType, RelayType, DrpType
}

// Network is the network information, contains all peers/policies in the network
type Network struct {
	Address     string    `json:"address"`
	AllowedIps  []string  `json:"allowedIps"`
	Port        int       `json:"port"`
	NetworkId   string    `json:"networkId"`
	NetworkName string    `json:"networkName"`
	Policies    []*Policy `json:"policies"`
	Peers       []*Peer   `json:"peers"`
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

func (m *Message) WithNode(node *Peer) *Message {
	m.Current = node
	return m
}

func (m *Message) WithNetwork(network *Network) *Message {
	m.Network = network
	return m
}

// FullConfig 全量配置
func (m *Message) FullConfig() string {

	return ""
}

func (m *Message) String() string {
	data, _ := json.Marshal(m)
	return string(data)
}

func (n *Network) WithPolicy(policy *Policy) *Network {
	n.Policies = append(n.Policies, policy)
	return n
}

func (w *WatchManager) Send(clientId string, msg *Message) error {
	channel := w.GetChannel(clientId)
	channel.channel <- msg
	return nil
}

type EventType int

const (
	EventTypeJoinNetwork EventType = iota
	EventTypeLeaveNetwork
	EventTypeNodeUpdate
	EventTypeNodeAdd
	EventTypeNodeRemove
	EventTypeIPChange
	EventTypeKeyChanged
	EventTypeNetworkChanged
	EventTypePolicyChanged
	EventTypeNone
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

func (p *Peer) String() string {
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
	printf(&sb, "allowed_ips", p.AllowedIPs, nil)
	printf(&sb, "endpoint", p.Endpoint, nil)

	return sb.String()
}

type Status string

const (
	Active   Status = "Active"
	Inactive Status = "Inactive"
)
