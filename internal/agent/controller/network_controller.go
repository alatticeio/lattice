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
	"github.com/alatticeio/lattice/internal/agent/ipam"
	"reflect"
	"strings"
	"time"

	"github.com/alatticeio/lattice/api/v1alpha1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// NetworkReconciler reconciles a LatticeNetwork object
type NetworkReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	IPAM *ipam.IPAM
}

// +kubebuilder:rbac:groups=alattice.io,resources=latticenetworks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=alattice.io,resources=latticenetworks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=alattice.io,resources=latticenetworks/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the LatticeNetwork object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
func (r *NetworkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var (
		network v1alpha1.LatticeNetwork
		err     error
		updated bool
		cidr    string
	)

	log := logf.FromContext(ctx)
	log.Info("Reconciling LatticeNetwork", "namespace", req.NamespacedName, "name", req.Name)

	if err = r.Get(ctx, req.NamespacedName, &network); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get LatticeNetwork")
		return ctrl.Result{}, err
	}

	// Update phase to Creating
	if network.Status.Phase == "" {
		if _, err = r.updateStatus(ctx, &network, func(network *v1alpha1.LatticeNetwork) error {
			network.Status.Phase = v1alpha1.NetworkPhaseCreating
			return nil
		}); err != nil {
			return ctrl.Result{}, err
		}
		// Do not return early: because GenerationChangedPredicate is used, status-only changes will not trigger a new reconcile,
		// so ActiveCIDR allocation must be completed in the same reconcile.
	}

	if network.Status.ActiveCIDR == "" {
		//get subnet
		var pool v1alpha1.LatticeGlobalIPPool
		poolKey := client.ObjectKey{Name: "lattice-ip-pool"}
		if err = r.Get(ctx, poolKey, &pool); err != nil {
			if errors.IsNotFound(err) {
				log.Info("lattice-ip-pool not found, requeuing")
				return ctrl.Result{RequeueAfter: time.Second * 10}, nil
			}
			return ctrl.Result{}, err
		}

		cidr, err = r.IPAM.AllocateSubnet(ctx, network.Name, &pool)
		if err != nil {
			log.Error(err, "Failed to allocate subnet from lattice-ip-pool")
			return ctrl.Result{RequeueAfter: time.Second * 10}, err
		}

		// Update status
		updated, err = r.updateStatus(ctx, &network, func(network *v1alpha1.LatticeNetwork) error {
			network.Status.ActiveCIDR = cidr
			network.Status.Phase = v1alpha1.NetworkPhaseReady
			return nil
		})

		if err != nil {
			log.Error(err, "Failed to update LatticeNetwork status")
			return ctrl.Result{}, err
		}

		if updated {
			return ctrl.Result{}, nil
		}
	}

	//get all latticepeer, one peer one endpoint
	var peers v1alpha1.LatticePeerList
	peers, err = r.findNodesByLabels(ctx, &network)
	if err != nil {
		return ctrl.Result{}, err
	}

	count := len(peers.Items)
	if network.Status.AllocatedCount != count {
		_, err = r.updateStatus(ctx, &network, func(network *v1alpha1.LatticeNetwork) error {
			network.Status.AllocatedCount = count
			return nil
		})
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

//func (r *NetworkReconciler) generateNodesMap(ctx context.Context, nodeList *v1alpha1.LatticePeerList) map[string]struct{} {
//	currentNodes := make(map[string]struct{})
//	for _, node := range nodeList.Items {
//		currentNodes[node.Name] = struct{}{}
//	}
//	return currentNodes
//}

// reconcileSpec checks and corrects LatticeNetwork.Spec fields.
// If the Spec was modified and written successfully, returns (true, nil); the caller should exit Reconcile immediately.
// Otherwise returns (false, nil) or (false, error).
// nolint:all
func (r *NetworkReconciler) updateSpec(ctx context.Context, network *v1alpha1.LatticeNetwork, updateFunc func(node *v1alpha1.LatticeNetwork)) (bool, error) {
	log := logf.FromContext(ctx)
	networkCopy := network.DeepCopy()

	// Apply the update function to the network copy
	updateFunc(networkCopy)

	// Use Patch to send the diff. client.MergeFrom automatically checks differences between networkCopy and node.
	if err := r.Patch(ctx, networkCopy, client.MergeFrom(network)); err != nil {
		if errors.IsConflict(err) {
			// Encountered a conflict (409), do not return error, let Manager retry via a new event.
			log.Info("Conflict detected during LatticeNetwork Spec patch, will retry on next reconcile.")
			return false, nil
		}
		// Other write errors (e.g., insufficient permissions)
		log.Error(err, "Failed to patch LatticeNetwork Spec")
		return false, err
	}

	// If the original and current resources have no differences in Metadata/Spec/Annotation, the Patch is a no-op.
	// Note: the simplest way to determine whether Patch performed a write is to compare the original and current Labels/Annotations/Spec fields.
	if !reflect.DeepEqual(networkCopy.Spec, network.Spec) ||
		!reflect.DeepEqual(networkCopy.Labels, network.Labels) ||
		!reflect.DeepEqual(networkCopy.Annotations, network.Annotations) {

		log.Info("LatticeNetwork Metadata/Spec successfully patched. Returning to trigger next reconcile.")
		// Spec or Metadata was modified and written to API Server successfully
		return true, nil
	}

	// Spec was not modified
	return false, nil
}

func (r *NetworkReconciler) updateStatus(ctx context.Context, network *v1alpha1.LatticeNetwork, updateFunc func(network *v1alpha1.LatticeNetwork) error) (bool, error) {
	log := logf.FromContext(ctx)
	networkCopy := network.DeepCopy()
	if err := updateFunc(networkCopy); err != nil {
		return false, err
	}

	// Use Patch to send the diff. client.MergeFrom automatically checks differences between nodeCopy and node.
	if err := r.Status().Patch(ctx, networkCopy, client.MergeFrom(network)); err != nil {
		if errors.IsConflict(err) {
			// Encountered a conflict (409), do not return error, let Manager retry via a new event.
			log.Info("Conflict detected during LatticeNetwork Spec patch, will retry on next reconcile.")
			return false, nil
		}
		// Other write errors (e.g., insufficient permissions)
		log.Error(err, "Failed to patch LatticeNetwork Spec")
		return false, err
	}

	if !reflect.DeepEqual(networkCopy.Status, network.Status) {

		log.Info("LatticeNetwork Metadata/Spec successfully patched. Returning to trigger next reconcile.")
		// Spec or Metadata was modified and written to API Server successfully
		return true, nil
	}

	// Spec was not modified
	return false, nil
}

// Query all nodes, then update the Network's Spec
func (r *NetworkReconciler) findNodesByLabels(ctx context.Context, network *v1alpha1.LatticeNetwork) (v1alpha1.LatticePeerList, error) {
	labels := fmt.Sprintf("alattice.io/network-%s", network.Name)
	var nodes v1alpha1.LatticePeerList
	if err := r.List(ctx, &nodes, client.InNamespace(network.Namespace), client.MatchingLabels(map[string]string{labels: "true"})); err != nil {
		return nodes, err
	}
	return nodes, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *NetworkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Full informer load at startup generates Create events; filter them out with onlyUpdatePredicate
	// to avoid triggering Network reconcile for every Peer Create during cold start.
	// Also use GenerationChangedPredicate to ensure only real Peer spec changes (join/leave network) trigger,
	// filtering out status-only updates like Status.CurrentHash.
	peerChangedPredicate := predicate.And(
		predicate.Funcs{CreateFunc: func(e event.CreateEvent) bool { return false }},
		predicate.GenerationChangedPredicate{},
	)
	return ctrl.NewControllerManagedBy(mgr).
		// GenerationChangedPredicate filters status subresource updates, preventing NetworkReconciler from being re-triggered by its own status patch
		For(&v1alpha1.LatticeNetwork{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&v1alpha1.LatticePeer{}, handler.EnqueueRequestsFromMapFunc(r.mapNodeForNetworks),
			builder.WithPredicates(peerChangedPredicate)).
		Named("network").
		Complete(r)
}

// nolint:all
func (r *NetworkReconciler) mapNodeForNetworks(ctx context.Context, obj client.Object) []reconcile.Request {
	node := obj.(*v1alpha1.LatticePeer)

	var networkToUpdate []string
	//// 1. Get the node's spec which includes the network
	if node.Spec.Network != nil {
		networkToUpdate = append(networkToUpdate, *node.Spec.Network)
	}
	// Also get from node's labels
	labels := node.GetLabels()
	for key, value := range labels {
		if strings.HasPrefix(key, "alattice.io/network-") && value == "true" {
			networkName, b := strings.CutPrefix(key, "alattice.io/network-")
			if !b {
				continue
			}
			networkToUpdate = append(networkToUpdate, networkName)
		}
	}

	var requests []reconcile.Request
	for _, networkName := range networkToUpdate {
		// 2. Return a Reconcile Request for each LatticeNetwork
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: node.Namespace,
				Name:      networkName, // LatticeNetwork resources are cluster-scoped (non-namespaced)
			},
		})
	}
	return requests
}

