package controller

import (
	"context"
	"fmt"
	"sort"

	wireflowv1alpha1 "github.com/wireflowio/wireflow-controller/pkg/apis/wireflowcontroller/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
)

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// nodeQueue.
func (c *Controller) runNodeWorker(ctx context.Context) {
	for c.processNextWorkNode(ctx) {
	}
}

// processNextWorkItem will read a single work item off the nodeQueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkNode(ctx context.Context) bool {
	item, shutdown := c.nodeQueue.Get()
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
	defer c.nodeQueue.Done(item)

	// Run the syncHandler, passing it the structured reference to the object to be synced.
	err := c.syncNodeHandler(ctx, item)
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
	c.nodeQueue.AddRateLimited(item)
	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Node resource
// with the current status of the resource.
func (c *Controller) syncNodeHandler(ctx context.Context, item WorkerItem) error {
	// Get the Node resource with this namespace/name
	namespace, name := item.Key.Namespace, item.Key.Name
	logger := klog.FromContext(ctx)

	node, err := c.nodesLister.Nodes(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			//节点被删除，清理
			logger.Info("Node not found, delete node", "namespace", namespace, "name", name)
			return c.handleNodeCleanup(ctx, namespace, name)
		}
	}

	return c.reconcileNode(ctx, node)
}

// reconcileNode reconciles a Node object
func (c *Controller) reconcileNode(ctx context.Context, node *wireflowv1alpha1.Node) error {
	logger := klog.FromContext(ctx)

	if node.Status.ObservedGeneration == node.Generation {
		// Spec没有变化
	}

	action := c.determineAction(node)

	logger.Info("Reconcile node", "node", node.Name, "currentPhase", node.Status.Phase, "action", action, "specNetworks", node.Spec.Network)

	switch action {
	case ActionInitialize:
		return c.handleNodeInitialize(ctx, node.Namespace, node.Name)
	case ActionJoinNetwork:
		return c.handleNodeJoinNetwork(ctx, node.Namespace, node.Name)
	case ActionLeaveNetwork:
		return c.handleNodeLeaveNetwork(ctx, node.Namespace, node.Name)
	case ActionUpdateNetwork:
		return c.handleNodeUpdateNetwork(ctx, node.Namespace, node.Name)
	case ActionUpdatePolicy:
		return c.handleNodePolicyUpdate(ctx, node.Namespace, node.Name)

	default:
		return c.updateNodeStatus(ctx, node.Namespace, node.Name, func(status *wireflowv1alpha1.NodeStatus) {
			status.ObservedGeneration = node.Generation
		})
	}

	return nil
}

// determineAction
func (c *Controller) determineAction(node *wireflowv1alpha1.Node) NodeAction {
	// 1. 新创建的节点
	if node.Status.Phase == "" || node.Status.Phase == wireflowv1alpha1.NodePending {
		if len(node.Spec.Network) > 0 {
			return ActionInitialize
		}
		return ActionNone
	}

	// 2. 节点处于错误状态
	if node.Status.Phase == wireflowv1alpha1.NodeFailed {
		return ActionRecover
	}

	// 3. 节点正在处理中,等待完成
	if node.Status.Phase == wireflowv1alpha1.NodeProvisioning ||
		node.Status.Phase == wireflowv1alpha1.NodeUpdating ||
		node.Status.Phase == wireflowv1alpha1.NodeTerminating {
		// 继续当前流程
		return ActionNone
	}

	// 4. 比较 Spec.Network 和 Status.ActiveNetworks
	specNetworks := stringSet(node.Spec.Network)
	activeNetworks := stringSet(node.Status.ActiveNetworks)

	// 4.1 用户清空了网络配置 -> 离开所有网络
	if len(node.Spec.Network) == 0 && len(node.Status.ActiveNetworks) > 0 {
		return ActionLeaveNetwork
	}

	// 4.2 用户添加了网络 -> 加入新网络
	if len(specNetworks) > len(activeNetworks) {
		return ActionJoinNetwork
	}

	// 4.3 用户移除了某些网络 -> 离开网络
	if len(specNetworks) < len(activeNetworks) {
		return ActionLeaveNetwork
	}

	// 4.4 网络列表不同 -> 更新网络配置
	if !setsEqual(specNetworks, activeNetworks) {
		return ActionUpdateNetwork
	}

	if node.Status.Phase == wireflowv1alpha1.NodeUpdatingPolicy {
		return ActionUpdatePolicy
	}

	// 5. Spec 和 Status 一致,无需操作
	return ActionNone
}

