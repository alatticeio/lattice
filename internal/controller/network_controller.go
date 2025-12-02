/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	wireflowcontrollerv1alpha1 "github.com/wireflowio/wireflow-controller/api/v1alpha1"
)

// NetworkReconciler reconciles a Networks object
type NetworkReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Allocator *IPAllocator
}

// +kubebuilder:rbac:groups=wireflowcontroller.wireflow.io,resources=networks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=wireflowcontroller.wireflow.io,resources=networks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=wireflowcontroller.wireflow.io,resources=networks/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Networks object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *NetworkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	//更新status
	var (
		network wireflowcontrollerv1alpha1.Network
		err     error
	)

	log := logf.FromContext(ctx)
	log.Info("Reconciling Network", "namespace", req.NamespacedName, "name", req.Name)

	if err = r.Get(ctx, req.NamespacedName, &network); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Network")
		return ctrl.Result{}, err
	}

	// 不做任何处理
	cidr := network.Spec.CIDR
	//statusCidr := network.Status.ActiveCIDR
	if cidr == "" {
		//TODO implementing network disabled
		return ctrl.Result{}, nil
	}

	//获取node的变化，更新network spec
	var nodeList wireflowcontrollerv1alpha1.NodeList
	nodeList, err = r.findNodesByLabels(ctx, &network)
	if err != nil {
		return ctrl.Result{}, err
	}

	currentNodes := make(map[string]struct{})
	for _, node := range nodeList.Items {
		currentNodes[node.Name] = struct{}{}
	}

	//update nodes
	ok, err := r.updateSpec(ctx, &network, func(network *wireflowcontrollerv1alpha1.Network) {
		network.Spec.Nodes = setsToSlice(currentNodes)
	})

	if ok {
		return ctrl.Result{}, nil
	}

	//重新获取network用来更新status, 避免冲突
	if err = r.Get(ctx, req.NamespacedName, &network); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Network resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}

		log.Error(err, "Failed to get Network")
		return ctrl.Result{}, err
	}

	currentNodes = make(map[string]struct{})
	for _, node := range nodeList.Items {
		currentNodes[node.Name] = struct{}{}
	}

	// 配置好CIDR
	activeNodeAllocations := network.Status.AllocatedIPs
	activeNodes := make(map[string]struct{})
	for _, allocation := range activeNodeAllocations {
		activeNodes[allocation.Node] = struct{}{}
	}

	diff := setsDifference(currentNodes, activeNodes)
	if len(diff) == 0 {
		// no change
		return ctrl.Result{}, nil
	}

	for nodeName, _ := range diff {
		if _, ok = activeNodes[nodeName]; !ok {
			// 不存在， 则是添加node逻辑
			var node wireflowcontrollerv1alpha1.Node
			if err = r.Get(ctx, types.NamespacedName{Namespace: network.Namespace, Name: nodeName}, &node); err != nil {
				return ctrl.Result{}, err
			}
			var allocatedIP string
			if allocatedIP, err = r.allocateIPsForNode(ctx, &node); err != nil {
				return ctrl.Result{}, err
			}

			// 更新 Network 资源,记录 IP 分配
			if err = r.updateNetworkIPAllocation(ctx, &network, allocatedIP, node.Name); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to update network IP allocation: %v", err)
			}
		} else {
			//删除node逻辑
			if err = r.Allocator.ReleaseIP(&network, nodeName); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to release IP: %v", err)
			}
		}
	}

	//统一更新
	if err = r.Status().Update(ctx, &network); err != nil {
		return ctrl.Result{}, err
	}

	// CIDR changed
	//if cidr != statusCidr && statusCidr != "" {
	//	if err = r.reconcileCIDRChanged(ctx, req, network); err != nil {
	//		return ctrl.Result{}, err
	//	}
	//}

	return ctrl.Result{}, nil
}

