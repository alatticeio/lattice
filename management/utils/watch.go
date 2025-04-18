package utils

import (
	"encoding/json"
	"linkany/pkg/log"
	"slices"
	"sync"
)

var lock sync.Mutex
var once sync.Once
var manager *WatchManager

// WatchManager is a singleton that manages the watch channels for connected peers
// It is used to send messages to all connected peers
// watcher is a map of groupId to watcher, a watcher is a struct that contains the groupId
// and the channel to send messages to
// m is a map of groupId_nodeId to channel, a channel is used to send messages to the connected peer
// The key is a combination of groupId and publicKey, which is used to identify the connected peer
type WatchManager struct {
	mu         sync.Mutex
	groupNodes map[uint64][]string     // key: groupId, value: []publicKey
	channels   map[string]*NodeChannel // key: publicKey, value: channel
	logger     *log.Logger
}

type NodeChannel struct {
	nu      sync.Mutex
	groupId uint64
	channel chan *Message // key: publicKey, value: channel
}

func (n *NodeChannel) GetChannel() chan *Message {
	n.nu.Lock()
	defer n.nu.Unlock()
	if n.channel == nil {
		n.channel = make(chan *Message, 1000) // buffered channel
	}
	return n.channel
}

func (w *WatchManager) GetChannel(publicKey string) *NodeChannel {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.channels == nil {
		return nil
	}
	channel := w.channels[publicKey]

	if channel == nil {
		channel = &NodeChannel{
			channel: make(chan *Message, 1000), // buffered channel
		}
	}
	w.channels[publicKey] = channel
	return channel
}

func (w *WatchManager) Remove(publicKey string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	channel := w.channels[publicKey]
	if channel == nil {
		w.logger.Errorf("channel not found for publicKey: %s", publicKey)
		return
	}

	channel.nu.Lock()
	defer channel.nu.Unlock()
	close(channel.channel)
	delete(w.channels, publicKey)
	if channel.groupId != 0 {
		nodes := w.groupNodes[channel.groupId]
		for i, id := range nodes {
			if id == publicKey {
				w.groupNodes[channel.groupId] = append(nodes[:i], nodes[i+1:]...)
				break
			}
		}
	}
}

func (w *WatchManager) JoinToGroup(publicKey string, groupId uint64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	channel := w.channels[publicKey]
	if channel == nil {
		w.logger.Errorf("channel not found for publicKey: %s", publicKey)
		return
	}

	channel.nu.Lock()
	defer channel.nu.Unlock()
	channel.groupId = groupId

}

func (w *WatchManager) LeaveFromGroup(publicKey string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	channel := w.channels[publicKey]
	if channel == nil {
		w.logger.Errorf("channel not found for publicKey: %s", publicKey)
		return
	}

	channel.nu.Lock()
	defer channel.nu.Unlock()
	channel.groupId = 0
	nodes := w.groupNodes[channel.groupId]
	for i, id := range nodes {
		if id == publicKey {
			w.groupNodes[channel.groupId] = append(nodes[:i], nodes[i+1:]...)
			break
		}
	}
	delete(w.groupNodes, channel.groupId)
	channel.groupId = 0
	if len(nodes) == 0 {
		delete(w.groupNodes, channel.groupId)
	}
	channel.groupId = 0
	channel.channel = nil
	channel = nil
	w.channels[publicKey] = nil
	w.logger.Verbosef("channel removed for publicKey: %s", publicKey)
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
			groupNodes: make(map[uint64][]string),
			channels:   make(map[string]*NodeChannel),
			logger:     log.NewLogger(log.Loglevel, "watchmanager"),
		}
	})

	return manager
}