// NodeAction 定义需要执行的动作
type NodeAction string

const (
	ActionNone          NodeAction = "None"
	ActionInitialize    NodeAction = "Initialize"
	ActionJoinNetwork   NodeAction = "JoinNetwork"
	ActionLeaveNetwork  NodeAction = "LeaveNetwork"
	ActionUpdateNetwork NodeAction = "UpdateNetwork"
	ActionTerminate     NodeAction = "Terminate"
	ActionRecover       NodeAction = "Recover"
	ActionUpdatePolicy  NodeAction = "UpdatePolicy"
)

// allocateIPsForNode 为节点在其所属的网络中分配 IP
func (c *Controller) allocateIPsForNode(ctx context.Context, namespace, name string) (string, error) {

	node, err := c.nodesLister.Nodes(namespace).Get(name)
	if err != nil {
		return "", fmt.Errorf("failed to get node %s/%s: %v", namespace, name, err)
	}

	if len(node.Spec.Network) == 0 {
		//clear node's address
		return "", nil
	}
	primaryNetwork := node.Spec.Network[0]

	// 获取 Network 资源
	network, err := c.networkLister.Networks(namespace).Get(primaryNetwork)
	if err != nil {
		return "", fmt.Errorf("failed to get network %s: %v", primaryNetwork, err)
	}

	// 如果节点已经有 IP 地址,跳过
	currentAddress := node.Spec.Address
	if currentAddress != "" {
		//校验ip是否是network合法ip
		if err = c.ipAllocator.ValidateIP(network.Spec.CIDR, currentAddress); err == nil {
			klog.Infof("Node %s/%s already has IP: %s", node.Namespace, node.Name, node.Spec.Address)
			return currentAddress, nil
		}
	}

	// 检查节点是否已经在该网络中有 IP 分配
	existingIP := c.ipAllocator.GetNodeIP(network, node.Name)
	if existingIP != "" {
		//校验ip是否是network合法ip
		klog.Infof("Node %s already has IP %s in network %s", node.Name, existingIP, network.Name)
		return existingIP, nil
	}

	// 分配新的 IP
	return c.allocate(ctx, network, node)
}

func (c *Controller) allocate(ctx context.Context, network *wireflowv1alpha1.Network, node *wireflowv1alpha1.Node) (string, error) {
	var (
		err         error
		allocatedIP string
	)
	allocatedIP, err = c.ipAllocator.AllocateIP(network, node.Name)
	if err != nil {
		return "", fmt.Errorf("failed to allocate IP: %v", err)
	}

	klog.Infof("Allocated IP %s to node %s in network %s", allocatedIP, node.Name, network.Name)

	// 更新 Network 资源,记录 IP 分配
	if err = c.updateNetworkIPAllocation(ctx, network, allocatedIP, node.Name); err != nil {
		return "", fmt.Errorf("failed to update network IP allocation: %v", err)
	}

	return allocatedIP, nil
}

// updateNetworkIPAllocation 更新网络的 IP 分配记录
func (c *Controller) updateNetworkIPAllocation(ctx context.Context, network *wireflowv1alpha1.Network, ip, nodeName string) error {
	logger := klog.FromContext(ctx)
	networkCopy := network.DeepCopy()

	// 添加 IP 分配记录
	allocation := wireflowv1alpha1.IPAllocation{
		IP:          ip,
		Node:        nodeName,
		AllocatedAt: metav1.Now(),
	}

	networkCopy.Status.AllocatedIPs = append(networkCopy.Status.AllocatedIPs, allocation)

	// 更新可用 IP 数量
	availableIPs, err := c.ipAllocator.CountAvailableIPs(networkCopy)
	if err != nil {
		klog.Errorf("Failed to count available IPs: %v", err)
	} else {
		networkCopy.Status.AvailableIPs = availableIPs
	}

	// 更新 Network Status 资源
	_, err = c.wireflowclientset.WireflowcontrollerV1alpha1().Networks(network.Namespace).UpdateStatus(
		ctx, networkCopy, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update network status: %v", err)
	}

	logger.Info("Updated network status", network.Name, "ip", ip, "nodeName", nodeName)
	return nil
}

// handleNodeDeleteEvent 处理节点删除事件
func (c *Controller) handleNodeDeleteEvent(ctx context.Context, namespace, name string) error {
	logger := klog.FromContext(ctx)
	logger.Info("Node delete event", "namespace", namespace, "name", name)

	// 释放节点占用的 IP 地址
	if err := c.releaseIPsForNode(ctx, namespace, name); err != nil {
		return fmt.Errorf("failed to release IPs for node: %v", err)
	}

	return nil
}

