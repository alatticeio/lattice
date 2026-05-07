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
	"fmt"
	"testing"
	"time"

	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/alatticeio/lattice/internal/server/service"
	. "github.com/onsi/gomega"
)

// fakeSnapStore implements store.NetworkSnapshotRepository for testing debug tools.
type fakeSnapStore struct {
	snaps []*models.NetworkSnapshot
}

func (f *fakeSnapStore) Create(_ context.Context, s *models.NetworkSnapshot) error {
	f.snaps = append(f.snaps, s)
	return nil
}
func (f *fakeSnapStore) GetByID(_ context.Context, id string) (*models.NetworkSnapshot, error) {
	for _, s := range f.snaps {
		if s.ID == id {
			return s, nil
		}
	}
	return nil, fmt.Errorf("not found")
}
func (f *fakeSnapStore) List(_ context.Context, _ string, _, _ time.Time, triggerType string) ([]*models.NetworkSnapshot, error) {
	var result []*models.NetworkSnapshot
	for _, s := range f.snaps {
		if triggerType == "" || s.TriggerType == triggerType {
			result = append(result, s)
		}
	}
	return result, nil
}
func (f *fakeSnapStore) DeleteOlderThan(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}

func TestAIDebug_ToolListSnapshots(t *testing.T) {
	g := NewWithT(t)
	st := newTestDB(t)
	ctx := context.Background()

	// Create workspace to resolve namespace
	g.Expect(st.Workspaces().Create(ctx, &models.Workspace{
		Model: models.Model{ID: "ws-test"},
		Slug:  "test-ws", Namespace: "test-ns",
	})).To(Succeed())

	snapStore := &fakeSnapStore{snaps: []*models.NetworkSnapshot{
		{ID: "snap-001", WorkspaceID: "ws-test", CapturedAt: time.Now().Add(-10 * time.Minute), TriggerType: "policy_change", TriggerBy: "system"},
		{ID: "snap-002", WorkspaceID: "ws-test", CapturedAt: time.Now().Add(-5 * time.Minute), TriggerType: "peer_online", TriggerBy: "peer-a"},
	}}

	svc := service.NewAIServiceWithWorkflow(nil, st, nil, nil, 5, nil, nil)
	service.SetSnapStore(svc, snapStore)

	// list_snapshots with time range covering both
	input, _ := json.Marshal(map[string]string{
		"from": time.Now().Add(-time.Hour).Format(time.RFC3339),
		"to":   time.Now().Format(time.RFC3339),
	})
	result, err := svc.ExecuteTool(ctx, "test-ns", "list_snapshots", input)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(ContainSubstring("snap-001"))
	g.Expect(result).To(ContainSubstring("snap-002"))
	g.Expect(result).To(ContainSubstring("2 个快照"))
}

func TestAIDebug_ToolGetSnapshot(t *testing.T) {
	g := NewWithT(t)
	snapStore := &fakeSnapStore{snaps: []*models.NetworkSnapshot{
		{
			ID: "snap-001", WorkspaceID: "ws-test", CapturedAt: time.Now().Add(-10 * time.Minute),
			TriggerType: "policy_change", TriggerBy: "system",
			Peers: `[{"name":"peer-a"}]`, Policies: `[{"name":"allow-all","action":"ALLOW"}]`,
			Networks: `[{"name":"default"}]`, Presence: `{}`,
		},
	}}
	svc := service.NewAIServiceWithWorkflow(nil, nil, nil, nil, 5, nil, nil)
	service.SetSnapStore(svc, snapStore)

	input, _ := json.Marshal(map[string]string{"id": "snap-001"})
	result, err := svc.ExecuteTool(context.Background(), "test-ns", "get_snapshot", input)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(ContainSubstring("snap-001"))
	g.Expect(result).To(ContainSubstring("peer-a"))
	g.Expect(result).To(ContainSubstring("allow-all"))
}

func TestAIDebug_ToolGetSnapshot_NotFound(t *testing.T) {
	g := NewWithT(t)
	svc := service.NewAIServiceWithWorkflow(nil, nil, nil, nil, 5, nil, nil)
	service.SetSnapStore(svc, &fakeSnapStore{})

	input, _ := json.Marshal(map[string]string{"id": "nonexistent"})
	_, err := svc.ExecuteTool(context.Background(), "test-ns", "get_snapshot", input)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("not found"))
}

func TestAIDebug_ToolDiffSnapshots(t *testing.T) {
	g := NewWithT(t)
	snapStore := &fakeSnapStore{snaps: []*models.NetworkSnapshot{
		{
			ID: "snap-before", CapturedAt: time.Now().Add(-10 * time.Minute),
			Policies: `[{"name":"allow-all","action":"ALLOW"}]`,
		},
		{
			ID: "snap-after", CapturedAt: time.Now().Add(-5 * time.Minute),
			Policies: `[{"name":"allow-all","action":"ALLOW"},{"name":"deny-ssh","action":"DENY"}]`,
		},
	}}
	svc := service.NewAIServiceWithWorkflow(nil, nil, nil, nil, 5, nil, nil)
	service.SetSnapStore(svc, snapStore)

	input, _ := json.Marshal(map[string]string{"from_id": "snap-before", "to_id": "snap-after"})
	result, err := svc.ExecuteTool(context.Background(), "test-ns", "diff_snapshots", input)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(ContainSubstring("deny-ssh"))
	g.Expect(result).To(ContainSubstring("新增"))
}

func TestAIDebug_ToolCheckConnectivityAt_Allowed(t *testing.T) {
	g := NewWithT(t)
	snapStore := &fakeSnapStore{snaps: []*models.NetworkSnapshot{
		{
			ID:       "snap-001",
			Policies: `[{"name":"allow-frontend-api","action":"ALLOW"}]`,
		},
	}}
	svc := service.NewAIServiceWithWorkflow(nil, nil, nil, nil, 5, nil, nil)
	service.SetSnapStore(svc, snapStore)

	input, _ := json.Marshal(map[string]string{
		"snapshot_id": "snap-001",
		"from":        "frontend",
		"to":          "api",
	})
	result, err := svc.ExecuteTool(context.Background(), "test-ns", "check_connectivity_at", input)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(ContainSubstring("ALLOW 策略"))
}

func TestAIDebug_ToolCheckConnectivityAt_Blocked(t *testing.T) {
	g := NewWithT(t)
	snapStore := &fakeSnapStore{snaps: []*models.NetworkSnapshot{
		{
			ID:       "snap-001",
			Policies: `[{"name":"deny-all","action":"DENY"}]`,
		},
	}}
	svc := service.NewAIServiceWithWorkflow(nil, nil, nil, nil, 5, nil, nil)
	service.SetSnapStore(svc, snapStore)

	input, _ := json.Marshal(map[string]string{
		"snapshot_id": "snap-001",
		"from":        "frontend",
		"to":          "api",
	})
	result, err := svc.ExecuteTool(context.Background(), "test-ns", "check_connectivity_at", input)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result).To(ContainSubstring("未找到"))
	g.Expect(result).To(ContainSubstring("阻断"))
}

func TestAIDebug_SnapStoreNotConfigured_ReturnsError(t *testing.T) {
	g := NewWithT(t)
	// No snapStore configured
	svc := service.NewAIServiceWithWorkflow(nil, nil, nil, nil, 5, nil, nil)

	_, err := svc.ExecuteTool(context.Background(), "test-ns", "list_snapshots", json.RawMessage(`{"from":"2024-01-01T00:00:00Z","to":"2024-12-31T00:00:00Z"}`))
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("Pro"))
}
