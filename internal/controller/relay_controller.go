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
	"time"
	"wireflow/api/v1alpha1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// RelayReconciler reconciles WireflowRelayServer objects.
// On every change it propagates the relay's TcpUrl / QuicUrl into the Spec of
// every WireflowPeer that belongs to one of the relay's target namespaces
// (or all namespaces when Namespaces is empty).  It also maintains a
// per-peer label so cleanup on deletion is fast.
type RelayReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=wireflowcontroller.wireflow.run,resources=wireflowrelayservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=wireflowcontroller.wireflow.run,resources=wireflowrelayservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=wireflowcontroller.wireflow.run,resources=wireflowrelayservers/finalizers,verbs=update
// +kubebuilder:rbac:groups=wireflowcontroller.wireflow.run,resources=wireflowpeers,verbs=get;list;watch;update;patch

func (r *RelayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx).WithValues("relay", req.Name)

	var relay v1alpha1.WireflowRelayServer
	if err := r.Get(ctx, req.NamespacedName, &relay); err != nil {
		if errors.IsNotFound(err) {
			// Already deleted; peers were cleared by the finalizer path below.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// ── deletion path ────────────────────────────────────────────────────────
	if !relay.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&relay, v1alpha1.RelayFinalizer) {
			log.Info("relay deleted — clearing peer relay config")
			if err := r.clearPeers(ctx, relay.Name); err != nil {
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(&relay, v1alpha1.RelayFinalizer)
			if err := r.Update(ctx, &relay); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// ── ensure finalizer ─────────────────────────────────────────────────────
	if !controllerutil.ContainsFinalizer(&relay, v1alpha1.RelayFinalizer) {
		controllerutil.AddFinalizer(&relay, v1alpha1.RelayFinalizer)
		if err := r.Update(ctx, &relay); err != nil {
			return ctrl.Result{}, err
		}
	}

	// ── propagate to peers ───────────────────────────────────────────────────
	connected, err := r.syncPeers(ctx, &relay)
	if err != nil {
		return ctrl.Result{}, err
	}

	// ── update status ────────────────────────────────────────────────────────
	now := metav1.Now()
	patch := relay.DeepCopy()
	patch.Status.ConnectedPeers = connected
	if patch.Status.Health == "" {
		patch.Status.Health = v1alpha1.RelayHealthUnknown
	}
	if relay.Spec.Enabled {
		patch.Status.Phase = v1alpha1.RelayPhaseActive
	} else {
		patch.Status.Phase = v1alpha1.RelayPhaseDisabled
	}
	patch.Status.LastProbeTime = &now

	if err = r.Status().Patch(ctx, patch, client.MergeFrom(&relay)); err != nil {
		log.Error(err, "failed to patch relay status")
	}

	// Re-sync every 5 minutes to keep connectedPeers count accurate.
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// syncPeers iterates all WireflowPeers in matching namespaces and patches their
// WrrpUrl / WrrpQuicUrl to reflect the relay's current spec.
// Returns the number of peers that were (or remain) configured to use this relay.
func (r *RelayReconciler) syncPeers(ctx context.Context, relay *v1alpha1.WireflowRelayServer) (int, error) {
	log := logf.FromContext(ctx).WithValues("relay", relay.Name)

	var peerList v1alpha1.WireflowPeerList
	if err := r.List(ctx, &peerList); err != nil {
		return 0, err
	}

	wrrpUrl, wrrpQuicUrl := "", ""
	if relay.Spec.Enabled {
		wrrpUrl = relay.Spec.TcpUrl
		wrrpQuicUrl = relay.Spec.QuicUrl
	}

	connected := 0
	for i := range peerList.Items {
		peer := &peerList.Items[i]
		if !r.peerInScope(peer.Namespace, relay.Spec.Namespaces) {
			continue
		}

		needsPatch := peer.Spec.WrrpUrl != wrrpUrl || peer.Spec.WrrpQuicUrl != wrrpQuicUrl
		labelVal, hasLabel := peer.Labels[v1alpha1.RelayPeerLabel]
		needsLabel := !hasLabel || labelVal != relay.Name

		if !needsPatch && !needsLabel {
			connected++
			continue
		}

		peerCopy := peer.DeepCopy()
		peerCopy.Spec.WrrpUrl = wrrpUrl
		peerCopy.Spec.WrrpQuicUrl = wrrpQuicUrl

		if peerCopy.Labels == nil {
			peerCopy.Labels = make(map[string]string)
		}
		peerCopy.Labels[v1alpha1.RelayPeerLabel] = relay.Name

		if err := r.Patch(ctx, peerCopy, client.MergeFrom(peer),
			client.FieldOwner("relay-reconciler")); err != nil {
			log.Error(err, "failed to patch peer", "peer", peer.Name, "namespace", peer.Namespace)
			continue
		}
		connected++
	}
	return connected, nil
}

// clearPeers removes relay-related fields from all peers that carry this
// relay's label, then removes the label itself.
func (r *RelayReconciler) clearPeers(ctx context.Context, relayName string) error {
	log := logf.FromContext(ctx).WithValues("relay", relayName)

	selector := labels.SelectorFromSet(labels.Set{v1alpha1.RelayPeerLabel: relayName})
	var peerList v1alpha1.WireflowPeerList
	if err := r.List(ctx, &peerList, &client.ListOptions{LabelSelector: selector}); err != nil {
		return err
	}

	for i := range peerList.Items {
		peer := &peerList.Items[i]
		peerCopy := peer.DeepCopy()
		peerCopy.Spec.WrrpUrl = ""
		peerCopy.Spec.WrrpQuicUrl = ""
		delete(peerCopy.Labels, v1alpha1.RelayPeerLabel)

		if err := r.Patch(ctx, peerCopy, client.MergeFrom(peer),
			client.FieldOwner("relay-reconciler")); err != nil {
			log.Error(err, "failed to clear peer relay config", "peer", peer.Name)
		}
	}
	return nil
}

// peerInScope returns true when peer.Namespace is in targetNs,
// or when targetNs is empty (meaning "all namespaces").
func (r *RelayReconciler) peerInScope(ns string, targetNs []string) bool {
	if len(targetNs) == 0 {
		return true
	}
	for _, n := range targetNs {
		if n == ns {
			return true
		}
	}
	return false
}

// SetupWithManager registers the reconciler with the controller-runtime manager.
func (r *RelayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.WireflowRelayServer{}).
		Named("wireflow-relay").
		Complete(r)
}
