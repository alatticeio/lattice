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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
	"wireflow/internal/infra"
	"wireflow/internal/ipam"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
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
	generator     *Generator
	SnapshotCache map[types.NamespacedName]*PeerStateSnapshot

	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=wireflowcontroller.wireflow.run,resources=wireflowpeers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=wireflowcontroller.wireflow.run,resources=wireflowpeers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=wireflowcontroller.wireflow.run,resources=wireflowpeers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PeerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Reconciling WireflowPeer", "namespace", req.NamespacedName, "name", req.Name)

	var peer v1alpha1.WireflowPeer
	if err := r.Get(ctx, req.NamespacedName, &peer); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Finalizer / deletion handling.
	const finalizerName = "wireflow.run/node"
	if !peer.ObjectMeta.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&peer, finalizerName) {
			return r.handleDelete(ctx, &peer)
		}
		return ctrl.Result{}, nil
	}
	if !controllerutil.ContainsFinalizer(&peer, finalizerName) {
		controllerutil.AddFinalizer(&peer, finalizerName)
		if err := r.Update(ctx, &peer); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 100 * time.Nanosecond}, nil
	}

	// Shadow peers are managed exclusively by NetworkPeeringReconciler /
	// ClusterPeeringReconciler; skip them here to avoid conflicting updates.
	if peer.GetLabels()[LabelShadow] == "true" {
		log.Info("Skipping shadow peer", "namespace", req.Namespace, "name", req.Name)
		return ctrl.Result{}, nil
	}

	// Phase-based state machine.
	switch peer.Status.Phase {
	case "":
		return r.handleInitialization(ctx, &peer, req)
	case v1alpha1.NodePhasePending:
		return r.handlePending(ctx, &peer, req)
	case v1alpha1.NodePhaseReady:
		return r.handleReady(ctx, &peer, req)
	case v1alpha1.NodePhaseFailed:
		return r.handleFailed(ctx, &peer, req)
	default:
		return r.handleInitialization(ctx, &peer, req)
	}
}

// handleInitialization runs when Phase is empty (newly created peer).
// It generates WireGuard keys and advances to Pending.
func (r *PeerReconciler) handleInitialization(ctx context.Context, peer *v1alpha1.WireflowPeer, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Initializing peer", "name", req.Name)

	// Generate WireGuard keys if absent. Returns true when a spec patch was written,
	// requiring a requeue so the next reconcile sees the updated ResourceVersion.
	changed, err := r.ensureKeys(ctx, peer)
	if err != nil {
		return ctrl.Result{}, err
	}
	if changed {
		return ctrl.Result{}, nil
	}

	// Advance to Pending so the next reconcile evaluates network intent.
	if _, err := r.updateStatus(ctx, peer, func(p *v1alpha1.WireflowPeer) {
		p.Status.Phase = v1alpha1.NodePhasePending
		p.Status.Conditions = setCondition(p.Status.Conditions, metav1.Condition{
			Type:               v1alpha1.NodeConditionInitialized,
			Status:             metav1.ConditionTrue,
			Reason:             v1alpha1.ReasonReady,
			LastTransitionTime: metav1.Now(),
		})
	}); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: 100 * time.Millisecond}, nil
}

// handlePending dispatches to joinNetwork, leaveNetwork, or idle based on
// the delta between Spec.Network and Status.ActiveNetwork.
func (r *PeerReconciler) handlePending(ctx context.Context, peer *v1alpha1.WireflowPeer, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	specNet := peer.Spec.Network
	activeNet := peer.Status.ActiveNetwork

	switch {
	case specNet == nil && activeNet == nil:
		// Idle: no network intent; move directly to Ready.
		log.Info("Peer is idle (no network intent)", "name", req.Name)
		if _, err := r.updateStatus(ctx, peer, func(p *v1alpha1.WireflowPeer) {
			p.Status.Phase = v1alpha1.NodePhaseReady
		}); err != nil {
			return ctrl.Result{}, err
		}
		return r.lastReconcile(ctx, peer, req)

	case specNet == nil && activeNet != nil:
		log.Info("Leaving network", "name", req.Name, "network", *activeNet)
		return r.leaveNetwork(ctx, peer, req)

	default:
		// specNet != nil: join or switch networks.
		log.Info("Joining network", "name", req.Name, "network", *specNet)
		return r.joinNetwork(ctx, peer, req)
	}
}

