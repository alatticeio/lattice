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
	"fmt"
	"time"

	"wireflow/api/v1alpha1"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// NetworkPeeringReconciler reconciles WireflowNetworkPeering resources.
//
// For each peering it:
//  1. Annotates the gateway peer in each namespace with a per-peering route so
//     local peers include the remote CIDR in the gateway's AllowedIPs.
//  2. Creates a "shadow peer" in each namespace representing the remote gateway,
//     ensuring WireGuard can establish the inter-gateway tunnel.
//  3. Creates WireflowPolicy objects that admit the gateway and shadow peers
//     into ComputedPeers for the relevant nodes.
type NetworkPeeringReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=wireflowcontroller.wireflow.run,resources=wireflownetworkpeerings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=wireflowcontroller.wireflow.run,resources=wireflownetworkpeerings/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=wireflowcontroller.wireflow.run,resources=wireflownetworkpeerings/finalizers,verbs=update
// +kubebuilder:rbac:groups=wireflowcontroller.wireflow.run,resources=wireflowpeers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=wireflowcontroller.wireflow.run,resources=wireflowpeers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=wireflowcontroller.wireflow.run,resources=wireflowpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=wireflowcontroller.wireflow.run,resources=wireflownetworks,verbs=get;list;watch

func (r *NetworkPeeringReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Reconciling WireflowNetworkPeering", "name", req.Name)

	var peering v1alpha1.WireflowNetworkPeering
	if err := r.Get(ctx, req.NamespacedName, &peering); err != nil {
		if k8serrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !peering.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, &peering)
	}

	// Ensure finalizer
	if !controllerutil.ContainsFinalizer(&peering, PeeringFinalizer) {
		controllerutil.AddFinalizer(&peering, PeeringFinalizer)
		if err := r.Update(ctx, &peering); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	return r.reconcileNormal(ctx, &peering)
}

func (r *NetworkPeeringReconciler) reconcileNormal(ctx context.Context, peering *v1alpha1.WireflowNetworkPeering) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 1. Fetch both networks and verify they are ready.
	networkA, err := r.getReadyNetwork(ctx, peering.Spec.NamespaceA, peering.Spec.NetworkA)
	if err != nil {
		return r.setError(ctx, peering, fmt.Sprintf("network A not ready: %v", err))
	}
	networkB, err := r.getReadyNetwork(ctx, peering.Spec.NamespaceB, peering.Spec.NetworkB)
	if err != nil {
		return r.setError(ctx, peering, fmt.Sprintf("network B not ready: %v", err))
	}

	cidrA := networkA.Status.ActiveCIDR
	cidrB := networkB.Status.ActiveCIDR

	// 2. Find designated gateway peers.
	gatewayA, err := r.findGateway(ctx, peering.Spec.NamespaceA, peering.Spec.NetworkA)
	if err != nil || gatewayA == nil {
		msg := fmt.Sprintf("no gateway peer in %s/%s: label %s=true required", peering.Spec.NamespaceA, peering.Spec.NetworkA, LabelGateway)
		log.Info(msg)
		return r.setError(ctx, peering, msg)
	}
	gatewayB, err := r.findGateway(ctx, peering.Spec.NamespaceB, peering.Spec.NetworkB)
	if err != nil || gatewayB == nil {
		msg := fmt.Sprintf("no gateway peer in %s/%s: label %s=true required", peering.Spec.NamespaceB, peering.Spec.NetworkB, LabelGateway)
		log.Info(msg)
		return r.setError(ctx, peering, msg)
	}

	// 3. Annotate gateway peers with per-peering routes.
	//    Other local peers will route the remote CIDR through this gateway.
	annotationKey := peeringRouteAnnotationKey(peering.Name)
	if err := r.ensureAnnotation(ctx, gatewayA, annotationKey, cidrB); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.ensureAnnotation(ctx, gatewayB, annotationKey, cidrA); err != nil {
		return ctrl.Result{}, err
	}

	// 4. Create/update shadow peer of GatewayA in NamespaceB.
	//    GatewayB will connect to this shadow to establish the inter-gateway tunnel.
	if err := r.ensureShadowPeer(ctx, peering, gatewayA, peering.Spec.NamespaceB, networkB.Name, cidrA); err != nil {
		return ctrl.Result{}, fmt.Errorf("shadow peer for gateway A in namespace B: %w", err)
	}
	// 5. Create/update shadow peer of GatewayB in NamespaceA.
	if err := r.ensureShadowPeer(ctx, peering, gatewayB, peering.Spec.NamespaceA, networkA.Name, cidrB); err != nil {
		return ctrl.Result{}, fmt.Errorf("shadow peer for gateway B in namespace A: %w", err)
	}

	// 6. Ensure policies so ComputedPeers includes the gateway and shadow peers.
	if err := r.ensurePolicies(ctx, peering, networkA.Name, peering.Spec.NamespaceA); err != nil {
		return ctrl.Result{}, fmt.Errorf("policies namespace A: %w", err)
	}
	if err := r.ensurePolicies(ctx, peering, networkB.Name, peering.Spec.NamespaceB); err != nil {
		return ctrl.Result{}, fmt.Errorf("policies namespace B: %w", err)
	}

	// 7. Update status.
	return r.setReady(ctx, peering, cidrA, cidrB)
}

