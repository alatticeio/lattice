// pkg/controller/policy_detector.go
package controller

import (
	"fmt"

	wireflowv1alpha1 "github.com/wireflowio/wireflow-controller/pkg/apis/wireflowcontroller/v1alpha1"
)

// PolicyChangeDetector 检测策略变更
type PolicyChangeDetector struct{}

func NewPolicyChangeDetector() *PolicyChangeDetector {
	return &PolicyChangeDetector{}
}

// DetectChanges 检测策略的变更类型和详情
func (pcd *PolicyChangeDetector) DetectChanges(
	old, new *wireflowv1alpha1.NetworkPolicy) *PolicyChangeDetail {

	// 新建策略
	if old == nil && new != nil {
		return &PolicyChangeDetail{
			Type:                    PolicyChangeCreated,
			PolicyName:              new.Name,
			PolicyNamespace:         new.Namespace,
			NewPolicy:               new,
			Summary:                 "Policy created",
			RequiresImmediateAction: !new.Spec.Disabled,
		}
	}

	// 删除策略
	if old != nil && new == nil {
		return &PolicyChangeDetail{
			Type:                    PolicyChangeDeleted,
			PolicyName:              old.Name,
			PolicyNamespace:         old.Namespace,
			OldPolicy:               old,
			Summary:                 "Policy deleted",
			RequiresImmediateAction: true,
		}
	}

	// 更新策略
	if old != nil && new != nil {
		return pcd.detectUpdateChanges(old, new)
	}

	return nil
}

// detectUpdateChanges 检测更新类型的变更
func (pcd *PolicyChangeDetector) detectUpdateChanges(
	old, new *wireflowv1alpha1.NetworkPolicy) *PolicyChangeDetail {

	detail := &PolicyChangeDetail{
		Type:            PolicyChangeUpdated,
		PolicyName:      new.Name,
		PolicyNamespace: new.Namespace,
		OldPolicy:       old,
		NewPolicy:       new,
		AffectedFields:  []string{},
	}

	// 检查 Disabled 状态变化
	if old.Spec.Disabled != new.Spec.Disabled {
		if new.Spec.Disabled {
			detail.Type = PolicyChangeDisabled
			detail.Summary = "Policy disabled"
			detail.RequiresImmediateAction = true
		} else {
			detail.Type = PolicyChangeEnabled
			detail.Summary = "Policy enabled"
			detail.RequiresImmediateAction = true
		}
		detail.AffectedFields = append(detail.AffectedFields, "disabled")
		return detail
	}

	// 如果策略被禁用,其他变更不重要
	if new.Spec.Disabled {
		detail.Summary = "Policy is disabled, changes ignored"
		detail.RequiresImmediateAction = false
		return detail
	}

	// 检查 Action 变化 (Allow/Deny)
	if old.Spec.Action != new.Spec.Action {
		detail.Type = PolicyChangeActionChanged
		detail.Summary = fmt.Sprintf("Action changed: %s → %s", old.Spec.Action, new.Spec.Action)
		detail.AffectedFields = append(detail.AffectedFields, "action")
		detail.RequiresImmediateAction = true
		return detail
	}

	// 检查优先级变化
	if old.Spec.Priority != new.Spec.Priority {
		detail.Type = PolicyChangePriorityChanged
		detail.Summary = fmt.Sprintf("Priority changed: %d → %d", old.Spec.Priority, new.Spec.Priority)
		detail.AffectedFields = append(detail.AffectedFields, "priority")
		detail.RequiresImmediateAction = true
	}

	// 检查规则变化
	if !pcd.rulesEqual(old.Spec.Rules, new.Spec.Rules) {
		detail.Type = PolicyChangeRulesModified
		detail.Summary = fmt.Sprintf("Rules modified: %d rules → %d rules",
			len(old.Spec.Rules), len(new.Spec.Rules))
		detail.AffectedFields = append(detail.AffectedFields, "rules")
		detail.RequiresImmediateAction = true
	}

	// 如果没有实质性变更
	if len(detail.AffectedFields) == 0 {
		detail.Summary = "No significant changes detected"
		detail.RequiresImmediateAction = false
	}

	return detail
}

// rulesEqual 比较两个规则列表是否相等
func (pcd *PolicyChangeDetector) rulesEqual(
	old, new []wireflowv1alpha1.Rule) bool {

	if len(old) != len(new) {
		return false
	}

	for i := range old {
		if !pcd.ruleEqual(old[i], new[i]) {
			return false
		}
	}

	return true
}

// ruleEqual 比较两个规则是否相等
func (pcd *PolicyChangeDetector) ruleEqual(
	old, new wireflowv1alpha1.Rule) bool {

	if old.Name != new.Name {
		return false
	}

	if old.Action != new.Action {
		return false
	}

	if old.Protocols != new.Protocols {
		return false
	}

	if !pcd.ruleSelectorEqual(old.Source, new.Source) {
		return false
	}

	if !pcd.ruleSelectorEqual(old.Destination, new.Destination) {
		return false
	}

	// 可以添加 TimeWindow 比较

	return true
}

// ruleSelectorEqual 比较两个规则选择器是否相等
func (pcd *PolicyChangeDetector) ruleSelectorEqual(
	old, new wireflowv1alpha1.RuleSelector) bool {

	if old.Any != new.Any {
		return false
	}

	if !stringSliceEqual(old.NodeName, new.NodeName) {
		return false
	}

	if !stringSliceEqual(old.IPBlocks, new.IPBlocks) {
		return false
	}

	if !stringSliceEqual(old.LabelSelctor, new.LabelSelctor) {
		return false
	}

	return true
}