// handleReady detects spec changes that require a network transition and
// delegates to lastReconcile to refresh the ConfigMap when there is nothing to change.
func (r *PeerReconciler) handleReady(ctx context.Context, peer *v1alpha1.WireflowPeer, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	specNet := peer.Spec.Network
	activeNet := peer.Status.ActiveNetwork

	needTransition := (specNet == nil && activeNet != nil) ||
		(specNet != nil && (activeNet == nil || *specNet != *activeNet))

	if needTransition {
		log.Info("Network change detected, transitioning to Pending", "name", req.Name)
		if _, err := r.updateStatus(ctx, peer, func(p *v1alpha1.WireflowPeer) {
			p.Status.Phase = v1alpha1.NodePhasePending
		}); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 100 * time.Millisecond}, nil
	}

	return r.lastReconcile(ctx, peer, req)
}

// handleFailed resets the phase to Pending for a retry after a back-off period.
func (r *PeerReconciler) handleFailed(ctx context.Context, peer *v1alpha1.WireflowPeer, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Peer is in Failed state, scheduling retry", "name", req.Name)
	if _, err := r.updateStatus(ctx, peer, func(p *v1alpha1.WireflowPeer) {
		p.Status.Phase = v1alpha1.NodePhasePending
	}); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *PeerReconciler) handleDelete(ctx context.Context, node *v1alpha1.WireflowPeer) (ctrl.Result, error) {
	// 1. 定义 Finalizer 名称
	const finalizerName = "wireflow.run/node"

	// 2. 检查 Finalizer 是否存在
	if !controllerutil.ContainsFinalizer(node, finalizerName) {
		return ctrl.Result{}, nil // 已经处理过了，直接返回
	}

	// 3. 执行清理逻辑
	// 关键点：这里必须是幂等的！即便执行多次，结果也一样
	if err := r.performCleanup(ctx, node); err != nil {
		// 如果清理失败，不要立即移除 Finalizer，返回 error 触发重试
		return ctrl.Result{}, err
	}

	// 4. 清理成功后，移除 Finalizer
	controllerutil.RemoveFinalizer(node, finalizerName)
	if err := r.Update(ctx, node); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *PeerReconciler) performCleanup(ctx context.Context, node *v1alpha1.WireflowPeer) error {
	// 目前没有外部资源需要清理
	// 预留位置：如果未来需要清理 Relay 缓存或数据库记录，加在这里
	return nil
}

// ensureKeys generates a WireGuard key pair and PeerId when they are absent.
// Does not depend on Spec.Network; safe to call during initialization.
// Returns (true, nil) when a spec patch was written.
func (r *PeerReconciler) ensureKeys(ctx context.Context, peer *v1alpha1.WireflowPeer) (bool, error) {
	if peer.Spec.PrivateKey != "" {
		return false, nil
	}
	return r.updateSpec(ctx, peer, func(node *v1alpha1.WireflowPeer) error {
		key, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return err
		}
		node.Spec.PrivateKey = key.String()
		node.Spec.PublicKey = key.PublicKey().String()
		node.Spec.PeerId = fmt.Sprintf("%d", infra.FromKey(key.PublicKey()).ToUint64())
		return nil
	})
}

