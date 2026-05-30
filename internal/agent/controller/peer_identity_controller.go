// Copyright 2026 The Lattice Authors, Inc.
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
	"time"

	v1alpha1 "github.com/alatticeio/lattice/api/v1alpha1"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// PeerIdentityReconciler reconciles PeerIdentity objects
type PeerIdentityReconciler struct {
	client.Client
	log logr.Logger
}

// NewPeerIdentityReconciler 创建一个新的 PeerIdentity reconciler
func NewPeerIdentityReconciler(c client.Client) *PeerIdentityReconciler {
	return &PeerIdentityReconciler{
		Client: c,
		log:    logf.Log.WithName("peer-identity-controller"),
	}
}

// +kubebuilder:rbac:groups=alattice.io,resources=peeridentities,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=alattice.io,resources=peeridentities/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=alattice.io,resources=latticepeers,verbs=get;list;watch;update;patch

// Reconcile 处理 PeerIdentity 变更
func (r *PeerIdentityReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.log.WithValues("peeridentity", req.NamespacedName)

	// 获取 PeerIdentity 实例
	peerIdentity := &v1alpha1.PeerIdentity{}
	if err := r.Get(ctx, req.NamespacedName, peerIdentity); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("PeerIdentity not found, ignoring")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get PeerIdentity")
		return ctrl.Result{}, err
	}

	log.Info("Reconciling PeerIdentity", "peerRef", peerIdentity.Spec.PeerRef)

	// 解析 peerRef
	resolvedIP, err := r.resolvePeerRef(ctx, peerIdentity.Spec.PeerRef, peerIdentity.Spec.Network)
	if err != nil {
		log.Error(err, "Failed to resolve peerRef", "peerRef", peerIdentity.Spec.PeerRef)
		r.setCondition(peerIdentity, v1alpha1.PeerIdentityConditionPeerBound, metav1.ConditionFalse,
			v1alpha1.PeerIdentityReasonPeerNotFound, err.Error())
		if err := r.Status().Update(ctx, peerIdentity); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// 更新 resolvedPeerIP
	peerIdentity.Status.ResolvedPeerIP = resolvedIP
	r.setCondition(peerIdentity, v1alpha1.PeerIdentityConditionPeerBound, metav1.ConditionTrue,
		v1alpha1.PeerIdentityReasonPeerFound, "Peer successfully bound")

	// 处理 previousPeerRef（宽限期）
	if peerIdentity.Spec.PreviousPeerRef != "" {
		previousIP, err := r.resolvePeerRef(ctx, peerIdentity.Spec.PreviousPeerRef, peerIdentity.Spec.Network)
		if err != nil {
			log.V(1).Info("Previous peer not found, ignoring", "previousPeerRef", peerIdentity.Spec.PreviousPeerRef)
			peerIdentity.Status.PreviousPeerIP = ""
		} else {
			peerIdentity.Status.PreviousPeerIP = previousIP

			// 设置宽限期过期时间
			if peerIdentity.Status.GracePeriodExpiresAt == nil {
				gracePeriod := time.Duration(peerIdentity.Spec.GracePeriodSeconds) * time.Second
				expiresAt := metav1.NewTime(time.Now().Add(gracePeriod))
				peerIdentity.Status.GracePeriodExpiresAt = &expiresAt
				log.Info("Grace period started", "expiresAt", expiresAt.Time)
			}

			r.setCondition(peerIdentity, v1alpha1.PeerIdentityConditionGracePeriodActive, metav1.ConditionTrue,
				v1alpha1.PeerIdentityReasonGraceActive, "Grace period active")
		}
	} else {
		peerIdentity.Status.PreviousPeerIP = ""
		peerIdentity.Status.GracePeriodExpiresAt = nil
		r.setCondition(peerIdentity, v1alpha1.PeerIdentityConditionGracePeriodActive, metav1.ConditionFalse,
			v1alpha1.PeerIdentityReasonNoGracePeriod, "No grace period configured")
	}

	// 更新状态
	if err := r.Status().Update(ctx, peerIdentity); err != nil {
		log.Error(err, "Failed to update PeerIdentity status")
		return ctrl.Result{}, err
	}

	// 回写 LatticePeer.status.identityRef
	if err := r.updatePeerIdentityRef(ctx, peerIdentity.Spec.PeerRef, peerIdentity.Spec.Network, peerIdentity.Name); err != nil {
		log.Error(err, "Failed to update LatticePeer identityRef")
		// 不返回错误，因为这不是关键路径
	}

	// 如果宽限期已过期，触发重新 reconcile 以清空 previousPeerRef
	if peerIdentity.Status.GracePeriodExpiresAt != nil {
		if time.Now().After(peerIdentity.Status.GracePeriodExpiresAt.Time) {
			log.Info("Grace period expired, clearing previousPeerRef")
			peerIdentity.Spec.PreviousPeerRef = ""
			peerIdentity.Status.PreviousPeerIP = ""
			peerIdentity.Status.GracePeriodExpiresAt = nil
			if err := r.Update(ctx, peerIdentity); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}

		// 安排在宽限期过期时重新 reconcile
		return ctrl.Result{RequeueAfter: time.Until(peerIdentity.Status.GracePeriodExpiresAt.Time)}, nil
	}

	return ctrl.Result{}, nil
}

