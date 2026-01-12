// Copyright 2025 The Wireflow Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"
	"wireflow/internal/ipam"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"wireflow/api/v1alpha1"
)

// NetworkReconciler reconciles a WireflowNetwork object
type NetworkReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	IPAM *ipam.IPAM
}

// +kubebuilder:rbac:groups=wireflowcontroller.wireflow.run,resources=wireflownetworks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=wireflowcontroller.wireflow.run,resources=wireflownetworks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=wireflowcontroller.wireflow.run,resources=wireflownetworks/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the WireflowNetwork object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
func (r *NetworkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var (
		network v1alpha1.WireflowNetwork
		err     error
		updated bool
		cidr    string
	)

	log := logf.FromContext(ctx)
	log.Info("Reconciling WireflowNetwork", "namespace", req.NamespacedName, "name", req.Name)

	if err = r.Get(ctx, req.NamespacedName, &network); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get WireflowNetwork")
		return ctrl.Result{}, err
	}

	// 更新Phase为Creating
	if network.Status.Phase == "" {
		if _, err = r.updateStatus(ctx, &network, func(network *v1alpha1.WireflowNetwork) error {
			network.Status.Phase = v1alpha1.NetworkPhaseCreating
			return nil
		}); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	if network.Status.ActiveCIDR == "" {
		//get subnet
		var pool v1alpha1.WireflowGlobalIPPool
		poolKey := client.ObjectKey{Name: "wireflow-ip-pool"}
		if err = r.Get(ctx, poolKey, &pool); err != nil {
			return ctrl.Result{}, err
		}

		cidr, err = r.IPAM.AllocateSubnet(ctx, network.Name, &pool)
		if err != nil {
			log.Error(err, "Failed to allocate subnet from ippool")
			return ctrl.Result{RequeueAfter: time.Second * 10}, err
		}

		//更新status
		updated, err = r.updateStatus(ctx, &network, func(network *v1alpha1.WireflowNetwork) error {
			network.Status.ActiveCIDR = cidr
			network.Status.Phase = v1alpha1.NetworkPhaseReady
			return nil
		})

		if updated {
			return ctrl.Result{}, nil
		}
	}

	//get all wireflowpeer, one peer one endpoint
	var peers v1alpha1.WireflowPeerList
	peers, err = r.findNodesByLabels(ctx, &network)
	if err != nil {
		return ctrl.Result{}, err
	}
	_, err = r.updateStatus(ctx, &network, func(network *v1alpha1.WireflowNetwork) error {
		network.Status.AllocatedCount = len(peers.Items)
		return nil
	})

	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

//func (r *NetworkReconciler) generateNodesMap(ctx context.Context, nodeList *v1alpha1.WireflowPeerList) map[string]struct{} {
//	currentNodes := make(map[string]struct{})
//	for _, node := range nodeList.Items {
//		currentNodes[node.Name] = struct{}{}
//	}
//	return currentNodes
//}

// reconcileSpec 检查并修正 WireflowNetwork.Spec 字段。
// 如果 Spec 被修改并成功写入，返回 (true, nil)，调用者应立即退出 Reconcile。
// 否则返回 (false, nil) 或 (false, error)。
func (r *NetworkReconciler) updateSpec(ctx context.Context, network *v1alpha1.WireflowNetwork, updateFunc func(node *v1alpha1.WireflowNetwork)) (bool, error) {
	log := logf.FromContext(ctx)
	networkCopy := network.DeepCopy()

	// 添加network spec
	updateFunc(networkCopy)

	// 使用 Patch 发送差异。client.MergeFrom 会自动检查 networkCopy 和 node 之间的差异。
	if err := r.Patch(ctx, networkCopy, client.MergeFrom(network)); err != nil {
		if errors.IsConflict(err) {
			// 遇到并发冲突 (409)，不返回错误，让 Manager 自动通过新的事件重试。
			log.Info("Conflict detected during WireflowNetwork Spec patch, will retry on next reconcile.")
			return false, nil
		}
		// 其他写入错误（例如权限不足）
		log.Error(err, "Failed to patch WireflowNetwork Spec")
		return false, err
	}

	// 如果原始资源和当前资源在 Metadata/Spec/Annotation 上没有差异，说明 Patch 只是空操作。
	// 注意：判断 Patch 是否执行写入，最简单的方法是比较原始和当前的 Labels/Annotations/Spec 字段。
	if !reflect.DeepEqual(networkCopy.Spec, network.Spec) ||
		!reflect.DeepEqual(networkCopy.Labels, network.Labels) ||
		!reflect.DeepEqual(networkCopy.Annotations, network.Annotations) {

		log.Info("WireflowNetwork Metadata/Spec successfully patched. Returning to trigger next reconcile.")
		// Spec 或 Metadata 被修改并成功写入 API Server
		return true, nil
	}

	// Spec 未发生修改
	return false, nil
}

func (r *NetworkReconciler) updateStatus(ctx context.Context, network *v1alpha1.WireflowNetwork, updateFunc func(network *v1alpha1.WireflowNetwork) error) (bool, error) {
	log := logf.FromContext(ctx)
	networkCopy := network.DeepCopy()
	if err := updateFunc(networkCopy); err != nil {
		return false, err
	}

	// 使用 Patch 发送差异。client.MergeFrom 会自动检查 nodeCopy 和 node 之间的差异。
	if err := r.Status().Patch(ctx, networkCopy, client.MergeFrom(network)); err != nil {
		if errors.IsConflict(err) {
			// 遇到并发冲突 (409)，不返回错误，让 Manager 自动通过新的事件重试。
			log.Info("Conflict detected during WireflowNetwork Spec patch, will retry on next reconcile.")
			return false, nil
		}
		// 其他写入错误（例如权限不足）
		log.Error(err, "Failed to patch WireflowNetwork Spec")
		return false, err
	}

	if !reflect.DeepEqual(networkCopy.Status, network.Status) {

		log.Info("WireflowNetwork Metadata/Spec successfully patched. Returning to trigger next reconcile.")
		// Spec 或 Metadata 被修改并成功写入 API Server
		return true, nil
	}

	// Spec 未发生修改
	return false, nil
}

// 查询所有的node， 然后更新Network的Spec
func (r *NetworkReconciler) findNodesByLabels(ctx context.Context, network *v1alpha1.WireflowNetwork) (v1alpha1.WireflowPeerList, error) {
	labels := fmt.Sprintf("wireflow.run/network-%s", network.Name)
	var nodes v1alpha1.WireflowPeerList
	if err := r.List(ctx, &nodes, client.InNamespace(network.Namespace), client.MatchingLabels(map[string]string{labels: "true"})); err != nil {
		return nodes, err
	}
	return nodes, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NetworkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.WireflowNetwork{}).
		//Watches(&v1alpha1.WireflowPeer{},
		//	handler.EnqueueRequestsFromMapFunc(r.mapNodeForNetworks),
		//	builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Named("network").
		Complete(r)
}

