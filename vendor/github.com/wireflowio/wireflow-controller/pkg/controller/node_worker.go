package controller

import (
	"context"
	"fmt"

	wireflowv1alpha1 "github.com/wireflowio/wireflow-controller/pkg/apis/wireflowcontroller/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
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

	switch item.EventType {
	case AddEvent:
		logger.Info("Add event", "namespace", namespace, "name", name, "resource type", "Node")
	case UpdateEvent:
		newNode := item.NewObject.(*wireflowv1alpha1.Node)
		if err := c.handleNodeAddEvent(ctx, newNode); err != nil {
			return err
		}
		logger.Info(
			"Update event",
			"Add node network label & spec networks",
			"namespace", namespace,
			"name", name,
		)
	case DeleteEvent:
		logger.Info("Delete event", "namespace", namespace, "name", name, "resource type", "Node")
	}

	return nil
}

// handleNodeAddEvent 处理节点添加事件
func (c *Controller) handleNodeAddEvent(ctx context.Context, node *wireflowv1alpha1.Node) error {
	klog.Infof("Handling node add: %s/%s", node.Namespace, node.Name)

	// 为节点分配 IP 地址
	if err := c.allocateIPsForNode(ctx, node); err != nil {
		return fmt.Errorf("failed to allocate IPs for node: %v", err)
	}

	// 添加 network labels
	if err := c.syncNodeNetworkLabels(ctx, node); err != nil {
		return fmt.Errorf("failed to sync node labels: %v", err)
	}

	return nil
}

// allocateIPsForNode 为节点在其所属的网络中分配 IP
func (c *Controller) allocateIPsForNode(ctx context.Context, node *wireflowv1alpha1.Node) error {

	primaryNetwork := node.Spec.Network[0]

	// 获取 Network 资源
	network, err := c.networkLister.Networks(node.Namespace).Get(primaryNetwork)
	if err != nil {
		return fmt.Errorf("failed to get network %s: %v", primaryNetwork, err)
	}

	// 如果节点已经有 IP 地址,跳过
	currentAddress := node.Spec.Address
	if currentAddress != "" {
		//校验ip是否是network合法ip
		if err = c.ipAllocator.ValidateIP(network.Spec.CIDR, currentAddress); err != nil {
			// 分配新的 IP
			return c.allocate(ctx, network, node)
		} else {
			klog.Infof("Node %s already has IP %s in network %s", node.Name, currentAddress, network.Name)
			return nil
		}
		klog.Infof("Node %s/%s already has IP: %s", node.Namespace, node.Name, node.Spec.Address)
		return nil
	}

	// 节点可能属于多个网络,这里只为第一个网络分配 IP
	// 您可以根据需求修改为多网络 IP 分配
	if len(node.Spec.Network) == 0 {
		return fmt.Errorf("node %s/%s has no network", node.Namespace, node.Name)
	}

	// 检查节点是否已经在该网络中有 IP 分配
	existingIP := c.ipAllocator.GetNodeIP(network, node.Name)
	if existingIP != "" {
		//校验ip是否是network合法ip
		if err = c.ipAllocator.ValidateIP(network.Spec.CIDR, existingIP); err != nil {
			// 分配新的 IP
			return c.allocate(ctx, network, node)
		} else {
			klog.Infof("Node %s already has IP %s in network %s", node.Name, existingIP, network.Name)
			return nil
		}
	}

	// 分配新的 IP
	return c.allocate(ctx, network, node)
}

func (c *Controller) allocate(ctx context.Context, network *wireflowv1alpha1.Network, node *wireflowv1alpha1.Node) error {
	var (
		err         error
		allocatedIP string
	)
	allocatedIP, err = c.ipAllocator.AllocateIP(network, node.Name)
	if err != nil {
		return fmt.Errorf("failed to allocate IP: %v", err)
	}

	klog.Infof("Allocated IP %s to node %s in network %s", allocatedIP, node.Name, network.Name)

	// 更新 Network 资源,记录 IP 分配
	if err := c.updateNetworkIPAllocation(ctx, network, allocatedIP, node.Name); err != nil {
		return fmt.Errorf("failed to update network IP allocation: %v", err)
	}

	// 更新 Node 资源的 Address 字段
	if err := c.updateNodeAddress(ctx, node, allocatedIP); err != nil {
		return fmt.Errorf("failed to update node address: %v", err)
	}

	return nil
}

