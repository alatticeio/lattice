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

	"github.com/alatticeio/lattice/api/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// NetworkPolicyReconciler reconciles a LatticePolicy object
type NetworkPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=alattice.io,resources=latticepolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=alattice.io,resources=latticepolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=alattice.io,resources=latticepolicies/finalizers,verbs=update
// +kubebuilder:rbac:groups=alattice.io,resources=latticepeers,verbs=get;list;watch

// Reconcile counts the LatticePeers matched by the policy's PeerSelector and
// the total ingress+egress rules, then writes those counts back to the
// LatticePolicy status subresource.
func (r *NetworkPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Reconciling LatticePolicy", "namespace", req.Namespace, "name", req.Name)

	var policy v1alpha1.LatticePolicy
	if err := r.Get(ctx, req.NamespacedName, &policy); err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "Failed to get LatticePolicy")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Count peers matched by PeerSelector.
	selector, err := metav1.LabelSelectorAsSelector(&policy.Spec.PeerSelector)
	if err != nil {
		log.Error(err, "Failed to parse PeerSelector")
		return ctrl.Result{}, err
	}

	var peerList v1alpha1.LatticePeerList
	if err := r.List(ctx, &peerList,
		client.InNamespace(req.Namespace),
		client.MatchingLabelsSelector{Selector: selector},
	); err != nil {
		log.Error(err, "Failed to list LatticePeers")
		return ctrl.Result{}, err
	}

	ruleCount := len(policy.Spec.Ingress) + len(policy.Spec.Egress)

	patch := client.MergeFrom(policy.DeepCopy())
	policy.Status.TargetNodes = len(peerList.Items)
	policy.Status.RuleCount = ruleCount

	if err := r.Status().Patch(ctx, &policy, patch); err != nil {
		log.Error(err, "Failed to update LatticePolicy status")
		return ctrl.Result{}, err
	}

	log.Info("Updated LatticePolicy status",
		"targetNodes", policy.Status.TargetNodes,
		"ruleCount", policy.Status.RuleCount,
	)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NetworkPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.LatticePolicy{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Named("networkpolicy").
		Complete(r)
}