// reconcileDelete removes all resources created by this reconciler and drops
// the finalizer so Kubernetes can delete the peering object.
func (r *NetworkPeeringReconciler) reconcileDelete(ctx context.Context, peering *v1alpha1.WireflowNetworkPeering) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Deleting WireflowNetworkPeering", "name", peering.Name)

	annotationKey := peeringRouteAnnotationKey(peering.Name)
	shadowName := shadowPeerName(peering.Name)

	// Remove peering-route annotations from gateway peers.
	for _, ns := range []string{peering.Spec.NamespaceA, peering.Spec.NamespaceB} {
		networkName := peering.Spec.NetworkA
		if ns == peering.Spec.NamespaceB {
			networkName = peering.Spec.NetworkB
		}
		if gw, err := r.findGateway(ctx, ns, networkName); err == nil && gw != nil {
			if err := r.removeAnnotation(ctx, gw, annotationKey); err != nil {
				log.Error(err, "failed to remove peering route annotation", "peer", gw.Name)
			}
		}
	}

	// Delete shadow peers and policies in both namespaces.
	for _, ns := range []string{peering.Spec.NamespaceA, peering.Spec.NamespaceB} {
		_ = r.deleteIfExists(ctx, &v1alpha1.WireflowPeer{}, ns, shadowName)
		_ = r.deleteIfExists(ctx, &v1alpha1.WireflowPolicy{}, ns, gwAccessPolicyName(peering.Name))
		_ = r.deleteIfExists(ctx, &v1alpha1.WireflowPolicy{}, ns, shadowPolicyName(peering.Name))
	}

	controllerutil.RemoveFinalizer(peering, PeeringFinalizer)
	return ctrl.Result{}, r.Update(ctx, peering)
}

// getReadyNetwork fetches a WireflowNetwork and returns an error if it is not
// yet in Ready phase or if ActiveCIDR has not been allocated.
func (r *NetworkPeeringReconciler) getReadyNetwork(ctx context.Context, ns, name string) (*v1alpha1.WireflowNetwork, error) {
	var network v1alpha1.WireflowNetwork
	if err := r.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, &network); err != nil {
		return nil, err
	}
	if network.Status.Phase != v1alpha1.NetworkPhaseReady || network.Status.ActiveCIDR == "" {
		return nil, fmt.Errorf("network %s/%s not ready (phase=%s, cidr=%q)",
			ns, name, network.Status.Phase, network.Status.ActiveCIDR)
	}
	return &network, nil
}

// findGateway returns the first WireflowPeer labeled wireflow.run/gateway=true
// in the given namespace that belongs to the given network.
func (r *NetworkPeeringReconciler) findGateway(ctx context.Context, ns, networkName string) (*v1alpha1.WireflowPeer, error) {
	var peerList v1alpha1.WireflowPeerList
	if err := r.List(ctx, &peerList, client.InNamespace(ns), client.MatchingLabels{
		LabelGateway: "true",
		networkLabelKey(networkName): "true",
	}); err != nil {
		return nil, err
	}
	if len(peerList.Items) == 0 {
		return nil, nil
	}
	return &peerList.Items[0], nil
}