// ensurePeerSpec sets the network label for the current Spec.Network, clearing
// stale labels from any previously joined network so the peer never appears in
// more than one network at a time. Requires Spec.Network != nil.
// Returns (true, nil) when a patch was written.
func (r *PeerReconciler) ensurePeerSpec(ctx context.Context, peer *v1alpha1.WireflowPeer) (bool, error) {
	return r.updateSpec(ctx, peer, func(node *v1alpha1.WireflowPeer) error {
		network, err := r.getNetwork(ctx, node)
		if err != nil {
			return err
		}

		lbls := node.GetLabels()
		if lbls == nil {
			lbls = make(map[string]string)
		}
		// 切换网络时，先删除所有旧的 network label，再添加新的，避免 peer 同时出现在多个网络
		for label := range lbls {
			if strings.HasPrefix(label, "wireflow.run/network-") {
				delete(lbls, label)
			}
		}
		lbls[networkLabelKey(network.Name)] = "true"
		node.SetLabels(lbls)
		return nil
	})
}

// joinNetwork allocates an IP for the peer in Spec.Network and marks it Ready.
func (r *PeerReconciler) joinNetwork(ctx context.Context, peer *v1alpha1.WireflowPeer, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var network v1alpha1.WireflowNetwork
	if err := r.Get(ctx, types.NamespacedName{Namespace: peer.Namespace, Name: *peer.Spec.Network}, &network); err != nil {
		if _, sErr := r.updateStatus(ctx, peer, func(p *v1alpha1.WireflowPeer) {
			p.Status.Phase = v1alpha1.NodePhaseFailed
			p.Status.Conditions = setCondition(p.Status.Conditions, metav1.Condition{
				Type:               v1alpha1.NodeConditionJoiningNetwork,
				Status:             metav1.ConditionFalse,
				Reason:             v1alpha1.ReasonNotReady,
				Message:            fmt.Sprintf("Network %q not found: %v", *peer.Spec.Network, err),
				LastTransitionTime: metav1.Now(),
			})
		}); sErr != nil {
			log.Error(sErr, "failed to record JoiningNetwork condition")
		}
		return ctrl.Result{}, err
	}

	// Wait for NetworkReconciler to assign a CIDR before allocating an IP.
	// networkReadyPredicate in SetupWithManager re-enqueues peers when ActiveCIDR transitions.
	if network.Status.ActiveCIDR == "" {
		log.Info("Network not ready yet, waiting for ActiveCIDR", "network", network.Name)
		if _, sErr := r.updateStatus(ctx, peer, func(p *v1alpha1.WireflowPeer) {
			p.Status.Conditions = setCondition(p.Status.Conditions, metav1.Condition{
				Type:               v1alpha1.NodeConditionJoiningNetwork,
				Status:             metav1.ConditionFalse,
				Reason:             v1alpha1.ReasonConfiguring,
				Message:            fmt.Sprintf("Waiting for network %q to assign an ActiveCIDR", network.Name),
				LastTransitionTime: metav1.Now(),
			})
		}); sErr != nil {
			log.Error(sErr, "failed to record JoiningNetwork condition")
		}
		return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
	}

	// Apply network label. Labels don't change Generation, so we requeue explicitly.
	changed, err := r.ensurePeerSpec(ctx, peer)
	if err != nil {
		if _, sErr := r.updateStatus(ctx, peer, func(p *v1alpha1.WireflowPeer) {
			p.Status.Phase = v1alpha1.NodePhaseFailed
			p.Status.Conditions = setCondition(p.Status.Conditions, metav1.Condition{
				Type:               v1alpha1.NodeConditionJoiningNetwork,
				Status:             metav1.ConditionFalse,
				Reason:             v1alpha1.ReasonNotReady,
				Message:            "Failed to apply network labels: " + err.Error(),
				LastTransitionTime: metav1.Now(),
			})
		}); sErr != nil {
			log.Error(sErr, "failed to record JoiningNetwork condition")
		}
		return ctrl.Result{}, err
	}
	if changed {
		return ctrl.Result{RequeueAfter: 100 * time.Millisecond}, nil
	}

	address, err := r.IPAM.AllocateIP(ctx, &network, peer)
	if err != nil {
		if _, sErr := r.updateStatus(ctx, peer, func(p *v1alpha1.WireflowPeer) {
			p.Status.Phase = v1alpha1.NodePhaseFailed
			p.Status.Conditions = setCondition(p.Status.Conditions, metav1.Condition{
				Type:               v1alpha1.NodeConditionIPAllocated,
				Status:             metav1.ConditionFalse,
				Reason:             v1alpha1.ReasonAllocationFailed,
				Message:            err.Error(),
				LastTransitionTime: metav1.Now(),
			})
		}); sErr != nil {
			log.Error(sErr, "failed to record IPAllocated condition")
		}
		return ctrl.Result{}, err
	}
	log.Info("IP allocated", "address", address)

	if _, err = r.updateStatus(ctx, peer, func(p *v1alpha1.WireflowPeer) {
		p.Status.Phase = v1alpha1.NodePhaseReady
		p.Status.AllocatedAddress = &address
		p.Status.ActiveNetwork = p.Spec.Network
		p.Status.Conditions = setCondition(p.Status.Conditions, metav1.Condition{
			Type:               v1alpha1.NodeConditionJoiningNetwork,
			Status:             metav1.ConditionTrue,
			Reason:             v1alpha1.ReasonReady,
			LastTransitionTime: metav1.Now(),
		})
		p.Status.Conditions = setCondition(p.Status.Conditions, metav1.Condition{
			Type:               v1alpha1.NodeConditionIPAllocated,
			Status:             metav1.ConditionTrue,
			Reason:             v1alpha1.ReasonReady,
			LastTransitionTime: metav1.Now(),
		})
	}); err != nil {
		return ctrl.Result{}, err
	}

	return r.lastReconcile(ctx, peer, req)
}

