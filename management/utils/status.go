package utils

import (
	"encoding/json"
	"fmt"
)

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

type Status int

const (
	DISABLED Status = iota
	ENABLED
)

func (s Status) String() string {
	switch s {
	case DISABLED:
		return "disabled"
	case ENABLED:
		return "enabled"
	default:
		return "unknown"
	}
}

func (s Status) MarshalJSON() ([]byte, error) {
	// 将枚举值转换为字符串
	return json.Marshal(s.String())
}

var statusMap = map[string]Status{
	"disabled": DISABLED,
	"enabled":  ENABLED,
}

func (s *Status) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	// 根据字符串设置Status值
	if status, ok := statusMap[str]; ok {
		*s = status
		return nil
	}

	return fmt.Errorf("invalid status: %s", str)
}

type RuleType int

const (
	NodeToNode RuleType = iota
	NodeToTag
	TagToNode
	TagToTag
)

func (r RuleType) String() string {
	switch r {
	case NodeToNode:
		return "节点到节点"
	case NodeToTag:
		return "节点到标签"
	case TagToNode:
		return "标签到节点"
	case TagToTag:
		return "标签到标签"
	default:
		return "未知"
	}
}

func (r RuleType) MarshalJSON() ([]byte, error) {
	// 将枚举值转换为字符串
	return json.Marshal(r.String())
}

type ActiveStatus int

func (a ActiveStatus) String() string {
	switch a {
	case 0:
		return "forbidden"
	case 1:
		return "active"
	default:
		return "unknown"
	}
}
