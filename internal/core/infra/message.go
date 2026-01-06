// Copyright 2025 The Wireflow Authors, Inc.
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

package infra

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// Message is the message which is sent to connected peers
type Message struct {
	EventType     EventType      `json:"eventType"`               //主事件类型
	ConfigVersion string         `json:"configVersion"`           //版本号
	Timestamp     int64          `json:"timestamp"`               //时间戳
	Changes       *ChangeDetails `json:"changes"`                 // 配置变化详情
	Current       *Peer          `json:"peer"`                    //当前节点信息
	Network       *Network       `json:"network"`                 //当前节点网络信息
	Policies      []*Policy      `json:"policies,omitempty"`      //当前节点的策略
	ComputedPeers []*Peer        `json:"computedpeers,omitempty"` //当前要连接的节点, 由controller计算完成返回给wireflow
	ComputedRules *FirewallRule  `json:"computedrules,omitempty"`
}

func (m *Message) Equal(b *Message) bool {
	if m.EventType != b.EventType {
		return false
	}

	if !reflect.DeepEqual(m.Network, b.Network) {
		return false
	}

	if !reflect.DeepEqual(m.Policies, b.Policies) {
		return false
	}

	if !reflect.DeepEqual(m.ComputedPeers, b.ComputedPeers) {
		return false
	}

	if !reflect.DeepEqual(m.ComputedRules, b.ComputedRules) {
		return false
	}

	if !reflect.DeepEqual(m.Current.Name, b.Current.Name) {
		return false
	}

	return true
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
	PoliciesAdded   []*Policy `json:"policiesAdded,omitempty"`
	PoliciesRemoved []*Policy `json:"policiesRemoved,omitempty"`
	PoliciesUpdated []*Policy `json:"policiesUpdated,omitempty"`

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
	Name                string  `json:"name,omitempty"`
	InterfaceName       string  `json:"interfaceName,omitempty"`
	Platform            string  `json:"platform,omitempty"`
	Description         string  `json:"description,omitempty"`
	NetworkId           string  `json:"NetworkId,omitempty"` // belong to which group
	CreatedBy           string  `json:"createdBy,omitempty"` // ownerID
	UserId              uint64  `json:"userId,omitempty"`
	Hostname            string  `json:"hostname,omitempty"`
	AppID               string  `json:"appId,omitempty"`
	Address             *string `json:"address,omitempty"`
	Endpoint            string  `json:"endpoint,omitempty"`
	Remove              bool    `json:"remove,omitempty"` // whether to remove node
	PresharedKey        string  `json:"presharedKey,omitempty"`
	PersistentKeepalive int     `json:"persistentKeepalive,omitempty"`
	PrivateKey          string  `json:"privateKey,omitempty"`
	PublicKey           string  `json:"publicKey,omitempty"`
	AllowedIPs          string  `json:"allowedIps,omitempty"`
	ReplacePeers        bool    `json:"replacePeers,omitempty"` // whether to replace peers when updating node
	Port                int     `json:"port"`
	GroupName           string  `json:"groupName"`
	Version             uint64  `json:"version"`
	LastUpdatedAt       string  `json:"lastUpdatedAt"`
}

// Network is the network information, contains all peers/policies in the network
type Network struct {
	Address     string   `json:"address"`
	AllowedIps  []string `json:"allowedIps"`
	Port        int      `json:"port"`
	NetworkId   string   `json:"NetworkId"`
	NetworkName string   `json:"networkName"`
	Peers       []*Peer  `json:"peers"`
}

type Policy struct {
	PolicyName string  `json:"policyName"`
	Ingress    []*Rule `json:"ingress"`
	Egress     []*Rule `json:"egress"`
}

type FirewallRule struct {
	Platform     string   `json:"platform"`
	IngressRules []string `json:"ingressRules"`
	EgressRules  []string `json:"egressRules"`
}

type Rule struct {
	Peers    []*Peer `json:"peers"`
	Protocol string  `json:"protocol"`
	Port     string  `json:"port"`
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

func (m *Message) WithPolicies(policies []*Policy) *Message {
	m.Policies = policies
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

//func (n *Network) WithPolicy(policy *Policy) *Network {
//	n.Policies = append(n.Policies, policy)
//	return n
//}

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