// ensureAnnotation adds or updates a single annotation on a WireflowPeer.
func (r *NetworkPeeringReconciler) ensureAnnotation(ctx context.Context, peer *v1alpha1.WireflowPeer, key, value string) error {
	current := peer.GetAnnotations()
	if current[key] == value {
		return nil
	}
	peerCopy := peer.DeepCopy()
	if peerCopy.Annotations == nil {
		peerCopy.Annotations = make(map[string]string)
	}
	peerCopy.Annotations[key] = value
	return r.Patch(ctx, peerCopy, client.MergeFrom(peer))
}

// removeAnnotation deletes a single annotation from a WireflowPeer.
func (r *NetworkPeeringReconciler) removeAnnotation(ctx context.Context, peer *v1alpha1.WireflowPeer, key string) error {
	if _, ok := peer.GetAnnotations()[key]; !ok {
		return nil
	}
	peerCopy := peer.DeepCopy()
	delete(peerCopy.Annotations, key)
	return r.Patch(ctx, peerCopy, client.MergeFrom(peer))
}

// ensureShadowPeer creates or updates the shadow WireflowPeer that represents
// srcGateway in the given target namespace.
func (r *NetworkPeeringReconciler) ensureShadowPeer(
	ctx context.Context,
	peering *v1alpha1.WireflowNetworkPeering,
	srcGateway *v1alpha1.WireflowPeer,
	targetNS, targetNetwork, srcCIDR string,
) error {
	name := shadowPeerName(peering.Name)
	networkLabel := networkLabelKey(targetNetwork)

	desired := &v1alpha1.WireflowPeer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: targetNS,
			Labels: map[string]string{
				LabelShadow:   "true",
				networkLabel:  "true",
			},
			Annotations: map[string]string{
				AnnotationShadowAllowedIPs: srcCIDR,
			},
		},
		Spec: v1alpha1.WireflowPeerSpec{
			AppId:     srcGateway.Spec.AppId,
			PublicKey: srcGateway.Spec.PublicKey,
			PeerId:    srcGateway.Spec.PeerId,
		},
	}

	var existing v1alpha1.WireflowPeer
	err := r.Get(ctx, types.NamespacedName{Namespace: targetNS, Name: name}, &existing)
	if k8serrors.IsNotFound(err) {
		if createErr := r.Create(ctx, desired); createErr != nil {
			return createErr
		}
		// Set Status.AllocatedAddress so the peer appears in WireGuard configs.
		if srcGateway.Status.AllocatedAddress != nil {
			created := desired.DeepCopy()
			created.Status.AllocatedAddress = srcGateway.Status.AllocatedAddress
			created.Status.Phase = v1alpha1.NodePhaseReady
			return r.Status().Update(ctx, created)
		}
		return nil
	}
	if err != nil {
		return err
	}

	// Update if spec or annotations changed.
	peerCopy := existing.DeepCopy()
	peerCopy.Labels = desired.Labels
	peerCopy.Annotations = desired.Annotations
	peerCopy.Spec.PublicKey = desired.Spec.PublicKey
	peerCopy.Spec.AppId = desired.Spec.AppId
	peerCopy.Spec.PeerId = desired.Spec.PeerId
	if err := r.Patch(ctx, peerCopy, client.MergeFrom(&existing)); err != nil {
		return err
	}

	// Sync AllocatedAddress in status.
	if srcGateway.Status.AllocatedAddress != nil &&
		(existing.Status.AllocatedAddress == nil ||
			*existing.Status.AllocatedAddress != *srcGateway.Status.AllocatedAddress) {
		statusCopy := peerCopy.DeepCopy()
		statusCopy.Status.AllocatedAddress = srcGateway.Status.AllocatedAddress
		statusCopy.Status.Phase = v1alpha1.NodePhaseReady
		return r.Status().Update(ctx, statusCopy)
	}
	return nil
}