// releaseIPsForNode 释放节点占用的 IP 地址
func (c *Controller) releaseIPsForNode(ctx context.Context, namespace, name string) error {
	node, err := c.nodesLister.Nodes(namespace).Get(name)
	if err != nil {
		return fmt.Errorf("failed to get node %s/%s: %v", namespace, name, err)
	}
	for _, networkName := range node.Spec.Network {
		network, err := c.networkLister.Networks(namespace).Get(networkName)
		if err != nil {
			klog.Errorf("Failed to get network %s: %v", networkName, err)
			continue
		}

		// 查找并释放该节点的 IP
		var nodeIP string
		for _, allocation := range network.Status.AllocatedIPs {
			if allocation.Node == name {
				nodeIP = allocation.IP
				break
			}
		}

		if nodeIP == "" {
			continue
		}

		// 从 Network 的 AllocatedIPs 中移除
		networkCopy := network.DeepCopy()
		newAllocations := []wireflowv1alpha1.IPAllocation{}
		for _, allocation := range networkCopy.Status.AllocatedIPs {
			if allocation.Node != node.Name {
				newAllocations = append(newAllocations, allocation)
			}
		}
		networkCopy.Status.AllocatedIPs = newAllocations

		// 更新可用 IP 数量
		availableIPs, err := c.ipAllocator.CountAvailableIPs(networkCopy)
		if err != nil {
			klog.Errorf("Failed to count available IPs: %v", err)
		} else {
			networkCopy.Status.AvailableIPs = availableIPs
		}

		// 更新 Network
		_, err = c.wireflowclientset.WireflowcontrollerV1alpha1().Networks(network.Namespace).Update(
			ctx, networkCopy, metav1.UpdateOptions{})
		if err != nil {
			klog.Errorf("Failed to update network %s: %v", networkName, err)
			continue
		}

		klog.Infof("Released IP %s from node %s in network %s", nodeIP, node.Name, networkName)
	}

	return nil
}

// updateNodeSpec 更新Node的Spec和Labels
func (c *Controller) updateNodeSpec(ctx context.Context, namespace, name string, updateFunc func(node *wireflowv1alpha1.Node)) error {
	logger := klog.FromContext(ctx)
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		node, err := c.nodesLister.Nodes(namespace).Get(name)
		if err != nil {
			return err
		}
		nodeCopy := node.DeepCopy()
		if nodeCopy.Labels == nil {
			nodeCopy.Labels = make(map[string]string)
		}

		oldSpec := nodeCopy.Spec.DeepCopy()

		// update the node spec
		updateFunc(nodeCopy)

		if SpecEqual(oldSpec, &nodeCopy.Spec) {
			logger.V(5).Info("Node spec not changed", "node", nodeCopy.Name)
			return nil
		}

		logger.V(4).Info("Updating node spec", "node", nodeCopy.Name)

		_, err = c.wireflowclientset.WireflowcontrollerV1alpha1().Nodes(namespace).Update(
			ctx, nodeCopy, metav1.UpdateOptions{})
		if err != nil {
			if errors.IsConflict(err) {
				logger.V(4).Info("Node spec update conflicted, retrying")
			} else {
				logger.Error(err, "Failed to update node spec", nodeCopy.Name)
			}
		}
		return err
	})
}

// SpecEqual 比较两个 Spec 是否相等
func SpecEqual(old, new *wireflowv1alpha1.NodeSpec) bool {
	if old.Address != new.Address {
		return false
	}
	if !stringSliceEqual(old.Network, new.Network) {
		return false
	}
	// 根据需要添加其他字段比较
	return true
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (c *Controller) GetNodeByNetworkName(networkName string) ([]*wireflowv1alpha1.Node, error) {
	objs, err := c.nodeInformer.Informer().GetIndexer().ByIndex("network", networkName)

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

func (c *Controller) updateNodeForPolicyUpdate() error {
	return nil
}

// updateNodeStatus
func (c *Controller) updateNodeStatus(ctx context.Context,
	namespace, name string,
	updateFunc func(status *wireflowv1alpha1.NodeStatus)) error {
	logger := klog.FromContext(ctx)
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		latest, err := c.nodesLister.Nodes(namespace).Get(name)
		if err != nil {
			return err
		}

		nodeCopy := latest.DeepCopy()
		oldStatus := nodeCopy.Status.DeepCopy()
		updateFunc(&nodeCopy.Status)

		if StatusEqual(oldStatus, &nodeCopy.Status) {
			logger.V(5).Info("Node status not changed", "node", nodeCopy.Name)
			return nil
		}

		// 5. 打印变化日志
		logger.V(4).Info("Updating node status",
			"node", nodeCopy.Name,
			"oldPhase", oldStatus.Phase,
			"newPhase", nodeCopy.Status.Phase,
			"oldGeneration", oldStatus.ObservedGeneration,
			"newGeneration", nodeCopy.Status.ObservedGeneration)
		_, err = c.wireflowclientset.WireflowcontrollerV1alpha1().Nodes(namespace).UpdateStatus(
			ctx, nodeCopy, metav1.UpdateOptions{})

		if err != nil {
			if errors.IsConflict(err) {
				logger.V(4).Info("Node status update conflicted, retrying")
			} else {
				logger.Error(err, "Failed to update node status", nodeCopy.Name)
			}
		}
		return err
	})
}

