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

	"github.com/alatticeio/lattice/api/v1alpha1"
	"github.com/alatticeio/lattice/internal/agent/log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const agentExpiresAtAnnotation = "lattice.io/agent-expires-at"

// AgentTTLReconciler deletes LatticePeer resources that have passed their TTL.
type AgentTTLReconciler struct {
	client client.Client
	logger *log.Logger
}

// NewAgentTTLReconciler creates a reconciler that uses the provided client directly.
// Useful for testing without a manager.
func NewAgentTTLReconciler(c client.Client) *AgentTTLReconciler {
	return &AgentTTLReconciler{client: c, logger: log.GetLogger("agent-ttl")}
}

func (r *AgentTTLReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var peer v1alpha1.LatticePeer
	if err := r.client.Get(ctx, req.NamespacedName, &peer); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	expiresStr, ok := peer.Annotations[agentExpiresAtAnnotation]
	if !ok {
		return reconcile.Result{}, nil // no TTL set
	}

	expiresAt, err := time.Parse(time.RFC3339, expiresStr)
	if err != nil {
		r.logger.Warn("invalid expires-at annotation", "peer", peer.Name, "value", expiresStr)
		return reconcile.Result{}, fmt.Errorf("parse expires-at: %w", err)
	}

	now := time.Now()
	if now.After(expiresAt) {
		r.logger.Info("deleting expired agent peer", "peer", peer.Name, "expiredAt", expiresAt)
		if err := r.client.Delete(ctx, &peer); err != nil && !apierrors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("delete peer: %w", err)
		}
		return reconcile.Result{}, nil
	}

	// Requeue just after the expiry time.
	requeueAfter := expiresAt.Sub(now) + time.Second
	return reconcile.Result{RequeueAfter: requeueAfter}, nil
}

// SetupWithManager registers the reconciler with a controller-runtime manager.
func (r *AgentTTLReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.client = mgr.GetClient()
	r.logger = log.GetLogger("agent-ttl")
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.LatticePeer{}).
		WithEventFilter(predicate.NewPredicateFuncs(func(obj client.Object) bool {
			return obj.GetLabels()["lattice.io/agent-managed"] == "true"
		})).
		Complete(r)
}
