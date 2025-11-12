package controller

import (
	"context"
	"fmt"

	wireflowv1alpha1 "github.com/wireflowio/wireflow-controller/pkg/apis/wireflowcontroller/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
)

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// nodeQueue.
func (c *Controller) runNetworkWorker(ctx context.Context) {
	for c.processNextWorkNetwork(ctx) {
	}
}

// processNextWorkItem will read a single work item off the nodeQueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkNetwork(ctx context.Context) bool {
	item, shutdown := c.networkQueue.Get()
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
	defer c.networkQueue.Done(item)

	// Run the syncHandler, passing it the structured reference to the object to be synced.
	err := c.syncNetworkHandler(ctx, item)
	if err == nil {
		// If no error occurs then we Forget this item so it does not
		// get queued again until another change happens.
		c.networkQueue.Forget(item)
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
	c.networkQueue.AddRateLimited(item)
	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Node resource
// with the current status of the resource.
func (c *Controller) syncNetworkHandler(ctx context.Context, item WorkerItem) error {
	logger := klog.FromContext(ctx)
	// Get the Node resource with this namespace/name
	namespace, name := item.Key.Namespace, item.Key.Name

	_, err := c.networkLister.Networks(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Network not found, delete network", "namespace", namespace, "name", name)
			return c.handleNetworkDeletion(ctx, namespace, name)
		}
	}

	return c.reconcileNetwork(ctx, item)
}

func (c *Controller) reconcileNetwork(ctx context.Context, item WorkerItem) error {
	logger := klog.FromContext(ctx)
	network := item.NewObject.(*wireflowv1alpha1.Network)
	logger.Info("Reconciling network", "network", network.Name,
		"nodes", network.Spec.Nodes)

	// 1. 确定哪些节点应该加入此网络
	desiredNodes, err := c.getDesiredNodesForNetwork(ctx, network)
	if err != nil {
		return fmt.Errorf("failed to get desired nodes: %v", err)
	}

	//check policy changed
	policyChanged, err := c.checkPolicy(ctx, item)
	if err != nil {
		return fmt.Errorf("failed to check policy changes: %v", err)
	}

	// 2. 获取当前实际加入此网络的节点
	actualNodes, err := c.getActualNodesInNetwork(ctx, network)
	if err != nil {
		return fmt.Errorf("failed to get actual nodes: %v", err)
	}

	// 3. 计算差异
	toAdd, toRemove := c.calculateNodeDiff(desiredNodes, actualNodes)

	logger.Info("Node diff calculated",
		"network", network.Name,
		"desired", len(desiredNodes),
		"actual", len(actualNodes),
		"toAdd", len(toAdd),
		"toRemove", len(toRemove))

	// 4. 将节点加入网络
	for _, nodeName := range toAdd {
		if err = c.addNodeToNetwork(ctx, network, nodeName); err != nil {
			logger.Error(err, "Failed to add node to network",
				"node", nodeName, "network", network.Name)
			// 继续处理其他节点
		}
	}

	// 5. 将节点从网络移除
	for _, nodeName := range toRemove {
		if err = c.removeNodeFromNetwork(ctx, network, nodeName); err != nil {
			logger.Error(err, "Failed to remove node from network",
				"node", nodeName, "network", network.Name)
			// 继续处理其他节点
		}
	}

	// ====== 步骤 4: 如果有 Policy 变更,更新所有 Nodes ======
	if policyChanged {
		logger.Info("Policy changed, updating all nodes in network",
			"network", network.Name)

		if err := c.updateNodesForPolicyChange(ctx, network); err != nil {
			logger.Error(err, "Failed to update nodes for policy change",
				"network", network.Name)
			// 不返回错误,继续更新 Network 状态
		}
	}

	// 6. 更新 Network 状态
	return c.updateNetworkStatus(ctx, network, func(status *wireflowv1alpha1.NetworkStatus) {
		status.Phase = wireflowv1alpha1.NetworkPhaseReady
		status.AddedNodes = len(desiredNodes)
		status.ObservedGeneration = network.Generation

		c.setNetworkCondition(status, "Ready", metav1.ConditionTrue,
			"NetworkReady", "Network is ready")

		if policyChanged {
			c.setNetworkCondition(status, "PolicyChanged", metav1.ConditionTrue,
				"PolicyChanged", "Policy Updated")
		}
	})

}

func (c *Controller) checkPolicy(ctx context.Context, item WorkerItem) (bool, error) {

	switch item.EventType {
	case AddEvent:
		network := item.NewObject.(*wireflowv1alpha1.Network)
		if len(network.Spec.Polices) > 0 {
			return true, nil
		}
	case UpdateEvent:
		oldNetwork, network := item.OldObject.(*wireflowv1alpha1.Network), item.NewObject.(*wireflowv1alpha1.Network)
		if !policesEqual(oldNetwork.Spec.Polices, network.Spec.Polices) {
			return true, nil
		}

		return c.checkAndHandlePolicyChanges(ctx, network)
	}

	return false, nil
}

func policesEqual(p1, p2 []string) bool {
	if len(p1) != len(p2) {
		return false
	}

	for i := range p1 {
		if p1[i] != p2[i] {
			return false
		}
	}

	return true
}

