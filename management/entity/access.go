package entity

import (
	"encoding/json"
	"fmt"
	"gorm.io/gorm"
)

// AccessPolicy policy for node
type AccessPolicy struct {
	gorm.Model
	Name        string `json:"name"`                  // 策略名称
	GroupID     uint   `json:"group_id"`              // 所属分组
	Priority    int    `json:"priority"`              // 策略优先级（数字越大优先级越高）
	Effect      string `json:"effect"`                // 效果：allow/deny
	Description string `json:"description,omitempty"` // 策略描述
	Status      Status `json:"status"`                // 策略状态：启用/禁用
	CreatedBy   string `json:"created_by"`            // 创建者
	UpdatedBy   string
}

func (a *AccessPolicy) TableName() string {
	return "la_access_policy"
}

// AccessRule rule for access policy
type AccessRule struct {
	gorm.Model
	OwnerId    uint     `json:"owner_id"`             // 所属ID
	RuleType   RuleType `json:"rule_type"`            // 规则类型
	PolicyID   uint     `json:"policy_id"`            // 所属策略ID
	SourceType string   `json:"source_type"`          // 源类型：node/tag/all
	SourceID   string   `json:"source_id"`            // 源标识（节点ID或标签）
	TargetType string   `json:"target_type"`          // 目标类型：node/tag/all
	TargetID   string   `json:"target_id"`            // 目标标识（节点ID或标签）
	Actions    string   `json:"actions"`              // 允许的操作列表
	TimeType   string   `json:"time_type"`            // 时间类型
	Conditions string   `json:"conditions,omitempty"` // 额外条件（如时间限制、带宽限制等）
	Status     Status   `json:"status"`
}

type RuleType int

const (
	NodeToNode RuleType = iota
	NodeToTag
	TagToNode
	TagToTag
)

func (a RuleType) String() string {
	switch a {
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

type Status int

const (
	DISABLED Status = iota
	ENABLED
)

func (a Status) String() string {
	switch a {
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

func (s *Status) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}

	// 根据字符串设置Status值
	switch str {
	case "disabled":
		*s = DISABLED
	case "enabled":
		*s = ENABLED
	default:
		return fmt.Errorf("invalid Status value: %s", str)
	}
	return nil
}

func (a *AccessRule) TableName() string {
	return "la_access_rule"
}

// Label node label
type Label struct {
	gorm.Model
	Label     string `gorm:"column:label;size:64" json:"label"`
	OwnerId   uint64 `gorm:"column:owner_id;size:64" json:"OwnerId"`
	CreatedBy string `gorm:"column:created_by;size:64" json:"createdBy"`
	UpdatedBy string `gorm:"column:updated_by;size:64" json:"updatedBy"`
}

func (n *Label) TableName() string {
	return "la_label"
}

type NodeLabel struct {
	gorm.Model
	NodeId    uint64
	LabelId   uint64
	LabelName string
	CreatedBy string
	UpdatedBy string
}

func (n *NodeLabel) TableName() string {
	return "la_node_label"
}

// AccessLog access log for node
type AccessLog struct {
	gorm.Model
	SourceNodeID uint   `json:"source_node_id"`
	TargetNodeID uint   `json:"target_node_id"`
	Action       string `json:"action"`
	Result       bool   `json:"result"`
	PolicyID     uint   `json:"policy_id"`
	Reason       string `json:"reason,omitempty"`
}

func (a *AccessLog) TableName() string {
	return "la_access_log"
}
