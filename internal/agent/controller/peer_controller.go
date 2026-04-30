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
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/agent/ipam"
	"reflect"
	"strings"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/alatticeio/lattice/api/v1alpha1"
)

// PeerReconciler reconciles a LatticePeer object
type PeerReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	IPAM          *ipam.IPAM
	generator     *Generator
	SnapshotCache map[types.NamespacedName]*PeerStateSnapshot

	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=alattice.io,resources=latticepeers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=alattice.io,resources=latticepeers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=alattice.io,resources=latticepeers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PeerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Reconciling LatticePeer", "namespace", req.NamespacedName, "name", req.Name)

	var peer v1alpha1.LatticePeer
	if err := r.Get(ctx, req.NamespacedName, &peer); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Finalizer / deletion handling.
	const finalizerName = "alattice.io/node"
	if !peer.ObjectMeta.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&peer, finalizerName) {
			return r.handleDelete(ctx, &peer)
		}
		return ctrl.Result{}, nil
	}
	if !controllerutil.ContainsFinalizer(&peer, finalizerName) {
		controllerutil.AddFinalizer(&peer, finalizerName)
		if err := r.Update(ctx, &peer); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 100 * time.Nanosecond}, nil
	}

	// Shadow peers are managed exclusively by NetworkPeeringReconciler /
	// ClusterPeeringReconciler; skip them here to avoid conflicting updates.
	if peer.GetLabels()[LabelShadow] == "true" {
		log.Info("Skipping shadow peer", "namespace", req.Namespace, "name", req.Name)
		return ctrl.Result{}, nil
	}

	// Phase-based state machine.
	switch peer.Status.Phase {
	case "":
		return r.handleInitialization(ctx, &peer, req)
	case v1alpha1.NodePhasePending:
		return r.handlePending(ctx, &peer, req)
	case v1alpha1.NodePhaseReady:
		return r.handleReady(ctx, &peer, req)
	case v1alpha1.NodePhaseFailed:
		return r.handleFailed(ctx, &peer, req)
	default:
		return r.handleInitialization(ctx, &peer, req)
	}
}

// handleInitialization runs when Phase is empty (newly created peer).
// It generates WireGuard keys and advances to Pending.
func (r *PeerReconciler) handleInitialization(ctx context.Context, peer *v1alpha1.LatticePeer, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Initializing peer", "name", req.Name)

	// Generate WireGuard keys if absent. Returns true when a spec patch was written,
	// requiring a requeue so the next reconcile sees the updated ResourceVersion.
	changed, err := r.ensureKeys(ctx, peer)
	if err != nil {
		return ctrl.Result{}, err
	}
	if changed {
		return ctrl.Result{}, nil
	}

	// Advance to Pending so the next reconcile evaluates network intent.
	if _, err := r.updateStatus(ctx, peer, func(p *v1alpha1.LatticePeer) {
		p.Status.Phase = v1alpha1.NodePhasePending
		p.Status.Conditions = setCondition(p.Status.Conditions, metav1.Condition{
			Type:               v1alpha1.NodeConditionInitialized,
			Status:             metav1.ConditionTrue,
			Reason:             v1alpha1.ReasonReady,
			LastTransitionTime: metav1.Now(),
		})
	}); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: 100 * time.Millisecond}, nil
}

// handlePending dispatches to joinNetwork, leaveNetwork, or idle based on
// the delta between Spec.Network and Status.ActiveNetwork.
func (r *PeerReconciler) handlePending(ctx context.Context, peer *v1alpha1.LatticePeer, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	specNet := peer.Spec.Network
	activeNet := peer.Status.ActiveNetwork

	switch {
	case specNet == nil && activeNet == nil:
		// Idle: no network intent; move directly to Ready.
		log.Info("Peer is idle (no network intent)", "name", req.Name)
		if _, err := r.updateStatus(ctx, peer, func(p *v1alpha1.LatticePeer) {
			p.Status.Phase = v1alpha1.NodePhaseReady
		}); err != nil {
			return ctrl.Result{}, err
		}
		return r.lastReconcile(ctx, peer, req)

	case specNet == nil && activeNet != nil:
		log.Info("Leaving network", "name", req.Name, "network", *activeNet)
		return r.leaveNetwork(ctx, peer, req)

	default:
		// specNet != nil. Differentiate between first-join and network-switch.
		// On a network switch the old IP must be released before a new one is
		// allocated; otherwise joinNetwork would allocate a second address while
		// the first endpoint is still alive.
		if activeNet != nil && *activeNet != *specNet {
			log.Info("Switching network, leaving old network first", "name", req.Name, "from", *activeNet, "to", *specNet)
			return r.leaveNetwork(ctx, peer, req)
		}
		log.Info("Joining network", "name", req.Name, "network", *specNet)
		return r.joinNetwork(ctx, peer, req)
	}
}