// reconcileSpec 检查并修正 Network.Spec 字段。
// 如果 Spec 被修改并成功写入，返回 (true, nil)，调用者应立即退出 Reconcile。
// 否则返回 (false, nil) 或 (false, error)。
func (r *NetworkReconciler) updateSpec(ctx context.Context, network *wireflowcontrollerv1alpha1.Network, updateFunc func(node *wireflowcontrollerv1alpha1.Network)) (bool, error) {
	log := logf.FromContext(ctx)

	// 1. 深拷贝原始资源，用于 Patch 的对比基准。
	networkCopy := network.DeepCopy()

	// 2. --- 核心 Spec 修正逻辑 ---
	// 添加network spec
	updateFunc(networkCopy)
	//
	//if _, exists := node.Labels[requiredLabelKey]; !exists {
	//	if node.Labels == nil {
	//		node.Labels = make(map[string]string)
	//	}
	//	// 🚨 注意：这里假设你可以从某种外部信息源确定 Zone
	//	// 在生产环境中，这可能更适合在 Admission Webhook 中处理，但作为 Controller 演示，我们在此修正。
	//	node.Labels[requiredLabelKey] = "default-zone"
	//	log.Info("Spec field correction: Setting default Zone Label", "Label", requiredLabelKey)
	//}

	// --- 核心 Spec 修正逻辑结束 ---

	// 3. 比较和写入差异 (使用 Patch)

	// 使用 Patch 发送差异。client.MergeFrom 会自动检查 networkCopy 和 node 之间的差异。
	if err := r.Patch(ctx, networkCopy, client.MergeFrom(network)); err != nil {
		if errors.IsConflict(err) {
			// 遇到并发冲突 (409)，不返回错误，让 Manager 自动通过新的事件重试。
			log.Info("Conflict detected during Node Spec patch, will retry on next reconcile.")
			return false, nil
		}
		// 其他写入错误（例如权限不足）
		log.Error(err, "Failed to patch Network Spec")
		return false, err
	}

	// 4. 检查是否发生了修改
	// 如果原始资源和当前资源在 Metadata/Spec/Annotation 上没有差异，说明 Patch 只是空操作。
	// 注意：判断 Patch 是否执行写入，最简单的方法是比较原始和当前的 Labels/Annotations/Spec 字段。
	if !reflect.DeepEqual(networkCopy.Spec, network.Spec) ||
		!reflect.DeepEqual(networkCopy.Labels, network.Labels) ||
		!reflect.DeepEqual(networkCopy.Annotations, network.Annotations) {

		log.Info("Node Metadata/Spec successfully patched. Returning to trigger next reconcile.")
		// Spec 或 Metadata 被修改并成功写入 API Server
		return true, nil
	}

	// Spec 未发生修改
	return false, nil
}

// 查询所有的node， 然后更新Network的Spec
func (r *NetworkReconciler) findNodesByLabels(ctx context.Context, network *wireflowcontrollerv1alpha1.Network) (wireflowcontrollerv1alpha1.NodeList, error) {
	labels := fmt.Sprintf("wireflow.io/network-%s", network.Name)
	var nodes wireflowcontrollerv1alpha1.NodeList
	if err := r.List(ctx, &nodes, client.InNamespace(network.Namespace), client.MatchingLabels(map[string]string{labels: "true"})); err != nil {
		return nodes, err
	}
	return nodes, nil
}

func (r *NetworkReconciler) reconcileCIDRChanged(ctx context.Context, req ctrl.Request, network wireflowcontrollerv1alpha1.Network) error {
	var err error
	log := logf.FromContext(ctx)
	log.Info("CIDR changed", "oldCIDR", network.Status.ActiveCIDR, "newCIDR", network.Spec.CIDR, "reallocateIPs", true)
	network.Status.ActiveCIDR = network.Spec.CIDR

	//为所有节点重新分配ip
	var nodeList wireflowcontrollerv1alpha1.NodeList
	if err = r.List(ctx, &nodeList, client.InNamespace(req.Namespace)); err != nil {
		// TODO add label selector
		return err
	}

	//先删除原来的status中的数据
	network.Status.AllocatedIPs = []wireflowcontrollerv1alpha1.IPAllocation{}
	network.Status.AvailableIPs = 0

	for _, node := range nodeList.Items {
		var allocatedIP string
		if allocatedIP, err = r.allocateIPsForNode(ctx, &node); err != nil {
			return err
		}

		// 更新 Network 资源,记录 IP 分配
		if err = r.updateNetworkIPAllocation(ctx, &network, allocatedIP, node.Name); err != nil {
			return fmt.Errorf("failed to update network IP allocation: %v", err)
		}
	}

	//统一更新
	if err = r.Status().Update(ctx, &network); err != nil {
		return err
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NetworkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&wireflowcontrollerv1alpha1.Network{}).
		Watches(&wireflowcontrollerv1alpha1.Node{},
			handler.EnqueueRequestsFromMapFunc(r.mapNodeForNetworks),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Named("network").
		Complete(r)
}

func (r *NetworkReconciler) mapNodeForNetworks(ctx context.Context, obj client.Object) []reconcile.Request {
	node := obj.(*wireflowcontrollerv1alpha1.Node)

	networkToUpdate := make([]string, 0)
	// 1. 获取node的spec包含network
	networkToUpdate = append(networkToUpdate, node.Spec.Networks...)
	//通过node的label获取
	labels := node.GetLabels()
	for key, value := range labels {
		if strings.HasPrefix(key, "wireflow.io/network-") && value == "true" {
			networkName, b := strings.CutPrefix(key, "wireflow.io/network-")
			if !b {
				continue
			}
			networkToUpdate = append(networkToUpdate, networkName)
		}
	}

	var requests []reconcile.Request
	for _, networkName := range networkToUpdate {
		// 2. 为每个 Network 返回一个 Reconcile Request
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: node.Namespace,
				Name:      networkName, // Network 资源是非命名空间的
			},
		})
	}
	return requests
}

