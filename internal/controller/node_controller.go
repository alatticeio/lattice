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
	"wireflow/internal"

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

// NodeReconciler reconciles a Node object
type NodeReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Detector     *ChangeDetector
	NodeCtxCache map[types.NamespacedName]*NodeContext
}

// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=wireflowcontroller.wireflowio.com,resources=nodes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=wireflowcontroller.wireflowio.com,resources=nodes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=wireflowcontroller.wireflowio.com,resources=nodes/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Node object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
func (r *NodeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Reconciling Node", "namespace", req.NamespacedName, "node", req.Name)

	var (
		err  error
		node v1alpha1.Node
	)

	if err = r.Get(ctx, req.NamespacedName, &node); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Node resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}

		log.Error(err, "Failed to get Node")
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
		return r.reconcileConfigMap(ctx, &node, req)
	}

}

type Action string

const (
	NodeJoinNetwork  Action = "joinNetwork"
	NodeLeaveNetwork Action = "leaveNetwork"
	ActionNone       Action = "none"
)

// reconcileJoinNetwork handle join network
func (r *NodeReconciler) reconcileJoinNetwork(ctx context.Context, node *v1alpha1.Node, request ctrl.Request) (ctrl.Result, error) {
	var (
		err error
		ok  bool
	)
	log := logf.FromContext(ctx)
	log.Info("Join network", "namespace", request.Namespace, "name", request.Name)

	//1. 更新Phase为Pending
	if node.Status.Phase != v1alpha1.NodePhasePending {
		ok, err = r.updateStatus(ctx, node, func(node *v1alpha1.Node) {
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
	ok, err = r.updateSpec(ctx, node, func(node *v1alpha1.Node) {
		associatedNetworks, err := r.getAssociatedNetworks(ctx, node)
		if err != nil {
			return
		}
		labels := node.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
		}
		for _, network := range associatedNetworks {
			labels[fmt.Sprintf("wireflowio.com/network-%s", network.Name)] = "true"
		}
		node.SetLabels(labels)

		if node.Spec.PrivateKey == "" {
			key, err := wgtypes.GeneratePrivateKey()
			if err != nil {
				return
			}

			node.Spec.PrivateKey = key.String()
			node.Spec.PublicKey = key.PublicKey().String()
		}
	})

	if err != nil {
		return ctrl.Result{}, err
	}

	if ok {
		//直接返回，等下次reconcile
		return ctrl.Result{}, nil
	}

	//重新获取node用来更新status, 避免冲突
	if err = r.Get(ctx, request.NamespacedName, node); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Node resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}

		log.Error(err, "Failed to get Node")
		return ctrl.Result{}, err
	}

	//查询primary network 分配的ip
	primaryNetwork := node.Spec.Networks[0]
	var network v1alpha1.Network
	if err = r.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s/%s", node.Namespace, primaryNetwork)}, &network); err != nil {
		return ctrl.Result{}, err
	}

	var allocatedIP string
	for _, ipAllcations := range network.Status.AllocatedIPs {
		if ipAllcations.Node == node.Name {
			allocatedIP = ipAllcations.IP
			break
		}
	}

	if allocatedIP == "" {
		// networks not ready
		return ctrl.Result{}, nil
	}

	if ok, err = r.updateStatus(ctx, node, func(node *v1alpha1.Node) {
		node.Status.Phase = v1alpha1.NodePhaseReady
		node.Status.ActiveNetworkPolicies = node.Spec.Networks
		node.Status.AllocatedAddress = allocatedIP
		node.Status.ActiveNetworks = node.Spec.Networks
	}); err != nil {
		return ctrl.Result{}, err
	}

	if ok {
		return ctrl.Result{}, nil
	}

	return r.reconcileConfigMap(ctx, node, request)
}

