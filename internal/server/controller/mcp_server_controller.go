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
	"strings"
	"time"

	"github.com/alatticeio/lattice/api/v1alpha1"
	"github.com/alatticeio/lattice/internal/agent/log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// MCPServerReconciler reconciles MCPServer status from the underlying LatticePeer.
type MCPServerReconciler struct {
	client client.Client
	logger *log.Logger
}

// NewMCPServerReconciler creates a reconciler.
func NewMCPServerReconciler(c client.Client) *MCPServerReconciler {
	return &MCPServerReconciler{client: c, logger: log.GetLogger("mcp-server")}
}

func (r *MCPServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.client = mgr.GetClient()
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.MCPServer{}).
		Complete(r)
}

func (r *MCPServerReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var mcpSrv v1alpha1.MCPServer
	if err := r.client.Get(ctx, req.NamespacedName, &mcpSrv); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	patch := client.MergeFrom(mcpSrv.DeepCopy())
	now := metav1.NewTime(time.Now())
	mcpSrv.Status.LastSyncedAt = &now

	if mcpSrv.Spec.PeerName == "" {
		// External mode: no LatticePeer dependency; always Ready.
		mcpSrv.Status.Mode = v1alpha1.MCPServerModeExternal
		mcpSrv.Status.Phase = v1alpha1.MCPServerPhaseReady
		mcpSrv.Status.PeerAddress = ""
	} else {
		// Internal mode: look up the LatticePeer.
		mcpSrv.Status.Mode = v1alpha1.MCPServerModeInternal
		var peer v1alpha1.LatticePeer
		err := r.client.Get(ctx, client.ObjectKey{
			Namespace: req.Namespace,
			Name:      mcpSrv.Spec.PeerName,
		}, &peer)
		if err != nil {
			if apierrors.IsNotFound(err) {
				mcpSrv.Status.Phase = v1alpha1.MCPServerPhasePending
				mcpSrv.Status.PeerAddress = ""
			} else {
				return reconcile.Result{}, err
			}
		} else if peer.Status.Phase == "Ready" && peer.Status.AllocatedAddress != nil {
			mcpSrv.Status.Phase = v1alpha1.MCPServerPhaseReady
			// Strip CIDR suffix if present (e.g. "10.0.7.5/32" → "10.0.7.5")
			addr := *peer.Status.AllocatedAddress
			if idx := strings.Index(addr, "/"); idx != -1 {
				addr = addr[:idx]
			}
			mcpSrv.Status.PeerAddress = addr
		} else {
			mcpSrv.Status.Phase = v1alpha1.MCPServerPhaseDegraded
			mcpSrv.Status.PeerAddress = ""
		}
	}

	if err := r.client.Status().Patch(ctx, &mcpSrv, patch); err != nil {
		return reconcile.Result{}, err
	}
	r.logger.Info("MCPServer reconciled", "name", mcpSrv.Name, "namespace", mcpSrv.Namespace,
		"phase", mcpSrv.Status.Phase, "mode", mcpSrv.Status.Mode)
	return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
}