// allocateIPsForNode 为节点在其所属的网络中分配 IP
func (r *NetworkReconciler) allocateIPsForNode(ctx context.Context, node *wireflowcontrollerv1alpha1.Node) (string, error) {
	log := logf.FromContext(ctx)
	var err error
	if len(node.Spec.Networks) == 0 {
		//clear node's address
		return "", nil
	}
	primaryNetwork := node.Spec.Networks[0]

	// 获取 Network 资源
	var network wireflowcontrollerv1alpha1.Network
	if err = r.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s/%s", node.Namespace, primaryNetwork)}, &network); err != nil {
		return "", err
	}

	// 如果节点已经有 IP 地址,跳过
	currentAddress := node.Status.AllocatedAddress
	if currentAddress != "" {
		//校验ip是否是network合法ip
		if err = r.Allocator.ValidateIP(network.Spec.CIDR, currentAddress); err == nil {
			log.Info("Node already has IP address", "address", currentAddress)
			return currentAddress, nil
		}
	}

	// 检查节点是否已经在该网络中有 IP 分配
	existingIP := r.Allocator.GetNodeIP(&network, node.Name)
	if existingIP != "" {
		//校验ip是否是network合法ip
		klog.Infof("Node %s already has IP %s in network %s", node.Name, existingIP, network.Name)
		return existingIP, nil
	}

	// 分配新的 IP
	return r.allocate(ctx, &network, node)
}

func (r *NetworkReconciler) allocate(ctx context.Context, network *wireflowcontrollerv1alpha1.Network, node *wireflowcontrollerv1alpha1.Node) (string, error) {
	log := logf.FromContext(ctx)
	var (
		err         error
		allocatedIP string
	)
	allocatedIP, err = r.Allocator.AllocateIP(network, node.Name)
	if err != nil {
		return "", fmt.Errorf("failed to allocate IP: %v", err)
	}

	log.Info("Allocated IP", "ip", allocatedIP, "nodeName", node.Name)

	return allocatedIP, nil
}

// updateNetworkIPAllocation 更新网络的 IP 分配记录
func (r *NetworkReconciler) updateNetworkIPAllocation(ctx context.Context, network *wireflowcontrollerv1alpha1.Network, ip, nodeName string) error {

	allocations := make(map[string]wireflowcontrollerv1alpha1.IPAllocation)
	for _, allocation := range network.Status.AllocatedIPs {
		allocations[allocation.Node] = allocation
	}

	if _, ok := allocations[nodeName]; ok {
		return nil
	}
	// 添加 IP 分配记录
	allocation := wireflowcontrollerv1alpha1.IPAllocation{
		IP:          ip,
		Node:        nodeName,
		AllocatedAt: metav1.Now(),
	}

	network.Status.AllocatedIPs = append(network.Status.AllocatedIPs, allocation)

	// 更新可用 IP 数量
	availableIPs, err := r.Allocator.CountAvailableIPs(network)
	if err != nil {
		klog.Errorf("Failed to count available IPs: %v", err)
	} else {
		network.Status.AvailableIPs = availableIPs
	}

	return nil
}
