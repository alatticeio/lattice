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
	"time"

	"github.com/alatticeio/lattice/api/v1alpha1"
	"github.com/alatticeio/lattice/internal/agent/log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// PolicyTTLReconciler deletes LatticePolicy resources when spec.expiresAt passes.
type PolicyTTLReconciler struct {
	client client.Client
	logger *log.Logger
}

// NewPolicyTTLReconciler creates a reconciler using the provided client directly.
// Useful for testing without a manager.
func NewPolicyTTLReconciler(c client.Client) *PolicyTTLReconciler {
	return &PolicyTTLReconciler{client: c, logger: log.GetLogger("policy-ttl")}
}

func (r *PolicyTTLReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var policy v1alpha1.LatticePolicy
	if err := r.client.Get(ctx, req.NamespacedName, &policy); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if policy.Spec.ExpiresAt == nil {
		return reconcile.Result{}, nil // permanent policy, nothing to do
	}

	now := time.Now()
	expiresAt := policy.Spec.ExpiresAt.Time

	if now.After(expiresAt) {
		r.logger.Info("deleting expired policy", "name", policy.Name, "namespace", policy.Namespace, "expiredAt", expiresAt)
		if err := r.client.Delete(ctx, &policy); err != nil && !apierrors.IsNotFound(err) {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	// Requeue just after the expiry time.
	requeueAfter := expiresAt.Sub(now) + time.Second
	return reconcile.Result{RequeueAfter: requeueAfter}, nil
}

// SetupWithManager registers the reconciler with a controller-runtime manager.
func (r *PolicyTTLReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.client = mgr.GetClient()
	r.logger = log.GetLogger("policy-ttl")
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.LatticePolicy{}).
		WithEventFilter(predicate.NewPredicateFuncs(func(obj client.Object) bool {
			p, ok := obj.(*v1alpha1.LatticePolicy)
			if !ok {
				return false
			}
			return p.Spec.ExpiresAt != nil
		})).
		Complete(r)
}