// leaveNetwork clears the network label and status fields, then marks the peer Ready (idle).
func (r *PeerReconciler) leaveNetwork(ctx context.Context, peer *v1alpha1.WireflowPeer, req ctrl.Request) (ctrl.Result, error) {
	if _, err := r.updateSpec(ctx, peer, func(p *v1alpha1.WireflowPeer) error {
		lbls := p.GetLabels()
		for label := range lbls {
			if strings.HasPrefix(label, "wireflow.run/network-") {
				delete(lbls, label)
			}
		}
		p.SetLabels(lbls)
		return nil
	}); err != nil {
		log := logf.FromContext(ctx)
		if _, sErr := r.updateStatus(ctx, peer, func(p *v1alpha1.WireflowPeer) {
			p.Status.Phase = v1alpha1.NodePhaseFailed
			p.Status.Conditions = setCondition(p.Status.Conditions, metav1.Condition{
				Type:               v1alpha1.NodeConditionProvisioned,
				Status:             metav1.ConditionFalse,
				Reason:             v1alpha1.ReasonNotReady,
				Message:            "Failed to remove network labels: " + err.Error(),
				LastTransitionTime: metav1.Now(),
			})
		}); sErr != nil {
			log.Error(sErr, "failed to record Provisioned condition")
		}
		return ctrl.Result{}, err
	}

	if _, err := r.updateStatus(ctx, peer, func(p *v1alpha1.WireflowPeer) {
		p.Status.ActiveNetwork = nil
		p.Status.AllocatedAddress = nil
		p.Status.Phase = v1alpha1.NodePhaseReady
		p.Status.Conditions = setCondition(p.Status.Conditions, metav1.Condition{
			Type:               v1alpha1.NodeConditionJoiningNetwork,
			Status:             metav1.ConditionFalse,
			Reason:             v1alpha1.ReasonLeaving,
			LastTransitionTime: metav1.Now(),
		})
	}); err != nil {
		return ctrl.Result{}, err
	}

	return r.lastReconcile(ctx, peer, req)
}

