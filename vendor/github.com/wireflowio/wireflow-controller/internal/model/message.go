package model

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Message is the message which is sent to connected peers
type Message struct {
	EventType     EventType      `json:"eventType"`     //主事件类型
	ConfigVersion string         `json:"configVersion"` //版本号
	Timestamp     int64          `json:"timestamp"`     //时间戳
	Changes       *ChangeDetails `json:"changes"`       // 配置变化详情
	Current       *Peer          `json:"peer"`          //当前节点信息
	Network       *Network       `json:"network"`       //网络信息
}

func (m *Message) String() string {
	data, _ := json.Marshal(m)
	return string(data)
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
	Name                string     `json:"name,omitempty"`
	Description         string     `json:"description,omitempty"`
	NetworkId           string     `json:"networkId,omitempty"` // belong to which group
	CreatedBy           string     `json:"createdBy,omitempty"` // ownerID
	UserId              uint64     `json:"userId,omitempty"`
	Hostname            string     `json:"hostname,omitempty"`
	AppID               string     `json:"appId,omitempty"`
	Address             string     `json:"address,omitempty"`
	Endpoint            string     `json:"endpoint,omitempty"`
	Remove              bool       `json:"remove,omitempty"` // whether to remove node
	PresharedKey        string     `json:"presharedKey,omitempty"`
	PersistentKeepalive int        `json:"persistentKeepalive,omitempty"`
	PrivateKey          string     `json:"privateKey,omitempty"`
	PublicKey           string     `json:"publicKey,omitempty"`
	AllowedIPs          string     `json:"allowedIps,omitempty"`
	ReplacePeers        bool       `json:"replacePeers,omitempty"` // whether to replace peers when updating node
	Port                int        `json:"port"`
	Status              NodeStatus `json:"status"`
	GroupName           string     `json:"groupName"`
	Version             uint64     `json:"version"`
	LastUpdatedAt       string     `json:"lastUpdatedAt"`

	//conn type
	DrpAddr     string      `json:"drpAddr,omitempty"`     // drp server address, if is drp node
	ConnectType ConnectType `json:"connectType,omitempty"` // DirectType, RelayType, DrpType
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

type NodeStatus int

const (
	Unregisterd NodeStatus = iota
	Registered
	Online
	Offline
	Disabled
)

func (n NodeStatus) String() string {
	switch n {
	case Unregisterd:
		return "unregistered"
	case Registered:
		return "registered"
	case Online:
		return "online"
	case Offline:
		return "offline"
	case Disabled:
		return "disabled"
	default:
		return "unknown"
	}
}

type ConnectType int

const (
	DirectType ConnectType = iota
	RelayType
	DrpType
)

func (s ConnectType) String() string {
	switch s {
	case DirectType:
		return "direct"
	case RelayType:
		return "Relay"
	case DrpType:
		return "drp"
	default:
		return "unknown"
	}
}
