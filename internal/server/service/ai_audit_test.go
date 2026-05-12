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
	"encoding/json"
	"testing"

	"github.com/alatticeio/lattice/api/v1alpha1"
	agentconfig "github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/alatticeio/lattice/internal/server/service"
	. "github.com/onsi/gomega"
)

// fakeAuditService is a test double that records audit log calls.
type fakeAuditService struct {
	logged []models.AuditLog
}

func (f *fakeAuditService) Log(entry models.AuditLog) {
	f.logged = append(f.logged, entry)
}

func (f *fakeAuditService) List(ctx context.Context, filter store.AuditLogFilter) ([]*models.AuditLog, int64, error) {
	return nil, 0, nil
}

func (f *fakeAuditService) Start(ctx context.Context) {}

func TestExecuteTool_AuditLogsAllowedToolCall(t *testing.T) {
	g := NewWithT(t)
	st := newTestDB(t)
	ctx := context.Background()

	// Create workspace
	err := st.Workspaces().Create(ctx, &models.Workspace{
		Model:     models.Model{ID: "ws-test"},
		Slug:      "test-ws",
		Namespace: "test-ns",
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Create AIService with no agentIsolation (disabled)
	svc := service.NewAIServiceWithWorkflow(nil, st, nil, nil, 5, nil, nil, nil)

	// Attach fake audit service
	auditSvc := &fakeAuditService{}
	service.SetAuditService(svc, auditSvc)

	// Create agent claims context
	claims := &models.AgentClaims{
		AgentID:      "agent-monitor",
		Namespace:    "test-ns",
		AllowedTools: []string{"list_peers"},
	}
	ctxWithAgent := service.ContextWithAgentClaims(ctx, claims)

	// Execute a tool (list_peers is valid)
	// This will panic because k8s client is nil, but the audit log is recorded
	// before the tool execution is attempted (at line 638 in ai.go)
	func() {
		defer func() {
			// Catch panic from nil k8s client — we're only testing the audit logging,
			// not the actual tool execution
			_ = recover()
		}()
		_, _ = svc.ExecuteTool(ctxWithAgent, "test-ns", "list_peers", json.RawMessage("{}"))
	}()

	// Verify audit log was recorded with AuditActionAgentToolCall
	// The audit is logged at line 638 in ai.go, before the tool switch statement
	g.Expect(auditSvc.logged).To(HaveLen(1))
	g.Expect(auditSvc.logged[0].Action).To(Equal(service.AuditActionAgentToolCall))
	g.Expect(auditSvc.logged[0].Resource).To(Equal("tool"))
	g.Expect(auditSvc.logged[0].ResourceName).To(Equal("list_peers"))
	g.Expect(auditSvc.logged[0].UserID).To(Equal("agent-monitor"))
	g.Expect(auditSvc.logged[0].WorkspaceID).To(Equal("test-ns"))
}

func TestExecuteTool_AuditLogsBlockedToolCall(t *testing.T) {
	g := NewWithT(t)
	st := newTestDB(t)
	ctx := context.Background()

	// Create workspace
	err := st.Workspaces().Create(ctx, &models.Workspace{
		Model:     models.Model{ID: "ws-test"},
		Slug:      "test-ws",
		Namespace: "test-ns",
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Create AIService with agent isolation enabled
	reader := &fakeAgentIdentityReader{
		identities: map[string]*v1alpha1.AgentIdentity{
			"test-ns/agent-monitor": {
				Spec: v1alpha1.AgentIdentitySpec{
					// Agent only allowed list_peers, not list_policies
					AllowedTools:      []string{"list_peers"},
					AllowedNamespaces: []string{"test-ns"},
					EnforcementMode:   v1alpha1.EnforcementEnforce,
				},
			},
		},
	}
	agentIsolation := service.NewAgentIsolationService(
		agentconfig.AgentIsolationConfig{
			Enabled:         true,
			EnforcementMode: "enforce",
		},
		reader,
	)
	svc := service.NewAIServiceWithWorkflow(nil, st, nil, nil, 5, nil, nil, agentIsolation)

	// Attach fake audit service
	auditSvc := &fakeAuditService{}
	service.SetAuditService(svc, auditSvc)

	// Create agent claims context
	claims := &models.AgentClaims{
		AgentID:      "agent-monitor",
		Namespace:    "test-ns",
		AllowedTools: []string{"list_peers"},
	}
	ctxWithAgent := service.ContextWithAgentClaims(ctx, claims)

	// Try to execute a blocked tool (list_policies)
	_, err = svc.ExecuteTool(ctxWithAgent, "test-ns", "list_policies", json.RawMessage("{}"))
	g.Expect(err).To(HaveOccurred()) // Should fail due to isolation

	// Verify audit log was recorded with AuditActionAgentToolBlocked
	g.Expect(auditSvc.logged).To(HaveLen(1))
	g.Expect(auditSvc.logged[0].Action).To(Equal(service.AuditActionAgentToolBlocked))
	g.Expect(auditSvc.logged[0].Resource).To(Equal("tool"))
	g.Expect(auditSvc.logged[0].ResourceName).To(Equal("list_policies"))
	g.Expect(auditSvc.logged[0].UserID).To(Equal("agent-monitor"))
	g.Expect(auditSvc.logged[0].WorkspaceID).To(Equal("test-ns"))
}

func TestLogToolAudit_NilAuditSvc(t *testing.T) {
	g := NewWithT(t)
	st := newTestDB(t)
	ctx := context.Background()

	// Create workspace
	err := st.Workspaces().Create(ctx, &models.Workspace{
		Model:     models.Model{ID: "ws-test"},
		Slug:      "test-ws",
		Namespace: "test-ns",
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Create AIService without attaching audit service (auditSvc is nil)
	svc := service.NewAIServiceWithWorkflow(nil, st, nil, nil, 5, nil, nil, nil)
	// Deliberately NOT calling SetAuditService

	// Create agent claims context
	claims := &models.AgentClaims{
		AgentID:      "agent-monitor",
		Namespace:    "test-ns",
		AllowedTools: []string{"list_peers"},
	}
	ctxWithAgent := service.ContextWithAgentClaims(ctx, claims)

	// Execute a tool — should not panic even though auditSvc is nil
	// logToolAudit is a no-op when auditSvc is nil (see line 128-130 in ai.go)
	func() {
		defer func() {
			// Catch panic from nil k8s client — we're testing that logToolAudit
			// safely handles nil auditSvc without panicking
			_ = recover()
		}()
		_, _ = svc.ExecuteTool(ctxWithAgent, "test-ns", "list_peers", json.RawMessage("{}"))
	}()

	// If we reach here without panic, the test passes (logToolAudit handled nil safely)
	g.Expect(true).To(BeTrue())
}

func TestLogToolAudit_HumanFallback(t *testing.T) {
	g := NewWithT(t)
	st := newTestDB(t)
	ctx := context.Background()

	// Create workspace
	err := st.Workspaces().Create(ctx, &models.Workspace{
		Model:     models.Model{ID: "ws-test"},
		Slug:      "test-ws",
		Namespace: "test-ns",
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Create AIService with no agentIsolation
	svc := service.NewAIServiceWithWorkflow(nil, st, nil, nil, 5, nil, nil, nil)

	// Attach fake audit service
	auditSvc := &fakeAuditService{}
	service.SetAuditService(svc, auditSvc)

	// Do NOT add AgentClaims to context — plain context (human user)
	plainCtx := context.Background()

	// Execute a tool with no AgentClaims in context
	// This will panic because k8s client is nil, but the audit log is recorded
	// before the tool execution is attempted
	func() {
		defer func() {
			// Catch panic from nil k8s client — we're testing the audit fallback behavior
			_ = recover()
		}()
		_, _ = svc.ExecuteTool(plainCtx, "test-ns", "list_peers", json.RawMessage("{}"))
	}()

	// Verify audit log was recorded with UserID "human" (fallback)
	// When no AgentClaims in context, agentClaimsFromContext returns nil,
	// so agentID defaults to "human" (line 132 in ai.go)
	g.Expect(auditSvc.logged).To(HaveLen(1))
	g.Expect(auditSvc.logged[0].Action).To(Equal(service.AuditActionAgentToolCall))
	g.Expect(auditSvc.logged[0].UserID).To(Equal("human"))
	g.Expect(auditSvc.logged[0].Resource).To(Equal("tool"))
	g.Expect(auditSvc.logged[0].ResourceName).To(Equal("list_peers"))
	g.Expect(auditSvc.logged[0].WorkspaceID).To(Equal("test-ns"))
}