// handleReady detects spec changes that require a network transition and
// delegates to lastReconcile to refresh the ConfigMap when there is nothing to change.
func (r *PeerReconciler) handleReady(ctx context.Context, peer *v1alpha1.LatticePeer, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	specNet := peer.Spec.Network
	activeNet := peer.Status.ActiveNetwork

	needTransition := (specNet == nil && activeNet != nil) ||
		(specNet != nil && (activeNet == nil || *specNet != *activeNet))

	if needTransition {
		log.Info("Network change detected, transitioning to Pending", "name", req.Name)
		if _, err := r.updateStatus(ctx, peer, func(p *v1alpha1.LatticePeer) {
			p.Status.Phase = v1alpha1.NodePhasePending
		}); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 100 * time.Millisecond}, nil
	}

	return r.lastReconcile(ctx, peer, req)
}

// handleFailed resets the phase to Pending for a retry after a back-off period.
func (r *PeerReconciler) handleFailed(ctx context.Context, peer *v1alpha1.LatticePeer, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Peer is in Failed state, scheduling retry", "name", req.Name)
	if _, err := r.updateStatus(ctx, peer, func(p *v1alpha1.LatticePeer) {
		p.Status.Phase = v1alpha1.NodePhasePending
	}); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *PeerReconciler) handleDelete(ctx context.Context, node *v1alpha1.LatticePeer) (ctrl.Result, error) {
	// 1. Finalizer name.
	const finalizerName = "alattice.io/node"

	// 2. Check if the finalizer is present.
	if !controllerutil.ContainsFinalizer(node, finalizerName) {
		return ctrl.Result{}, nil // Already removed, nothing to do.
	}

	// 3. Run cleanup logic.
	// Critical: this must be idempotent — running it multiple times must yield the same result.
	if err := r.performCleanup(ctx, node); err != nil {
		// If cleanup fails, do not remove the finalizer yet; return the error to trigger a retry.
		return ctrl.Result{}, err
	}

	// 4. Cleanup succeeded — remove the finalizer.
	controllerutil.RemoveFinalizer(node, finalizerName)
	if err := r.Update(ctx, node); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *PeerReconciler) performCleanup(ctx context.Context, node *v1alpha1.LatticePeer) error {
	// No external resources need cleaning up at this time.
	// Reserved: add relay cache or database cleanup here if needed in the future.
	return nil
}

// ensureKeys generates a WireGuard key pair and PeerId when they are absent.
// Does not depend on Spec.Network; safe to call during initialization.
// Returns (true, nil) when a spec patch was written.
func (r *PeerReconciler) ensureKeys(ctx context.Context, peer *v1alpha1.LatticePeer) (bool, error) {
	if peer.Spec.PrivateKey != "" {
		return false, nil
	}
	return r.updateSpec(ctx, peer, func(node *v1alpha1.LatticePeer) error {
		key, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return err
		}
		node.Spec.PrivateKey = key.String()
		node.Spec.PublicKey = key.PublicKey().String()
		node.Spec.PeerId = fmt.Sprintf("%d", infra.FromKey(key.PublicKey()).ToUint64())
		return nil
	})
}

// ensurePeerSpec sets the network label for the current Spec.Network, clearing
// stale labels from any previously joined network so the peer never appears in
// more than one network at a time. Requires Spec.Network != nil.
// Returns (true, nil) when a patch was written.
func (r *PeerReconciler) ensurePeerSpec(ctx context.Context, peer *v1alpha1.LatticePeer) (bool, error) {
	return r.updateSpec(ctx, peer, func(node *v1alpha1.LatticePeer) error {
		network, err := r.getNetwork(ctx, node)
		if err != nil {
			return err
		}

		lbls := node.GetLabels()
		if lbls == nil {
			lbls = make(map[string]string)
		}
		// On a network switch, remove all old network labels before adding the new one
		// so the peer never appears in more than one network at a time.
		for label := range lbls {
			if strings.HasPrefix(label, "alattice.io/network-") {
				delete(lbls, label)
			}
		}
		lbls[networkLabelKey(network.Name)] = "true"
		node.SetLabels(lbls)
		return nil
	})
}