// StatusEqual 比较两个 Status 是否相等
func StatusEqual(old, new *wireflowv1alpha1.NodeStatus) bool {
	if old.Phase != new.Phase {
		return false
	}
	if old.ObservedGeneration != new.ObservedGeneration {
		return false
	}
	if old.AllocatedAddress != new.AllocatedAddress {
		return false
	}
	if !stringSliceEqual(old.ActiveNetworks, new.ActiveNetworks) {
		return false
	}
	if !conditionsEqual(old.Conditions, new.Conditions) {
		return false
	}
	return true
}

func conditionsEqual(old, new []metav1.Condition) bool {
	if len(old) != len(new) {
		return false
	}

	oldMap := make(map[string]metav1.Condition)
	for _, c := range old {
		oldMap[c.Type] = c
	}

	for _, newCond := range new {
		oldCond, exists := oldMap[newCond.Type]
		if !exists {
			return false
		}
		// 比较除了 LastTransitionTime 之外的字段
		if oldCond.Status != newCond.Status ||
			oldCond.Reason != newCond.Reason ||
			oldCond.Message != newCond.Message ||
			oldCond.ObservedGeneration != newCond.ObservedGeneration {
			return false
		}
	}

	return true
}

// updateNodeCondition
func (c *Controller) updateNodeCondition(ctx context.Context, node *wireflowv1alpha1.Node, conditionType string, status metav1.ConditionStatus, reason, message string) error {

	laest, err := c.nodesLister.Nodes(node.Namespace).Get(node.Name)
	if err != nil {
		return err
	}

	nodeCopy := laest.DeepCopy()
	found := false

	//更新或者创建 condition
	now := metav1.Now()
	for i := range nodeCopy.Status.Conditions {
		if nodeCopy.Status.Conditions[i].Type == conditionType {
			nodeCopy.Status.Conditions[i].Status = status
			nodeCopy.Status.Conditions[i].LastTransitionTime = now
			nodeCopy.Status.Conditions[i].Reason = reason
			nodeCopy.Status.Conditions[i].Message = message
			found = true
			break
		}
	}

	if !found {
		nodeCopy.Status.Conditions = append(nodeCopy.Status.Conditions, metav1.Condition{
			Type:               conditionType,
			Status:             status,
			LastTransitionTime: now,
			Reason:             reason,
			Message:            message,
		})
	}

	_, err = c.wireflowclientset.WireflowcontrollerV1alpha1().Nodes(node.Namespace).Update(
		ctx, nodeCopy, metav1.UpdateOptions{})
	return err
}

// 辅助函数
func stringSet(list []string) map[string]struct{} {
	set := make(map[string]struct{}, len(list))
	for _, item := range list {
		set[item] = struct{}{}
	}
	return set
}

func setsEqual(a, b map[string]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, exists := b[k]; !exists {
			return false
		}
	}
	return true
}

