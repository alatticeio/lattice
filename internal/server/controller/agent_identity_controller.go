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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// AgentIdentityReconciler transitions AgentIdentity phase to Expired when spec.expiresAt passes.
type AgentIdentityReconciler struct {
	client client.Client
	logger *log.Logger
}

// NewAgentIdentityReconciler creates a reconciler using the provided client directly.
// Useful for testing without a manager.
func NewAgentIdentityReconciler(c client.Client) *AgentIdentityReconciler {
	return &AgentIdentityReconciler{client: c, logger: log.GetLogger("agent-identity")}
}

func (r *AgentIdentityReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var identity v1alpha1.AgentIdentity
	if err := r.client.Get(ctx, req.NamespacedName, &identity); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Already in a terminal phase — nothing to do.
	if identity.Status.Phase == v1alpha1.AgentPhaseExpired ||
		identity.Status.Phase == v1alpha1.AgentPhaseRevoked {
		return reconcile.Result{}, nil
	}

	now := time.Now()

	// Check expiry before activating.
	if identity.Spec.ExpiresAt != nil && now.After(identity.Spec.ExpiresAt.Time) {
		patch := client.MergeFrom(identity.DeepCopy())
		identity.Status.Phase = v1alpha1.AgentPhaseExpired
		if err := r.client.Status().Patch(ctx, &identity, patch); err != nil {
			return reconcile.Result{}, err
		}
		r.logger.Info("agent identity expired", "name", identity.Name, "namespace", identity.Namespace)
		return reconcile.Result{}, nil
	}

	// Transition new / pending identities to Active.
	if identity.Status.Phase == "" || identity.Status.Phase == v1alpha1.AgentPhasePending {
		patch := client.MergeFrom(identity.DeepCopy())
		identity.Status.Phase = v1alpha1.AgentPhaseActive
		if err := r.client.Status().Patch(ctx, &identity, patch); err != nil {
			return reconcile.Result{}, err
		}
		r.logger.Info("agent identity activated", "name", identity.Name, "namespace", identity.Namespace)
	}

	if identity.Spec.ExpiresAt == nil {
		return reconcile.Result{}, nil // no TTL set, nothing more to do
	}

	// Requeue just after the expiry time.
	requeueAfter := identity.Spec.ExpiresAt.Sub(now) + time.Second
	return reconcile.Result{RequeueAfter: requeueAfter}, nil
}

// SetupWithManager registers the reconciler with a controller-runtime manager.
func (r *AgentIdentityReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.client = mgr.GetClient()
	r.logger = log.GetLogger("agent-identity")
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.AgentIdentity{}).
		Complete(r)
}
