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
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"wireflow/internal/core/infra"
	"wireflow/internal/ipam"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"wireflow/api/v1alpha1"
)

// PeerReconciler reconciles a WireflowPeer object
type PeerReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	IPAM          *ipam.IPAM
	Detector      *ChangeDetector
	SnapshotCache map[types.NamespacedName]*PeerStateSnapshot
}

// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=wireflowcontroller.wireflow.run,resources=wireflowpeers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=wireflowcontroller.wireflow.run,resources=wireflowpeers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=wireflowcontroller.wireflow.run,resources=wireflowpeers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the WireflowPeer object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
func (r *PeerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Reconciling WireflowPeer", "namespace", req.NamespacedName, "node", req.Name)

	var (
		err  error
		node v1alpha1.WireflowPeer
	)

	if err = r.Get(ctx, req.NamespacedName, &node); err != nil {
		if errors.IsNotFound(err) {
			log.Info("WireflowPeer resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}

		log.Error(err, "Failed to get WireflowPeer")
		return ctrl.Result{}, err
	}

	action, err := r.determineAction(ctx, &node)
	if err != nil {
		return ctrl.Result{}, err
	}
	switch action {
	case NodeJoinNetwork:
		log.Info("Handing join network", "namespace", req.Namespace, "name", req.Name)
		return r.reconcileJoinNetwork(ctx, &node, req)
	case NodeLeaveNetwork:
		log.Info("Handing leave network", "namespace", req.Namespace, "name", req.Name)
		return r.reconcileLeaveNetwork(ctx, &node, req)
	default:
		log.Info("No action to handle", "namespace", req.Namespace, "name", req.Name)
		return r.reconcileConfigmap(ctx, &node, req)
	}

}

type Action string

const (
	NodeJoinNetwork  Action = "joinNetwork"
	NodeLeaveNetwork Action = "leaveNetwork"
	ActionNone       Action = "none"
)

// reconcileJoinNetwork handle join network
func (r *PeerReconciler) reconcileJoinNetwork(ctx context.Context, peer *v1alpha1.WireflowPeer, request ctrl.Request) (ctrl.Result, error) {
	var (
		err error
		ok  bool
	)
	log := logf.FromContext(ctx)
	log.Info("Join network", "namespace", request.Namespace, "name", request.Name)

	//1. 更新Phase为Pending
	if peer.Status.Phase != v1alpha1.NodePhasePending {
		ok, err = r.updateStatus(ctx, peer, func(node *v1alpha1.WireflowPeer) {
			node.Status.Phase = v1alpha1.NodePhasePending
		})

		if err != nil {
			return ctrl.Result{}, err
		}

		if ok {
			return ctrl.Result{}, nil
		}
	}

	// 2.修改Spec
	ok, err = r.updateSpec(ctx, peer, func(node *v1alpha1.WireflowPeer) error {
		network, err := r.getNetwork(ctx, node)
		if err != nil {
			return err
		}
		labels := node.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
		}
		labels[fmt.Sprintf("wireflow.run/network-%s", network.Name)] = "true"
		node.SetLabels(labels)

		if node.Spec.PrivateKey == "" {
			key, err := wgtypes.GeneratePrivateKey()
			if err != nil {
				return err
			}

			node.Spec.PrivateKey = key.String()
			node.Spec.PublicKey = key.PublicKey().String()
		}

		// get ip

		return nil
	})

	if err != nil {
		return ctrl.Result{}, err
	}

	if ok {
		//直接返回，等下次reconcile
		return ctrl.Result{}, nil
	}

	//重新获取node用来更新status, 避免冲突
	if err = r.Get(ctx, request.NamespacedName, peer); err != nil {
		if errors.IsNotFound(err) {
			log.Info("WireflowPeer resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}

		log.Error(err, "Failed to get WireflowPeer")
		return ctrl.Result{}, err
	}

	//查询primary network 分配的ip
	if peer.Spec.Network == nil {
		return ctrl.Result{}, fmt.Errorf("WireflowNetwork field is missing")

	}

	var network v1alpha1.WireflowNetwork
	if err = r.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s/%s", peer.Namespace, *peer.Spec.Network)}, &network); err != nil {
		return ctrl.Result{}, err
	}

	// get ip
	address, err := r.IPAM.AllocateIP(ctx, &network, peer)
	if err != nil {
		return ctrl.Result{}, err
	}

	if ok, err = r.updateStatus(ctx, peer, func(node *v1alpha1.WireflowPeer) {
		node.Status.Phase = v1alpha1.NodePhaseReady
		node.Status.AllocatedAddress = &address
		node.Status.ActiveNetwork = node.Spec.Network
	}); err != nil {
		return ctrl.Result{}, err
	}

	if ok {
		return ctrl.Result{}, nil
	}

	return r.reconcileConfigmap(ctx, peer, request)
}

// reconcileConfigmap create or update the configmap
func (r *PeerReconciler) reconcileConfigmap(ctx context.Context, peer *v1alpha1.WireflowPeer, request ctrl.Request) (ctrl.Result, error) {
	var (
		err              error
		changes          *infra.ChangeDetails
		message          *infra.Message
		desiredConfigMap *corev1.ConfigMap
	)
	logger := logf.FromContext(ctx)

	oldNodeCtx := r.SnapshotCache[request.NamespacedName]
	snapshot := r.getNodeContext(ctx, peer, request)
	// 1. 定义期望状态 (Desired State)
	configMapName := fmt.Sprintf("%s-config", peer.Name)
	// 2. 获取当前状态 (Current State)
	foundConfigMap := &corev1.ConfigMap{}
	err = r.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: peer.Namespace}, foundConfigMap)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Creating configmap", "name", configMapName)
			// first time create cm
			message, err = r.Detector.generateConfigmap(ctx, peer, snapshot, changes, "init")
			desiredConfigMap = r.newConfigmap(peer.Namespace, configMapName, message.String())
			// 关键步骤：设置 OwnerReference
			// 这确保了当主资源 (peer) 被删除时，这个 reconcileConfigmap 也会被 K8s 垃圾回收器自动删除。
			if err := controllerutil.SetControllerReference(peer, desiredConfigMap, r.Scheme); err != nil {
				logger.Error(err, "Failed to set owner reference on configmap")
				return ctrl.Result{}, err
			}
			// --- A. 不存在：执行创建操作 ---
			r.SnapshotCache[request.NamespacedName] = snapshot
			// 使用SSA模式
			manager := client.FieldOwner("wireflow-controller-manager")
			if err = r.Patch(ctx, desiredConfigMap, client.Apply, manager); err != nil {
				logger.Error(err, "Failed to create configmap")
				return ctrl.Result{}, err
			}

			logger.Info("Creating configmap success", "name", configMapName)
		}
		return ctrl.Result{}, err
	} else {
		r.SnapshotCache[request.NamespacedName] = snapshot
		if oldNodeCtx == nil {
			//maybe controller restart
			oldNodeCtx = snapshot
			// reconcile next time
			return ctrl.Result{}, nil
		}
		changes = r.Detector.DetectNodeChanges(ctx, oldNodeCtx, oldNodeCtx.Peer, snapshot.Peer, oldNodeCtx.Network, snapshot.Network, oldNodeCtx.Policies, snapshot.Policies, request)
		if changes.HasChanges() {
			var currentMessage infra.Message
			err = json.Unmarshal([]byte(foundConfigMap.Data["config.json"]), &currentMessage)
			if err != nil {
				return ctrl.Result{}, err
			}
			message, err = r.Detector.generateConfigmap(ctx, peer, snapshot, changes, r.Detector.generateConfigVersion())
			if err != nil {
				return ctrl.Result{}, err
			}
			desiredConfigMap = r.newConfigmap(peer.Namespace, configMapName, message.String())

			// --- B. 已存在：执行更新操作 (保证幂等性) ---
			if !currentMessage.Equal(message) {
				logger.Info("Updating configmap data", "name", configMapName)

				// 使用SSA模式
				// 使用SSA模式
				manager := client.FieldOwner("wireflow-controller-manager")
				if err = r.Patch(ctx, desiredConfigMap, client.Apply, manager); err != nil {
					logger.Error(err, "Failed to update configmap")
					return ctrl.Result{}, err
				}
				// 写入成功：立即返回
				return ctrl.Result{}, nil
			}

		}
		return ctrl.Result{}, nil
	}

}