// Push sends a message to all connected peer's channel
func (w *WatchManager) Push(key string, msg *Message) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.logger.Verbosef("manager: %v, ch: %v", w, msg)

	nodeChannel := w.channels[key]
	if nodeChannel == nil {
		w.logger.Errorf("channel not found for key: %s", key)
		return
	}

	switch msg.EventType {
	case EventTypeGroupNodeAdd:
		nodeChannel.nu.Lock()
		defer nodeChannel.nu.Unlock()
		nodeChannel.groupId = msg.GroupId
		nodes := w.groupNodes[nodeChannel.groupId]
		if !slices.Contains(nodes, key) {
			w.groupNodes[nodeChannel.groupId] = append(nodes, key)
		}
	}

	groupId := nodeChannel.groupId

	// push to all connected group nodes
	for _, id := range w.groupNodes[groupId] {
		//if id == key {
		//	continue
		//}
		ch := w.channels[id]
		if ch == nil {
			w.logger.Errorf("channel not found for groupId: %d, key: %s", groupId, id)
			continue
		}
		ch.channel <- msg
		w.logger.Verbosef("push message to groupId: %d, key: %s, msg: %v", groupId, id, msg)
	}
}

type Message struct {
	EventType EventType
	*GroupMessage
}

type GroupMessage struct {
	GroupId   uint64
	GroupName string
	Nodes     []*NodeMessage
	Policies  []*PolicyMessage
}

func (m *Message) AddNode(node *NodeMessage) *Message {
	m.EventType = EventTypeGroupNodeAdd
	if m.GroupMessage == nil {
		m.GroupMessage = &GroupMessage{
			GroupId: node.GroupID,
		}
	}
	m.Nodes = append(m.GroupMessage.Nodes, node)
	return m
}

func (m *Message) RemoveNode(node *NodeMessage) *Message {
	m.EventType = EventTypeGroupNodeRemove
	m.Nodes = append(m.GroupMessage.Nodes, node)
	return m
}

func (m *Message) UpdateNode(node *NodeMessage) *Message {
	m.EventType = EventTypeGroupNodeUpdate
	m.Nodes = append(m.GroupMessage.Nodes, node)
	return m
}

func (m *Message) AddGroup(groupId uint64, groupName string) *Message {
	m.EventType = EventTypeGroupAdd
	m.GroupId = groupId
	m.GroupName = groupName
	return m
}

func (m *Message) RemoveGroup(groupId uint64, groupName string) *Message {
	m.EventType = EventTypeGroupRemove
	m.GroupId = groupId
	m.GroupName = groupName
	return m
}

func (m *Message) GroupChanged(groupId uint64, groupName string) *Message {
	m.EventType = EventTypeGroupChanged
	m.GroupId = groupId
	m.GroupName = groupName
	return m
}

func (m *Message) AddPolicy(policyMessage PolicyMessage) *Message {
	m.EventType = EventTypeGroupPolicyAdd
	m.Policies = append(m.Policies, &policyMessage)
	return m
}

func (m *Message) UpdatePolicy(policyMessage PolicyMessage) *Message {
	m.EventType = EventTypeGroupPolicyChanged
	m.Policies = append(m.Policies, &policyMessage)
	return m
}

func (m *Message) RemovePolicy(policyMessage PolicyMessage) *Message {
	m.EventType = EventTypeGroupPolicyRemove
	m.Policies = append(m.Policies, &policyMessage)
	return m
}

func (m *Message) AddPolicyRule(policyId uint64, ruleMessage AccessRuleMessage) *Message {
	m.EventType = EventTypePolicyRuleAdd
	for _, policy := range m.Policies {
		if policy.PolicyId == policyId {
			policy.Rules = append(policy.Rules, &ruleMessage)
			return m
		}
	}

	return m
}

func (m *Message) UpdatePolicyRule(ruleMessage AccessRuleMessage) *Message {
	m.EventType = EventTypePolicyRuleChanged
	for _, policy := range m.Policies {
		if policy.PolicyId == ruleMessage.PolicyId {
			policy.Rules = append(policy.Rules, &ruleMessage)
			return m
		}
	}

	return m
}

