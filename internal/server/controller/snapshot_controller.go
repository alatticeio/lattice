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
	"sync"
	"time"

	"github.com/alatticeio/lattice/api/v1alpha1"
	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/models"

	"github.com/google/uuid"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// SnapshotController captures NetworkSnapshots on CRD change events.
// It is debounced: multiple changes within 1 second produce a single snapshot.
type SnapshotController struct {
	client    client.Client
	snapStore store.NetworkSnapshotRepository
	wsStore   store.WorkspaceRepository

	mu           sync.Mutex
	lastCapture  map[string]time.Time // namespace -> last capture time
	workspaceIDs map[string]string    // namespace -> workspaceID (cached)
}

func NewSnapshotController(c client.Client, snapStore store.NetworkSnapshotRepository, wsStore store.WorkspaceRepository) *SnapshotController {
	return &SnapshotController{
		client:       c,
		snapStore:    snapStore,
		wsStore:      wsStore,
		lastCapture:  make(map[string]time.Time),
		workspaceIDs: make(map[string]string),
	}
}

func (r *SnapshotController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	namespace := req.Namespace

	// Debounce: skip if we captured within the last second.
	r.mu.Lock()
	last := r.lastCapture[namespace]
	if time.Since(last) < time.Second {
		r.mu.Unlock()
		return reconcile.Result{RequeueAfter: time.Second}, nil
	}
	r.lastCapture[namespace] = time.Now()
	r.mu.Unlock()

	if err := r.captureSnapshot(ctx, namespace, "policy_change", "system"); err != nil {
		log.GetLogger("snapshot-ctrl").Warn("snapshot capture failed", "namespace", namespace, "err", err)
	}
	return reconcile.Result{}, nil
}

func (r *SnapshotController) captureSnapshot(ctx context.Context, namespace, triggerType, triggerBy string) error {
	// Resolve workspaceID from namespace.
	wsID := r.resolveWorkspaceID(ctx, namespace)

	var peers v1alpha1.LatticePeerList
	_ = r.client.List(ctx, &peers, client.InNamespace(namespace))

	var policies v1alpha1.LatticePolicyList
	_ = r.client.List(ctx, &policies, client.InNamespace(namespace))

	var networks v1alpha1.LatticeNetworkList
	_ = r.client.List(ctx, &networks, client.InNamespace(namespace))

	peersJSON, _ := json.Marshal(simplifyPeers(peers.Items))
	policiesJSON, _ := json.Marshal(simplifyPolicies(policies.Items))
	networksJSON, _ := json.Marshal(simplifyNetworks(networks.Items))

	snap := &models.NetworkSnapshot{
		ID:          uuid.New().String(),
		WorkspaceID: wsID,
		Namespace:   namespace,
		CapturedAt:  time.Now().UTC(),
		TriggerType: triggerType,
		TriggerBy:   triggerBy,
		Peers:       string(peersJSON),
		Policies:    string(policiesJSON),
		Networks:    string(networksJSON),
		Presence:    "{}",
	}
	return r.snapStore.Create(ctx, snap)
}

type simplePeer struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels,omitempty"`
	IP     string            `json:"ip,omitempty"`
}

func simplifyPeers(peers []v1alpha1.LatticePeer) []simplePeer {
	result := make([]simplePeer, 0, len(peers))
	for _, p := range peers {
		sp := simplePeer{Name: p.Name, Labels: p.Labels}
		if p.Status.AllocatedAddress != nil {
			sp.IP = *p.Status.AllocatedAddress
		}
		result = append(result, sp)
	}
	return result
}

type simplePolicy struct {
	Name     string `json:"name"`
	Action   string `json:"action"`
	Network  string `json:"network"`
	Selector string `json:"selector,omitempty"`
}

func simplifyPolicies(policies []v1alpha1.LatticePolicy) []simplePolicy {
	result := make([]simplePolicy, 0, len(policies))
	for _, p := range policies {
		sel, _ := json.Marshal(p.Spec.PeerSelector)
		result = append(result, simplePolicy{
			Name:     p.Name,
			Action:   p.Spec.Action,
			Network:  p.Spec.Network,
			Selector: string(sel),
		})
	}
	return result
}

type simpleNetwork struct {
	Name  string `json:"name"`
	Phase string `json:"phase"`
	CIDR  string `json:"cidr,omitempty"`
}

func simplifyNetworks(networks []v1alpha1.LatticeNetwork) []simpleNetwork {
	result := make([]simpleNetwork, 0, len(networks))
	for _, n := range networks {
		result = append(result, simpleNetwork{Name: n.Name, Phase: string(n.Status.Phase), CIDR: n.Status.ActiveCIDR})
	}
	return result
}

func (r *SnapshotController) SetupWithManager(mgr ctrl.Manager) error {
	r.client = mgr.GetClient()
	return ctrl.NewControllerManagedBy(mgr).
		Named("snapshot").
		For(&v1alpha1.LatticePolicy{}).
		Complete(r)
}

// resolveWorkspaceID resolves a K8s namespace to a workspace ID.
// Results are cached to avoid repeated DB lookups.
func (r *SnapshotController) resolveWorkspaceID(ctx context.Context, namespace string) string {
	r.mu.Lock()
	if id, ok := r.workspaceIDs[namespace]; ok {
		r.mu.Unlock()
		return id
	}
	r.mu.Unlock()

	if r.wsStore == nil {
		return ""
	}
	ws, err := r.wsStore.GetByNamespace(ctx, namespace)
	if err != nil {
		return ""
	}

	r.mu.Lock()
	r.workspaceIDs[namespace] = ws.ID
	r.mu.Unlock()
	return ws.ID
}