func (r *PeerReconciler) newConfigmap(namespace, configMapName, message string) *corev1.ConfigMap {
	// 根据 CRD 的 Spec 构建 reconcileConfigmap 的内容
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "wireflow-controller",
			},
		},
		Data: map[string]string{
			"config.json": message,
		},
	}
}

// reconcileLeaveNetwork handle leave network
func (r *PeerReconciler) reconcileLeaveNetwork(ctx context.Context, peer *v1alpha1.WireflowPeer, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Leaving network", "namespace", req.Namespace, "name", req.Name)
	var (
		err error
		ok  bool
	)

	//1. 更新Phase为Pending
	if peer.Status.Phase != v1alpha1.NodePhasePending {
		ok, err = r.updateStatus(ctx, peer, func(node *v1alpha1.WireflowPeer) {
			node.Status.Phase = v1alpha1.NodePhasePending
		})
		if err != nil {
			return ctrl.Result{}, err
		}

		if ok {
			return ctrl.Result{}, nil
		}
	}

	// 2.修改Spec
	ok, err = r.updateSpec(ctx, peer, func(node *v1alpha1.WireflowPeer) error {

		labels := node.GetLabels()
		for label, _ := range labels {
			if strings.HasPrefix(label, "wireflow.run/network-") {
				delete(labels, label)
			}
			// 删除network in spec
		}
		node.SetLabels(labels)

		// update spec networks
		node.Spec.Network = nil
		return nil
	})

	if err != nil {
		return ctrl.Result{}, err
	}

	if ok {
		//直接返回，等下次reconcile
		return ctrl.Result{}, nil
	}

	//重新获取node用来更新status, 避免冲突
	if err = r.Get(ctx, req.NamespacedName, peer); err != nil {
		if errors.IsNotFound(err) {
			log.Info("WireflowPeer resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}

		log.Error(err, "Failed to get WireflowPeer")
		return ctrl.Result{}, err
	}

	return r.reconcileConfigmap(ctx, peer, req)
}

// reconcileSpec 检查并修正 WireflowPeer.Spec 字段。
// 如果 Spec 被修改并成功写入，返回 (true, nil)，调用者应立即退出 Reconcile。
// 否则返回 (false, nil) 或 (false, error)。
func (r *PeerReconciler) updateSpec(ctx context.Context, node *v1alpha1.WireflowPeer, updateFunc func(node *v1alpha1.WireflowPeer) error) (bool, error) {
	log := logf.FromContext(ctx)

	// 深拷贝原始资源，用于 Patch 的对比基准。
	nodeCopy := node.DeepCopy()

	// 添加network spec
	updateFunc(nodeCopy)

	// 使用 Patch 发送差异。client.MergeFrom 会自动检查 nodeCopy 和 node 之间的差异。
	if err := r.Patch(ctx, nodeCopy, client.MergeFrom(node)); err != nil {
		if errors.IsConflict(err) {
			// 遇到并发冲突 (409)，不返回错误，让 Manager 自动通过新的事件重试。
			log.Info("Conflict detected during WireflowPeer Spec patch, will retry on next reconcile.")
			return false, nil
		}
		// 其他写入错误（例如权限不足）
		log.Error(err, "Failed to patch WireflowPeer Spec")
		return false, err
	}

	// 如果原始资源和当前资源在 Metadata/Spec/Annotation 上没有差异，说明 Patch 只是空操作。
	// 注意：判断 Patch 是否执行写入，最简单的方法是比较原始和当前的 Labels/Annotations/Spec 字段。
	if !reflect.DeepEqual(nodeCopy.Spec, node.Spec) ||
		!reflect.DeepEqual(nodeCopy.Labels, node.Labels) ||
		!reflect.DeepEqual(nodeCopy.Annotations, node.Annotations) {

		log.Info("WireflowPeer Metadata/Spec successfully patched. Returning to trigger next reconcile.")
		// Spec 或 Metadata 被修改并成功写入 API Server
		return true, nil
	}

	// Spec 未发生修改
	return false, nil
}

// reconcileSpec 检查并修正 WireflowPeer.Spec 字段。
// 如果 Spec 被修改并成功写入，返回 (true, nil)，调用者应立即退出 Reconcile。
// 否则返回 (false, nil) 或 (false, error)。
func (r *PeerReconciler) updateStatus(ctx context.Context, node *v1alpha1.WireflowPeer, updateFunc func(node *v1alpha1.WireflowPeer)) (bool, error) {
	log := logf.FromContext(ctx)

	// 1. 深拷贝原始资源，用于 Patch 的对比基准。
	nodeCopy := node.DeepCopy()

	// 添加network spec
	updateFunc(nodeCopy)

	// 使用 Patch 发送差异。client.MergeFrom 会自动检查 nodeCopy 和 node 之间的差异。
	if err := r.Status().Patch(ctx, nodeCopy, client.MergeFrom(node)); err != nil {
		if errors.IsConflict(err) {
			// 遇到并发冲突 (409)，不返回错误，让 Manager 自动通过新的事件重试。
			log.Info("Conflict detected during WireflowPeer Spec patch, will retry on next reconcile.")
			return false, nil
		}
		// 其他写入错误（例如权限不足）
		log.Error(err, "Failed to patch WireflowPeer Spec")
		return false, err
	}

	// 如果原始资源和当前资源在 Metadata/Spec/Annotation 上没有差异，说明 Patch 只是空操作。
	// 注意：判断 Patch 是否执行写入，最简单的方法是比较原始和当前的 Labels/Annotations/Spec 字段。
	if !reflect.DeepEqual(nodeCopy.Status, node.Status) {

		log.Info("WireflowPeer Metadata/Spec successfully patched. Returning to trigger next reconcile.")
		// Spec 或 Metadata 被修改并成功写入 API Server
		return true, nil
	}

	// Spec 未发生修改
	return false, nil
}

func (r *PeerReconciler) determineAction(ctx context.Context, node *v1alpha1.WireflowPeer) (Action, error) {
	log := logf.FromContext(ctx)
	log.Info("Determine action for node", "namespace", node.Namespace, "name", node.Name)
	specNet, activeNet := node.Spec.Network, node.Status.ActiveNetwork

	if specNet == nil {
		if activeNet == nil {
			return ActionNone, nil
		} else {
			return NodeLeaveNetwork, nil
		}
	} else {
		if activeNet == nil {
			return NodeJoinNetwork, nil
		}

		if *specNet == *activeNet {
			return ActionNone, nil
		}

		return NodeJoinNetwork, nil
	}

}

// SetupWithManager sets up the controller with the Manager.
func (r *PeerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.WireflowPeer{}).
		Watches(&v1alpha1.WireflowNetwork{},
			handler.EnqueueRequestsFromMapFunc(r.mapNetworkForNodes),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Watches(&v1alpha1.WireflowEndpoint{},
			handler.EnqueueRequestsFromMapFunc(r.mapEndpointForNodes),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Watches(&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(r.mapConfigMapForNodes),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Watches(&v1alpha1.WireflowPolicy{},
			handler.EnqueueRequestsFromMapFunc(r.mapPolicyForNodes),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).Named("node").Complete(r)
}

// mapNetworkForNodes returns a list of Reconcile Requests for Peers that should be updated based on the given WireflowNetwork.
func (r *PeerReconciler) mapNetworkForNodes(ctx context.Context, obj client.Object) []reconcile.Request {
	network := obj.(*v1alpha1.WireflowNetwork)
	var requests []reconcile.Request

	// 1. 获取所有 WireflowPeer (或只获取匹配 WireflowNetwork.Spec.PeerSelector 的 WireflowPeer)
	nodeList := &v1alpha1.WireflowPeerList{}
	if err := r.List(ctx, nodeList, client.MatchingLabels(network.Spec.PeerSelector)); err != nil {
		return nil
	}

	// 2. 将所有匹配的 WireflowPeer 加入请求队列
	for _, node := range nodeList.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: node.Namespace,
				Name:      node.Name,
			},
		})
	}
	return requests
}

