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
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// TokenReconciler reconciles a LatticeNetwork object
type TokenReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=alattice.io,resources=latticeenrollmenttokens,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=alattice.io,resources=latticeenrollmenttokens/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=alattice.io,resources=latticeenrollmenttokens/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the LatticeNetwork object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
func (r *TokenReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var (
		err   error
		token v1alpha1.LatticeEnrollmentToken
	)
	log := logf.FromContext(ctx)
	log.Info("Reconciling Token", "namespace", req.NamespacedName, "name", req.Name)

	if err = r.Get(ctx, req.NamespacedName, &token); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get LatticeEnrollmentToken")
		return ctrl.Result{}, err
	}

	if token.Status.Phase == "Expired" {
		return ctrl.Result{}, nil
	}

	// 2. Calculate remaining time until expiry
	remaining := time.Until(token.Spec.Expiry.Time)

	if remaining <= 0 {
		ok, err := r.updateStatus(ctx, &token, func(token *v1alpha1.LatticeEnrollmentToken) error {
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
		ok, err := r.updateStatus(ctx, &token, func(token *v1alpha1.LatticeEnrollmentToken) error {
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
		ok, err := r.updateStatus(ctx, &token, func(token *v1alpha1.LatticeEnrollmentToken) error {
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

	// 4. Time has not elapsed yet, set a timer; K8s will automatically trigger this Reconcile again when it expires
	return ctrl.Result{RequeueAfter: remaining}, nil

}

func (r *TokenReconciler) updateStatus(ctx context.Context, token *v1alpha1.LatticeEnrollmentToken, updateFunc func(token *v1alpha1.LatticeEnrollmentToken) error) (bool, error) {
	log := logf.FromContext(ctx)

	// 1. Deep copy the original object to avoid side effects
	tokenCopy := token.DeepCopy()

	// 2. Execute business logic modifications
	if err := updateFunc(tokenCopy); err != nil {
		return false, err
	}

	// 3. Check whether there are actual changes; if none, no need to request API Server
	if reflect.DeepEqual(token.Status, tokenCopy.Status) {
		return false, nil
	}

	// 4. Execute Status Patch (using MergeFrom mode)
	// client.MergeFrom calculates the diff between token and tokenCopy, sending only the delta
	if err := r.Status().Patch(ctx, tokenCopy, client.MergeFrom(token)); err != nil {
		if errors.IsConflict(err) {
			log.Info("Conflict detected during status patch, will retry on next reconcile.")
			return false, nil // on conflict, let the Controller re-Reconcile
		}
		log.Error(err, "Failed to patch LatticeEnrollmentToken status")
		return false, err
	}

	log.Info("LatticeEnrollmentToken status successfully updated.")
	return true, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TokenReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&v1alpha1.LatticeEnrollmentToken{}).Named("token").Complete(r)
}