// lastReconcile create or update the configmap
func (r *PeerReconciler) lastReconcile(ctx context.Context, peer *v1alpha1.WireflowPeer, request ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.Info("Last reconciling", "name", request.NamespacedName)

	configMapName := fmt.Sprintf("%s-config", peer.Name)

	// 1) 每次都重新构建 snapshot（不再做 changes 检查）
	snapshot := r.getPeerStateSnapshot(ctx, peer, request)

	// 2) 用 WireflowPolicy 计算 computedPeers / computedRules，并生成最终 message
	message, err := r.generator.generate(ctx, peer, snapshot, r.generator.generateConfigVersion())
	if err != nil {
		return ctrl.Result{}, err
	}

	var newHash string
	newHash, err = computeMessageHash(message)
	if err != nil {
		return ctrl.Result{}, err
	}

	desiredConfigMap := r.newConfigmap(peer.Namespace, configMapName, message.String(), newHash)
	if err = controllerutil.SetControllerReference(peer, desiredConfigMap, r.Scheme); err != nil {
		logger.Error(err, "Failed to set owner reference on configmap")
		return ctrl.Result{}, err
	}

	// 3) 获取当前 CM，看 hash 是否一致；不一致才更新
	var found corev1.ConfigMap
	err = r.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: peer.Namespace}, &found)
	if err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}

		logger.Info("Creating configmap", "name", configMapName, "hash", newHash)
		manager := client.FieldOwner("wireflow-controller-manager")
		if err := r.Patch(ctx, desiredConfigMap, client.Apply, manager); err != nil {
			logger.Error(err, "Failed to create configmap")
			if _, sErr := r.updateStatus(ctx, peer, func(p *v1alpha1.WireflowPeer) {
				p.Status.Conditions = setCondition(p.Status.Conditions, metav1.Condition{
					Type:               v1alpha1.NodeConditionNetworkConfigured,
					Status:             metav1.ConditionFalse,
					Reason:             v1alpha1.ReasonConfigFailed,
					Message:            "Failed to create config: " + err.Error(),
					LastTransitionTime: metav1.Now(),
				})
			}); sErr != nil {
				logger.Error(sErr, "failed to record NetworkConfigured condition")
			}
			return ctrl.Result{}, err
		}

		// Also persist hash in status so the next reconcile (triggered by the
		// ConfigMap Create event) sees CurrentHash == newHash and skips cleanly.
		if _, err := r.updateStatus(ctx, peer, func(node *v1alpha1.WireflowPeer) {
			node.Status.CurrentHash = newHash
			node.Status.Conditions = setCondition(node.Status.Conditions, metav1.Condition{
				Type:               v1alpha1.NodeConditionNetworkConfigured,
				Status:             metav1.ConditionTrue,
				Reason:             v1alpha1.ReasonReady,
				LastTransitionTime: metav1.Now(),
			})
		}); err != nil {
			return ctrl.Result{}, err
		}

		if r.Recorder != nil {
			r.Recorder.Eventf(peer, corev1.EventTypeNormal, "ConfigMapCreated",
				"configmap=%s hash=%s version=%s", configMapName, newHash, message.ConfigVersion)
		}
		return ctrl.Result{}, nil
	}

	// Fix oldHash using peer status CurrentHash
	oldHash := peer.Status.CurrentHash

	if oldHash == newHash {
		logger.Info("Configmap unchanged by hash, skipping update", "name", configMapName, "hash", newHash)
		return ctrl.Result{}, nil
	}

	logger.Info("Updating configmap by hash", "namespace", peer.Namespace, "name", configMapName, "oldHash", oldHash, "newHash", newHash)
	manager := client.FieldOwner("wireflow-controller-manager")
	if err := r.Patch(ctx, desiredConfigMap, client.Apply, manager); err != nil {
		logger.Error(err, "Failed to update configmap")
		if _, sErr := r.updateStatus(ctx, peer, func(p *v1alpha1.WireflowPeer) {
			p.Status.Conditions = setCondition(p.Status.Conditions, metav1.Condition{
				Type:               v1alpha1.NodeConditionNetworkConfigured,
				Status:             metav1.ConditionFalse,
				Reason:             v1alpha1.ReasonConfigFailed,
				Message:            "Failed to update config: " + err.Error(),
				LastTransitionTime: metav1.Now(),
			})
		}); sErr != nil {
			logger.Error(sErr, "failed to record NetworkConfigured condition")
		}
		return ctrl.Result{}, err
	}

	ok, err := r.updateStatus(ctx, peer, func(node *v1alpha1.WireflowPeer) {
		node.Status.CurrentHash = newHash
		node.Status.Conditions = setCondition(node.Status.Conditions, metav1.Condition{
			Type:               v1alpha1.NodeConditionNetworkConfigured,
			Status:             metav1.ConditionTrue,
			Reason:             v1alpha1.ReasonReady,
			LastTransitionTime: metav1.Now(),
		})
	})

	if err != nil {
		return ctrl.Result{}, err
	}

	if ok {
		return ctrl.Result{}, nil
	}

	if r.Recorder != nil {
		r.Recorder.Eventf(peer, corev1.EventTypeNormal, "ConfigMapUpdated",
			"configmap=%s oldHash=%s newHash=%s version=%s", configMapName, oldHash, newHash, message.ConfigVersion)
	}
	return ctrl.Result{}, nil
}

