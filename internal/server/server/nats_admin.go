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

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/alatticeio/lattice/api/v1alpha1"
	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/server/dto"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ── request types ─────────────────────────────────────────────────────────────

type peerLabelReq struct {
	Namespace string            `json:"namespace"`
	PeerName  string            `json:"peer_name"`
	Labels    map[string]string `json:"labels"`
}

type peerListReq struct {
	Namespace string `json:"namespace"`
}

type allowAllReq struct {
	Namespace string `json:"namespace"`
}

type workspaceAddReq struct {
	Slug        string `json:"slug"`
	Namespace   string `json:"namespace"`    // optional; auto-generated if empty
	DisplayName string `json:"display_name"` // optional; defaults to slug
}

type workspaceRemoveReq struct {
	Namespace string `json:"namespace"`
}

type policyAddReq struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	Action      string `json:"action"`      // ALLOW or DENY (default ALLOW)
	Description string `json:"description"` // optional
}

type policyRemoveReq struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type policyListReq struct {
	Namespace string `json:"namespace"`
}

type tokenListReq struct {
	Namespace string `json:"namespace"` // empty = all namespaces
}

type tokenRemoveReq struct {
	Namespace string `json:"namespace"`
	Token     string `json:"token"` // token value (not ID)
}

// ── helper ────────────────────────────────────────────────────────────────────

// workspaceCtxByNs looks up the workspace record for the given K8s namespace
// and injects its UUID into the returned context under infra.WorkspaceKey.
// Policy and token services require this to scope K8s CRD operations correctly.
func (s *Server) workspaceCtxByNs(parent context.Context, namespace string) (context.Context, error) {
	ws, err := s.store.Workspaces().GetByNamespace(parent, namespace)
	if err != nil {
		return nil, fmt.Errorf("no workspace for namespace %q — run 'lattice workspace list': %w", namespace, err)
	}
	return context.WithValue(parent, infra.WorkspaceKey, ws.ID), nil
}

func marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

// ── workspace handlers ────────────────────────────────────────────────────────

func (s *Server) NatsAddWorkspace(data []byte) ([]byte, error) {
	var req workspaceAddReq
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}
	if req.Slug == "" {
		return nil, fmt.Errorf("slug is required")
	}
	displayName := req.DisplayName
	if displayName == "" {
		displayName = req.Slug
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	vo, err := s.workspaceController.AddWorkspace(ctx, &dto.WorkspaceDto{
		Slug:        req.Slug,
		Namespace:   req.Namespace,
		DisplayName: displayName,
	})
	if err != nil {
		return nil, err
	}
	return marshal(vo)
}

func (s *Server) NatsRemoveWorkspace(data []byte) ([]byte, error) {
	var req workspaceRemoveReq
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}
	if req.Namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ws, err := s.store.Workspaces().GetByNamespace(ctx, req.Namespace)
	if err != nil {
		return nil, fmt.Errorf("workspace not found for namespace %q: %w", req.Namespace, err)
	}
	return nil, s.workspaceController.DeleteWorkspace(ctx, ws.ID)
}

func (s *Server) NatsListWorkspaces(data []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := s.workspaceController.ListWorkspaces(ctx, &dto.PageRequest{Page: 1, PageSize: 200})
	if err != nil {
		return nil, err
	}
	return marshal(result.List)
}

// ── policy handlers ───────────────────────────────────────────────────────────

func (s *Server) NatsAddPolicy(data []byte) ([]byte, error) {
	var req policyAddReq
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}
	if req.Namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	if req.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if req.Action == "" {
		req.Action = "ALLOW"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ctx, err := s.workspaceCtxByNs(ctx, req.Namespace)
	if err != nil {
		return nil, err
	}
	// Empty Ingress/Egress + empty PeerSelector means "match all peers, all ports".
	vo, err := s.policyController.ApplyDirect(ctx, ctx.Value(infra.WorkspaceKey).(string), "", "", &dto.PolicyDto{
		Name:        req.Name,
		Namespace:   req.Namespace,
		Action:      req.Action,
		Description: req.Description,
		PolicyTypes: []string{"Ingress", "Egress"},
	})
	if err != nil {
		return nil, err
	}
	return marshal(vo)
}

func (s *Server) NatsRemovePolicy(data []byte) ([]byte, error) {
	var req policyRemoveReq
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}
	if req.Namespace == "" || req.Name == "" {
		return nil, fmt.Errorf("namespace and name are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ctx, err := s.workspaceCtxByNs(ctx, req.Namespace)
	if err != nil {
		return nil, err
	}
	return nil, s.policyController.DeletePolicy(ctx, req.Name)
}

func (s *Server) NatsListPolicies(data []byte) ([]byte, error) {
	var req policyListReq
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}
	if req.Namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ctx, err := s.workspaceCtxByNs(ctx, req.Namespace)
	if err != nil {
		return nil, err
	}
	result, err := s.policyController.ListPolicy(ctx, &dto.PageRequest{Page: 1, PageSize: 200})
	if err != nil {
		return nil, err
	}
	return marshal(result.List)
}

// ── token handlers ────────────────────────────────────────────────────────────

func (s *Server) NatsListTokens(data []byte) ([]byte, error) {
	var req tokenListReq
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}
	if req.Namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ctx, err := s.workspaceCtxByNs(ctx, req.Namespace)
	if err != nil {
		return nil, err
	}
	tokens, err := s.networkController.ListTokens(ctx, &dto.PageRequest{Page: 1, PageSize: 200})
	if err != nil {
		return nil, err
	}
	return marshal(tokens.List)
}

