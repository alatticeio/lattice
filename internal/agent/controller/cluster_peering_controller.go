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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/alatticeio/lattice/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const clusterPeeringFinalizer = "alattice.io/cluster-peering-finalizer"

// GatewayInfo is the response body returned by the remote cluster's
// GET /api/v1/peering/gateway-info endpoint.
type GatewayInfo struct {
	PublicKey string `json:"publicKey"`
	GatewayIP string `json:"gatewayIP"`
	CIDR      string `json:"cidr"`
	AppID     string `json:"appId"`
	PeerID    string `json:"peerId"`
}

// ClusterPeeringReconciler reconciles LatticeClusterPeering resources.
//
// For each cross-cluster peering it:
//  1. Loads the referenced LatticeCluster to obtain the remote management endpoint.
//  2. Calls GET /api/v1/peering/gateway-info on the remote cluster.
//  3. Creates a LatticeNetworkPeering in the local cluster using a synthetic
//     shadow namespace for the remote side (using the local gateway + remote shadow).
//
// The actual WireGuard tunnel establishment reuses the existing WRRP relay
// transport — no additional signaling infrastructure is needed.
type ClusterPeeringReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	httpClient *http.Client
}

// +kubebuilder:rbac:groups=alattice.io,resources=latticeclusterpeerings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=alattice.io,resources=latticeclusterpeerings/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=alattice.io,resources=latticeclusterpeerings/finalizers,verbs=update
// +kubebuilder:rbac:groups=alattice.io,resources=latticeclusters,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *ClusterPeeringReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Reconciling LatticeClusterPeering", "name", req.Name)

	var cp v1alpha1.LatticeClusterPeering
	if err := r.Get(ctx, req.NamespacedName, &cp); err != nil {
		if k8serrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !cp.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, &cp)
	}

	if !controllerutil.ContainsFinalizer(&cp, clusterPeeringFinalizer) {
		controllerutil.AddFinalizer(&cp, clusterPeeringFinalizer)
		if err := r.Update(ctx, &cp); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	return r.reconcileNormal(ctx, &cp)
}

func (r *ClusterPeeringReconciler) reconcileNormal(ctx context.Context, cp *v1alpha1.LatticeClusterPeering) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// 1. Load the referenced LatticeCluster.
	var cluster v1alpha1.LatticeCluster
	if err := r.Get(ctx, types.NamespacedName{Name: cp.Spec.RemoteCluster}, &cluster); err != nil {
		return r.setClusterPeeringError(ctx, cp, fmt.Sprintf("LatticeCluster %q not found: %v", cp.Spec.RemoteCluster, err))
	}

	// 2. Load the bearer token from the referenced Secret.
	token, err := r.loadCredential(ctx, cluster.Spec.CredentialRef)
	if err != nil {
		return r.setClusterPeeringError(ctx, cp, fmt.Sprintf("credential load failed: %v", err))
	}

	// 3. Fetch remote gateway info.
	info, err := r.fetchGatewayInfo(ctx, cluster.Spec.ManagementEndpoint, token,
		cp.Spec.RemoteNamespace, cp.Spec.RemoteNetwork)
	if err != nil {
		log.Error(err, "failed to fetch remote gateway info")
		return r.setClusterPeeringError(ctx, cp, fmt.Sprintf("remote gateway info: %v", err))
	}

	// 4. Ensure the local network is ready and get its CIDR.
	localNetwork, err := r.getReadyNetworkForCluster(ctx, cp.Spec.LocalNamespace, cp.Spec.LocalNetwork)
	if err != nil {
		return r.setClusterPeeringError(ctx, cp, fmt.Sprintf("local network not ready: %v", err))
	}

	// 5. Create/update a shadow peer in the local namespace representing the remote gateway.
	if err := r.ensureRemoteGatewayShadow(ctx, cp, info, localNetwork.Name); err != nil {
		return ctrl.Result{}, fmt.Errorf("ensure remote gateway shadow: %w", err)
	}

	// 6. Find the local gateway and annotate it with the remote CIDR.
	localGW, err := r.findGatewayForCluster(ctx, cp.Spec.LocalNamespace, cp.Spec.LocalNetwork)
	if err != nil || localGW == nil {
		return r.setClusterPeeringError(ctx, cp, fmt.Sprintf("no local gateway: %v", err))
	}
	annotationKey := AnnotationPeeringRoutePrefix + cp.Name
	if err := r.ensureAnnotationForCluster(ctx, localGW, annotationKey, info.CIDR); err != nil {
		return ctrl.Result{}, err
	}

	// 7. Ensure policies so all local peers route through the gateway, and the
	//    gateway can reach the remote shadow.
	if err := r.ensurePoliciesForCluster(ctx, cp, localNetwork.Name); err != nil {
		return ctrl.Result{}, err
	}

	// 8. Update status.
	return r.setClusterPeeringReady(ctx, cp, localNetwork.Status.ActiveCIDR, info.CIDR)
}