// 如果你想看具体哪里变了
//func diffConfig(oldData, newData string) {
//	// 假设是 JSON 格式字符串，先反序列化
//	var oldMap, newMap map[string]interface{}
//	json.Unmarshal([]byte(oldData), &oldMap)
//	json.Unmarshal([]byte(newData), &newMap)
//
//	// 使用 go-cmp 打印详细差异
//	fmt.Println(cmp.Diff(oldMap, newMap))
//}

func (r *PeerReconciler) newConfigmap(namespace, configMapName, message, hash string) *corev1.ConfigMap {
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
			Annotations: map[string]string{
				"wireflow.run/config-hash": hash,
			},
		},
		Data: map[string]string{
			"config.json": message,
		},
	}
}

func computeMessageHash(msg *infra.Message) (string, error) {
	// 排除 ConfigVersion 和 Timestamp，避免这两个字段每次变化导致 hash 不稳定
	tmp := struct {
		*infra.Message
		ConfigVersion interface{} `json:"configVersion,omitempty"` // 覆盖排除
		Timestamp     interface{} `json:"timestamp,omitempty"`     // 覆盖排除
	}{
		Message:       msg,
		ConfigVersion: nil,
		Timestamp:     nil,
	}

	b, err := json.Marshal(tmp)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(b)), nil
}

// updateSpec applies updateFunc to a deep copy of node and patches only when
// something actually changed. Returns (true, nil) when a patch was written so
// the caller can exit early and let the next reconcile continue the flow.
func (r *PeerReconciler) updateSpec(ctx context.Context, node *v1alpha1.WireflowPeer, updateFunc func(node *v1alpha1.WireflowPeer) error) (bool, error) {
	log := logf.FromContext(ctx)

	nodeCopy := node.DeepCopy()
	if err := updateFunc(nodeCopy); err != nil {
		log.Error(err, "Failed to build WireflowPeer Spec update")
		return false, err
	}

	// Check before patching — skip empty API calls that could spuriously
	// update ResourceVersion and trigger further watch events.
	if reflect.DeepEqual(nodeCopy.Spec, node.Spec) &&
		reflect.DeepEqual(nodeCopy.Labels, node.Labels) &&
		reflect.DeepEqual(nodeCopy.Annotations, node.Annotations) {
		return false, nil
	}

	if err := r.Patch(ctx, nodeCopy, client.MergeFrom(node)); err != nil {
		if errors.IsConflict(err) {
			log.Info("Conflict detected during WireflowPeer Spec patch, will retry on next reconcile.")
			return false, nil
		}
		log.Error(err, "Failed to patch WireflowPeer Spec")
		return false, err
	}

	// Sync patched state back into the caller's pointer so subsequent patches
	// in the same reconcile use the updated ResourceVersion (avoids conflicts).
	node.Spec = nodeCopy.Spec
	node.Labels = nodeCopy.Labels
	node.Annotations = nodeCopy.Annotations
	node.ResourceVersion = nodeCopy.ResourceVersion

	log.Info("WireflowPeer Spec/Metadata patched")
	return true, nil
}

