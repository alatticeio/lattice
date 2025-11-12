package controller

import (
	"context"
	"fmt"

	wireflowv1alpha1 "github.com/wireflowio/wireflow-controller/pkg/apis/wireflowcontroller/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

// handleNodeInitialize 处理Node初始化
func (c *Controller) handleNodeInitialize(ctx context.Context, namespace, name string) error {
	logger := klog.FromContext(ctx)
	logger.Info("Initializing node", "node", name)

	// 1. 更新 Phase 为 Provisioning
	if err := c.updateNodeStatus(ctx, namespace, name, func(status *wireflowv1alpha1.NodeStatus) {
		status.Phase = wireflowv1alpha1.NodeProvisioning
		c.setNodeCondition(status, wireflowv1alpha1.NodeConditionInitialized, metav1.ConditionUnknown,
			wireflowv1alpha1.ReasonInitializing, "Initializing node")
		c.setNodeCondition(status, wireflowv1alpha1.NodeConditionIPAllocated, metav1.ConditionUnknown,
			wireflowv1alpha1.ReasonInitializing, "Waiting for IP allocation")
		c.setNodeCondition(status, wireflowv1alpha1.NodeConditionNetworkConfigured, metav1.ConditionUnknown,
			wireflowv1alpha1.ReasonInitializing, "Waiting for network configuration")
	}); err != nil {
		return fmt.Errorf("failed to mark node as provisioning: %v", err)
	}

	// 2. 分配 IP 地址
	var (
		allocatedIP string
		err         error
	)
	if allocatedIP, err = c.allocateIPsForNode(ctx, namespace, name); err != nil {
		c.updateNodeStatus(ctx, namespace, name, func(status *wireflowv1alpha1.NodeStatus) {
			status.Phase = wireflowv1alpha1.NodeFailed
			c.setNodeCondition(status, wireflowv1alpha1.NodeConditionIPAllocated, metav1.ConditionFalse,
				wireflowv1alpha1.ReasonAllocationFailed, err.Error())
		})
		return err
	}

	// 3. 同步网络标签 Spec + labels
	if err = c.updateNodeSpec(ctx, namespace, name, func(node *wireflowv1alpha1.Node) {
		node.Spec.Address = allocatedIP
		labels := node.Labels

		for _, networkName := range node.Spec.Network {
			network, err := c.networkLister.Networks(namespace).Get(networkName)
			if err != nil {
				klog.Errorf("Failed to get network %s: %v", networkName, err)
				continue
			}
			if _, ok := labels[fmt.Sprintf("wireflow.io/network-%s", networkName)]; ok {
				continue
			}

			labels[fmt.Sprintf("wireflow.io/network-%s", networkName)] = network.Spec.Name
		}
		if _, ok := labels["wireflow.io/has-network"]; !ok {
			labels["wireflow.io/has-network"] = "true"
		}
		node.Labels = labels
	}); err != nil {
		return err
	}

	// 4. 更新状态为Status: Ready
	return c.updateNodeStatus(ctx, namespace, name, func(status *wireflowv1alpha1.NodeStatus) {
		node, err := c.nodesLister.Nodes(namespace).Get(name)
		if err != nil {
			logger.Error(err, "failed to get node")
		}
		status.Phase = wireflowv1alpha1.NodeReady
		status.ActiveNetworks = node.Spec.Network
		status.AllocatedAddress = node.Spec.Address
		status.ObservedGeneration = node.Generation
		now := metav1.Now()
		status.LastSyncTime = &now

		c.setNodeCondition(status, wireflowv1alpha1.NodeConditionProvisioned, metav1.ConditionTrue,
			wireflowv1alpha1.ReasonReady, "Node is ready")
		c.setNodeCondition(status, wireflowv1alpha1.NodeConditionIPAllocated, metav1.ConditionTrue,
			wireflowv1alpha1.ReasonReady, "IP allocated successfully")
		c.setNodeCondition(status, wireflowv1alpha1.NodeConditionNetworkConfigured, metav1.ConditionTrue,
			wireflowv1alpha1.ReasonReady, "Network configured")
	})
}

// handleNodeJoinNetwork
func (c *Controller) handleNodeJoinNetwork(ctx context.Context, namespace, name string) error {
	logger := klog.FromContext(ctx)
	logger.Info("Node joining network", "namespace", namespace, "name", name)

	// 1. 更新 Phase
	if err := c.updateNodeStatus(ctx, namespace, name, func(status *wireflowv1alpha1.NodeStatus) {
		status.Phase = wireflowv1alpha1.NodeUpdating
		c.setNodeCondition(status, wireflowv1alpha1.NodeConditionNetworkConfigured, metav1.ConditionFalse,
			wireflowv1alpha1.ReasonUpdating, "Joining network")
	}); err != nil {
		return err
	}

	// 2. 分配/更新 IP
	var (
		allocatedIP string
		err         error
	)
	if allocatedIP, err = c.allocateIPsForNode(ctx, namespace, name); err != nil {
		c.updateNodeStatus(ctx, namespace, name, func(status *wireflowv1alpha1.NodeStatus) {
			status.Phase = wireflowv1alpha1.NodeFailed
			c.setNodeCondition(status, wireflowv1alpha1.NodeConditionIPAllocated, metav1.ConditionFalse,
				wireflowv1alpha1.ReasonAllocationFailed, err.Error())
		})
		return err
	}

	// 3. 同步网络标签 Spec + labels
	if err = c.updateNodeSpec(ctx, namespace, name, func(node *wireflowv1alpha1.Node) {
		node.Spec.Address = allocatedIP
		labels := node.Labels

		for _, networkName := range node.Spec.Network {
			network, err := c.networkLister.Networks(namespace).Get(networkName)
			if err != nil {
				klog.Errorf("Failed to get network %s: %v", networkName, err)
				continue
			}
			if _, ok := labels[fmt.Sprintf("wireflow.io/network-%s", networkName)]; ok {
				continue
			}

			labels[fmt.Sprintf("wireflow.io/network-%s", networkName)] = network.Spec.Name
		}
		if _, ok := labels["wireflow.io/has-network"]; !ok {
			labels["wireflow.io/has-network"] = "true"
		}
		node.Labels = labels
	}); err != nil {
		return err
	}

	// 4. 完成更新status
	return c.updateNodeStatus(ctx, namespace, name, func(status *wireflowv1alpha1.NodeStatus) {
		node, err := c.nodesLister.Nodes(namespace).Get(name)
		if err != nil {
			logger.Error(err, "failed to get node")
			return
		}
		status.Phase = wireflowv1alpha1.NodeReady
		status.ActiveNetworks = node.Spec.Network
		status.AllocatedAddress = node.Spec.Address
		status.ObservedGeneration = node.Generation
		now := metav1.Now()
		status.LastSyncTime = &now

		c.setNodeCondition(status, wireflowv1alpha1.NodeConditionNetworkConfigured, metav1.ConditionTrue,
			wireflowv1alpha1.ReasonReady, "Network joined successfully")
	})
}

// handleNodeLeaveNetwork
func (c *Controller) handleNodeLeaveNetwork(ctx context.Context, namespace, name string) error {
	logger := klog.FromContext(ctx)
	logger.Info("Node leaving network", "namespace", namespace, "name", name)

	node, err := c.nodesLister.Nodes(namespace).Get(name)
	if err != nil {
		return err
	}

	// 1. 更新 Phase
	if err := c.updateNodeStatus(ctx, namespace, name, func(status *wireflowv1alpha1.NodeStatus) {
		status.Phase = wireflowv1alpha1.NodeTerminating
		c.setNodeCondition(status, wireflowv1alpha1.NodeConditionNetworkConfigured, metav1.ConditionFalse,
			wireflowv1alpha1.ReasonLeaving, "Leaving network")
	}); err != nil {
		return err
	}

	// 2. 释放 IP
	if err := c.releaseIPsForNode(ctx, namespace, name); err != nil {
		return err
	}

	// 3. 同步标签
	if err := c.updateNodeSpec(ctx, namespace, name, func(node *wireflowv1alpha1.Node) {

	}); err != nil {
		return err
	}

	// 4. 完成
	if len(node.Spec.Network) == 0 {
		// 完全离开所有网络
		return c.updateNodeStatus(ctx, namespace, name, func(status *wireflowv1alpha1.NodeStatus) {
			status.Phase = wireflowv1alpha1.NodePending
			status.ActiveNetworks = nil
			status.AllocatedAddress = ""
			status.ObservedGeneration = node.Generation

			c.setNodeCondition(status, wireflowv1alpha1.NodeConditionProvisioned, metav1.ConditionFalse,
				wireflowv1alpha1.ReasonReady, "Node left all networks")
		})
	}

	// 部分离开,仍有网络
	return c.handleNodeJoinNetwork(ctx, namespace, name)
}

// handleNodeUpdateNetwork
func (c *Controller) handleNodeUpdateNetwork(ctx context.Context, namespace, name string) error {
	// 更新网络 = 先离开再加入
	if err := c.handleNodeLeaveNetwork(ctx, namespace, name); err != nil {
		return err
	}
	return c.handleNodeJoinNetwork(ctx, namespace, name)
}

// handleNodeRecover 处理错误恢复
func (c *Controller) handleNodeRecover(ctx context.Context, namespace, name string) error {
	logger := klog.FromContext(ctx)
	logger.Info("Recovering node from failed state", "namespace", namespace, "name", name)

	// 重新初始化
	return c.handleNodeInitialize(ctx, namespace, name)
}

func (c *Controller) handleNodeCleanup(ctx context.Context, namespace, name string) error {
	return nil
}

// setNodeCondition 设置或更新 Condition
func (c *Controller) setNodeCondition(status *wireflowv1alpha1.NodeStatus,
	conditionType string, conditionStatus metav1.ConditionStatus,
	reason, message string) {

	now := metav1.Now()

	for i := range status.Conditions {
		if status.Conditions[i].Type == conditionType {
			// 更新现有 Condition
			if status.Conditions[i].Status != conditionStatus {
				status.Conditions[i].LastTransitionTime = now
			}
			status.Conditions[i].Status = conditionStatus
			status.Conditions[i].Reason = reason
			status.Conditions[i].Message = message
			status.Conditions[i].ObservedGeneration = status.ObservedGeneration
			return
		}
	}

	// 添加新 Condition
	status.Conditions = append(status.Conditions, metav1.Condition{
		Type:               conditionType,
		Status:             conditionStatus,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: status.ObservedGeneration,
	})
}