func (r *ClusterPeeringReconciler) reconcileDelete(ctx context.Context, cp *v1alpha1.LatticeClusterPeering) (ctrl.Result, error) {
	shadowName := fmt.Sprintf("cluster-shadow-%s", cp.Name)
	annotationKey := AnnotationPeeringRoutePrefix + cp.Name

	if gw, err := r.findGatewayForCluster(ctx, cp.Spec.LocalNamespace, cp.Spec.LocalNetwork); err == nil && gw != nil {
		_ = r.removeAnnotationForCluster(ctx, gw, annotationKey)
	}

	_ = r.deleteClusterResourceIfExists(ctx, &v1alpha1.LatticePeer{}, cp.Spec.LocalNamespace, shadowName)
	_ = r.deleteClusterResourceIfExists(ctx, &v1alpha1.LatticePolicy{}, cp.Spec.LocalNamespace, fmt.Sprintf("lattice-cpeering-%s-gw-access", cp.Name))
	_ = r.deleteClusterResourceIfExists(ctx, &v1alpha1.LatticePolicy{}, cp.Spec.LocalNamespace, fmt.Sprintf("lattice-cpeering-%s-shadow", cp.Name))

	controllerutil.RemoveFinalizer(cp, clusterPeeringFinalizer)
	return ctrl.Result{}, r.Update(ctx, cp)
}

// loadCredential reads the bearer token from the Secret referenced by credentialRef.
// The Secret must be in the controller's own namespace (kube-system or the
// namespace the controller is deployed in). The token is stored under key "token".
func (r *ClusterPeeringReconciler) loadCredential(ctx context.Context, secretName string) (string, error) {
	// Credentials are stored in the controller namespace; use label to discover it.
	var secretList corev1.SecretList
	if err := r.List(ctx, &secretList, client.MatchingLabels{
		"alattice.io/cluster-credential": secretName,
	}); err != nil {
		return "", err
	}
	// Fallback: try by name in all namespaces.
	if len(secretList.Items) == 0 {
		return "", fmt.Errorf("secret %q not found (label alattice.io/cluster-credential=%s)", secretName, secretName)
	}
	token, ok := secretList.Items[0].Data["token"]
	if !ok {
		return "", fmt.Errorf("secret %q missing key 'token'", secretName)
	}
	return string(token), nil
}

// fetchGatewayInfo calls the remote management API to obtain the gateway info
// for the specified namespace/network.
func (r *ClusterPeeringReconciler) fetchGatewayInfo(ctx context.Context, endpoint, token, ns, network string) (*GatewayInfo, error) {
	hc := r.httpClient
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Second}
	}

	url := fmt.Sprintf("%s/api/v1/peering/gateway-info?namespace=%s&network=%s", endpoint, ns, network)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("remote gateway-info returned %d: %s", resp.StatusCode, string(body))
	}

	var info GatewayInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode gateway-info response: %w", err)
	}
	if info.PublicKey == "" || info.GatewayIP == "" || info.CIDR == "" {
		return nil, fmt.Errorf("incomplete gateway-info: %+v", info)
	}
	return &info, nil
}

