package controller

import (
	"context"
	"fmt"

	wireflowv1alpha1 "github.com/wireflowio/wireflow-controller/pkg/apis/wireflowcontroller/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
)

// getDesiredNodesForNetwork 获取应该加入网络的节点列表
func (c *Controller) getDesiredNodesForNetwork(ctx context.Context, network *wireflowv1alpha1.Network) ([]string, error) {
	nodeSet := make(map[string]struct{})

	// 1. 处理显式指定的节点
	for _, nodeName := range network.Spec.Nodes {
		nodeSet[nodeName] = struct{}{}
	}

	// 2. 处理通过 NodeSelector 选择的节点
	if network.Spec.NodeSelector != nil {
		selector, err := metav1.LabelSelectorAsSelector(network.Spec.NodeSelector)
		if err != nil {
			return nil, fmt.Errorf("invalid node selector: %v", err)
		}

		// 列出匹配的节点
		nodes, err := c.nodesLister.Nodes(network.Namespace).List(selector)
		if err != nil {
			return nil, fmt.Errorf("failed to list nodes: %v", err)
		}

		for _, node := range nodes {
			nodeSet[node.Name] = struct{}{}
		}
	}

	// 转换为列表
	result := make([]string, 0, len(nodeSet))
	for nodeName := range nodeSet {
		result = append(result, nodeName)
	}

	return result, nil
}

// getActualNodesInNetwork 获取当前已加入网络的节点
func (c *Controller) getActualNodesInNetwork(ctx context.Context, network *wireflowv1alpha1.Network) ([]string, error) {
	// 使用索引查找所有属于此网络的节点
	nodes, err := c.GetNodeByNetworkName(network.Name)
	if err != nil {
		return nil, err
	}

	result := make([]string, 0, len(nodes))
	for _, node := range nodes {
		// 检查节点的 Spec.Network 是否包含此网络
		for _, net := range node.Spec.Network {
			if net == network.Name {
				result = append(result, node.Name)
				break
			}
		}
	}

	return result, nil
}

// calculateNodeDiff 计算需要添加和移除的节点
func (c *Controller) calculateNodeDiff(desired, actual []string) (toAdd, toRemove []string) {
	desiredSet := make(map[string]struct{})
	for _, name := range desired {
		desiredSet[name] = struct{}{}
	}

	actualSet := make(map[string]struct{})
	for _, name := range actual {
		actualSet[name] = struct{}{}
	}

	// 需要添加的节点 = desired - actual
	for name := range desiredSet {
		if _, exists := actualSet[name]; !exists {
			toAdd = append(toAdd, name)
		}
	}

	// 需要移除的节点 = actual - desired
	for name := range actualSet {
		if _, exists := desiredSet[name]; !exists {
			toRemove = append(toRemove, name)
		}
	}

	return
}

// addNodeToNetwork 将节点加入网络
func (c *Controller) addNodeToNetwork(ctx context.Context, network *wireflowv1alpha1.Network, nodeName string) error {
	logger := klog.FromContext(ctx)

	// 获取节点
	node, err := c.nodesLister.Nodes(network.Namespace).Get(nodeName)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Node not found, skipping", "node", nodeName)
			return nil
		}
		return err
	}

	// 检查节点是否已经在网络中
	for _, net := range node.Spec.Network {
		if net == network.Name {
			logger.V(4).Info("Node already in network", "node", nodeName, "network", network.Name)
			return nil
		}
	}

	// 更新节点的 Spec.Network
	nodeCopy := node.DeepCopy()
	nodeCopy.Spec.Network = append(nodeCopy.Spec.Network, network.Name)

	_, err = c.wireflowclientset.WireflowcontrollerV1alpha1().Nodes(node.Namespace).Update(
		ctx, nodeCopy, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update node: %v", err)
	}

	logger.Info("Added node to network",
		"node", nodeName,
		"network", network.Name,
		"networks", nodeCopy.Spec.Network)

	objectRef, err := cache.ObjectToName(nodeCopy)
	// 触发 Node 控制器协调
	c.nodeQueue.Add(WorkerItem{
		Key:       objectRef,
		EventType: UpdateEvent,
	})

	return nil
}

