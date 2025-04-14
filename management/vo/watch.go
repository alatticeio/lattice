package vo

import (
	"linkany/pkg/log"
	"sync"
)

// Message used to wrapper the message for watch
/*
  a node event message like:
{
	"code": 200,
	"msg": "success",
	"data": {
		"drpUrl": "http://drp.linkany.io/drp",
		"device": {
			"privateKey": "mBngM2k7qWp9pVFGWMO0q1l7tiWjiIIAgsU/jwj+BHU=",
			"publicKey": "3Hyx1Sbq0F9SZc6CUnmJ1pCPMgaAi6JRIxwoTrc1wSA=",
			"address": "10.0.0.4",
			"listenPort": 51820
		},
		"list": [{
			"id": 1866305058815524900,
			"instanceId": 1865958132886462500,
			"userId": 1865418224707231700,
			"name": null,
			"hostname": "VM-4-3-opencloudos",
			"appId": "64d583324d",
			"insPrivateKey": null,
			"insPublicKey": null,
			"address": "10.0.0.4",
			"endpoint": null,
			"persistentKeepalive": 25,
			"publicKey": "lFTblXWiDQTACHfiEqlJ6ORpBMCCiIGER1YgF729xVY=",
			"privateKey": "gEJu/+pPlNa7CFANMvohx12iPf+/XUrpY+F39ntguEc=",
			"allowedIPs": null,
			"hostIp": null,
			"srflxIp": null,
			"relayIp": null,
			"createDate": null
		}]
	}
}
*/
type Message struct {
	EventType    EventType
	GroupMessage *GroupMessage
}

type GroupMessage struct {
	GroupId   uint64
	GroupName string
	Nodes     []*NodeVo
	Policies  []*PolicyMessage
}

type PolicyMessage struct {
	PolicyId   uint64
	PolicyName string
	Rules      []*AccessRuleVo
}

//type RuleMessageVo struct {
//	RuleId     uint
//	RuleName   string
//	RuleType   string
//	RuleValue  string
//	RuleAction string
//}

func NewNodeMessage(eventType EventType, nodeVo []*NodeVo) *Message {
	return &Message{
		EventType: eventType,
		GroupMessage: &GroupMessage{
			Nodes: nodeVo,
		},
	}
}

func NewPolicyMessage(eventType EventType, groupName string, policyVos []*AccessPolicyVo) *Message {

	var policyMessages []*PolicyMessage
	for _, policyVo := range policyVos {
		policyMessages = append(policyMessages, &PolicyMessage{
			PolicyId:   policyVo.ID,
			PolicyName: policyVo.Name,
			Rules:      policyVo.Rules,
		})
	}

	msg := &Message{
		EventType: eventType,
		GroupMessage: &GroupMessage{
			GroupName: groupName,
			Policies:  policyMessages,
		},
	}

	return msg
}

func NewGroupMessage(eventType EventType, group NodeGroupVo) *Message {
	return &Message{
		EventType: eventType,
		GroupMessage: &GroupMessage{
			GroupName: group.Name,
		},
	}
}

type MessageConfig struct {
	EventType    EventType
	GroupMessage *GroupMessage
}

func NewMessage(cfg *MessageConfig) *Message {
	return &Message{
		EventType:    cfg.EventType,
		GroupMessage: cfg.GroupMessage,
	}
}

type EventType int

const (
	EventTypeNodeAdd EventType = iota
	EventTypeNodeRemove
	EventTypeNodeUpdate
	EventTypeGroupAdd
	EventTypeGroupRemove
	EventTypeGroupChanged
	EventTypePolicyAdd
	EventTypePolicyChanged
	EventTypePolicyRemove
	EventTypeRuleAdd
	EventTypeRuleChanged
	EventTypeRuleRemove
)

func (e EventType) String() string {
	switch e {
	case EventTypeNodeAdd:
		return "nodeAdd"
	case EventTypeNodeRemove:
		return "nodeRemove"
	case EventTypeNodeUpdate:
		return "nodeUpdate"
	case EventTypeGroupAdd:
		return "groupAdd"
	case EventTypeGroupRemove:
		return "groupRemove"
	case EventTypeGroupChanged:
		return "groupChanged"
	case EventTypePolicyAdd:
		return "policyAdd"
	case EventTypePolicyChanged:
		return "policyChanged"
	case EventTypePolicyRemove:
		return "policyRemove"
	case EventTypeRuleAdd:
		return "ruleAdd"
	case EventTypeRuleChanged:
		return "ruleChanged"
	case EventTypeRuleRemove:
		return "ruleRemove"

	}
	return "unknown"
}

var lock sync.Mutex
var once sync.Once
var manager *WatchManager

type WatchManager struct {
	lock   sync.Mutex
	m      map[string]chan *Message // key: clientID, value: channel
	logger *log.Logger
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
			m:      make(map[string]chan *Message),
			logger: log.NewLogger(log.Loglevel, "watchmanager"),
		}
	})

	return manager
}

type RangeFunc func()

func (w *WatchManager) Clientsets() map[string]chan *Message {
	w.lock.Lock()
	defer w.lock.Unlock()

	return w.m
}

// Add adds a new channel to the watch manager for a new connected peer
func (w *WatchManager) Add(key string, ch chan *Message) {
	w.lock.Lock()
	defer w.lock.Unlock()

	w.logger.Verbosef("manager: %v, ch: %v", w, ch)
	w.m[key] = ch
}

// Remove removes a channel from the watch manager for a disconnected peer
func (w *WatchManager) Remove(key string) {
	w.lock.Lock()
	defer w.lock.Unlock()

	delete(w.m, key)
}

// Push sends a message to all connected peer's channel
func (w *WatchManager) Push(key string, msg *Message) {
	w.lock.Lock()
	defer w.lock.Unlock()

	if ch, ok := w.m[key]; ok {
		ch <- msg
	}
}

func (w *WatchManager) Get(key string) chan *Message {
	w.lock.Lock()
	defer w.lock.Unlock()
	ch := w.m[key]
	w.logger.Verbosef("Get channel: %v for node: %v, manager: %v", ch, key, w)
	return ch
}