// joinNetwork allocates an IP for the peer in Spec.Network and marks it Ready.
func (r *PeerReconciler) joinNetwork(ctx context.Context, peer *v1alpha1.LatticePeer, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Idempotency guard: the peer is already in the target network, nothing to do.
	// The caller (handlePending) guarantees that activeNet is nil or equals specNet
	// by the time we arrive here, so checking ActiveNetwork against specNet is sufficient.
	// AllocateIP is itself idempotent: if the peer already owns a LatticeEndpoint it
	// returns the existing address, preventing a second allocation on retry after a
	// failed updateStatus call.
	if peer.Status.ActiveNetwork != nil && *peer.Status.ActiveNetwork == *peer.Spec.Network {
		log.Info("Peer is already in the target network, skip join")
		return ctrl.Result{}, nil
	}

	var network v1alpha1.LatticeNetwork
	if err := r.Get(ctx, types.NamespacedName{Namespace: peer.Namespace, Name: *peer.Spec.Network}, &network); err != nil {
		if _, sErr := r.updateStatus(ctx, peer, func(p *v1alpha1.LatticePeer) {
			p.Status.Phase = v1alpha1.NodePhaseFailed
			p.Status.Conditions = setCondition(p.Status.Conditions, metav1.Condition{
				Type:               v1alpha1.NodeConditionJoiningNetwork,
				Status:             metav1.ConditionFalse,
				Reason:             v1alpha1.ReasonNotReady,
				Message:            fmt.Sprintf("Network %q not found: %v", *peer.Spec.Network, err),
				LastTransitionTime: metav1.Now(),
			})
		}); sErr != nil {
			log.Error(sErr, "failed to record JoiningNetwork condition")
		}
		return ctrl.Result{}, err
	}

	// Wait for NetworkReconciler to assign a CIDR before allocating an IP.
	// networkReadyPredicate in SetupWithManager re-enqueues peers when ActiveCIDR transitions.
	if network.Status.ActiveCIDR == "" {
		log.Info("Network not ready yet, waiting for ActiveCIDR", "network", network.Name)
		if _, sErr := r.updateStatus(ctx, peer, func(p *v1alpha1.LatticePeer) {
			p.Status.Conditions = setCondition(p.Status.Conditions, metav1.Condition{
				Type:               v1alpha1.NodeConditionJoiningNetwork,
				Status:             metav1.ConditionFalse,
				Reason:             v1alpha1.ReasonConfiguring,
				Message:            fmt.Sprintf("Waiting for network %q to assign an ActiveCIDR", network.Name),
				LastTransitionTime: metav1.Now(),
			})
		}); sErr != nil {
			log.Error(sErr, "failed to record JoiningNetwork condition")
		}
		return ctrl.Result{RequeueAfter: 3 * time.Second}, nil
	}

	// Apply network label. Labels don't change Generation, so we requeue explicitly.
	changed, err := r.ensurePeerSpec(ctx, peer)
	if err != nil {
		if _, sErr := r.updateStatus(ctx, peer, func(p *v1alpha1.LatticePeer) {
			p.Status.Phase = v1alpha1.NodePhaseFailed
			p.Status.Conditions = setCondition(p.Status.Conditions, metav1.Condition{
				Type:               v1alpha1.NodeConditionJoiningNetwork,
				Status:             metav1.ConditionFalse,
				Reason:             v1alpha1.ReasonNotReady,
				Message:            "Failed to apply network labels: " + err.Error(),
				LastTransitionTime: metav1.Now(),
			})
		}); sErr != nil {
			log.Error(sErr, "failed to record JoiningNetwork condition")
		}
		return ctrl.Result{}, err
	}
	if changed {
		return ctrl.Result{RequeueAfter: 100 * time.Millisecond}, nil
	}

	address, err := r.IPAM.AllocateIP(ctx, &network, peer)
	if err != nil {
		if _, sErr := r.updateStatus(ctx, peer, func(p *v1alpha1.LatticePeer) {
			p.Status.Phase = v1alpha1.NodePhaseFailed
			p.Status.Conditions = setCondition(p.Status.Conditions, metav1.Condition{
				Type:               v1alpha1.NodeConditionIPAllocated,
				Status:             metav1.ConditionFalse,
				Reason:             v1alpha1.ReasonAllocationFailed,
				Message:            err.Error(),
				LastTransitionTime: metav1.Now(),
			})
		}); sErr != nil {
			log.Error(sErr, "failed to record IPAllocated condition")
		}
		return ctrl.Result{}, err
	}
	log.Info("IP allocated", "address", address)

	if _, err = r.updateStatus(ctx, peer, func(p *v1alpha1.LatticePeer) {
		p.Status.Phase = v1alpha1.NodePhaseReady
		p.Status.AllocatedAddress = &address
		p.Status.ActiveNetwork = p.Spec.Network
		p.Status.Conditions = setCondition(p.Status.Conditions, metav1.Condition{
			Type:               v1alpha1.NodeConditionJoiningNetwork,
			Status:             metav1.ConditionTrue,
			Reason:             v1alpha1.ReasonReady,
			LastTransitionTime: metav1.Now(),
		})
		p.Status.Conditions = setCondition(p.Status.Conditions, metav1.Condition{
			Type:               v1alpha1.NodeConditionIPAllocated,
			Status:             metav1.ConditionTrue,
			Reason:             v1alpha1.ReasonReady,
			LastTransitionTime: metav1.Now(),
		})
	}); err != nil {
		return ctrl.Result{}, err
	}

	return r.lastReconcile(ctx, peer, req)
}

