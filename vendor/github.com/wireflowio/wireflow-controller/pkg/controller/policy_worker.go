package controller

import (
	"context"
	"fmt"
	"time"

	wireflowv1alpha1 "github.com/wireflowio/wireflow-controller/pkg/apis/wireflowcontroller/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
)

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// nodeQueue.
func (c *Controller) runNetworkPolicyWorker(ctx context.Context) {
	for c.processNextNetworkPolicy(ctx) {
	}
}

// processNextWorkItem will read a single work item off the nodeQueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextNetworkPolicy(ctx context.Context) bool {
	item, shutdown := c.networkPolicyQueue.Get()
	logger := klog.FromContext(ctx)

	if shutdown {
		return false
	}

	// We call Done at the end of this func so the nodeQueue knows we have
	// finished processing this item. We also must remember to call Forget
	// if we do not want this work item being re-queued. For example, we do
	// not call Forget if a transient error occurs, instead the item is
	// put back on the nodeQueue and attempted again after a back-off
	// period.
	defer c.networkPolicyQueue.Done(item)

	// Run the syncHandler, passing it the structured reference to the object to be synced.
	err := c.syncNetworkPolicyHandler(ctx, item)
	if err == nil {
		// If no error occurs then we Forget this item so it does not
		// get queued again until another change happens.
		c.nodeQueue.Forget(item)
		logger.Info("Successfully synced", "objectName", item)
		return true
	}
	// there was a failure so be sure to report it.  This method allows for
	// pluggable error handling which can be used for things like
	// cluster-monitoring.
	utilruntime.HandleErrorWithContext(ctx, err, "Error syncing; requeuing for later retry", "objectReference", item)
	// since we failed, we should requeue the item to work on later.  This
	// method will add a backoff to avoid hotlooping on particular items
	// (they're probably still not going to work right away) and overall
	// controller protection (everything I've done is broken, this controller
	// needs to calm down or it can starve other useful work) cases.
	c.networkPolicyQueue.AddRateLimited(item)
	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Node resource
// with the current status of the resource.
func (c *Controller) syncNetworkPolicyHandler(ctx context.Context, item WorkerItem) error {
	// Get the Node resource with this namespace/name
	namespace, name := item.Key.Namespace, item.Key.Name
	logger := klog.FromContext(ctx)
	switch item.EventType {
	case AddEvent:
		logger.Info("Handling network policy add: %s/%s", namespace, name)
	case UpdateEvent:
		logger.Info("Handling network policy update: %s/%s", namespace, name)
		return c.handleNetworkPolicyUpdate(ctx, item)
	case DeleteEvent:
		logger.Info("Handling network policy delete: %s/%s", namespace, name)
		return c.handleNetworkPolicyCleanup(ctx, namespace, name)
	}

	return nil

}

// handleNetworkPolicyAdd 处理策略创建
func (c *Controller) handleNetworkPolicyAdd(ctx context.Context, namespace, name string) error {
	logger := klog.FromContext(ctx)
	logger.Info("🆕 Handling policy add", "policy", name)

	policy, err := c.networkPolicyLister.NetworkPolicies(namespace).Get(name)
	if err != nil {
		return err
	}

	// 创建变更详情
	changeDetail := c.policyChangeDetector.DetectChanges(nil, policy)

	// 如果策略被禁用,不需要立即应用
	if policy.Spec.Disabled {
		logger.Info("Policy is disabled, skipping propagation", "policy", name)
		return nil
	}

	// 传播到相关的 Networks
	return c.propagatePolicyChange(ctx, changeDetail)
}

// handleNetworkPolicyUpdate

// handleNetworkPolicyUpdate 处理策略更新
func (c *Controller) handleNetworkPolicyUpdate(ctx context.Context, item WorkerItem) error {
	logger := klog.FromContext(ctx)
	namespace, name := item.Key.Namespace, item.Key.Name
	logger.Info("🔄 Handling policy update", "policy", item.Key.Name)

	newPolicy, err := c.networkPolicyLister.NetworkPolicies(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Policy not found, might be deleted", "policy", name)
			return nil
		}
		return err
	}

	// 获取旧版本 (从缓存中获取)
	var oldPolicy *wireflowv1alpha1.NetworkPolicy
	if item.OldObject != nil {
		oldPolicy = item.OldObject.(*wireflowv1alpha1.NetworkPolicy)
	}

	// 检测变更
	changeDetail := c.policyChangeDetector.DetectChanges(oldPolicy, newPolicy)

	logger.Info("Policy change detected",
		"policy", name,
		"changeType", changeDetail.Type,
		"summary", changeDetail.Summary,
		"requiresAction", changeDetail.RequiresImmediateAction)

	// 如果没有需要处理的变更,跳过
	if !changeDetail.RequiresImmediateAction {
		logger.V(4).Info("No immediate action required for policy change", "policy", name)
		return nil
	}

	// 传播变更
	return c.propagatePolicyChange(ctx, changeDetail)
}

