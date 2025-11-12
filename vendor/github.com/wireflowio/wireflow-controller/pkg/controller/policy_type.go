package controller

import (
	"fmt"

	wireflowv1alpha1 "github.com/wireflowio/wireflow-controller/pkg/apis/wireflowcontroller/v1alpha1"
)

type PolicyChangeType string

const (
	// PolicyChangeCreated 策略被创建
	PolicyChangeCreated PolicyChangeType = "Created"

	// PolicyChangeUpdated 策略被更新
	PolicyChangeUpdated PolicyChangeType = "Updated"

	// PolicyChangeDeleted 策略被删除
	PolicyChangeDeleted PolicyChangeType = "Deleted"

	// PolicyChangeEnabled 策略被启用 (Disabled: false → true)
	PolicyChangeEnabled PolicyChangeType = "Enabled"

	// PolicyChangeDisabled 策略被禁用 (Disabled: true → false)
	PolicyChangeDisabled PolicyChangeType = "Disabled"

	// PolicyChangePriorityChanged 优先级变更
	PolicyChangePriorityChanged PolicyChangeType = "PriorityChanged"

	// PolicyChangeRulesModified 规则被修改
	PolicyChangeRulesModified PolicyChangeType = "RulesModified"

	// PolicyChangeActionChanged Action 变更 (Allow/Deny)
	PolicyChangeActionChanged PolicyChangeType = "ActionChanged"
)

// PolicyChangeDetail 策略变更详情
type PolicyChangeDetail struct {
	// 变更类型
	Type PolicyChangeType

	// 策略名称
	PolicyName string

	// 策略命名空间
	PolicyNamespace string

	// 旧策略 (仅 Update 和 Delete 时有值)
	OldPolicy *wireflowv1alpha1.NetworkPolicy

	// 新策略 (仅 Create 和 Update 时有值)
	NewPolicy *wireflowv1alpha1.NetworkPolicy

	// 变更摘要
	Summary string

	// 影响的字段列表
	AffectedFields []string

	// 是否需要立即应用 (高优先级变更)
	RequiresImmediateAction bool
}

// String 返回变更详情的字符串表示
func (pcd *PolicyChangeDetail) String() string {
	return fmt.Sprintf("PolicyChange{Type=%s, Policy=%s/%s, Summary=%s, Immediate=%v}",
		pcd.Type, pcd.PolicyNamespace, pcd.PolicyName, pcd.Summary, pcd.RequiresImmediateAction)
}

// PolicyChangeEvent 策略变更事件
type PolicyChangeEvent struct {
	// 变更详情
	Detail PolicyChangeDetail

	// 受影响的 Network 列表
	AffectedNetworks []string

	// 受影响的 Node 列表 (通过 Networks 间接影响)
	AffectedNodes []string
}