// leaveNetwork releases the peer's IP, clears its network labels and status,
// then marks the peer Ready (idle).
func (r *PeerReconciler) leaveNetwork(ctx context.Context, peer *v1alpha1.LatticePeer, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Release the allocated IP back to the pool before wiping status.
	if peer.Status.AllocatedAddress != nil {
		if err := r.IPAM.ReleaseIP(ctx, peer.Namespace, *peer.Status.AllocatedAddress); err != nil {
			log.Error(err, "failed to release IP", "address", *peer.Status.AllocatedAddress)
			return ctrl.Result{}, err
		}
		log.Info("IP released", "address", *peer.Status.AllocatedAddress)
	}

	if _, err := r.updateSpec(ctx, peer, func(p *v1alpha1.LatticePeer) error {
		lbls := p.GetLabels()
		for label := range lbls {
			if strings.HasPrefix(label, "alattice.io/network-") {
				delete(lbls, label)
			}
		}
		p.SetLabels(lbls)
		return nil
	}); err != nil {
		log := logf.FromContext(ctx)
		if _, sErr := r.updateStatus(ctx, peer, func(p *v1alpha1.LatticePeer) {
			p.Status.Phase = v1alpha1.NodePhaseFailed
			p.Status.Conditions = setCondition(p.Status.Conditions, metav1.Condition{
				Type:               v1alpha1.NodeConditionProvisioned,
				Status:             metav1.ConditionFalse,
				Reason:             v1alpha1.ReasonNotReady,
				Message:            "Failed to remove network labels: " + err.Error(),
				LastTransitionTime: metav1.Now(),
			})
		}); sErr != nil {
			log.Error(sErr, "failed to record Provisioned condition")
		}
		return ctrl.Result{}, err
	}

	if _, err := r.updateStatus(ctx, peer, func(p *v1alpha1.LatticePeer) {
		p.Status.ActiveNetwork = nil
		p.Status.AllocatedAddress = nil
		p.Status.Phase = v1alpha1.NodePhaseReady
		p.Status.Conditions = setCondition(p.Status.Conditions, metav1.Condition{
			Type:               v1alpha1.NodeConditionJoiningNetwork,
			Status:             metav1.ConditionFalse,
			Reason:             v1alpha1.ReasonLeaving,
			LastTransitionTime: metav1.Now(),
		})
	}); err != nil {
		return ctrl.Result{}, err
	}

	// If the peer is switching networks (Spec.Network != nil), the status/label patches
	// above do not bump Generation, so GenerationChangedPredicate will not enqueue a new
	// reconcile. Requeue explicitly so handleReady can detect the pending join and advance
	// to Pending → joinNetwork. For a true leave (Spec.Network == nil) no further
	// reconcile is needed, so fall through to lastReconcile to flush the ConfigMap.
	if peer.Spec.Network != nil {
		return ctrl.Result{RequeueAfter: 100 * time.Millisecond}, nil
	}

	return r.lastReconcile(ctx, peer, req)
}