func (m *Message) RemovePolicyRule(ruleMessage *AccessRuleMessage) *Message {
	m.EventType = EventTypePolicyRuleRemove
	for _, policy := range m.Policies {
		if policy.PolicyId == ruleMessage.PolicyId {
			for _, rule := range policy.Rules {
				if rule.PolicyId == ruleMessage.PolicyId {
					policy.Rules = append(policy.Rules, ruleMessage)
					return m
				}
			}
		}
	}

	return m
}

type NodeMessage struct {
	ID                  uint64     `json:"id,string"`
	Name                string     `json:"name,omitempty"`
	Description         string     `json:"description,omitempty"`
	GroupID             uint64     `json:"groupID,omitempty"`   // belong to which group
	CreatedBy           string     `json:"createdBy,omitempty"` // ownerID
	UserId              uint64     `json:"userId,omitempty"`
	Hostname            string     `json:"hostname,omitempty"`
	AppID               string     `json:"appId,omitempty"`
	Address             string     `json:"address,omitempty"`
	Endpoint            string     `json:"endpoint,omitempty"`
	PersistentKeepalive int        `json:"persistentKeepalive,omitempty"`
	PublicKey           string     `json:"publicKey,omitempty"`
	AllowedIPs          string     `json:"allowedIps,omitempty"`
	RelayIP             string     `json:"relayIp,omitempty"`
	TieBreaker          int64      `json:"tieBreaker"`
	Ufrag               string     `json:"ufrag"`
	Pwd                 string     `json:"pwd"`
	Port                int        `json:"port"`
	Status              NodeStatus `json:"status"`
	GroupName           string     `json:"groupName"`
	Version             uint64     `json:"version"`
	LastUpdatedAt       string     `json:"lastUpdatedAt"`
}

func (n *NodeMessage) String() string {
	bs, err := json.Marshal(n)
	if err != nil {
		return ""
	}
	return string(bs)
}

type PolicyMessage struct {
	PolicyId   uint64
	PolicyName string
	Rules      []*AccessRuleMessage
}

type AccessRuleMessage struct {
	PolicyId   uint64
	PolicyName string
}

//type RuleMessageVo struct {
//	RuleId     uint
//	RuleName   string
//	RuleType   string
//	RuleValue  string
//	RuleAction string
//}

type MessageConfig struct {
	EventType    EventType
	GroupMessage *GroupMessage
}

func NewMessage() *Message {
	return &Message{
		GroupMessage: &GroupMessage{},
	}
}

type EventType int

const (
	EventTypeGroupNodeAdd EventType = iota
	EventTypeGroupNodeRemove
	EventTypeGroupNodeUpdate
	EventTypeGroupAdd
	EventTypeGroupRemove
	EventTypeGroupChanged
	EventTypeGroupPolicyAdd
	EventTypeGroupPolicyChanged
	EventTypeGroupPolicyRemove
	EventTypePolicyRuleAdd
	EventTypePolicyRuleChanged
	EventTypePolicyRuleRemove
)

func (e EventType) String() string {
	switch e {
	case EventTypeGroupNodeAdd:
		return "GroupNodeAdd"
	case EventTypeGroupNodeRemove:
		return "GroupNodeRemove"
	case EventTypeGroupNodeUpdate:
		return "GroupNodeUpdate"
	case EventTypeGroupAdd:
		return "groupAdd"
	case EventTypeGroupRemove:
		return "groupRemove"
	case EventTypeGroupChanged:
		return "groupChanged"
	case EventTypeGroupPolicyAdd:
		return "GroupPolicyAdd"
	case EventTypeGroupPolicyChanged:
		return "GroupPolicyChanged"
	case EventTypeGroupPolicyRemove:
		return "GroupPolicyRemove"
	case EventTypePolicyRuleAdd:
		return "PolicyRuleAdd"
	case EventTypePolicyRuleChanged:
		return "PolicyRuleChanged"
	case EventTypePolicyRuleRemove:
		return "PolicyRuleRemove"

	}
	return "unknown"
}