func (r *PeerReconciler) mapConfigMapForNodes(ctx context.Context, obj client.Object) []reconcile.Request {
	cm := obj.(*corev1.ConfigMap)
	var requests []reconcile.Request

	// 1. 获取所有 WireflowPeer (或只获取匹配 WireflowNetwork.Spec.PeerSelector 的 WireflowPeer)
	var node v1alpha1.WireflowPeer
	name := strings.TrimSuffix(cm.Name, "-config")
	if err := r.Get(ctx, types.NamespacedName{Namespace: cm.Namespace, Name: name}, &node); err != nil {
		return nil
	}

	// 2. 将所有匹配的 WireflowPeer 加入请求队列
	requests = append(requests, reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: node.Namespace,
			Name:      node.Name,
		},
	})
	return requests
}

func (r *PeerReconciler) mapEndpointForNodes(ctx context.Context, obj client.Object) []reconcile.Request {
	endpoint := obj.(*v1alpha1.WireflowEndpoint)
	var requests []reconcile.Request

	//获取所有nsName下的WireflowPeer
	peerList := &v1alpha1.WireflowPeerList{}

	// 使用 ListOptions 锁定命名空间
	listOpts := []client.ListOption{
		client.InNamespace(endpoint.Namespace),
	}

	if err := r.List(ctx, peerList, listOpts...); err != nil {
		return nil
	}

	// 2. 将所有匹配的 WireflowPeer 加入请求队列
	for _, item := range peerList.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: item.Namespace,
				Name:      item.Name,
			},
		})
	}

	return requests
}

