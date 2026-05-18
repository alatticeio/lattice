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
	"time"

	"github.com/alatticeio/lattice/api/v1alpha1"
	agentconfig "github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/alatticeio/lattice/internal/server/service"
	. "github.com/onsi/gomega"
)

// fakeToolSpanRepo records Write calls for test assertions.
type fakeToolSpanRepo struct {
	spans []*models.ToolSpan
}

func (r *fakeToolSpanRepo) Write(_ context.Context, s *models.ToolSpan) error {
	r.spans = append(r.spans, s)
	return nil
}
func (r *fakeToolSpanRepo) Get(_ context.Context, _ string) (*models.ToolSpan, error) {
	return nil, nil
}
func (r *fakeToolSpanRepo) List(_ context.Context, _ string, _, _ time.Time, _ int) ([]*models.ToolSpan, error) {
	return nil, nil
}

func TestExecuteTool_WritesToolSpan(t *testing.T) {
	g := NewWithT(t)

	repo := &fakeToolSpanRepo{}
	st := newTestDB(t)
	// Use workflow=nil so create_policy returns error (no panic) without k8s client
	wf := &fakeWorkflow{}
	svc := service.NewAIServiceWithWorkflow(nil, st, nil, nil, 5, wf, map[string]bool{}, nil)
	service.SetToolSpanRepo(svc, repo)

	input, _ := json.Marshal(map[string]interface{}{
		"name":    "allow-frontend",
		"network": "default",
		"action":  "ALLOW",
	})
	_, _ = svc.ExecuteTool(context.Background(), "default", "create_policy", input)

	g.Expect(repo.spans).To(HaveLen(1))
	g.Expect(repo.spans[0].Tool).To(Equal("create_policy"))
	g.Expect(repo.spans[0].TraceID).NotTo(BeEmpty())
	g.Expect(repo.spans[0].Status).To(Equal("ok")) // workflow submit returns ok
	g.Expect(repo.spans[0].DurationMs).To(BeNumerically(">=", 0))
}

func TestExecuteTool_BlockedWritesBlockedSpan(t *testing.T) {
	g := NewWithT(t)

	repo := &fakeToolSpanRepo{}
	st := newTestDB(t)
	// Build an isolation service that blocks list_peers (only allows create_peer)
	identity := &fakeAgentIdentityReader{
		identities: map[string]*v1alpha1.AgentIdentity{
			"default/agent-x": {
				Spec: v1alpha1.AgentIdentitySpec{
					AllowedTools:      []string{"create_peer"},
					AllowedNamespaces: []string{"default"},
					EnforcementMode:   v1alpha1.EnforcementEnforce,
				},
			},
		},
	}
	isolation := service.NewAgentIsolationService(
		agentconfig.AgentIsolationConfig{
			Enabled:         true,
			EnforcementMode: "enforce",
		},
		identity,
	)
	svc := service.NewAIServiceWithWorkflow(nil, st, nil, nil, 5, nil, nil, isolation)
	service.SetToolSpanRepo(svc, repo)

	claims := &models.AgentClaims{
		AgentID:      "agent-x",
		Namespace:    "default",
		AllowedTools: []string{"create_peer"},
	}
	ctx := service.ContextWithAgentClaims(context.Background(), claims)
	_, err := svc.ExecuteTool(ctx, "default", "list_peers", json.RawMessage(`{}`))

	g.Expect(err).NotTo(BeNil())
	g.Expect(repo.spans).To(HaveLen(1))
	g.Expect(repo.spans[0].Status).To(Equal("blocked"))
	g.Expect(repo.spans[0].ErrorMsg).NotTo(BeEmpty())
}

func TestExecuteTool_NoSpanWhenRepoNil(t *testing.T) {
	g := NewWithT(t)

	// No SetToolSpanRepo call — repo is nil
	st := newTestDB(t)
	svc := service.NewAIServiceWithWorkflow(nil, st, nil, nil, 5, nil, nil, nil)

	// Should not panic — use unknown tool to trigger default branch (no k8s client needed)
	_, err := svc.ExecuteTool(context.Background(), "default", "nonexistent_tool", json.RawMessage(`{}`))
	g.Expect(err).To(HaveOccurred()) // unknown tool returns error, but no panic
	g.Expect(true).To(BeTrue())      // just verify no panic
}