// ── peer handlers ─────────────────────────────────────────────────────────────

// NatsPeerList lists LatticePeers in the given namespace.
func (s *Server) NatsPeerList(data []byte) ([]byte, error) {
	var req peerListReq
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}
	if req.Namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var peerList v1alpha1.LatticePeerList
	if err := s.client.List(ctx, &peerList, client.InNamespace(req.Namespace)); err != nil {
		return nil, fmt.Errorf("list peers: %w", err)
	}
	type peerRow struct {
		Name    string            `json:"name"`
		AppID   string            `json:"app_id"`
		IP      string            `json:"ip"`
		Network string            `json:"network"`
		Phase   string            `json:"phase"`
		Labels  map[string]string `json:"labels"`
	}
	rows := make([]peerRow, 0, len(peerList.Items))
	for _, p := range peerList.Items {
		ip := ""
		if p.Status.AllocatedAddress != nil {
			ip = *p.Status.AllocatedAddress
		}
		network := ""
		if p.Spec.Network != nil {
			network = *p.Spec.Network
		}
		rows = append(rows, peerRow{
			Name:    p.Name,
			AppID:   p.Spec.AppId,
			IP:      ip,
			Network: network,
			Phase:   string(p.Status.Phase),
			Labels:  p.Labels,
		})
	}
	return marshal(rows)
}

// NatsPeerLabel merges new labels into a LatticePeer's metadata.labels.
func (s *Server) NatsPeerLabel(data []byte) ([]byte, error) {
	var req peerLabelReq
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}
	if req.Namespace == "" || req.PeerName == "" {
		return nil, fmt.Errorf("namespace and peer_name are required")
	}
	if len(req.Labels) == 0 {
		return nil, fmt.Errorf("at least one label key=value is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	peer := &v1alpha1.LatticePeer{}
	if err := s.client.Get(ctx, types.NamespacedName{
		Name:      req.PeerName,
		Namespace: req.Namespace,
	}, peer); err != nil {
		return nil, fmt.Errorf("peer %q not found in %q: %w", req.PeerName, req.Namespace, err)
	}

	original := peer.DeepCopy()
	if peer.Labels == nil {
		peer.Labels = make(map[string]string)
	}
	for k, v := range req.Labels {
		peer.Labels[k] = v
	}
	if err := s.client.Patch(ctx, peer, client.MergeFrom(original)); err != nil {
		return nil, fmt.Errorf("patch peer labels: %w", err)
	}
	return marshal(map[string]any{
		"peer":   req.PeerName,
		"labels": peer.Labels,
	})
}

// NatsAllowAll creates a full-mesh ALLOW policy using the network label selector
// that the peer controller automatically assigns: alattice.io/network-{name}=true.
func (s *Server) NatsAllowAll(data []byte) ([]byte, error) {
	var req allowAllReq
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}
	if req.Namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Find the LatticeNetwork in this namespace to get the network name.
	// The peer controller labels each LatticePeer with
	//   alattice.io/network-{networkName}=true
	// so we need the network name to build the matching PeerSelector.
	var netList v1alpha1.LatticeNetworkList
	if err := s.client.List(ctx, &netList, client.InNamespace(req.Namespace)); err != nil {
		return nil, fmt.Errorf("list LatticeNetworks: %w", err)
	}
	if len(netList.Items) == 0 {
		return nil, fmt.Errorf("no LatticeNetwork found in namespace %q — is the workspace ready?", req.Namespace)
	}
	networkName := netList.Items[0].Name
	labelKey := fmt.Sprintf("alattice.io/network-%s", networkName)

	peerSel := metav1.LabelSelector{
		MatchLabels: map[string]string{labelKey: "true"},
	}

	ctx, err := s.workspaceCtxByNs(ctx, req.Namespace)
	if err != nil {
		return nil, err
	}

	vo, err := s.policyController.ApplyDirect(ctx, ctx.Value(infra.WorkspaceKey).(string), "", "", &dto.PolicyDto{
		Name:        "allow-all",
		Namespace:   req.Namespace,
		Action:      "ALLOW",
		Description: "full-mesh allow-all (created by CLI)",
		PolicyTypes: []string{"Ingress", "Egress"},
		LatticePolicySpec: v1alpha1.LatticePolicySpec{
			Network:      networkName,
			PeerSelector: peerSel,
			Action:       "ALLOW",
			Ingress: []v1alpha1.IngressRule{
				{From: []v1alpha1.PeerSelection{{PeerSelector: &peerSel}}},
			},
			Egress: []v1alpha1.EgressRule{
				{To: []v1alpha1.PeerSelection{{PeerSelector: &peerSel}}},
			},
		},
	})
	if err != nil {
		return nil, err
	}
	return marshal(vo)
}

func (s *Server) NatsRemoveToken(data []byte) ([]byte, error) {
	var req tokenRemoveReq
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}
	if req.Namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	if req.Token == "" {
		return nil, fmt.Errorf("token is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ctx, err := s.workspaceCtxByNs(ctx, req.Namespace)
	if err != nil {
		return nil, err
	}
	// K8s CRD name is always lowercased at creation time (peer.go: strings.ToLower(tokenDto.Name)),
	// but Status.Token retains the original case, so normalise here to match.
	return nil, s.tokenController.Delete(ctx, strings.ToLower(req.Token))
}