// handleNodePolicyUpdate 处理 Node 的策略更新
func (c *Controller) handleNodePolicyUpdate(ctx context.Context,
	namespace, name string) error {
	logger := klog.FromContext(ctx)
	node, err := c.nodesLister.Nodes(namespace).Get(name)
	if err != nil {
		return fmt.Errorf("failed to get node %s/%s: %v", namespace, name, err)
	}
	logger.Info("🔒 Handling node policy update", "node", node.Name)

	// 1. 标记为 Updating
	if err := c.updateNodeStatus(ctx, node.Namespace, node.Name,
		func(status *wireflowv1alpha1.NodeStatus) {
			status.Phase = wireflowv1alpha1.NodeUpdating

			c.setNodeCondition(status,
				wireflowv1alpha1.NodeConditionPolicyApplied,
				metav1.ConditionFalse,
				wireflowv1alpha1.ReasonUpdating,
				"Applying policy updates")

		}); err != nil {
		return err
	}

	// 2. 获取所有相关的 Polices
	policies, err := c.getPoliciesForNode(ctx, node)
	if err != nil {
		return fmt.Errorf("failed to get policies for node: %w", err)
	}

	logger.Info("Found policies for node",
		"node", node.Name,
		"policyCount", len(policies))

	// 3. 应用策略 (实际的策略应用逻辑)
	if err := c.applyPoliciesToNode(ctx, node, policies); err != nil {
		// 标记为失败
		c.updateNodeStatus(ctx, node.Namespace, node.Name,
			func(status *wireflowv1alpha1.NodeStatus) {
				status.Phase = wireflowv1alpha1.NodeFailed

				c.setNodeCondition(status,
					wireflowv1alpha1.NodeConditionPolicyApplied,
					metav1.ConditionFalse,
					wireflowv1alpha1.ReasonConfigFailed,
					fmt.Sprintf("Failed to apply policies: %v", err))

			})
		return err
	}

	// 4. 标记为成功
	logger.Info("✅ Policy applied successfully", "node", node.Name)
	return c.updateNodeStatus(ctx, node.Namespace, node.Name,
		func(status *wireflowv1alpha1.NodeStatus) {
			status.Phase = wireflowv1alpha1.NodeReady

			c.setNodeCondition(status,
				wireflowv1alpha1.NodeConditionPolicyApplied,
				metav1.ConditionTrue,
				wireflowv1alpha1.ReasonReady,
				fmt.Sprintf("Applied %d policies successfully", len(policies)))
		})
}

// getPoliciesForNode 获取 Node 相关的所有 Polices
func (c *Controller) getPoliciesForNode(ctx context.Context,
	node *wireflowv1alpha1.Node) ([]*wireflowv1alpha1.NetworkPolicy, error) {

	var allPolicies []*wireflowv1alpha1.NetworkPolicy

	// 遍历 Node 所属的每个 Network
	for _, networkName := range node.Spec.Network {
		network, err := c.networkLister.Networks(node.Namespace).Get(networkName)
		if err != nil {
			if errors.IsNotFound(err) {
				klog.Warningf("Network %s not found for node %s", networkName, node.Name)
				continue
			}
			return nil, err
		}

		// 获取 Network 的所有 Polices
		for _, policyName := range network.Spec.Polices {
			policy, err := c.networkPolicyLister.NetworkPolicies(node.Namespace).Get(policyName)
			if err != nil {
				if errors.IsNotFound(err) {
					klog.Warningf("Policy %s not found", policyName)
					continue
				}
				return nil, err
			}

			// 跳过被禁用的策略
			if policy.Spec.Disabled {
				klog.V(4).Infof("Policy %s is disabled, skipping", policyName)
				continue
			}

			allPolicies = append(allPolicies, policy)
		}
	}

	// 按优先级排序
	sort.Slice(allPolicies, func(i, j int) bool {
		return allPolicies[i].Spec.Priority > allPolicies[j].Spec.Priority
	})

	return allPolicies, nil
}

// applyPoliciesToNode 将策略应用到节点 (实际实现)
func (c *Controller) applyPoliciesToNode(ctx context.Context,
	node *wireflowv1alpha1.Node,
	policies []*wireflowv1alpha1.NetworkPolicy) error {

	logger := klog.FromContext(ctx)
	logger.Info("Applying policies to node",
		"node", node.Name,
		"policyCount", len(policies))

	// TODO: 实际的策略应用逻辑
	// 这里应该包含:
	// 1. 生成防火墙规则
	// 2. 更新网络配置
	// 3. 通知节点代理
	// 4. 等等...

	// 示例实现
	for _, policy := range policies {
		logger.V(4).Info("Applying policy",
			"node", node.Name,
			"policy", policy.Name,
			"action", policy.Spec.Action,
			"priority", policy.Spec.Priority)

		// 实际应用逻辑...
		// err := c.applyPolicyRules(node, policy)
		// if err != nil {
		//     return err
		// }
	}

	return c.updateNodeStatus(ctx, node.Namespace, node.Name, func(status *wireflowv1alpha1.NodeStatus) {
		status.Phase = wireflowv1alpha1.NodeReady
		c.setNodeCondition(status, wireflowv1alpha1.NodeConditionPolicyApplied,
			metav1.ConditionTrue,
			wireflowv1alpha1.ReasonReady,
			"Applied all policies successfully")
	})
}
