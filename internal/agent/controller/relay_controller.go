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

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// RelayReconciler reconciles LatticeRelayServer objects.
// On every change it propagates the relay's TcpUrl / QuicUrl into the Spec of
// every LatticePeer that belongs to one of the relay's target namespaces
// (or all namespaces when Namespaces is empty).  It also maintains a
// per-peer label so cleanup on deletion is fast.
type RelayReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=alattice.io,resources=latticerelayservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=alattice.io,resources=latticerelayservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=alattice.io,resources=latticerelayservers/finalizers,verbs=update
// +kubebuilder:rbac:groups=alattice.io,resources=latticepeers,verbs=get;list;watch;update;patch

func (r *RelayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx).WithValues("relay", req.Name)

	var relay v1alpha1.LatticeRelayServer
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
		return ctrl.Result{Requeue: true}, nil
	}

	// ── count peers associated via label ─────────────────────────────────────
	connected, err := r.countConnectedPeers(ctx, &relay)
	if err != nil {
		return ctrl.Result{}, err
	}

	// ── update status only when something actually changed ────────────────────
	desiredPhase := v1alpha1.RelayPhaseActive
	if !relay.Spec.Enabled {
		desiredPhase = v1alpha1.RelayPhaseDisabled
	}
	desiredHealth := relay.Status.Health
	if desiredHealth == "" {
		desiredHealth = v1alpha1.RelayHealthUnknown
	}

	if relay.Status.ConnectedPeers != connected ||
		relay.Status.Phase != desiredPhase ||
		relay.Status.Health != desiredHealth {

		patch := relay.DeepCopy()
		patch.Status.ConnectedPeers = connected
		patch.Status.Phase = desiredPhase
		patch.Status.Health = desiredHealth
		now := metav1.Now()
		patch.Status.LastProbeTime = &now

		if err = r.Status().Patch(ctx, patch, client.MergeFrom(&relay)); err != nil {
			log.Error(err, "failed to patch relay status")
		}
	}

	// Re-sync periodically to keep connectedPeers count accurate.
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// countConnectedPeers counts the LatticePeers that are associated with this
// relay via the RelayPeerLabel. Scope is limited to relay.Spec.Namespaces when
// set, or all namespaces when empty.
func (r *RelayReconciler) countConnectedPeers(ctx context.Context, relay *v1alpha1.LatticeRelayServer) (int, error) {
	selector := labels.SelectorFromSet(labels.Set{v1alpha1.RelayPeerLabel: relay.Name})
	listOpts := []client.ListOption{client.MatchingLabelsSelector{Selector: selector}}

	if len(relay.Spec.Namespaces) == 1 {
		listOpts = append(listOpts, client.InNamespace(relay.Spec.Namespaces[0]))
	}

	var peerList v1alpha1.LatticePeerList
	if err := r.List(ctx, &peerList, listOpts...); err != nil {
		return 0, err
	}

	if len(relay.Spec.Namespaces) > 1 {
		nsSet := make(map[string]struct{}, len(relay.Spec.Namespaces))
		for _, ns := range relay.Spec.Namespaces {
			nsSet[ns] = struct{}{}
		}
		count := 0
		for _, p := range peerList.Items {
			if _, ok := nsSet[p.Namespace]; ok {
				count++
			}
		}
		return count, nil
	}

	return len(peerList.Items), nil
}

// clearPeers removes the RelayPeerLabel from all peers that reference this
// relay, so stale associations are cleaned up when the relay is deleted.
func (r *RelayReconciler) clearPeers(ctx context.Context, relayName string) error {
	log := logf.FromContext(ctx).WithValues("relay", relayName)

	selector := labels.SelectorFromSet(labels.Set{v1alpha1.RelayPeerLabel: relayName})
	var peerList v1alpha1.LatticePeerList
	if err := r.List(ctx, &peerList, &client.ListOptions{LabelSelector: selector}); err != nil {
		return err
	}

	for i := range peerList.Items {
		peer := &peerList.Items[i]
		peerCopy := peer.DeepCopy()
		delete(peerCopy.Labels, v1alpha1.RelayPeerLabel)
		if err := r.Patch(ctx, peerCopy, client.MergeFrom(peer),
			client.FieldOwner("relay-reconciler")); err != nil {
			log.Error(err, "failed to remove relay label from peer", "peer", peer.Name)
		}
	}
	return nil
}

// SetupWithManager registers the reconciler with the controller-runtime manager.
func (r *RelayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Only react to LatticeRelayServer spec changes (GenerationChangedPredicate),
	// not to status updates — status patches must not re-trigger reconcile.
	relayLabelChangedPredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			_, has := e.Object.GetLabels()[v1alpha1.RelayPeerLabel]
			return has
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.ObjectOld.GetLabels()[v1alpha1.RelayPeerLabel] !=
				e.ObjectNew.GetLabels()[v1alpha1.RelayPeerLabel]
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			_, has := e.Object.GetLabels()[v1alpha1.RelayPeerLabel]
			return has
		},
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.LatticeRelayServer{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		// Re-count when a peer gains or loses the relay label.
		Watches(&v1alpha1.LatticePeer{},
			handler.EnqueueRequestsFromMapFunc(r.mapPeerToRelay),
			builder.WithPredicates(relayLabelChangedPredicate)).
		Named("lattice-relay").
		Complete(r)
}

// mapPeerToRelay returns a reconcile request for the relay referenced by a
// peer's RelayPeerLabel. Called when the label changes on a LatticePeer.
func (r *RelayReconciler) mapPeerToRelay(ctx context.Context, obj client.Object) []reconcile.Request {
	// Check both old and new label values via the object passed by the watch.
	// EnqueueRequestsFromMapFunc only receives the new object; for delete/update
	// events where the label was just removed we fall back to RequeueAfter.
	relayName := obj.GetLabels()[v1alpha1.RelayPeerLabel]
	if relayName == "" {
		return nil
	}
	return []reconcile.Request{
		{NamespacedName: types.NamespacedName{Name: relayName}},
	}
}