// updateNetworkIPAllocation 更新网络的 IP 分配记录
func (c *Controller) updateNetworkIPAllocation(ctx context.Context, network *wireflowv1alpha1.Network, ip, nodeName string) error {
	networkCopy := network.DeepCopy()

	// 添加 IP 分配记录
	allocation := wireflowv1alpha1.IPAllocation{
		IP:          ip,
		Node:        nodeName,
		AllocatedAt: metav1.Now(),
	}

	networkCopy.Spec.AllocatedIPs = append(networkCopy.Spec.AllocatedIPs, allocation)

	// 更新可用 IP 数量
	availableIPs, err := c.ipAllocator.CountAvailableIPs(networkCopy)
	if err != nil {
		klog.Errorf("Failed to count available IPs: %v", err)
	} else {
		networkCopy.Spec.AvailableIPs = availableIPs
	}

	// 更新 Network 资源
	_, err = c.wireflowclientset.WireflowcontrollerV1alpha1().Networks(network.Namespace).Update(
		ctx, networkCopy, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update network: %v", err)
	}

	klog.Infof("Updated network %s with IP allocation: %s -> %s", network.Name, ip, nodeName)
	return nil
}

// updateNodeAddress 更新节点的 IP 地址
func (c *Controller) updateNodeAddress(ctx context.Context, node *wireflowv1alpha1.Node, address string) error {
	nodeCopy := node.DeepCopy()
	nodeCopy.Spec.Address = address

	_, err := c.wireflowclientset.WireflowcontrollerV1alpha1().Nodes(node.Namespace).Update(
		ctx, nodeCopy, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update node: %v", err)
	}

	klog.Infof("Updated node %s address to %s", node.Name, address)
	return nil
}

// handleNodeDeleteEvent 处理节点删除事件
func (c *Controller) handleNodeDeleteEvent(ctx context.Context, node *wireflowv1alpha1.Node) error {
	klog.Infof("Handling node delete: %s/%s", node.Namespace, node.Name)

	// 释放节点占用的 IP 地址
	if err := c.releaseIPsForNode(ctx, node); err != nil {
		return fmt.Errorf("failed to release IPs for node: %v", err)
	}

	return nil
}

// releaseIPsForNode 释放节点占用的 IP 地址
func (c *Controller) releaseIPsForNode(ctx context.Context, node *wireflowv1alpha1.Node) error {
	for _, networkName := range node.Spec.Network {
		network, err := c.networkLister.Networks(node.Namespace).Get(networkName)
		if err != nil {
			klog.Errorf("Failed to get network %s: %v", networkName, err)
			continue
		}

		// 查找并释放该节点的 IP
		var nodeIP string
		for _, allocation := range network.Spec.AllocatedIPs {
			if allocation.Node == node.Name {
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
		for _, allocation := range networkCopy.Spec.AllocatedIPs {
			if allocation.Node != node.Name {
				newAllocations = append(newAllocations, allocation)
			}
		}
		networkCopy.Spec.AllocatedIPs = newAllocations

		// 更新可用 IP 数量
		availableIPs, err := c.ipAllocator.CountAvailableIPs(networkCopy)
		if err != nil {
			klog.Errorf("Failed to count available IPs: %v", err)
		} else {
			networkCopy.Spec.AvailableIPs = availableIPs
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

// syncNodeNetworkLabels 同步节点的网络标签
func (c *Controller) syncNodeNetworkLabels(ctx context.Context, node *wireflowv1alpha1.Node) error {
	nodeCopy := node.DeepCopy()

	if nodeCopy.Labels == nil {
		nodeCopy.Labels = make(map[string]string)
	}

	// 添加网络标签
	for _, network := range node.Spec.Network {
		labelKey := fmt.Sprintf("wireflow.io/network-%s", network)
		nodeCopy.Labels[labelKey] = "true"
	}

	if len(node.Spec.Network) > 0 {
		nodeCopy.Labels["wireflow.io/has-network"] = "true"
	}

	_, err := c.wireflowclientset.WireflowcontrollerV1alpha1().Nodes(node.Namespace).Update(
		ctx, nodeCopy, metav1.UpdateOptions{})
	return err
}