// ensurePolicies creates or updates the two policies needed in a namespace:
//  1. gwAccessPolicy  — allows all peers to egress to the gateway (so all local
//     peers get the gateway in ComputedPeers with expanded AllowedIPs).
//  2. shadowPolicy    — allows the gateway to egress to shadow peers (so the
//     gateway gets the remote shadow in ComputedPeers to establish the tunnel).
func (r *NetworkPeeringReconciler) ensurePolicies(ctx context.Context, peering *v1alpha1.WireflowNetworkPeering, networkName, ns string) error {
	// Policy 1: all peers → gateway
	gwPolicy := &v1alpha1.WireflowPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gwAccessPolicyName(peering.Name),
			Namespace: ns,
			Labels:    map[string]string{"wireflow.run/peering": safeLabelValue(peering.Name)},
		},
		Spec: v1alpha1.WireflowPolicySpec{
			Network:      networkName,
			PeerSelector: metav1.LabelSelector{}, // match all peers
			Egress: []v1alpha1.EgressRule{
				{
					To: []v1alpha1.PeerSelection{
						{
							PeerSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{LabelGateway: "true"},
							},
						},
					},
				},
			},
			Ingress: []v1alpha1.IngressRule{
				{
					From: []v1alpha1.PeerSelection{
						{
							PeerSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{LabelGateway: "true"},
							},
						},
					},
				},
			},
		},
	}

	// Policy 2: gateway → shadow peers
	shadowPolicy := &v1alpha1.WireflowPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      shadowPolicyName(peering.Name),
			Namespace: ns,
			Labels:    map[string]string{"wireflow.run/peering": safeLabelValue(peering.Name)},
		},
		Spec: v1alpha1.WireflowPolicySpec{
			Network: networkName,
			PeerSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{LabelGateway: "true"},
			},
			Egress: []v1alpha1.EgressRule{
				{
					To: []v1alpha1.PeerSelection{
						{
							PeerSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{LabelShadow: "true"},
							},
						},
					},
				},
			},
		},
	}

	for _, policy := range []*v1alpha1.WireflowPolicy{gwPolicy, shadowPolicy} {
		if err := r.applyPolicy(ctx, policy); err != nil {
			return err
		}
	}
	return nil
}

// applyPolicy creates the policy if it does not exist, or patches it if the spec differs.
func (r *NetworkPeeringReconciler) applyPolicy(ctx context.Context, desired *v1alpha1.WireflowPolicy) error {
	var existing v1alpha1.WireflowPolicy
	err := r.Get(ctx, client.ObjectKeyFromObject(desired), &existing)
	if k8serrors.IsNotFound(err) {
		return r.Create(ctx, desired)
	}
	if err != nil {
		return err
	}
	copy := existing.DeepCopy()
	copy.Spec = desired.Spec
	return r.Patch(ctx, copy, client.MergeFrom(&existing))
}

// deleteIfExists deletes a namespaced resource if it exists, ignoring NotFound.
func (r *NetworkPeeringReconciler) deleteIfExists(ctx context.Context, obj client.Object, ns, name string) error {
	obj.SetNamespace(ns)
	obj.SetName(name)
	err := r.Delete(ctx, obj)
	if k8serrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (r *NetworkPeeringReconciler) setReady(ctx context.Context, peering *v1alpha1.WireflowNetworkPeering, cidrA, cidrB string) (ctrl.Result, error) {
	copy := peering.DeepCopy()
	copy.Status.Phase = v1alpha1.NetworkPhaseReady
	copy.Status.CIDRA = cidrA
	copy.Status.CIDRB = cidrB
	copy.Status.Conditions = setCondition(copy.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "PeeringEstablished",
		Message:            fmt.Sprintf("peering between %s and %s established", cidrA, cidrB),
		LastTransitionTime: metav1.Now(),
	})
	return ctrl.Result{}, r.Status().Patch(ctx, copy, client.MergeFrom(peering))
}