// checkAndHandlePolicyChanges 检查并处理 Policy 变更
func (c *Controller) checkAndHandlePolicyChanges(ctx context.Context,
	network *wireflowv1alpha1.Network) (bool, error) {

	logger := klog.FromContext(ctx)

	// 检查是否有 Policy 变更注解
	if !c.hasPolicyChangeAnnotation(network) {
		logger.V(4).Info("No policy change annotation found", "network", network.Name)
		return false, nil
	}

	// 获取变更信息
	changeType := network.Annotations["wireflow.io/policy-change-type"]
	policyName := network.Annotations["wireflow.io/policy-name"]

	logger.Info("Detected policy change",
		"network", network.Name,
		"changeType", changeType,
		"policy", policyName)

	// 验证 Policy 是否仍在 Network 的 Polices 列表中
	policyStillApplied := false
	for _, p := range network.Spec.Polices {
		if p == policyName {
			policyStillApplied = true
			break
		}
	}

	if !policyStillApplied && changeType != string(PolicyChangeDeleted) {
		logger.Info("Policy no longer applied to network, skipping update",
			"network", network.Name,
			"policy", policyName)

		// 清理注解
		c.clearPolicyChangeAnnotation(ctx, network)
		return false, nil
	}

	// 清理注解 (处理完后清除)
	if err := c.clearPolicyChangeAnnotation(ctx, network); err != nil {
		logger.Error(err, "Failed to clear policy change annotation",
			"network", network.Name)
		// 不返回错误,继续处理
	}

	return true, nil
}

// hasPolicyChangeAnnotation 检查是否有 Policy 变更注解
func (c *Controller) hasPolicyChangeAnnotation(network *wireflowv1alpha1.Network) bool {
	if network.Annotations == nil {
		return false
	}
	_, hasType := network.Annotations["wireflow.io/policy-change-type"]
	_, hasName := network.Annotations["wireflow.io/policy-name"]
	return hasType && hasName
}

// clearPolicyChangeAnnotation 清理 Policy 变更注解
func (c *Controller) clearPolicyChangeAnnotation(ctx context.Context,
	network *wireflowv1alpha1.Network) error {

	return c.updateNetworkSpec(ctx, network.Namespace, network.Name, func(network *wireflowv1alpha1.Network) {
		delete(network.Annotations, "wireflow.io/policy-change-type")
		delete(network.Annotations, "wireflow.io/policy-change-time")
		delete(network.Annotations, "wireflow.io/policy-name")
	})
}

// updateNodesForPolicyChange 为 Policy 变更更新所有 Nodes
func (c *Controller) updateNodesForPolicyChange(ctx context.Context,
	network *wireflowv1alpha1.Network) error {

	logger := klog.FromContext(ctx)

	// 1. 获取所有属于该网络的 Nodes
	nodes, err := c.GetNodesByNetworkName(ctx, network)
	if err != nil {
		return fmt.Errorf("failed to get nodes for network: %w", err)
	}

	if len(nodes) == 0 {
		logger.Info("No nodes in network", "network", network.Name)
		return nil
	}

	logger.Info("Updating nodes for policy change",
		"network", network.Name,
		"nodeCount", len(nodes))

	// 2. 为每个 Node 触发策略更新
	var updateErrors []error
	successCount := 0

	for _, node := range nodes {
		logger.Info("Triggering policy update for node",
			"node", node.Name,
			"network", network.Name)

		if err := c.triggerNodePolicyUpdate(ctx, node, network); err != nil {
			logger.Error(err, "Failed to trigger node policy update",
				"node", node.Name)
			updateErrors = append(updateErrors, err)
		} else {
			successCount++
		}
	}

	logger.Info("Policy update triggered for nodes",
		"network", network.Name,
		"success", successCount,
		"failed", len(updateErrors),
		"total", len(nodes))

	if len(updateErrors) > 0 {
		return fmt.Errorf("failed to update %d/%d nodes",
			len(updateErrors), len(nodes))
	}

	return nil
}

func (c *Controller) GetNodesByNetworkName(ctx context.Context,
	network *wireflowv1alpha1.Network) ([]*wireflowv1alpha1.Node, error) {
	logger := klog.FromContext(ctx)
	logger.V(4).Info("Getting nodes by network", "network", network.Name)
	objs, err := c.nodeInformer.Informer().GetIndexer().ByIndex("network", network.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes by network: %v", err)
	}

	ans := make([]*wireflowv1alpha1.Node, 0)
	for _, obj := range objs {
		node := obj.(*wireflowv1alpha1.Node)
		ans = append(ans, node)
	}

	return ans, nil
}

// triggerNodePolicyUpdate 触发单个 Node 的策略更新
func (c *Controller) triggerNodePolicyUpdate(ctx context.Context,
	node *wireflowv1alpha1.Node, network *wireflowv1alpha1.Network) error {

	logger := klog.FromContext(ctx)

	// 方式 1: 更新 Node Status 标记需要重新应用策略
	if err := c.updateNodeStatus(ctx, node.Namespace, node.Name,
		func(status *wireflowv1alpha1.NodeStatus) {
			// 标记策略需要更新
			status.Phase = wireflowv1alpha1.NodeUpdatingPolicy
		}); err != nil {
		return err
	}

	logger.V(4).Info("Node policy update triggered",
		"node", node.Name,
		"network", network.Name)

	return nil
}