// reconcileConfigMap create or update the configmap
func (r *NodeReconciler) reconcileConfigMap(ctx context.Context, node *v1alpha1.Node, request ctrl.Request) (ctrl.Result, error) {
	var (
		err              error
		changes          *internal.ChangeDetails
		message          *internal.Message
		desiredConfigMap *corev1.ConfigMap
	)
	logger := logf.FromContext(ctx)

	//最后处理configmap
	oldNodeCtx := r.NodeCtxCache[request.NamespacedName]
	newNodeCtx := r.getNodeContext(ctx, node, request)
	// 1. 定义期望状态 (Desired State)
	configMapName := fmt.Sprintf("%s-config", node.Name)
	// 2. 获取当前状态 (Current State)
	foundConfigMap := &corev1.ConfigMap{}
	err = r.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: node.Namespace}, foundConfigMap)
	if oldNodeCtx == nil || (err != nil && errors.IsNotFound(err)) {
		if oldNodeCtx == nil {
			// first time create cm
			message, err = r.Detector.buildFullConfig(ctx, node, newNodeCtx, changes, "init")
		} else {
			// updated
			message, err = r.Detector.buildFullConfig(ctx, node, newNodeCtx, changes, r.Detector.generateConfigVersion())
		}

		desiredConfigMap = r.buildConfigMap(node.Namespace, configMapName, message.String())

		// 关键步骤：设置 OwnerReference
		// 这确保了当主资源 (node) 被删除时，这个 reconcileConfigMap 也会被 K8s 垃圾回收器自动删除。
		if err := controllerutil.SetControllerReference(node, desiredConfigMap, r.Scheme); err != nil {
			logger.Error(err, "Failed to set owner reference on reconcileConfigMap")
			return ctrl.Result{}, err
		}

		// --- A. 不存在：执行创建操作 ---
		logger.Info("Creating reconcileConfigMap", "reconcileConfigMap.Name", configMapName)
		r.NodeCtxCache[request.NamespacedName] = newNodeCtx
		if err = r.Create(ctx, desiredConfigMap); err != nil {
			logger.Error(err, "Failed to create reconcileConfigMap")
			return ctrl.Result{}, err
		}
		// 写入成功：立即返回，等待新的事件触发下一次 Reconcile
		return ctrl.Result{}, nil
	} else {
		r.NodeCtxCache[request.NamespacedName] = newNodeCtx
		changes = r.Detector.DetectNodeChanges(ctx, oldNodeCtx, oldNodeCtx.Node, newNodeCtx.Node, oldNodeCtx.Network, newNodeCtx.Network, oldNodeCtx.Policies, newNodeCtx.Policies, request)
		if changes.HasChanges() {
			message, err = r.Detector.buildFullConfig(ctx, node, newNodeCtx, changes, r.Detector.generateConfigVersion())
			desiredConfigMap = r.buildConfigMap(node.Namespace, configMapName, message.String())

			// --- B. 已存在：执行更新操作 (保证幂等性) ---
			if !reflect.DeepEqual(foundConfigMap.Data, desiredConfigMap.Data) {
				logger.Info("Updating reconcileConfigMap data", "reconcileConfigMap.Name", configMapName)

				// 复制最新的 Data 到已存在的对象上 (保持 ResourceVersion 和其他字段)
				foundConfigMap.Data = desiredConfigMap.Data

				if err := r.Update(ctx, foundConfigMap); err != nil {
					logger.Error(err, "Failed to update reconcileConfigMap")
					return ctrl.Result{}, err
				}
				// 写入成功：立即返回
				return ctrl.Result{}, nil
			}

		}
		return ctrl.Result{}, nil
	}

	//return ctrl.Result{}, nil
}