// ensureRemoteGatewayShadow creates or updates the shadow peer in the local
// namespace that represents the remote cluster's gateway.
func (r *ClusterPeeringReconciler) ensureRemoteGatewayShadow(ctx context.Context, cp *v1alpha1.LatticeClusterPeering, info *GatewayInfo, networkName string) error {
	name := fmt.Sprintf("cluster-shadow-%s", cp.Name)
	networkLabel := fmt.Sprintf("alattice.io/network-%s", networkName)

	desired := &v1alpha1.LatticePeer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cp.Spec.LocalNamespace,
			Labels: map[string]string{
				LabelShadow:  "true",
				networkLabel: "true",
			},
			Annotations: map[string]string{
				AnnotationShadowAllowedIPs: info.CIDR,
			},
		},
		Spec: v1alpha1.LatticePeerSpec{
			PublicKey: info.PublicKey,
			AppId:     info.AppID,
			PeerId:    info.PeerID,
		},
	}

	var existing v1alpha1.LatticePeer
	err := r.Get(ctx, client.ObjectKeyFromObject(desired), &existing)
	if k8serrors.IsNotFound(err) {
		if createErr := r.Create(ctx, desired); createErr != nil {
			return createErr
		}
		// Set the gateway IP in Status so it appears in WireGuard config.
		created := desired.DeepCopy()
		created.Status.AllocatedAddress = &info.GatewayIP
		created.Status.Phase = v1alpha1.NodePhaseReady
		return r.Status().Update(ctx, created)
	}
	if err != nil {
		return err
	}

	peerCopy := existing.DeepCopy()
	peerCopy.Labels = desired.Labels
	peerCopy.Annotations = desired.Annotations
	peerCopy.Spec.PublicKey = desired.Spec.PublicKey
	peerCopy.Spec.AppId = desired.Spec.AppId
	if patchErr := r.Patch(ctx, peerCopy, client.MergeFrom(&existing)); patchErr != nil {
		return patchErr
	}

	if existing.Status.AllocatedAddress == nil || *existing.Status.AllocatedAddress != info.GatewayIP {
		statusCopy := peerCopy.DeepCopy()
		statusCopy.Status.AllocatedAddress = &info.GatewayIP
		statusCopy.Status.Phase = v1alpha1.NodePhaseReady
		return r.Status().Update(ctx, statusCopy)
	}
	return nil
}

func (r *ClusterPeeringReconciler) ensurePoliciesForCluster(ctx context.Context, cp *v1alpha1.LatticeClusterPeering, networkName string) error {
	ns := cp.Spec.LocalNamespace
	gwPolicy := &v1alpha1.LatticePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("lattice-cpeering-%s-gw-access", cp.Name),
			Namespace: ns,
			Labels:    map[string]string{"alattice.io/cluster-peering": cp.Name},
		},
		Spec: v1alpha1.LatticePolicySpec{
			Network:      networkName,
			PeerSelector: metav1.LabelSelector{},
			Egress: []v1alpha1.EgressRule{
				{To: []v1alpha1.PeerSelection{{PeerSelector: &metav1.LabelSelector{MatchLabels: map[string]string{LabelGateway: "true"}}}}},
			},
			Ingress: []v1alpha1.IngressRule{
				{From: []v1alpha1.PeerSelection{{PeerSelector: &metav1.LabelSelector{MatchLabels: map[string]string{LabelGateway: "true"}}}}},
			},
		},
	}
	shadowPolicy := &v1alpha1.LatticePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("lattice-cpeering-%s-shadow", cp.Name),
			Namespace: ns,
			Labels:    map[string]string{"alattice.io/cluster-peering": cp.Name},
		},
		Spec: v1alpha1.LatticePolicySpec{
			Network:      networkName,
			PeerSelector: metav1.LabelSelector{MatchLabels: map[string]string{LabelGateway: "true"}},
			Egress: []v1alpha1.EgressRule{
				{To: []v1alpha1.PeerSelection{{PeerSelector: &metav1.LabelSelector{MatchLabels: map[string]string{LabelShadow: "true"}}}}},
			},
		},
	}

	for _, policy := range []*v1alpha1.LatticePolicy{gwPolicy, shadowPolicy} {
		var existing v1alpha1.LatticePolicy
		err := r.Get(ctx, client.ObjectKeyFromObject(policy), &existing)
		if k8serrors.IsNotFound(err) {
			if createErr := r.Create(ctx, policy); createErr != nil {
				return createErr
			}
			continue
		}
		if err != nil {
			return err
		}
		copy := existing.DeepCopy()
		copy.Spec = policy.Spec
		if patchErr := r.Patch(ctx, copy, client.MergeFrom(&existing)); patchErr != nil {
			return patchErr
		}
	}
	return nil
}