//
//// allocateIPsForNode allocates IPs for a node in its network
//func (r *NetworkReconciler) allocateIPsForNode(ctx context.Context, node *v1alpha1.LatticePeer) (string, error) {
//	log := logf.FromContext(ctx)
//	var err error
//	primaryNetwork := node.Spec.Network
//
//	var network v1alpha1.LatticeNetwork
//	if primaryNetwork != nil {
//		// Get the LatticeNetwork resource
//		if err = r.Get(ctx, types.NamespacedName{Name: fmt.Sprintf("%s/%s", node.Namespace, *primaryNetwork)}, &network); err != nil {
//			return "", err
//		}
//	}
//
//	// If the node already has an IP address, skip
//	currentAddress := node.Status.AllocatedAddress
//	if currentAddress != nil {
//		// validate if the IP is a valid network IP
//		if err = r.Allocator.ValidateIP(network.Spec.CIDR, *currentAddress); err == nil {
//			log.Info("LatticePeer already has IP address", "address", currentAddress)
//			return *currentAddress, nil
//		}
//	}
//
//	// Check if the node already has an IP allocation in this network
//	existingIP := r.Allocator.GetNodeIP(&network, node.Name)
//	if existingIP != "" {
//		// validate if the IP is a valid network IP
//		klog.Infof("LatticePeer %s already has IP %s in network %s", node.Name, existingIP, network.Name)
//		return existingIP, nil
//	}
//
//	// Allocate a new IP
//	return r.allocate(ctx, &network, node)
//}
//
//func (r *NetworkReconciler) allocate(ctx context.Context, network *v1alpha1.LatticeNetwork, node *v1alpha1.LatticePeer) (string, error) {
//	log := logf.FromContext(ctx)
//	var (
//		err         error
//		allocatedIP string
//	)
//	allocatedIP, err = r.Allocator.AllocateIP(network, node.Name)
//	if err != nil {
//		return "", fmt.Errorf("failed to allocate IP: %v", err)
//	}
//
//	log.Info("Allocated IP", "ip", allocatedIP, "nodeName", node.Name)
//
//	return allocatedIP, nil
//}
//
//// updateNetworkIPAllocation updates the network's IP allocation records
//func (r *NetworkReconciler) updateNetworkIPAllocation(ctx context.Context, network *v1alpha1.LatticeNetwork, ip, nodeName string) error {
//
//	allocations := make(map[string]v1alpha1.IPAllocation)
//	for _, allocation := range network.Status.AllocatedIPs {
//		allocations[allocation.Peer] = allocation
//	}
//
//	if _, ok := allocations[nodeName]; ok {
//		return nil
//	}
//	// Add IP allocation record
//	allocation := v1alpha1.IPAllocation{
//		IP:          ip,
//		Peer:        nodeName,
//		AllocatedAt: metav1.Now(),
//	}
//
//	network.Status.AllocatedIPs = append(network.Status.AllocatedIPs, allocation)
//
//	// Update available IP count
//	availableIPs, err := r.Allocator.CountAvailableIPs(network)
//	if err != nil {
//		klog.Errorf("Failed to count available IPs: %v", err)
//	} else {
//		network.Status.AvailableIPs = availableIPs
//	}
//
//	return nil
//}
