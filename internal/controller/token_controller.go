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
	"reflect"
	"time"
	"wireflow/api/v1alpha1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// TokenReconciler reconciles a WireflowNetwork object
type TokenReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=wireflowcontroller.wireflow.run,resources=wireflowenrollmenttokens,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=wireflowcontroller.wireflow.run,resources=wireflowenrollmenttokens/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=wireflowcontroller.wireflow.run,resources=wireflowenrollmenttokens/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the WireflowNetwork object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
func (r *TokenReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var (
		err   error
		token v1alpha1.WireflowEnrollmentToken
	)
	log := logf.FromContext(ctx)
	log.Info("Reconciling Token", "namespace", req.NamespacedName, "name", req.Name)

	if err = r.Get(ctx, req.NamespacedName, &token); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get WireflowEnrollmentToken")
		return ctrl.Result{}, err
	}

	if token.Status.Phase == "Expired" {
		return ctrl.Result{}, nil
	}

	// 2. 计算过期剩余时间
	remaining := time.Until(token.Spec.Expiry.Time)

	if remaining <= 0 {
		ok, err := r.updateStatus(ctx, &token, func(token *v1alpha1.WireflowEnrollmentToken) error {
			token.Status.Phase = "Expired"
			token.Status.IsExpired = true
			return nil
		})

		if err != nil {
			return ctrl.Result{}, err
		}

		if ok {
			return ctrl.Result{}, nil
		}
	}

	// process token
	if token.Status.Token == "" {
		ok, err := r.updateStatus(ctx, &token, func(token *v1alpha1.WireflowEnrollmentToken) error {
			token.Status.Token = token.Spec.Token
			return nil
		})

		if err != nil {
			return ctrl.Result{}, err
		}

		if ok {
			return ctrl.Result{}, nil
		}
	}

	// process bound peers & UsedCount
	if !reflect.DeepEqual(token.Spec.BoundPeers, token.Status.BoundPeers) {
		statusPeers := stringSet(token.Status.BoundPeers)
		ok, err := r.updateStatus(ctx, &token, func(token *v1alpha1.WireflowEnrollmentToken) error {
			for _, peer := range token.Spec.BoundPeers {
				if _, ok := statusPeers[peer]; !ok {
					token.Status.BoundPeers = append(token.Status.BoundPeers, peer)
					token.Status.UsedCount++
				}
			}
			return nil
		})

		if err != nil {
			return ctrl.Result{}, err
		}

		if ok {
			return ctrl.Result{}, nil
		}
	}

	// 4. 时间没到，设置定时器，到期后 K8s 会自动再次触发这个 Reconcile
	return ctrl.Result{RequeueAfter: remaining}, nil

}

func (r *TokenReconciler) updateStatus(ctx context.Context, token *v1alpha1.WireflowEnrollmentToken, updateFunc func(token *v1alpha1.WireflowEnrollmentToken) error) (bool, error) {
	log := logf.FromContext(ctx)

	// 1. 深度拷贝原始对象，避免副作用
	tokenCopy := token.DeepCopy()

	// 2. 执行业务逻辑修改
	if err := updateFunc(tokenCopy); err != nil {
		return false, err
	}

	// 3. 检查是否真的有变化，如果没有变化则无需请求 API Server
	if reflect.DeepEqual(token.Status, tokenCopy.Status) {
		return false, nil
	}

	// 4. 执行 Status Patch (使用 MergeFrom 模式)
	// client.MergeFrom 会计算 token 与 tokenCopy 之间的 diff 仅发送增量
	if err := r.Status().Patch(ctx, tokenCopy, client.MergeFrom(token)); err != nil {
		if errors.IsConflict(err) {
			log.Info("Conflict detected during status patch, will retry on next reconcile.")
			return false, nil // 冲突时让 Controller 重新 Reconcile 即可
		}
		log.Error(err, "Failed to patch WireflowEnrollmentToken status")
		return false, err
	}

	log.Info("WireflowEnrollmentToken status successfully updated.")
	return true, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TokenReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&v1alpha1.WireflowEnrollmentToken{}).Named("token").Complete(r)
}