func (r *NetworkReconciler) mapNodeForNetworks(ctx context.Context, obj client.Object) []reconcile.Request {
	node := obj.(*v1alpha1.WireflowPeer)

	var networkToUpdate []string
	//// 1. 获取node的spec包含network
	if node.Spec.Network != nil {
		networkToUpdate = append(networkToUpdate, *node.Spec.Network)
	}
	//通过node的label获取
	labels := node.GetLabels()
	for key, value := range labels {
		if strings.HasPrefix(key, "wireflow.run/network-") && value == "true" {
			networkName, b := strings.CutPrefix(key, "wireflow.run/network-")
			if !b {
				continue
			}
			networkToUpdate = append(networkToUpdate, networkName)
		}
	}

	var requests []reconcile.Request
	for _, networkName := range networkToUpdate {
		// 2. 为每个 WireflowNetwork 返回一个 Reconcile Request
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: node.Namespace,
				Name:      networkName, // WireflowNetwork 资源是非命名空间的
			},
		})
	}
	return requests
}

//
//// allocateIPsForNode 为节点在其所属的网络中分配 IP
//func (r *NetworkReconciler) allocateIPsForNode(ctx context.Context, node *v1alpha1.WireflowPeer) (string, error) {
//	log := logf.FromContext(ctx)
//	var err error
//	primaryNetwork := node.Spec.Network
//
//	var network v1alpha1.WireflowNetwork
//	if primaryNetwork != nil {
//		// 获取 WireflowNetwork 资源
//		if err = r.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s/%s", node.Namespace, *primaryNetwork)}, &network); err != nil {
//			return "", err
//		}
//	}
//
//	// 如果节点已经有 IP 地址,跳过
//	currentAddress := node.Status.AllocatedAddress
//	if currentAddress != nil {
//		//校验ip是否是network合法ip
//		if err = r.Allocator.ValidateIP(network.Spec.CIDR, *currentAddress); err == nil {
//			log.Info("WireflowPeer already has IP address", "address", currentAddress)
//			return *currentAddress, nil
//		}
//	}
//
//	// 检查节点是否已经在该网络中有 IP 分配
//	existingIP := r.Allocator.GetNodeIP(&network, node.Name)
//	if existingIP != "" {
//		//校验ip是否是network合法ip
//		klog.Infof("WireflowPeer %s already has IP %s in network %s", node.Name, existingIP, network.Name)
//		return existingIP, nil
//	}
//
//	// 分配新的 IP
//	return r.allocate(ctx, &network, node)
//}
//
//func (r *NetworkReconciler) allocate(ctx context.Context, network *v1alpha1.WireflowNetwork, node *v1alpha1.WireflowPeer) (string, error) {
//	log := logf.FromContext(ctx)
//	var (
//		err         error
//		allocatedIP string
//	)
//	allocatedIP, err = r.Allocator.AllocateIP(network, node.Name)
//	if err != nil {
//		return "", fmt.Errorf("failed to allocate IP: %v", err)
//	}
//
//	log.Info("Allocated IP", "ip", allocatedIP, "nodeName", node.Name)
//
//	return allocatedIP, nil
//}
//
//// updateNetworkIPAllocation 更新网络的 IP 分配记录
//func (r *NetworkReconciler) updateNetworkIPAllocation(ctx context.Context, network *v1alpha1.WireflowNetwork, ip, nodeName string) error {
//
//	allocations := make(map[string]v1alpha1.IPAllocation)
//	for _, allocation := range network.Status.AllocatedIPs {
//		allocations[allocation.Peer] = allocation
//	}
//
//	if _, ok := allocations[nodeName]; ok {
//		return nil
//	}
//	// 添加 IP 分配记录
//	allocation := v1alpha1.IPAllocation{
//		IP:          ip,
//		Peer:        nodeName,
//		AllocatedAt: metav1.Now(),
//	}
//
//	network.Status.AllocatedIPs = append(network.Status.AllocatedIPs, allocation)
//
//	// 更新可用 IP 数量
//	availableIPs, err := r.Allocator.CountAvailableIPs(network)
//	if err != nil {
//		klog.Errorf("Failed to count available IPs: %v", err)
//	} else {
//		network.Status.AvailableIPs = availableIPs
//	}
//
//	return nil
//}
