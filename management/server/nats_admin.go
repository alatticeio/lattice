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

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"wireflow/internal/infra"
	"wireflow/management/dto"
)

// ── request types ─────────────────────────────────────────────────────────────

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
	Token string `json:"token"` // token value (not ID)
}

// ── helper ────────────────────────────────────────────────────────────────────

// workspaceCtxByNs looks up the workspace record for the given K8s namespace
// and injects its UUID into the returned context under infra.WorkspaceKey.
// Policy and token services require this to scope K8s CRD operations correctly.
func (s *Server) workspaceCtxByNs(parent context.Context, namespace string) (context.Context, error) {
	ws, err := s.store.Workspaces().GetByNamespace(parent, namespace)
	if err != nil {
		return nil, fmt.Errorf("no workspace for namespace %q — run 'wireflow workspace list': %w", namespace, err)
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
	vo, err := s.policyController.CreateOrUpdatePolicy(ctx, &dto.PolicyDto{
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

func (s *Server) NatsRemoveToken(data []byte) ([]byte, error) {
	var req tokenRemoveReq
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}
	if req.Token == "" {
		return nil, fmt.Errorf("token is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Look up the token record to find which workspace namespace it belongs to.
	t, err := s.store.Tokens().GetByToken(ctx, req.Token)
	if err != nil {
		return nil, fmt.Errorf("token not found: %w", err)
	}

	ctx, err = s.workspaceCtxByNs(ctx, t.Namespace)
	if err != nil {
		return nil, err
	}
	return nil, s.tokenController.Delete(ctx, req.Token)
}