// mapPolicyForNodes returns a list of Reconcile Requests for Peers that should be updated based on the given WireflowPolicy.
func (r *PeerReconciler) mapPolicyForNodes(ctx context.Context, obj client.Object) []reconcile.Request {
	policy := obj.(*v1alpha1.WireflowPolicy)
	var requests []reconcile.Request
	//获取对应的节点
	var nodeList v1alpha1.WireflowPeerList
	selector, err := metav1.LabelSelectorAsSelector(&policy.Spec.PeerSelector)
	if err != nil {
		// 记录错误，无法解析选择器
		return nil
	}
	//TODO 是不是不可用？
	if err = r.List(ctx, &nodeList, client.MatchingLabelsSelector{
		Selector: selector,
	}); err != nil {
		return nil
	}

	// 2. 将所有匹配的 WireflowPeer 加入请求队列
	for _, node := range nodeList.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: node.Namespace,
				Name:      node.Name,
			},
		})
	}
	return requests
}

// getNetwork 会获取所有的Networks，正向声明的或者反向声明的都包含
func (r *PeerReconciler) getNetwork(ctx context.Context, peer *v1alpha1.WireflowPeer) (*v1alpha1.WireflowNetwork, error) {

	// 1. 获取所有 WireflowNetwork 资源 (用于反向检查)
	var network v1alpha1.WireflowNetwork
	if err := r.Get(ctx, types.NamespacedName{Namespace: peer.Namespace, Name: *peer.Spec.Network}, &network); err != nil {
		return nil, fmt.Errorf("failed to get joined network: %w", err)
	}

	return &network, nil
}

