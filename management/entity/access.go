package entity

import (
	"wireflow/management/utils"
)

// AccessPolicy policy for node
type AccessPolicy struct {
	Model
	OwnId       uint64             `json:"owner_id"`               // 所属ID
	Name        string             `json:"name"`                   // 策略名称
	GroupID     uint64             `json:"group_id"`               // 所属分组
	Priority    int                `json:"priority"`               // 策略优先级（数字越大优先级越高）
	Effect      string             `json:"effect"`                 // 效果：allow/deny
	Description string             `json:"description,omitempty"`  // 策略描述
	Status      utils.ActiveStatus `json:"status" gorm:"type:int"` // 策略状态：启用/禁用
	CreatedBy   string             `json:"created_by"`             // 创建者
	UpdatedBy   string

	AccessRules []AccessRule `gorm:"foreignKey:PolicyId"`
}

func (a *AccessPolicy) TableName() string {
	return "la_access_policy"
}

// AccessRule rule for access policy
type AccessRule struct {
	Model
	OwnId      uint64             `json:"own_id"`               // 所属ID
	RuleType   utils.RuleType     `json:"rule_type"`            // 规则类型
	PolicyId   uint64             `json:"policy_id"`            // 所属策略ID
	SourceType string             `json:"source_type"`          // 源类型：node/tag/all
	SourceId   string             `json:"source_id"`            // 源标识（节点ID或标签）“,” 分隔
	TargetType string             `json:"target_type"`          // 目标类型：node/tag/all
	TargetId   string             `json:"target_id"`            // 目标标识（节点ID或标签）
	Actions    string             `json:"actions"`              // 允许的操作列表
	TimeType   string             `json:"time_type"`            // 时间类型
	Conditions string             `json:"conditions,omitempty"` // 额外条件（如时间限制、带宽限制等）
	Status     utils.ActiveStatus `json:"status"`

	SourceNode  *Node  `gorm:"foreignKey:SourceId"`
	TargetNode  *Node  `gorm:"foreignKey:TargetId"`
	SourceLabel *Label `gorm:"foreignKey:SourceId"`
	TargetLabel *Label `gorm:"foreignKey:TargetId"`
}

func (a *AccessRule) TableName() string {
	return "la_access_rule"
}

// Label node label
type Label struct {
	Model
	Label     string `gorm:"column:label;size:64" json:"label"`
	OwnerId   uint64 `gorm:"column:owner_id;size:64" json:"OwnerId"`
	CreatedBy string `gorm:"column:created_by;size:64" json:"createdBy"`
	UpdatedBy string `gorm:"column:updated_by;size:64" json:"updatedBy"`
}

func (n *Label) TableName() string {
	return "la_label"
}

type NodeLabel struct {
	Model
	NodeId    uint64 `gorm:"not null" json:"node_id"`
	LabelId   uint64 `gorm:"column:label_id;size:50" json:"label_id"`
	LabelName string `gorm:"column:label_name;size:100" json:"label_name"`
	CreatedBy string `gorm:"column:created_by;size:100" json:"created_by"`
	UpdatedBy string `gorm:"column:updated_by;size:100" json:"updated_by"`
}

func (n *NodeLabel) TableName() string {
	return "la_node_label"
}

// AccessLog access log for node
type AccessLog struct {
	Model
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

// RuleNode
type AccessRuleRel struct {
	Model
	RuleId      uint64
	RuleType    utils.RuleType
	SourceId    uint64
	TargetId    uint64
	SourceNode  Node  `gorm:"foreignKey:SourceId"`
	SourceLabel Label `gorm:"foreignKey:SourceId"`
	TargetNode  Node  `gorm:"foreignKey:TargetId"`
	TargetLabel Label `gorm:"foreignKey:TargetId"`
}
