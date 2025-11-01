package vo

import (
	"time"
	"wireflow/pkg/utils"
)

type AccessPolicyVo struct {
	ID          uint64             `json:"id"`
	Name        string             `json:"name"`                  // 策略名称
	GroupID     uint64             `json:"group_id"`              // 所属分组
	Priority    int                `json:"priority"`              // 策略优先级（数字越大优先级越高）
	Effect      string             `json:"effect"`                // 效果：allow/deny
	Description string             `json:"description,omitempty"` // 策略描述
	Status      utils.ActiveStatus `json:"status"`                // 策略状态：启用/禁用
	CreatedAt   time.Time          `json:"createdAt"`
	UpdatedAt   time.Time          `json:"updatedAt"`
	CreatedBy   string             `json:"createdBy"` // 创建者
	UpdatedBy   string             `json:"updatedBy"`
	DeletedAt   time.Time          `json:"deletedAt"`

	Rules []*AccessRuleVo `json:"rules"`
}

type AccessRuleVo struct {
	ID         uint64             `json:"id"`
	Name       string             `json:"name"`                 // 规则名称
	RuleType   utils.RuleType     `json:"ruleType"`             // 规则类型
	PolicyID   uint64             `json:"policyId"`             // 所属策略ID
	SourceType string             `json:"sourceType"`           // 源类型：node/tag/all
	SourceID   string             `json:"sourceId"`             // 源标识（节点ID或标签）
	TargetType string             `json:"targetType"`           // 目标类型：node/tag/all
	TargetID   string             `json:"targetId"`             // 目标标识（节点ID或标签）
	Actions    string             `json:"actions"`              // 允许的操作列表
	TimeType   string             `json:"timeType"`             // 时间类型
	Conditions string             `json:"conditions,omitempty"` // 额外条件（如时间限制、带宽限制等）
	CreatedAt  time.Time          `json:"createdAt"`
	UpdatedAt  time.Time          `json:"updatedAt"`
	CreatedBy  string             `json:"createdBy"`
	UpdatedBy  string             `json:"updatedBy"`
	DeletedAt  time.Time          `json:"deletedAt"`
	Status     utils.ActiveStatus `json:"status"`

	SourceNodeValues  *NodeResourceVo  `json:"sourceNodeValues"`
	TargetNodeValues  *NodeResourceVo  `json:"targetNodeValues"`
	SourceLabelValues *LabelResourceVo `json:"sourceLabelValues"`
	TargetLabelValues *LabelResourceVo `json:"targetLabelValues"`
}
