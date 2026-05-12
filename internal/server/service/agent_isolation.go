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

package service

import (
	"context"
	"fmt"

	"github.com/alatticeio/lattice/api/v1alpha1"
	agentconfig "github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/server/models"
)

// agentClaimsCtxKey is the context key for AgentClaims injected by middleware or tests.
type agentClaimsCtxKey struct{}

// ContextWithAgentClaims stores AgentClaims in a context (used by middleware and tests).
func ContextWithAgentClaims(ctx context.Context, claims *models.AgentClaims) context.Context {
	return context.WithValue(ctx, agentClaimsCtxKey{}, claims)
}

// agentClaimsFromContext extracts AgentClaims from context.
// Returns nil if no agent claims are present (e.g., user session).
func agentClaimsFromContext(ctx context.Context) *models.AgentClaims {
	v := ctx.Value(agentClaimsCtxKey{})
	if v == nil {
		return nil
	}
	c, _ := v.(*models.AgentClaims)
	return c
}

// AgentIdentityReader reads AgentIdentity CRDs from the K8s API.
// Kept as an interface for testability.
type AgentIdentityReader interface {
	Get(ctx context.Context, namespace, name string) (*v1alpha1.AgentIdentity, error)
}

// AgentIsolationService enforces tool-level RBAC for AI Agents.
// When nil (returned when Enabled=false), callers skip all checks.
type AgentIsolationService interface {
	// CheckToolAccess returns nil if the agent in ctx is allowed to call toolName
	// in namespace. Returns an error if the call should be blocked.
	// In audit mode, always returns nil but logs the violation.
	// If no agent claims are in ctx (human user), always returns nil.
	CheckToolAccess(ctx context.Context, namespace, toolName string) error
}

type agentIsolationService struct {
	logger          *log.Logger
	enforcementMode string // "disabled" | "audit" | "enforce"
	reader          AgentIdentityReader
}

// NewAgentIsolationService creates an AgentIsolationService from config.
// Returns nil if cfg.Enabled is false — callers must nil-check before use.
func NewAgentIsolationService(cfg agentconfig.AgentIsolationConfig, reader AgentIdentityReader) AgentIsolationService {
	if !cfg.Enabled {
		return nil
	}
	mode := cfg.EnforcementMode
	if mode == "" {
		mode = "disabled"
	}
	return &agentIsolationService{
		logger:          log.GetLogger("agent-isolation"),
		enforcementMode: mode,
		reader:          reader,
	}
}

func (s *agentIsolationService) CheckToolAccess(ctx context.Context, namespace, toolName string) error {
	if s.enforcementMode == "disabled" {
		return nil
	}

	claims := agentClaimsFromContext(ctx)
	if claims == nil {
		// No agent claims → human user session, skip agent RBAC.
		return nil
	}

	// Fetch live AgentIdentity from K8s (not cached; CRD is source of truth).
	identity, err := s.reader.Get(ctx, namespace, claims.AgentID)
	if err != nil {
		s.logger.Warn("AgentIdentity not found", "agent", claims.AgentID, "namespace", namespace)
		return s.violation(claims.AgentID, namespace, toolName, "identity_not_found")
	}

	// Reject terminal phases before checking tools/namespaces.
	switch identity.Status.Phase {
	case v1alpha1.AgentPhaseExpired:
		return s.violation(claims.AgentID, namespace, toolName, "expired")
	case v1alpha1.AgentPhaseRevoked:
		return s.violation(claims.AgentID, namespace, toolName, "revoked")
	}

	// Check namespace permission.
	if !containsStr(identity.Spec.AllowedNamespaces, namespace) {
		return s.violation(claims.AgentID, namespace, toolName, "namespace_not_allowed")
	}

	// Check tool permission.
	if !containsStr(identity.Spec.AllowedTools, toolName) {
		return s.violation(claims.AgentID, namespace, toolName, "tool_not_allowed")
	}

	return nil
}

// violation logs the violation and returns an error in enforce mode, nil in audit mode.
func (s *agentIsolationService) violation(agentID, namespace, toolName, reason string) error {
	s.logger.Warn("agent RBAC violation",
		"agent", agentID, "namespace", namespace, "tool", toolName, "reason", reason)
	if s.enforcementMode == "enforce" {
		return fmt.Errorf("agent %q is not permitted to call %q in namespace %q (%s)", agentID, toolName, namespace, reason)
	}
	// audit mode: log but don't block
	return nil
}

func containsStr(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