func (r *PeerReconciler) getNodeContext(ctx context.Context, current *v1alpha1.WireflowPeer, req ctrl.Request) *PeerStateSnapshot {
	if current == nil {
		return &PeerStateSnapshot{}
	}

	var (
		err error
	)
	snapshot := &PeerStateSnapshot{
		Peer: current,
	}

	// 获取网络信息
	if current.Spec.Network != nil {
		networkName := *current.Spec.Network
		var network v1alpha1.WireflowNetwork
		if err = r.Get(ctx, types.NamespacedName{
			Namespace: req.Namespace, Name: networkName,
		}, &network); err != nil {
			return snapshot
		}
		snapshot.Network = &network

		peerList, err := r.findPeersByNetwork(ctx, &network)
		if err != nil {
			return snapshot
		}
		for _, item := range peerList.Items {
			snapshot.Peers = append(snapshot.Peers, &item)
		}
	}

	//获取网络策略
	policyList, err := r.filterPoliciesForNode(ctx, snapshot.Peer)
	if err != nil {
		return snapshot
	}

	snapshot.Policies = policyList

	return snapshot
}

func (r *PeerReconciler) findPeersByNetwork(ctx context.Context, network *v1alpha1.WireflowNetwork) (*v1alpha1.WireflowPeerList, error) {
	labels := map[string]string{
		fmt.Sprintf("wireflow.run/network-%s", network.Name): "true",
	}

	var peers v1alpha1.WireflowPeerList
	if err := r.List(ctx, &peers, client.MatchingLabels(labels)); err != nil {
		return nil, err
	}

	return &peers, nil
}

func (r *PeerReconciler) filterPoliciesForNode(ctx context.Context, node *v1alpha1.WireflowPeer) ([]*v1alpha1.WireflowPolicy, error) {
	var policyList v1alpha1.WireflowPolicyList
	if err := r.List(ctx, &policyList, client.InNamespace(node.Namespace)); err != nil {
		return nil, err
	}

	matched := make([]*v1alpha1.WireflowPolicy, 0)
	nodeLabelSet := labels.Set(node.Labels)

	for _, policy := range policyList.Items {
		selector, err := metav1.LabelSelectorAsSelector(&policy.Spec.PeerSelector)
		if err != nil {
			// selector 写错时：你可以选择忽略该 policy 或直接返回错误
			return nil, fmt.Errorf("invalid nodeSelector in policy %s/%s: %w", policy.Namespace, policy.Name, err)
		}

		// 空 selector {} 会匹配所有对象（这点要注意）
		if selector.Matches(nodeLabelSet) {
			matched = append(matched, &policy)
		}
	}

	return matched, nil
}