// lastReconcile create or update the configmap
func (r *PeerReconciler) lastReconcile(ctx context.Context, peer *v1alpha1.LatticePeer, request ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.Info("Last reconciling", "name", request.NamespacedName)

	configMapName := fmt.Sprintf("%s-config", peer.Name)

	// 1) Rebuild the snapshot on every reconcile (no stale-change detection).
	snapshot := r.getPeerStateSnapshot(ctx, peer, request)

	// 2) Compute peers/rules from LatticePolicy and generate the final message.
	message, err := r.generator.generate(ctx, peer, snapshot, r.generator.generateConfigVersion())
	if err != nil {
		return ctrl.Result{}, err
	}

	var newHash string
	newHash, err = computeMessageHash(message)
	if err != nil {
		return ctrl.Result{}, err
	}

	desiredConfigMap := r.newConfigmap(peer.Namespace, configMapName, message.String(), newHash)
	if err = controllerutil.SetControllerReference(peer, desiredConfigMap, r.Scheme); err != nil {
		logger.Error(err, "Failed to set owner reference on configmap")
		return ctrl.Result{}, err
	}

	// 3) Fetch the existing ConfigMap and compare hashes; update only when they differ.
	var found corev1.ConfigMap
	err = r.Get(ctx, types.NamespacedName{Name: configMapName, Namespace: peer.Namespace}, &found)
	if err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}

		logger.Info("Creating configmap", "name", configMapName, "hash", newHash)
		manager := client.FieldOwner("lattice-controller-manager")
		if err := r.Patch(ctx, desiredConfigMap, client.Apply, manager); err != nil {
			logger.Error(err, "Failed to create configmap")
			if _, sErr := r.updateStatus(ctx, peer, func(p *v1alpha1.LatticePeer) {
				p.Status.Conditions = setCondition(p.Status.Conditions, metav1.Condition{
					Type:               v1alpha1.NodeConditionNetworkConfigured,
					Status:             metav1.ConditionFalse,
					Reason:             v1alpha1.ReasonConfigFailed,
					Message:            "Failed to create config: " + err.Error(),
					LastTransitionTime: metav1.Now(),
				})
			}); sErr != nil {
				logger.Error(sErr, "failed to record NetworkConfigured condition")
			}
			return ctrl.Result{}, err
		}

		// Also persist hash in status so the next reconcile (triggered by the
		// ConfigMap Create event) sees CurrentHash == newHash and skips cleanly.
		if _, err := r.updateStatus(ctx, peer, func(node *v1alpha1.LatticePeer) {
			node.Status.CurrentHash = newHash
			node.Status.Conditions = setCondition(node.Status.Conditions, metav1.Condition{
				Type:               v1alpha1.NodeConditionNetworkConfigured,
				Status:             metav1.ConditionTrue,
				Reason:             v1alpha1.ReasonReady,
				LastTransitionTime: metav1.Now(),
			})
		}); err != nil {
			return ctrl.Result{}, err
		}

		if r.Recorder != nil {
			r.Recorder.Eventf(peer, corev1.EventTypeNormal, "ConfigMapCreated",
				"configmap=%s hash=%s version=%s", configMapName, newHash, message.ConfigVersion)
		}
		return ctrl.Result{}, nil
	}

	// Fix oldHash using peer status CurrentHash
	oldHash := peer.Status.CurrentHash

	if oldHash == newHash {
		logger.Info("Configmap unchanged by hash, skipping update", "name", configMapName, "hash", newHash)
		return ctrl.Result{}, nil
	}

	logger.Info("Updating configmap by hash", "namespace", peer.Namespace, "name", configMapName, "oldHash", oldHash, "newHash", newHash)
	manager := client.FieldOwner("lattice-controller-manager")
	if err := r.Patch(ctx, desiredConfigMap, client.Apply, manager); err != nil {
		logger.Error(err, "Failed to update configmap")
		if _, sErr := r.updateStatus(ctx, peer, func(p *v1alpha1.LatticePeer) {
			p.Status.Conditions = setCondition(p.Status.Conditions, metav1.Condition{
				Type:               v1alpha1.NodeConditionNetworkConfigured,
				Status:             metav1.ConditionFalse,
				Reason:             v1alpha1.ReasonConfigFailed,
				Message:            "Failed to update config: " + err.Error(),
				LastTransitionTime: metav1.Now(),
			})
		}); sErr != nil {
			logger.Error(sErr, "failed to record NetworkConfigured condition")
		}
		return ctrl.Result{}, err
	}

	ok, err := r.updateStatus(ctx, peer, func(node *v1alpha1.LatticePeer) {
		node.Status.CurrentHash = newHash
		node.Status.Conditions = setCondition(node.Status.Conditions, metav1.Condition{
			Type:               v1alpha1.NodeConditionNetworkConfigured,
			Status:             metav1.ConditionTrue,
			Reason:             v1alpha1.ReasonReady,
			LastTransitionTime: metav1.Now(),
		})
	})

	if err != nil {
		return ctrl.Result{}, err
	}

	if ok {
		return ctrl.Result{}, nil
	}

	if r.Recorder != nil {
		r.Recorder.Eventf(peer, corev1.EventTypeNormal, "ConfigMapUpdated",
			"configmap=%s oldHash=%s newHash=%s version=%s", configMapName, oldHash, newHash, message.ConfigVersion)
	}
	return ctrl.Result{}, nil
}