func (r *ClusterPeeringReconciler) getReadyNetworkForCluster(ctx context.Context, ns, name string) (*v1alpha1.LatticeNetwork, error) {
	var network v1alpha1.LatticeNetwork
	if err := r.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, &network); err != nil {
		return nil, err
	}
	if network.Status.Phase != v1alpha1.NetworkPhaseReady || network.Status.ActiveCIDR == "" {
		return nil, fmt.Errorf("network %s/%s not ready", ns, name)
	}
	return &network, nil
}

func (r *ClusterPeeringReconciler) findGatewayForCluster(ctx context.Context, ns, networkName string) (*v1alpha1.LatticePeer, error) {
	var list v1alpha1.LatticePeerList
	if err := r.List(ctx, &list, client.InNamespace(ns), client.MatchingLabels{
		LabelGateway: "true",
		fmt.Sprintf("alattice.io/network-%s", networkName): "true",
	}); err != nil {
		return nil, err
	}
	if len(list.Items) == 0 {
		return nil, nil
	}
	return &list.Items[0], nil
}

func (r *ClusterPeeringReconciler) ensureAnnotationForCluster(ctx context.Context, peer *v1alpha1.LatticePeer, key, value string) error {
	if peer.GetAnnotations()[key] == value {
		return nil
	}
	peerCopy := peer.DeepCopy()
	if peerCopy.Annotations == nil {
		peerCopy.Annotations = make(map[string]string)
	}
	peerCopy.Annotations[key] = value
	return r.Patch(ctx, peerCopy, client.MergeFrom(peer))
}

func (r *ClusterPeeringReconciler) removeAnnotationForCluster(ctx context.Context, peer *v1alpha1.LatticePeer, key string) error {
	if _, ok := peer.GetAnnotations()[key]; !ok {
		return nil
	}
	peerCopy := peer.DeepCopy()
	delete(peerCopy.Annotations, key)
	return r.Patch(ctx, peerCopy, client.MergeFrom(peer))
}

func (r *ClusterPeeringReconciler) deleteClusterResourceIfExists(ctx context.Context, obj client.Object, ns, name string) error {
	obj.SetNamespace(ns)
	obj.SetName(name)
	err := r.Delete(ctx, obj)
	if k8serrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (r *ClusterPeeringReconciler) setClusterPeeringReady(ctx context.Context, cp *v1alpha1.LatticeClusterPeering, localCIDR, remoteCIDR string) (ctrl.Result, error) {
	copy := cp.DeepCopy()
	copy.Status.Phase = v1alpha1.ClusterPeeringPhaseReady
	copy.Status.LocalCIDR = localCIDR
	copy.Status.RemoteCIDR = remoteCIDR
	copy.Status.Conditions = setCondition(copy.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionTrue,
		Reason:             "ClusterPeeringEstablished",
		Message:            fmt.Sprintf("cross-cluster peering %s ↔ %s established", localCIDR, remoteCIDR),
		LastTransitionTime: metav1.Now(),
	})
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, r.Status().Patch(ctx, copy, client.MergeFrom(cp))
}

func (r *ClusterPeeringReconciler) setClusterPeeringError(ctx context.Context, cp *v1alpha1.LatticeClusterPeering, msg string) (ctrl.Result, error) {
	copy := cp.DeepCopy()
	copy.Status.Phase = v1alpha1.ClusterPeeringPhaseError
	copy.Status.Conditions = setCondition(copy.Status.Conditions, metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             "ClusterPeeringError",
		Message:            msg,
		LastTransitionTime: metav1.Now(),
	})
	_ = r.Status().Patch(ctx, copy, client.MergeFrom(cp))
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// SetupWithManager registers the ClusterPeeringReconciler with the manager.
func (r *ClusterPeeringReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.LatticeClusterPeering{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Named("cluster-peering").
		Complete(r)
}