// updateStatus applies updateFunc to a deep copy of node and patches the status
// subresource only when something actually changed.
func (r *PeerReconciler) updateStatus(ctx context.Context, node *v1alpha1.WireflowPeer, updateFunc func(node *v1alpha1.WireflowPeer)) (bool, error) {
	log := logf.FromContext(ctx)

	nodeCopy := node.DeepCopy()
	updateFunc(nodeCopy)

	// Check before patching — status patches that produce no diff still
	// update ResourceVersion on some API server versions.
	if reflect.DeepEqual(nodeCopy.Status, node.Status) {
		return false, nil
	}

	if err := r.Status().Patch(ctx, nodeCopy, client.MergeFrom(node)); err != nil {
		if errors.IsConflict(err) {
			log.Info("Conflict detected during WireflowPeer Status patch, will retry on next reconcile.")
			return false, nil
		}
		log.Error(err, "Failed to patch WireflowPeer Status")
		return false, err
	}

	// Sync patched state back so the next patch in the same reconcile sees the
	// updated ResourceVersion without an extra r.Get().
	node.Status = nodeCopy.Status
	node.ResourceVersion = nodeCopy.ResourceVersion

	log.Info("WireflowPeer Status patched")
	return true, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PeerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorderFor("wireflow-peer-controller")
	}

	if r.generator == nil {
		r.generator = NewGenerator(mgr.GetClient())
	}

	// 定义一个只管更新的过滤器
	onlyUpdatePredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			// 启动时的全量加载会触发这里，返回 false 拦截掉
			return false
		},
	}

	ownedCMPredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetLabels()["app.kubernetes.io/managed-by"] == "wireflow-controller"
	})

	configMapPredicate := predicate.Funcs{
		// 不响应 Create：控制器自己 Create configmap 后无需再 enqueue peer（
		// Status.CurrentHash 已写入，下次 reconcile 会直接跳过）
		CreateFunc: func(e event.CreateEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldCm := e.ObjectOld.(*corev1.ConfigMap)
			newCm := e.ObjectNew.(*corev1.ConfigMap)
			// 只有 Data 内容真正变了，才触发 map 函数（外部修改才需要自愈）
			return !reflect.DeepEqual(oldCm.Data, newCm.Data)
		},
	}
	// 监听 WireflowNetwork 的 spec 变化（generation changed）以及 ActiveCIDR 从空变非空（status patch）。
	// 使用自定义 predicate 是因为 GenerationChangedPredicate 只检测 spec 变化，
	// 而 NetworkReconciler 分配 ActiveCIDR 是 status patch，不改变 generation。
	networkReadyPredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldNet, ok1 := e.ObjectOld.(*v1alpha1.WireflowNetwork)
			newNet, ok2 := e.ObjectNew.(*v1alpha1.WireflowNetwork)
			if !ok1 || !ok2 {
				return false
			}
			// spec 变化 或 ActiveCIDR 刚被分配 → 触发 peer reconcile
			return oldNet.Generation != newNet.Generation ||
				(oldNet.Status.ActiveCIDR == "" && newNet.Status.ActiveCIDR != "")
		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.WireflowPeer{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&v1alpha1.WireflowNetwork{},
			handler.EnqueueRequestsFromMapFunc(r.mapNetworkForNodes),
			builder.WithPredicates(networkReadyPredicate)).
		Watches(&v1alpha1.WireflowEndpoint{},
			handler.EnqueueRequestsFromMapFunc(r.mapEndpointForNodes),
			builder.WithPredicates(predicate.And(onlyUpdatePredicate, predicate.GenerationChangedPredicate{}))).
		Watches(&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(r.mapConfigMapForNodes),
			builder.WithPredicates(predicate.And(configMapPredicate, ownedCMPredicate))).
		Watches(&v1alpha1.WireflowPolicy{},
			handler.EnqueueRequestsFromMapFunc(r.mapPolicyForNodes),
			// 不加 onlyUpdatePredicate：新建策略（Create）必须触发 peer reconcile 才能下发配置；
			// GenerationChangedPredicate 默认放行 Create/Delete，只过滤 generation 未变的 Update。
			builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Named("node").WithOptions(controller.Options{
		MaxConcurrentReconciles: 5,
	}).Complete(r)
}

