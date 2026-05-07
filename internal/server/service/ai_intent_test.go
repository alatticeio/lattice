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

	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/db/gormstore"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/alatticeio/lattice/internal/server/service"

	"github.com/glebarez/sqlite"
	. "github.com/onsi/gomega"
	"gorm.io/gorm"
)

// fakeIntentService implements service.IntentService for testing.
type fakeIntentService struct {
	planCalled bool
	planErr    error
}

func (f *fakeIntentService) Plan(_ context.Context, _ service.IntentRequest) (*service.IntentPlanView, error) {
	f.planCalled = true
	if f.planErr != nil {
		return nil, f.planErr
	}
	return &service.IntentPlanView{
		ID:        "plan-001",
		Summary:   "## 变更计划\n- 创建策略 allow-frontend",
		Changes:   []service.CRDChange{{Action: "create", Resource: "policy/allow-frontend"}},
		RiskLevel: "low",
	}, nil
}
func (f *fakeIntentService) Apply(_ context.Context, planID, _ string) ([]string, error) {
	return []string{planID}, nil
}

func newTestDB(t *testing.T) store.Store {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	st, err := gormstore.New(db)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestAIIntent_ToolPlanNetworkChange(t *testing.T) {
	g := NewWithT(t)
	fakeIntent := &fakeIntentService{}
	st := newTestDB(t)
	ctx := context.Background()

	// Create workspace to resolve namespace
	err := st.Workspaces().Create(ctx, &models.Workspace{
		Model: models.Model{ID: "ws-test"},
		Slug:  "test-ws", Namespace: "test-ns",
	})
	g.Expect(err).ToNot(HaveOccurred())

	svc := service.NewAIServiceWithWorkflow(nil, st, nil, nil, 5, nil, nil)
	service.SetIntentService(svc, fakeIntent)

	input, _ := json.Marshal(map[string]string{
		"intent": "allow frontend to access api",
	})
	result, err := svc.ExecuteTool(ctx, "test-ns", "plan_network_change", input)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(ContainSubstring("plan-001"))
	g.Expect(result).To(ContainSubstring("allow-frontend"))
	g.Expect(fakeIntent.planCalled).To(BeTrue())
}

func TestAIIntent_ToolPlanNetworkChange_NoIntentService(t *testing.T) {
	g := NewWithT(t)
	svc := service.NewAIServiceWithWorkflow(nil, nil, nil, nil, 5, nil, nil)

	input, _ := json.Marshal(map[string]string{"intent": "allow frontend to api"})
	_, err := svc.ExecuteTool(context.Background(), "test-ns", "plan_network_change", input)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("Pro feature"))
}

// Test community stub returns 402
func TestAIIntent_CommunityStub(t *testing.T) {
	g := NewWithT(t)
	intentSvc := service.NewIntentService(nil, nil, nil)

	_, err := intentSvc.Plan(context.Background(), service.IntentRequest{})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("Pro feature"))

	_, err = intentSvc.Apply(context.Background(), "", "")
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("Pro feature"))
}

func TestAIIntent_ToolApplyNetworkChange(t *testing.T) {
	g := NewWithT(t)
	fakeIntent := &fakeIntentService{}
	svc := service.NewAIServiceWithWorkflow(nil, nil, nil, nil, 5, nil, nil)
	service.SetIntentService(svc, fakeIntent)

	input, _ := json.Marshal(map[string]string{"plan_id": "plan-001"})
	result, err := svc.ExecuteTool(context.Background(), "test-ns", "apply_network_change", input)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(ContainSubstring("plan-001"))
}
