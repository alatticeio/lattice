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

package service_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/alatticeio/lattice/api/v1alpha1"
	agentconfig "github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/alatticeio/lattice/internal/server/service"
)

// fakeAgentIdentityReader satisfies the AgentIdentityReader interface.
type fakeAgentIdentityReader struct {
	identities map[string]*v1alpha1.AgentIdentity
}

func (f *fakeAgentIdentityReader) Get(ctx context.Context, namespace, name string) (*v1alpha1.AgentIdentity, error) {
	key := namespace + "/" + name
	id, ok := f.identities[key]
	if !ok {
		return nil, fmt.Errorf("not found: %s", key)
	}
	return id, nil
}

func makeTestClaims(agentID, namespace string, tools []string) *models.AgentClaims {
	return &models.AgentClaims{
		AgentID:      agentID,
		Namespace:    namespace,
		AllowedTools: tools,
	}
}

func TestAgentIsolationService_AllowedTool(t *testing.T) {
	reader := &fakeAgentIdentityReader{
		identities: map[string]*v1alpha1.AgentIdentity{
			"ws-a/agent-monitor": {
				Spec: v1alpha1.AgentIdentitySpec{
					AllowedTools:      []string{"list_peers", "check_connectivity"},
					AllowedNamespaces: []string{"ws-a"},
					EnforcementMode:   v1alpha1.EnforcementEnforce,
				},
			},
		},
	}
	cfg := agentconfig.AgentIsolationConfig{
		Enabled:         true,
		EnforcementMode: "enforce",
	}
	svc := service.NewAgentIsolationService(cfg, reader)
	claims := makeTestClaims("agent-monitor", "ws-a", []string{"list_peers"})

	ctx := service.ContextWithAgentClaims(context.Background(), claims)
	err := svc.CheckToolAccess(ctx, "ws-a", "list_peers")
	if err != nil {
		t.Errorf("expected allowed, got: %v", err)
	}
}

func TestAgentIsolationService_DeniedTool(t *testing.T) {
	reader := &fakeAgentIdentityReader{
		identities: map[string]*v1alpha1.AgentIdentity{
			"ws-a/agent-monitor": {
				Spec: v1alpha1.AgentIdentitySpec{
					AllowedTools:      []string{"list_peers"},
					AllowedNamespaces: []string{"ws-a"},
					EnforcementMode:   v1alpha1.EnforcementEnforce,
				},
			},
		},
	}
	cfg := agentconfig.AgentIsolationConfig{Enabled: true, EnforcementMode: "enforce"}
	svc := service.NewAgentIsolationService(cfg, reader)
	claims := makeTestClaims("agent-monitor", "ws-a", []string{"list_peers"})

	ctx := service.ContextWithAgentClaims(context.Background(), claims)
	err := svc.CheckToolAccess(ctx, "ws-a", "delete_peer")
	if err == nil {
		t.Error("expected error for disallowed tool")
	}
}

func TestAgentIsolationService_AuditModeDoesNotBlock(t *testing.T) {
	reader := &fakeAgentIdentityReader{
		identities: map[string]*v1alpha1.AgentIdentity{
			"ws-a/agent-monitor": {
				Spec: v1alpha1.AgentIdentitySpec{
					AllowedTools:      []string{"list_peers"},
					AllowedNamespaces: []string{"ws-a"},
					EnforcementMode:   v1alpha1.EnforcementAudit,
				},
			},
		},
	}
	cfg := agentconfig.AgentIsolationConfig{Enabled: true, EnforcementMode: "audit"}
	svc := service.NewAgentIsolationService(cfg, reader)
	claims := makeTestClaims("agent-monitor", "ws-a", []string{"list_peers"})

	ctx := service.ContextWithAgentClaims(context.Background(), claims)
	// delete_peer is not in allowedTools, but audit mode should not block
	err := svc.CheckToolAccess(ctx, "ws-a", "delete_peer")
	if err != nil {
		t.Errorf("audit mode should not block: %v", err)
	}
}

func TestAgentIsolationService_DisabledMode(t *testing.T) {
	// When Enabled=false, NewAgentIsolationService returns nil.
	// Test the exported helper directly — with nil service, need to handle nil.
	// This test verifies the disabled-mode behavior via a nil-mode service
	// that CheckToolAccess skips when service is nil.
	cfg := agentconfig.AgentIsolationConfig{Enabled: false}
	svc := service.NewAgentIsolationService(cfg, nil)

	if svc != nil {
		t.Error("expected nil service when disabled")
	}
}

func TestAgentIsolationService_NoAgentClaims_SkipsCheck(t *testing.T) {
	// No agent claims in context → human user → skip RBAC
	reader := &fakeAgentIdentityReader{identities: map[string]*v1alpha1.AgentIdentity{}}
	cfg := agentconfig.AgentIsolationConfig{Enabled: true, EnforcementMode: "enforce"}
	svc := service.NewAgentIsolationService(cfg, reader)

	// No ContextWithAgentClaims called — plain context
	err := svc.CheckToolAccess(context.Background(), "ws-a", "delete_peer")
	if err != nil {
		t.Errorf("no agent claims should skip check: %v", err)
	}
}

func TestAgentIsolationService_BlocksExpiredIdentity(t *testing.T) {
	reader := &fakeAgentIdentityReader{
		identities: map[string]*v1alpha1.AgentIdentity{
			"default/claude-1": {
				Spec: v1alpha1.AgentIdentitySpec{
					AllowedTools:      []string{"list_peers"},
					AllowedNamespaces: []string{"default"},
				},
				Status: v1alpha1.AgentIdentityStatus{
					Phase: v1alpha1.AgentPhaseExpired,
				},
			},
		},
	}

	cfg := agentconfig.AgentIsolationConfig{Enabled: true, EnforcementMode: "enforce"}
	svc := service.NewAgentIsolationService(cfg, reader)
	claims := makeTestClaims("claude-1", "default", []string{"list_peers"})
	ctx := service.ContextWithAgentClaims(context.Background(), claims)

	err := svc.CheckToolAccess(ctx, "default", "list_peers")
	if err == nil {
		t.Error("expected error for expired identity")
	}
	if err != nil && !containsStr(err.Error(), "expired") {
		t.Errorf("expected 'expired' in error message, got: %v", err)
	}
}

func TestAgentIsolationService_BlocksRevokedIdentity(t *testing.T) {
	reader := &fakeAgentIdentityReader{
		identities: map[string]*v1alpha1.AgentIdentity{
			"default/claude-1": {
				Spec: v1alpha1.AgentIdentitySpec{
					AllowedTools:      []string{"list_peers"},
					AllowedNamespaces: []string{"default"},
				},
				Status: v1alpha1.AgentIdentityStatus{
					Phase: v1alpha1.AgentPhaseRevoked,
				},
			},
		},
	}

	cfg := agentconfig.AgentIsolationConfig{Enabled: true, EnforcementMode: "enforce"}
	svc := service.NewAgentIsolationService(cfg, reader)
	claims := makeTestClaims("claude-1", "default", []string{"list_peers"})
	ctx := service.ContextWithAgentClaims(context.Background(), claims)

	err := svc.CheckToolAccess(ctx, "default", "list_peers")
	if err == nil {
		t.Error("expected error for revoked identity")
	}
	if err != nil && !containsStr(err.Error(), "revoked") {
		t.Errorf("expected 'revoked' in error message, got: %v", err)
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