func (r *NetworkPeeringReconciler) setError(ctx context.Context, peering *v1alpha1.WireflowNetworkPeering, msg string) (ctrl.Result, error) {
	copy := peering.DeepCopy()
	copy.Status.Phase = v1alpha1.NetworkPhaseFailed
	copy.Status.Conditions = setCondition(copy.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             "PeeringError",
		Message:            msg,
		LastTransitionTime: metav1.Now(),
	})
	_ = r.Status().Patch(ctx, copy, client.MergeFrom(peering))
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// setCondition upserts a condition in the slice by Type.
// LastTransitionTime is only updated when Status or Reason actually changes,
// preventing spurious status writes on every reconcile.
func setCondition(conditions []metav1.Condition, c metav1.Condition) []metav1.Condition {
	for i, existing := range conditions {
		if existing.Type == c.Type {
			if existing.Status == c.Status && existing.Reason == c.Reason {
				c.LastTransitionTime = existing.LastTransitionTime
			}
			conditions[i] = c
			return conditions
		}
	}
	return append(conditions, c)
}

// shadowPeerName returns the deterministic name for a shadow peer given a peering name.
func shadowPeerName(peeringName string) string {
	return fmt.Sprintf("peering-shadow-%s", peeringName)
}

func gwAccessPolicyName(peeringName string) string {
	return fmt.Sprintf("wireflow-peering-%s-gw-access", peeringName)
}

func shadowPolicyName(peeringName string) string {
	return fmt.Sprintf("wireflow-peering-%s-shadow", peeringName)
}

// SetupWithManager registers the controller with the manager.
func (r *NetworkPeeringReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Only re-enqueue peerings when a network's Phase or ActiveCIDR actually
	// changes. Without this predicate every Network status patch (e.g. metrics
	// updates) would trigger an unnecessary reconcile.
	networkReadyPredicate := predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return true },
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldNet, ok1 := e.ObjectOld.(*v1alpha1.WireflowNetwork)
			newNet, ok2 := e.ObjectNew.(*v1alpha1.WireflowNetwork)
			if !ok1 || !ok2 {
				return false
			}
			return oldNet.Status.Phase != newNet.Status.Phase ||
				oldNet.Status.ActiveCIDR != newNet.Status.ActiveCIDR
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.WireflowNetworkPeering{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		// Re-enqueue peerings when a gateway peer in either network comes online.
		Watches(&v1alpha1.WireflowPeer{},
			handler.EnqueueRequestsFromMapFunc(r.mapPeerToPeerings),
			builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		// Re-enqueue only when a network's Phase or ActiveCIDR changes.
		Watches(&v1alpha1.WireflowNetwork{},
			handler.EnqueueRequestsFromMapFunc(r.mapNetworkToPeerings),
			builder.WithPredicates(networkReadyPredicate)).
		Named("network-peering").
		Complete(r)
}

// mapPeerToPeerings returns reconcile requests for all WireflowNetworkPeerings
// whose NamespaceA or NamespaceB matches the peer's namespace.
func (r *NetworkPeeringReconciler) mapPeerToPeerings(ctx context.Context, obj client.Object) []reconcile.Request {
	var list v1alpha1.WireflowNetworkPeeringList
	if err := r.List(ctx, &list); err != nil {
		return nil
	}
	ns := obj.GetNamespace()
	var reqs []reconcile.Request
	for _, p := range list.Items {
		if p.Spec.NamespaceA == ns || p.Spec.NamespaceB == ns {
			reqs = append(reqs, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: p.Name},
			})
		}
	}
	return reqs
}

// mapNetworkToPeerings returns reconcile requests for peerings that reference
// the changed network.
func (r *NetworkPeeringReconciler) mapNetworkToPeerings(ctx context.Context, obj client.Object) []reconcile.Request {
	var list v1alpha1.WireflowNetworkPeeringList
	if err := r.List(ctx, &list); err != nil {
		return nil
	}
	ns, name := obj.GetNamespace(), obj.GetName()
	var reqs []reconcile.Request
	for _, p := range list.Items {
		if (p.Spec.NamespaceA == ns && p.Spec.NetworkA == name) ||
			(p.Spec.NamespaceB == ns && p.Spec.NetworkB == name) {
			reqs = append(reqs, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: p.Name},
			})
		}
	}
	return reqs
}