// resolvePeerRef 解析 PeerRef 到 overlay IP
func (r *PeerIdentityReconciler) resolvePeerRef(ctx context.Context, peerRef, network string) (string, error) {
	if peerRef == "" {
		return "", fmt.Errorf("peerRef is empty")
	}

	// 查找 LatticePeer
	peer := &v1alpha1.LatticePeer{}
	key := types.NamespacedName{
		Name: peerRef,
	}

	if err := r.Get(ctx, key, peer); err != nil {
		if apierrors.IsNotFound(err) {
			return "", fmt.Errorf("LatticePeer %q not found", peerRef)
		}
		return "", err
	}

	// 检查是否属于同一网络
	if peer.Spec.Network != nil && *peer.Spec.Network != network {
		return "", fmt.Errorf("LatticePeer %q belongs to network %q, expected %q", peerRef, *peer.Spec.Network, network)
	}

	// 获取分配的 IP
	if peer.Status.AllocatedAddress == nil || *peer.Status.AllocatedAddress == "" {
		return "", fmt.Errorf("LatticePeer %q has no allocated address", peerRef)
	}

	return *peer.Status.AllocatedAddress, nil
}

// updatePeerIdentityRef 回写 LatticePeer.status.identityRef
func (r *PeerIdentityReconciler) updatePeerIdentityRef(ctx context.Context, peerRef, network, identityName string) error {
	peer := &v1alpha1.LatticePeer{}
	key := types.NamespacedName{
		Name: peerRef,
	}

	if err := r.Get(ctx, key, peer); err != nil {
		return err
	}

	// 只在 identityRef 变化时更新
	if peer.Status.IdentityRef == identityName {
		return nil
	}

	peer.Status.IdentityRef = identityName
	return r.Status().Update(ctx, peer)
}

// setCondition 设置或更新条件
func (r *PeerIdentityReconciler) setCondition(
	peerIdentity *v1alpha1.PeerIdentity,
	condType string,
	status metav1.ConditionStatus,
	reason, message string,
) {
	now := metav1.Now()
	condition := metav1.Condition{
		Type:               condType,
		Status:             status,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
	}

	// 查找现有条件
	for i, c := range peerIdentity.Status.Conditions {
		if c.Type == condType {
			if c.Status != status {
				peerIdentity.Status.Conditions[i] = condition
			} else {
				// 状态未变，只更新时间
				peerIdentity.Status.Conditions[i].LastTransitionTime = now
				peerIdentity.Status.Conditions[i].Reason = reason
				peerIdentity.Status.Conditions[i].Message = message
			}
			return
		}
	}

	// 新条件
	peerIdentity.Status.Conditions = append(peerIdentity.Status.Conditions, condition)
}

// SetupWithManager 设置控制器
func (r *PeerIdentityReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.PeerIdentity{}).
		Complete(r)
}
