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
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/alatticeio/lattice/internal/server/service"
	. "github.com/onsi/gomega"
)

// fakeWorkflow satisfies service.WorkflowService for testing.
type fakeWorkflow struct {
	submitted []service.SubmitWorkflowReq
}

func (f *fakeWorkflow) Submit(_ context.Context, req service.SubmitWorkflowReq) (*models.WorkflowRequest, error) {
	f.submitted = append(f.submitted, req)
	return &models.WorkflowRequest{ID: "wf-test-001", Status: models.WorkflowStatusPending}, nil
}
func (f *fakeWorkflow) Approve(_ context.Context, id, rid, rname, note string) error { return nil }
func (f *fakeWorkflow) Reject(_ context.Context, id, rid, rname, note string) error  { return nil }
func (f *fakeWorkflow) List(_ context.Context, _ store.WorkflowFilter) ([]*models.WorkflowRequest, int64, error) {
	return nil, 0, nil
}
func (f *fakeWorkflow) GetByID(_ context.Context, id string) (*models.WorkflowRequest, error) {
	return nil, nil
}
func (f *fakeWorkflow) RegisterExecutor(_, _ string, _ service.ExecutorFunc) {}

func TestAIService_WriteToolSubmitsWorkflow(t *testing.T) {
	g := NewWithT(t)
	wf := &fakeWorkflow{}

	svc := service.NewAIServiceWithWorkflow(nil, nil, nil, nil, 5, wf,
		map[string]bool{}, nil) // auto_approve all false

	input, _ := json.Marshal(map[string]interface{}{
		"name":    "allow-frontend-to-api",
		"network": "default",
		"action":  "ALLOW",
	})
	result, err := svc.ExecuteTool(context.Background(), "test-ns", "create_policy", input)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(ContainSubstring("wf-test-001"))
	g.Expect(wf.submitted).To(HaveLen(1))
	g.Expect(wf.submitted[0].Action).To(Equal("create_policy"))
}