func (r *PeerReconciler) newConfigmap(namespace, configMapName, message, hash string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "lattice-controller",
			},
			Annotations: map[string]string{
				"alattice.io/config-hash": hash,
			},
		},
		Data: map[string]string{
			"config.json": message,
		},
	}
}

func computeMessageHash(msg *infra.Message) (string, error) {
	// Exclude ConfigVersion and Timestamp to keep the hash stable across reconciles.
	tmp := struct {
		*infra.Message
		ConfigVersion interface{} `json:"configVersion,omitempty"` // shadow to exclude
		Timestamp     interface{} `json:"timestamp,omitempty"`     // shadow to exclude
	}{
		Message:       msg,
		ConfigVersion: nil,
		Timestamp:     nil,
	}

	b, err := json.Marshal(tmp)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(b)), nil
}

// updateSpec applies updateFunc to a deep copy of node and patches only when
// something actually changed. Returns (true, nil) when a patch was written so
// the caller can exit early and let the next reconcile continue the flow.
func (r *PeerReconciler) updateSpec(ctx context.Context, node *v1alpha1.LatticePeer, updateFunc func(node *v1alpha1.LatticePeer) error) (bool, error) {
	log := logf.FromContext(ctx)

	nodeCopy := node.DeepCopy()
	if err := updateFunc(nodeCopy); err != nil {
		log.Error(err, "Failed to build LatticePeer Spec update")
		return false, err
	}

	// Check before patching — skip empty API calls that could spuriously
	// update ResourceVersion and trigger further watch events.
	if reflect.DeepEqual(nodeCopy.Spec, node.Spec) &&
		reflect.DeepEqual(nodeCopy.Labels, node.Labels) &&
		reflect.DeepEqual(nodeCopy.Annotations, node.Annotations) {
		return false, nil
	}

	if err := r.Patch(ctx, nodeCopy, client.MergeFrom(node)); err != nil {
		if errors.IsConflict(err) {
			log.Info("Conflict detected during LatticePeer Spec patch, will retry on next reconcile.")
			return false, nil
		}
		log.Error(err, "Failed to patch LatticePeer Spec")
		return false, err
	}

	// Sync patched state back into the caller's pointer so subsequent patches
	// in the same reconcile use the updated ResourceVersion (avoids conflicts).
	node.Spec = nodeCopy.Spec
	node.Labels = nodeCopy.Labels
	node.Annotations = nodeCopy.Annotations
	node.ResourceVersion = nodeCopy.ResourceVersion

	log.Info("LatticePeer Spec/Metadata patched")
	return true, nil
}