// handleNetworkPolicyDelete 处理策略删除
func (c *Controller) handleNetworkPolicyDelete(ctx context.Context, namespace, name string) error {
	logger := klog.FromContext(ctx)
	logger.Info("🗑️ Handling policy delete", "policy", name)

	// 创建删除变更详情
	changeDetail := &PolicyChangeDetail{
		Type:                    PolicyChangeDeleted,
		PolicyName:              name,
		PolicyNamespace:         namespace,
		Summary:                 "Policy deleted",
		RequiresImmediateAction: true,
	}

	// 传播删除事件
	return c.propagatePolicyChange(ctx, changeDetail)
}

func (c *Controller) handleNetworkPolicyCleanup(ctx context.Context, namespace, name string) error {
	return nil
}

// propagatePolicyChange 传播策略变更到 Networks 和 Nodes
func (c *Controller) propagatePolicyChange(ctx context.Context,
	changeDetail *PolicyChangeDetail) error {

	logger := klog.FromContext(ctx)
	logger.Info("Propagating policy change",
		"policy", changeDetail.PolicyName,
		"changeType", changeDetail.Type)

	// 1. 查找所有使用该 Policy 的 Networks
	affectedNetworks, err := c.findNetworksByPolicy(ctx,
		changeDetail.PolicyNamespace, changeDetail.PolicyName)
	if err != nil {
		return fmt.Errorf("failed to find affected networks: %w", err)
	}

	if len(affectedNetworks) == 0 {
		logger.Info("No networks affected by policy change", "policy", changeDetail.PolicyName)
		return nil
	}

	logger.Info("Found affected networks",
		"policy", changeDetail.PolicyName,
		"networkCount", len(affectedNetworks))

	// 2. 为每个 Network 触发更新
	var propagationErrors []error
	for _, network := range affectedNetworks {
		logger.Info("Notifying network of policy change",
			"network", network.Name,
			"policy", changeDetail.PolicyName)

		// 添加 Annotation 触发 Network reconcile
		if err = c.notifyNetworkOfPolicyChange(ctx, network, changeDetail); err != nil {
			logger.Error(err, "Failed to notify network", "network", network.Name)
			propagationErrors = append(propagationErrors, err)
			continue
		}
	}

	if len(propagationErrors) > 0 {
		return fmt.Errorf("failed to propagate to %d networks", len(propagationErrors))
	}

	return nil
}

// findNetworksByPolicy 查找使用指定 Policy 的所有 Networks
func (c *Controller) findNetworksByPolicy(ctx context.Context,
	namespace, policyName string) ([]*wireflowv1alpha1.Network, error) {
	logger := klog.FromContext(ctx)
	logger.Info("Finding networks using policy", "policy", policyName)
	objs, err := c.networkInformer.Informer().GetIndexer().ByIndex("policy", policyName)
	networks := make([]*wireflowv1alpha1.Network, 0, len(objs))
	if err == nil && len(objs) > 0 {
		for _, obj := range objs {
			networks = append(networks, obj.(*wireflowv1alpha1.Network))
		}
		return networks, nil
	}

	affectedNetworks := make([]*wireflowv1alpha1.Network, 0)
	for _, network := range networks {
		// 检查 Network 是否使用该 Policy
		for _, policy := range network.Spec.Polices {
			if policy == policyName {
				affectedNetworks = append(affectedNetworks, network)
				break
			}
		}
	}

	return affectedNetworks, nil
}

// notifyNetworkOfPolicyChange 通过注解通知 Network 策略已变更
func (c *Controller) notifyNetworkOfPolicyChange(ctx context.Context,
	network *wireflowv1alpha1.Network, changeDetail *PolicyChangeDetail) error {

	// 添加 Annotation 记录策略变更
	return c.updateNetworkSpec(ctx, network.Namespace, network.Name,
		func(net *wireflowv1alpha1.Network) {
			if net.Annotations == nil {
				net.Annotations = make(map[string]string)
			}

			// 记录最后一次策略变更
			net.Annotations["wireflow.io/policy-change-type"] = string(changeDetail.Type)
			net.Annotations["wireflow.io/policy-change-time"] = time.Now().Format(time.RFC3339)
			net.Annotations["wireflow.io/policy-name"] = changeDetail.PolicyName
		})
}