// mapNetworkForNodes returns a list of Reconcile Requests for Peers that should be updated based on the given WireflowNetwork.
func (r *PeerReconciler) mapNetworkForNodes(ctx context.Context, obj client.Object) []reconcile.Request {
	network := obj.(*v1alpha1.WireflowNetwork)
	var requests []reconcile.Request

	// 只获取真正加入了该网络的 WireflowPeer（通过网络标签），避免空 PeerSelector 匹配所有 peer
	networkLabel := networkLabelKey(network.Name)
	nodeList := &v1alpha1.WireflowPeerList{}
	if err := r.List(ctx, nodeList, client.InNamespace(network.Namespace), client.MatchingLabels(map[string]string{networkLabel: "true"})); err != nil {
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

// mapPolicyForNodes returns reconcile requests for peers affected by a policy change.
// Peers are scoped to the policy's network to avoid triggering peers from other
// networks in the same namespace (an empty PeerSelector would otherwise match all).
func (r *PeerReconciler) mapPolicyForNodes(ctx context.Context, obj client.Object) []reconcile.Request {
	policy := obj.(*v1alpha1.WireflowPolicy)

	selector, err := metav1.LabelSelectorAsSelector(&policy.Spec.PeerSelector)
	if err != nil {
		return nil
	}

	listOpts := []client.ListOption{
		client.InNamespace(policy.Namespace),
		client.MatchingLabelsSelector{Selector: selector},
	}
	// Scope to the policy's network so that an empty PeerSelector only reaches
	// peers that are actually part of this network, not all peers in the namespace.
	if policy.Spec.Network != "" {
		listOpts = append(listOpts, client.MatchingLabels{
			networkLabelKey(policy.Spec.Network): "true",
		})
	}

	var nodeList v1alpha1.WireflowPeerList
	if err = r.List(ctx, &nodeList, listOpts...); err != nil {
		return nil
	}

	requests := make([]reconcile.Request, 0, len(nodeList.Items))
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

func (r *PeerReconciler) getPeerStateSnapshot(ctx context.Context, current *v1alpha1.WireflowPeer, req ctrl.Request) *PeerStateSnapshot {
	var (
		err error
	)
	snapshot := &PeerStateSnapshot{
		Peer:   current,
		Labels: current.GetLabels(),
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

		var peerList *v1alpha1.WireflowPeerList
		peerList, err = r.findPeersByNetwork(ctx, &network)
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
		networkLabelKey(network.Name): "true",
	}

	var peers v1alpha1.WireflowPeerList
	if err := r.List(ctx, &peers, client.InNamespace(network.Namespace), client.MatchingLabels(labels)); err != nil {
		return nil, err
	}

	return &peers, nil
}

func (r *PeerReconciler) filterPoliciesForNode(ctx context.Context, peer *v1alpha1.WireflowPeer) ([]*v1alpha1.WireflowPolicy, error) {
	var policyList v1alpha1.WireflowPolicyList
	if err := r.List(ctx, &policyList, client.InNamespace(peer.Namespace)); err != nil {
		return nil, err
	}

	matched := make([]*v1alpha1.WireflowPolicy, 0)
	nodeLabelSet := labels.Set(peer.Labels)

	peerNetwork := ""
	if peer.Spec.Network != nil {
		peerNetwork = *peer.Spec.Network
	}

	for _, policy := range policyList.Items {
		// 只匹配同一网络的 policy，避免其他网络的 policy 影响当前 peer 的 hash
		if policy.Spec.Network != peerNetwork {
			continue
		}

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