// updateStatus applies updateFunc to a deep copy of node and patches the status
// subresource only when something actually changed.
func (r *PeerReconciler) updateStatus(ctx context.Context, node *v1alpha1.LatticePeer, updateFunc func(node *v1alpha1.LatticePeer)) (bool, error) {
	log := logf.FromContext(ctx)

	nodeCopy := node.DeepCopy()
	updateFunc(nodeCopy)

	// Check before patching — status patches that produce no diff still
	// update ResourceVersion on some API server versions.
	if reflect.DeepEqual(nodeCopy.Status, node.Status) {
		return false, nil
	}

	if err := r.Status().Patch(ctx, nodeCopy, client.MergeFrom(node)); err != nil {
		if errors.IsConflict(err) {
			log.Info("Conflict detected during LatticePeer Status patch, will retry on next reconcile.")
			return false, nil
		}
		log.Error(err, "Failed to patch LatticePeer Status")
		return false, err
	}

	// Sync patched state back so the next patch in the same reconcile sees the
	// updated ResourceVersion without an extra r.Get().
	node.Status = nodeCopy.Status
	node.ResourceVersion = nodeCopy.ResourceVersion

	log.Info("LatticePeer Status patched")
	return true, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PeerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorderFor("lattice-peer-controller")
	}

	if r.generator == nil {
		r.generator = NewGenerator(mgr.GetClient())
	}

	// Predicate that fires only on Update events.
	onlyUpdatePredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			// The initial list-and-watch bulk load fires Create events; ignore them.
			return false
		},
	}

	ownedCMPredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetLabels()["app.kubernetes.io/managed-by"] == "lattice-controller"
	})

	configMapPredicate := predicate.Funcs{
		// Ignore Create: after the controller creates the ConfigMap, Status.CurrentHash
		// is already written, so the next reconcile will skip cleanly without re-enqueue.
		CreateFunc: func(e event.CreateEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldCm := e.ObjectOld.(*corev1.ConfigMap)
			newCm := e.ObjectNew.(*corev1.ConfigMap)
			// Only trigger when Data actually changed (self-heal external edits).
			return !reflect.DeepEqual(oldCm.Data, newCm.Data)
		},
	}
	// Watch LatticeNetwork for spec changes (generation bump) and for the moment
	// ActiveCIDR transitions from empty to non-empty (status patch).
	// A custom predicate is needed because GenerationChangedPredicate only detects
	// spec changes, while NetworkReconciler assigns ActiveCIDR via a status patch
	// that does not bump generation.
	networkReadyPredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldNet, ok1 := e.ObjectOld.(*v1alpha1.LatticeNetwork)
			newNet, ok2 := e.ObjectNew.(*v1alpha1.LatticeNetwork)
			if !ok1 || !ok2 {
				return false
			}
			// Trigger peer reconcile on spec change or when ActiveCIDR is first assigned.
			return oldNet.Generation != newNet.Generation ||
				(oldNet.Status.ActiveCIDR == "" && newNet.Status.ActiveCIDR != "")
		},
		DeleteFunc:  func(e event.DeleteEvent) bool { return false },
		GenericFunc: func(e event.GenericEvent) bool { return false },
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.LatticePeer{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&v1alpha1.LatticeNetwork{},
			handler.EnqueueRequestsFromMapFunc(r.mapNetworkForNodes),
			builder.WithPredicates(networkReadyPredicate)).
		Watches(&v1alpha1.LatticeEndpoint{},
			handler.EnqueueRequestsFromMapFunc(r.mapEndpointForNodes),
			builder.WithPredicates(predicate.And(onlyUpdatePredicate, predicate.GenerationChangedPredicate{}))).
		Watches(&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(r.mapConfigMapForNodes),
			builder.WithPredicates(predicate.And(configMapPredicate, ownedCMPredicate))).
		Watches(&v1alpha1.LatticePolicy{},
			handler.EnqueueRequestsFromMapFunc(r.mapPolicyForNodes),
			// Do not add onlyUpdatePredicate: a newly created policy (Create event) must
			// trigger peer reconcile to push the config down.
			// GenerationChangedPredicate passes Create/Delete by default and only filters
			// Updates where generation has not changed.
			builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Named("node").WithOptions(controller.Options{
		MaxConcurrentReconciles: 5,
	}).Complete(r)
}

// mapNetworkForNodes returns a list of Reconcile Requests for Peers that should be updated based on the given LatticeNetwork.
func (r *PeerReconciler) mapNetworkForNodes(ctx context.Context, obj client.Object) []reconcile.Request {
	network := obj.(*v1alpha1.LatticeNetwork)
	var requests []reconcile.Request

	// Only list peers that have actually joined this network (via the network label)
	// to avoid an empty PeerSelector matching all peers in the namespace.
	networkLabel := networkLabelKey(network.Name)
	nodeList := &v1alpha1.LatticePeerList{}
	if err := r.List(ctx, nodeList, client.InNamespace(network.Namespace), client.MatchingLabels(map[string]string{networkLabel: "true"})); err != nil {
		return nil
	}

	for _, node := range nodeList.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: node.Namespace,
				Name:      node.Name,
			},
		})
	}
	return requests
}

func (r *PeerReconciler) mapConfigMapForNodes(ctx context.Context, obj client.Object) []reconcile.Request {
	cm := obj.(*corev1.ConfigMap)
	var requests []reconcile.Request

	var node v1alpha1.LatticePeer
	name := strings.TrimSuffix(cm.Name, "-config")
	if err := r.Get(ctx, types.NamespacedName{Namespace: cm.Namespace, Name: name}, &node); err != nil {
		return nil
	}

	requests = append(requests, reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: node.Namespace,
			Name:      node.Name,
		},
	})
	return requests
}

func (r *PeerReconciler) mapEndpointForNodes(ctx context.Context, obj client.Object) []reconcile.Request {
	endpoint := obj.(*v1alpha1.LatticeEndpoint)
	var requests []reconcile.Request

	peerList := &v1alpha1.LatticePeerList{}
	listOpts := []client.ListOption{
		client.InNamespace(endpoint.Namespace),
	}

	if err := r.List(ctx, peerList, listOpts...); err != nil {
		return nil
	}

	for _, item := range peerList.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: item.Namespace,
				Name:      item.Name,
			},
		})
	}

	return requests
}