// removeNodeFromNetwork 将节点从网络移除
func (c *Controller) removeNodeFromNetwork(ctx context.Context, network *wireflowv1alpha1.Network, nodeName string) error {
	logger := klog.FromContext(ctx)

	// 获取节点
	node, err := c.nodesLister.Nodes(network.Namespace).Get(nodeName)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Node not found, skipping", "node", nodeName)
			return nil
		}
		return err
	}

	// 检查节点是否在网络中
	found := false
	newNetworks := make([]string, 0, len(node.Spec.Network))
	for _, net := range node.Spec.Network {
		if net == network.Name {
			found = true
			if err = c.ipAllocator.ValidateIP(network.Spec.CIDR, node.Spec.Address); err == nil {
				node.Spec.Address = ""
			}
		} else {
			newNetworks = append(newNetworks, net)
		}
	}

	if !found {
		logger.V(4).Info("Node not in network", "node", nodeName, "network", network.Name)
		return nil
	}

	// 更新节点的 Spec.Network
	nodeCopy := node.DeepCopy()
	nodeCopy.Spec.Network = newNetworks

	_, err = c.wireflowclientset.WireflowcontrollerV1alpha1().Nodes(node.Namespace).Update(
		ctx, nodeCopy, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update node: %v", err)
	}

	logger.Info("Removed node from network",
		"node", nodeName,
		"network", network.Name,
		"remainingNetworks", newNetworks)

	objectRef, err := cache.ObjectToName(nodeCopy)
	// 触发 Node 控制器协调
	c.nodeQueue.Add(WorkerItem{
		Key:       objectRef,
		EventType: UpdateEvent,
	})

	return nil
}

// handleNetworkDeletion 处理网络删除
func (c *Controller) handleNetworkDeletion(ctx context.Context, namespace, networkName string) error {
	logger := klog.FromContext(ctx)
	logger.Info("Handling network deletion", "network", networkName)

	// 找到所有属于此网络的节点
	allNodes, err := c.nodesLister.Nodes(namespace).List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list nodes: %v", err)
	}

	// 从所有节点中移除此网络
	for _, node := range allNodes {
		err = c.updateNodeSpec(ctx, namespace, node.Name, func(node *wireflowv1alpha1.Node) {
			found := false
			newNetworks := make([]string, 0, len(node.Spec.Network))
			for _, net := range node.Spec.Network {
				if net == networkName {
					found = true
				} else {
					newNetworks = append(newNetworks, net)
				}
			}

			if !found {
				return
			}

			node.Spec.Network = newNetworks
		})

		if err != nil {
			logger.Error(err, "failed to update node spec")
		}

		logger.Info("Removed network from node", "node", node.Name, "network", networkName)
	}

	return nil
}

func (c *Controller) updateNetworkSpec(ctx context.Context, namespace, name string,
	updateFunc func(network *wireflowv1alpha1.Network)) error {
	logger := klog.FromContext(ctx)
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		network, err := c.networkLister.Networks(namespace).Get(name)
		if err != nil {
			return err
		}
		networkCopy := network.DeepCopy()
		updateFunc(networkCopy)

		_, err = c.wireflowclientset.WireflowcontrollerV1alpha1().Networks(namespace).Update(ctx, networkCopy, metav1.UpdateOptions{})
		if err != nil {
			if errors.IsConflict(err) {
				logger.V(4).Info("Node spec update conflicted, retrying")
			} else {
				logger.Error(err, "Failed to update node spec", networkCopy.Name)
			}
		}
		return err
	})
}

// updateNetworkStatus 更新网络状态
func (c *Controller) updateNetworkStatus(ctx context.Context, network *wireflowv1alpha1.Network,
	updateFunc func(*wireflowv1alpha1.NetworkStatus)) error {

	latest, err := c.networkLister.Networks(network.Namespace).Get(network.Name)
	if err != nil {
		return err
	}

	networkCopy := latest.DeepCopy()
	updateFunc(&networkCopy.Status)

	_, err = c.wireflowclientset.WireflowcontrollerV1alpha1().Networks(network.Namespace).UpdateStatus(
		ctx, networkCopy, metav1.UpdateOptions{})

	return err
}

// setNetworkCondition 设置网络条件
func (c *Controller) setNetworkCondition(status *wireflowv1alpha1.NetworkStatus,
	conditionType string, conditionStatus metav1.ConditionStatus,
	reason, message string) {

	now := metav1.Now()

	for i := range status.Conditions {
		if status.Conditions[i].Type == conditionType {
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

	status.Conditions = append(status.Conditions, metav1.Condition{
		Type:               conditionType,
		Status:             conditionStatus,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: status.ObservedGeneration,
	})
}

// pkg/controller/network_action.go (添加新方法)

// handlePolicyChange 处理策略变更
func (c *Controller) handlePolicyChange(ctx context.Context,
	network *wireflowv1alpha1.Network) error {
	return nil
}