func (r *NodeReconciler) buildConfigMap(namespace, configMapName, message string) *corev1.ConfigMap {
	// 根据 CRD 的 Spec 构建 reconcileConfigMap 的内容
	return &corev1.ConfigMap{
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
func (r *NodeReconciler) reconcileLeaveNetwork(ctx context.Context, node *v1alpha1.Node, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Leaving network", "namespace", req.Namespace, "name", req.Name)
	var (
		err error
		ok  bool
	)

	//1. 更新Phase为Pending
	if node.Status.Phase != v1alpha1.NodePhasePending {
		ok, err = r.updateStatus(ctx, node, func(node *v1alpha1.Node) {
			node.Status.Phase = v1alpha1.NodePhasePending
		})
		if err != nil {
			return ctrl.Result{}, err
		}

		if ok {
			return ctrl.Result{}, nil
		}
	}

	//2. 查询要更新的network
	leavingNetworks := r.getLeavingNetwork(ctx, node)
	specNetworks := stringSet(node.Spec.Networks)

	// 2.修改Spec
	ok, err = r.updateSpec(ctx, node, func(node *v1alpha1.Node) {

		labels := node.GetLabels()
		for _, network := range leavingNetworks {
			delete(labels, fmt.Sprintf("wireflowio.com/network-%s", network))
			// 删除network in spec
		}
		node.SetLabels(labels)

		//删除leavingNetworks
		for _, network := range leavingNetworks {
			if _, ok := specNetworks[network]; ok {
				delete(specNetworks, network)
			}
		}

		// update spec networks
		node.Spec.Networks = setsToSlice(specNetworks)
	})

	if err != nil {
		return ctrl.Result{}, err
	}

	if ok {
		//直接返回，等下次reconcile
		return ctrl.Result{}, nil
	}

	//重新获取node用来更新status, 避免冲突
	if err = r.Get(ctx, req.NamespacedName, node); err != nil {
		if errors.IsNotFound(err) {
			log.Info("Node resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}

		log.Error(err, "Failed to get Node")
		return ctrl.Result{}, err
	}

	//查询primary network 分配的ip
	var allocatedIP string
	if len(node.Spec.Networks) == 0 {
		ok, err = r.updateStatus(ctx, node, func(node *v1alpha1.Node) {
			node.Status.AllocatedAddress = allocatedIP
			node.Status.ActiveNetworks = node.Spec.Networks
		})
		if err != nil {
			return ctrl.Result{}, err
		}

		if ok {
			return ctrl.Result{}, nil
		}

	} else {
		primaryNetwork := node.Spec.Networks[0]
		var network v1alpha1.Network
		if err = r.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s/%s", node.Namespace, primaryNetwork)}, &network); err != nil {
			return ctrl.Result{}, err
		}

		for _, ipAllcations := range network.Status.AllocatedIPs {
			if ipAllcations.Node == node.Name {
				allocatedIP = ipAllcations.IP
				break
			}
		}

		ok, err = r.updateStatus(ctx, node, func(node *v1alpha1.Node) {
			node.Status.AllocatedAddress = allocatedIP
			node.Status.ActiveNetworks = node.Spec.Networks
		})
		if err != nil {
			return ctrl.Result{}, err
		}

		if ok {
			return ctrl.Result{}, nil
		}

	}

	// handle policy

	return r.reconcileConfigMap(ctx, node, req)
}

// reconcileNodePolicy handle node policy change
//func (r *NodeReconciler) buildNodePolicy(ctx context.Context, node *v1alpha1.Node, req ctrl.Request) (ctrl.Result, error) {
//	log := logf.FromContext(ctx)
//	log.Info("Reconcile node policy", "namespace", req.Namespace, "name", req.Name)
//	// find all policy
//	var policyList v1alpha1.NetworkPolicyList
//	if err := r.List(context.Background(), &policyList); err != nil {
//		log.Error(err, "Failed to list NetworkPolicies")
//		return ctrl.Result{}, err
//	}
//
//	currentLables := node.GetLabels()
//	for _, policyItem := range policyList.Items {
//		policyItem.Spec.NodeSelector.MatchLabels
//	}
//
//}

func (r *NodeReconciler) getLeavingNetwork(ctx context.Context, node *v1alpha1.Node) []string {
	specNetworks := stringSet(node.Spec.Networks)
	activeNetworks := stringSet(node.Status.ActiveNetworks)

	sets := setsDifference(specNetworks, activeNetworks)

	//转slices
	slice := make([]string, len(sets))
	i := 0
	for k := range sets {
		slice[i] = k
		i++
	}
	return slice
}

// reconcileSpec 检查并修正 Node.Spec 字段。
// 如果 Spec 被修改并成功写入，返回 (true, nil)，调用者应立即退出 Reconcile。
// 否则返回 (false, nil) 或 (false, error)。
func (r *NodeReconciler) updateSpec(ctx context.Context, node *v1alpha1.Node, updateFunc func(node *v1alpha1.Node)) (bool, error) {
	log := logf.FromContext(ctx)

	// 深拷贝原始资源，用于 Patch 的对比基准。
	nodeCopy := node.DeepCopy()

	// 添加network spec
	updateFunc(nodeCopy)

	// 使用 Patch 发送差异。client.MergeFrom 会自动检查 nodeCopy 和 node 之间的差异。
	if err := r.Patch(ctx, nodeCopy, client.MergeFrom(node)); err != nil {
		if errors.IsConflict(err) {
			// 遇到并发冲突 (409)，不返回错误，让 Manager 自动通过新的事件重试。
			log.Info("Conflict detected during Node Spec patch, will retry on next reconcile.")
			return false, nil
		}
		// 其他写入错误（例如权限不足）
		log.Error(err, "Failed to patch Node Spec")
		return false, err
	}

	// 如果原始资源和当前资源在 Metadata/Spec/Annotation 上没有差异，说明 Patch 只是空操作。
	// 注意：判断 Patch 是否执行写入，最简单的方法是比较原始和当前的 Labels/Annotations/Spec 字段。
	if !reflect.DeepEqual(nodeCopy.Spec, node.Spec) ||
		!reflect.DeepEqual(nodeCopy.Labels, node.Labels) ||
		!reflect.DeepEqual(nodeCopy.Annotations, node.Annotations) {

		log.Info("Node Metadata/Spec successfully patched. Returning to trigger next reconcile.")
		// Spec 或 Metadata 被修改并成功写入 API Server
		return true, nil
	}

	// Spec 未发生修改
	return false, nil
}

// reconcileSpec 检查并修正 Node.Spec 字段。
// 如果 Spec 被修改并成功写入，返回 (true, nil)，调用者应立即退出 Reconcile。
// 否则返回 (false, nil) 或 (false, error)。
func (r *NodeReconciler) updateStatus(ctx context.Context, node *v1alpha1.Node, updateFunc func(node *v1alpha1.Node)) (bool, error) {
	log := logf.FromContext(ctx)

	// 1. 深拷贝原始资源，用于 Patch 的对比基准。
	nodeCopy := node.DeepCopy()

	// 添加network spec
	updateFunc(nodeCopy)

	// 使用 Patch 发送差异。client.MergeFrom 会自动检查 nodeCopy 和 node 之间的差异。
	if err := r.Status().Patch(ctx, nodeCopy, client.MergeFrom(node)); err != nil {
		if errors.IsConflict(err) {
			// 遇到并发冲突 (409)，不返回错误，让 Manager 自动通过新的事件重试。
			log.Info("Conflict detected during Node Spec patch, will retry on next reconcile.")
			return false, nil
		}
		// 其他写入错误（例如权限不足）
		log.Error(err, "Failed to patch Node Spec")
		return false, err
	}

	// 如果原始资源和当前资源在 Metadata/Spec/Annotation 上没有差异，说明 Patch 只是空操作。
	// 注意：判断 Patch 是否执行写入，最简单的方法是比较原始和当前的 Labels/Annotations/Spec 字段。
	if !reflect.DeepEqual(nodeCopy.Status, node.Status) {

		log.Info("Node Metadata/Spec successfully patched. Returning to trigger next reconcile.")
		// Spec 或 Metadata 被修改并成功写入 API Server
		return true, nil
	}

	// Spec 未发生修改
	return false, nil
}

// reconcileNetworkChanged handle network changed
func (r *NodeReconciler) reconcileNetworkChanged(ctx context.Context, node *v1alpha1.Node, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	var (
		err         error
		networkList v1alpha1.NetworkList
	)
	// 查询监控的所有Networks
	if err = r.List(ctx, &networkList, client.InNamespace(req.Namespace)); err != nil {
		if !errors.IsNotFound(err) {
			log.Error(err, "Failed to list Networks")
			return ctrl.Result{}, err
		}
	}

	//处理当前node的network
	for _, network := range networkList.Items {
		//primary network
		if network.Name == node.Spec.Networks[0] {
			if network.Status.AllocatedIPs == nil {
				return ctrl.Result{}, nil
			}

			for _, ipAllcations := range network.Status.AllocatedIPs {
				if ipAllcations.Node == node.Name {
					node.Status.AllocatedAddress = ipAllcations.IP
					//更新node
					node.Status.Phase = v1alpha1.NodePhaseReady
					if err = r.Status().Update(ctx, node); err != nil {
						return ctrl.Result{}, err
					}
					break
				}
			}
		}

	}

	return ctrl.Result{}, nil
}

func (r *NodeReconciler) determineAction(ctx context.Context, node *v1alpha1.Node) (Action, error) {
	log := logf.FromContext(ctx)
	log.Info("Determine action for node", "namespace", node.Namespace, "name", node.Name)
	activeNets := node.Status.ActiveNetworks

	specNets := stringSet(node.Spec.Networks)

	if len(specNets) == 0 && len(activeNets) > 0 {
		return NodeLeaveNetwork, nil
	}

	if len(specNets) > 0 && len(activeNets) == 0 {
		return NodeJoinNetwork, nil
	}

	return ActionNone, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NodeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Node{}).
		Watches(&v1alpha1.Network{},
			handler.EnqueueRequestsFromMapFunc(r.mapNetworkForNodes),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Watches(&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(r.mapConfigMapForNodes),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Watches(&v1alpha1.NetworkPolicy{},
			handler.EnqueueRequestsFromMapFunc(r.mapPolicyForNodes),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).Named("node").Complete(r)
}

// mapNetworkForNodes returns a list of Reconcile Requests for Nodes that should be updated based on the given Network.
func (r *NodeReconciler) mapNetworkForNodes(ctx context.Context, obj client.Object) []reconcile.Request {
	network := obj.(*v1alpha1.Network)
	var requests []reconcile.Request

	// 1. 获取所有 Node (或只获取匹配 Network.Spec.NodeSelector 的 Node)
	nodeList := &v1alpha1.NodeList{}
	if err := r.List(ctx, nodeList, client.MatchingLabels(network.Spec.NodeSelector)); err != nil {
		return nil
	}

	// 2. 将所有匹配的 Node 加入请求队列
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

func (r *NodeReconciler) mapConfigMapForNodes(ctx context.Context, obj client.Object) []reconcile.Request {
	cm := obj.(*corev1.ConfigMap)
	var requests []reconcile.Request

	// 1. 获取所有 Node (或只获取匹配 Network.Spec.NodeSelector 的 Node)
	var node v1alpha1.Node
	names := strings.Split(cm.Name, "-")
	if err := r.Get(ctx, types.NamespacedName{Namespace: cm.Namespace, Name: names[0]}, &node); err != nil {
		return nil
	}

	// 2. 将所有匹配的 Node 加入请求队列
	requests = append(requests, reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: node.Namespace,
			Name:      node.Name,
		},
	})
	return requests
}

// mapPolicyForNodes returns a list of Reconcile Requests for Nodes that should be updated based on the given NetworkPolicy.
func (r *NodeReconciler) mapPolicyForNodes(ctx context.Context, obj client.Object) []reconcile.Request {
	policy := obj.(*v1alpha1.NetworkPolicy)
	var requests []reconcile.Request
	//获取对应的节点
	var nodeList v1alpha1.NodeList
	selector, err := metav1.LabelSelectorAsSelector(&policy.Spec.NodeSelector)
	if err != nil {
		// 记录错误，无法解析选择器
		return nil
	}
	//TODO 是不是不可用？
	if err = r.List(ctx, &nodeList, client.MatchingLabelsSelector{
		selector,
	}); err != nil {
		return nil
	}

	// 2. 将所有匹配的 Node 加入请求队列
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

// getAssociatedNetworks 会获取所有的Networks，正向声明的或者反向声明的都包含
func (r *NodeReconciler) getAssociatedNetworks(ctx context.Context, node *v1alpha1.Node) ([]v1alpha1.Network, error) {

	// 1. 获取所有 Network 资源 (用于反向检查)
	allNetworks := &v1alpha1.NetworkList{}
	if err := r.List(ctx, allNetworks); err != nil {
		return nil, fmt.Errorf("failed to list all networks: %w", err)
	}

	associatedNetworks := make(map[string]v1alpha1.Network) // 用 map 避免重复

	// 检查 Node 自己 Spec 中声明加入的 Network
	if node.Spec.Networks != nil { // 假设您扩展了 Node.Spec
		for _, netName := range node.Spec.Networks {
			for _, net := range allNetworks.Items {
				if net.Name == netName {
					associatedNetworks[netName] = net
					break
				}
			}
		}
	}

	// 检查 Network Spec 中声明包含该 Node 的 Network
	for _, net := range allNetworks.Items {
		// 检查 NodeSelector (Label 方式)
		if len(net.Spec.NodeSelector) > 0 {
			// 使用 Kubernetes 标签选择器匹配逻辑
			selector := labels.SelectorFromSet(net.Spec.NodeSelector)
			if selector.Matches(labels.Set(node.Labels)) {
				associatedNetworks[net.Name] = net
				continue // 如果通过 Label 加入，跳过下一个检查
			}
		}

		// 检查 Nodes 列表 (名称列表方式)
		for _, nodeName := range net.Spec.Nodes {
			if nodeName == node.Name {
				associatedNetworks[net.Name] = net
				break
			}
		}
	}

	// 将 map 转换为 slice
	result := make([]v1alpha1.Network, 0, len(associatedNetworks))
	for _, net := range associatedNetworks {
		result = append(result, net)
	}

	return result, nil
}