// mapPolicyForNodes returns reconcile requests for peers affected by a policy change.
// Peers are scoped to the policy's network to avoid triggering peers from other
// networks in the same namespace (an empty PeerSelector would otherwise match all).
func (r *PeerReconciler) mapPolicyForNodes(ctx context.Context, obj client.Object) []reconcile.Request {
	policy := obj.(*v1alpha1.LatticePolicy)

	selector, err := metav1.LabelSelectorAsSelector(&policy.Spec.PeerSelector)
	if err != nil {
		return nil
	}

	listOpts := []client.ListOption{
		client.InNamespace(policy.Namespace),
		client.MatchingLabelsSelector{Selector: selector},
	}
	// Scope to the policy's network so that an empty PeerSelector only reaches
	// peers that are actually part of this network, not all peers in the namespace.
	if policy.Spec.Network != "" {
		listOpts = append(listOpts, client.MatchingLabels{
			networkLabelKey(policy.Spec.Network): "true",
		})
	}

	var nodeList v1alpha1.LatticePeerList
	if err = r.List(ctx, &nodeList, listOpts...); err != nil {
		return nil
	}

	requests := make([]reconcile.Request, 0, len(nodeList.Items))
	for _, node := range nodeList.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: node.Namespace,
				Name:      node.Name,
			},
		})
	}
	return requests
}

// getNetwork returns the LatticeNetwork that the peer's Spec.Network points to.
func (r *PeerReconciler) getNetwork(ctx context.Context, peer *v1alpha1.LatticePeer) (*v1alpha1.LatticeNetwork, error) {
	var network v1alpha1.LatticeNetwork
	if err := r.Get(ctx, types.NamespacedName{Namespace: peer.Namespace, Name: *peer.Spec.Network}, &network); err != nil {
		return nil, fmt.Errorf("failed to get joined network: %w", err)
	}

	return &network, nil
}

func (r *PeerReconciler) getPeerStateSnapshot(ctx context.Context, current *v1alpha1.LatticePeer, req ctrl.Request) *PeerStateSnapshot {
	var (
		err error
	)
	snapshot := &PeerStateSnapshot{
		Peer:   current,
		Labels: current.GetLabels(),
	}

	if current.Spec.Network != nil {
		networkName := *current.Spec.Network
		var network v1alpha1.LatticeNetwork
		if err = r.Get(ctx, types.NamespacedName{
			Namespace: req.Namespace, Name: networkName,
		}, &network); err != nil {
			return snapshot
		}
		snapshot.Network = &network

		var peerList *v1alpha1.LatticePeerList
		peerList, err = r.findPeersByNetwork(ctx, &network)
		if err != nil {
			return snapshot
		}
		for _, item := range peerList.Items {
			snapshot.Peers = append(snapshot.Peers, &item)
		}
	}

	policyList, err := r.filterPoliciesForNode(ctx, snapshot.Peer)
	if err != nil {
		return snapshot
	}

	snapshot.Policies = policyList

	return snapshot
}

func (r *PeerReconciler) findPeersByNetwork(ctx context.Context, network *v1alpha1.LatticeNetwork) (*v1alpha1.LatticePeerList, error) {
	labels := map[string]string{
		networkLabelKey(network.Name): "true",
	}

	var peers v1alpha1.LatticePeerList
	if err := r.List(ctx, &peers, client.InNamespace(network.Namespace), client.MatchingLabels(labels)); err != nil {
		return nil, err
	}

	return &peers, nil
}

func (r *PeerReconciler) filterPoliciesForNode(ctx context.Context, peer *v1alpha1.LatticePeer) ([]*v1alpha1.LatticePolicy, error) {
	var policyList v1alpha1.LatticePolicyList
	if err := r.List(ctx, &policyList, client.InNamespace(peer.Namespace)); err != nil {
		return nil, err
	}

	matched := make([]*v1alpha1.LatticePolicy, 0)
	nodeLabelSet := labels.Set(peer.Labels)

	peerNetwork := ""
	if peer.Spec.Network != nil {
		peerNetwork = *peer.Spec.Network
	}

	for _, policy := range policyList.Items {
		// Only match policies from the same network to prevent cross-network policies
		// from affecting this peer's config hash.
		if policy.Spec.Network != peerNetwork {
			continue
		}

		selector, err := metav1.LabelSelectorAsSelector(&policy.Spec.PeerSelector)
		if err != nil {
			return nil, fmt.Errorf("invalid nodeSelector in policy %s/%s: %w", policy.Namespace, policy.Name, err)
		}

		// An empty selector {} matches all objects.
		if selector.Matches(nodeLabelSet) {
			matched = append(matched, &policy)
		}
	}

	return matched, nil
}
