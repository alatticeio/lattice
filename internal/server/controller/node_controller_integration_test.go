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

package controller_test

import (
	"context"
	"testing"

	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/db/gormstore"
	"github.com/alatticeio/lattice/internal/server/models"

	"github.com/glebarez/sqlite"
	. "github.com/onsi/gomega"
	"gorm.io/gorm"
)

// setupNodeTestStore creates an in-memory SQLite store for node controller integration tests.
func setupNodeTestStore(t *testing.T) (store.Store, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	st, err := gormstore.New(db)
	if err != nil {
		t.Fatal(err)
	}
	return st, db
}

func TestNodeWorkspaceIntegration(t *testing.T) {
	g := NewWithT(t)
	st, _ := setupNodeTestStore(t)
	defer st.Close()
	ctx := context.Background()

	// Create a workspace that will contain nodes/peers.
	ws := &models.Workspace{
		Slug:        "node-test-ws",
		DisplayName: "Node Test Workspace",
		Status:      "active",
	}
	err := st.Workspaces().Create(ctx, ws)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(ws.ID).NotTo(BeEmpty())

	// Retrieve workspace by ID.
	got, err := st.Workspaces().GetByID(ctx, ws.ID)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(got.Slug).To(Equal("node-test-ws"))
	g.Expect(got.Namespace).To(ContainSubstring("wf-"))
}

func TestNodePolicyCRUD(t *testing.T) {
	g := NewWithT(t)
	st, _ := setupNodeTestStore(t)
	defer st.Close()
	ctx := context.Background()

	// Create workspace.
	ws := &models.Workspace{Slug: "policy-test", DisplayName: "Policy Test", Status: "active"}
	g.Expect(st.Workspaces().Create(ctx, ws)).To(Succeed())

	// Create a policy for the workspace.
	policy := &models.Policy{
		WorkspaceID:   ws.ID,
		Name:          "allow-frontend-api",
		Action:        "Allow",
		Description:   "Allow frontend to API",
		Status:        models.PolicyStatusActive,
		CreatedBy:     "user-1",
		CreatedByName: "admin",
	}
	err := st.Policies().Create(ctx, policy)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(policy.ID).NotTo(BeEmpty())

	// List policies for workspace.
	policies, total, err := st.Policies().List(ctx, store.PolicyFilter{WorkspaceID: ws.ID})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(total).To(Equal(int64(1)))
	g.Expect(policies).To(HaveLen(1))
	g.Expect(policies[0].Name).To(Equal("allow-frontend-api"))
	g.Expect(policies[0].Action).To(Equal("Allow"))

	// Retrieve policy by name.
	got, err := st.Policies().GetByName(ctx, ws.ID, "allow-frontend-api")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(got.Description).To(Equal("Allow frontend to API"))
	g.Expect(got.Status).To(Equal(models.PolicyStatusActive))

	// Update policy status.
	got.Status = models.PolicyStatusPending
	g.Expect(st.Policies().Update(ctx, got)).To(Succeed())

	updated, err := st.Policies().GetByName(ctx, ws.ID, "allow-frontend-api")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(updated.Status).To(Equal(models.PolicyStatusPending))

	// Delete policy.
	g.Expect(st.Policies().Delete(ctx, ws.ID, "allow-frontend-api")).To(Succeed())

	_, err = st.Policies().GetByName(ctx, ws.ID, "allow-frontend-api")
	g.Expect(err).To(HaveOccurred())
}

func TestNodePolicyFiltering(t *testing.T) {
	g := NewWithT(t)
	st, _ := setupNodeTestStore(t)
	defer st.Close()
	ctx := context.Background()

	ws := &models.Workspace{Slug: "filter-test", DisplayName: "Filter Test", Status: "active"}
	g.Expect(st.Workspaces().Create(ctx, ws)).To(Succeed())

	// Create multiple policies.
	policies := []*models.Policy{
		{WorkspaceID: ws.ID, Name: "allow-db", Action: "Allow", Status: models.PolicyStatusActive},
		{WorkspaceID: ws.ID, Name: "deny-external", Action: "Deny", Status: models.PolicyStatusActive},
		{WorkspaceID: ws.ID, Name: "allow-api", Action: "Allow", Status: models.PolicyStatusPending},
		{WorkspaceID: ws.ID, Name: "allow-cache", Action: "Allow", Status: models.PolicyStatusActive},
	}
	for _, p := range policies {
		g.Expect(st.Policies().Create(ctx, p)).To(Succeed())
	}

	// Filter by keyword.
	results, total, err := st.Policies().List(ctx, store.PolicyFilter{
		WorkspaceID: ws.ID,
		Keyword:     "api",
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(total).To(Equal(int64(1)))
	g.Expect(results[0].Name).To(Equal("allow-api"))

	// Filter by status.
	results, total, err = st.Policies().List(ctx, store.PolicyFilter{
		WorkspaceID: ws.ID,
		Status:      string(models.PolicyStatusPending),
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(total).To(Equal(int64(1)))
	g.Expect(results[0].Name).To(Equal("allow-api"))

	// Pagination.
	results, total, err = st.Policies().List(ctx, store.PolicyFilter{
		WorkspaceID: ws.ID,
		Page:        1,
		PageSize:    2,
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(total).To(Equal(int64(4)))
	g.Expect(results).To(HaveLen(2))
}

func TestNodeWorkspaceIsolation(t *testing.T) {
	g := NewWithT(t)
	st, _ := setupNodeTestStore(t)
	defer st.Close()
	ctx := context.Background()

	// Create two workspaces with similarly named policies.
	ws1 := &models.Workspace{Slug: "ws-one", DisplayName: "Workspace One", Status: "active"}
	g.Expect(st.Workspaces().Create(ctx, ws1)).To(Succeed())

	ws2 := &models.Workspace{Slug: "ws-two", DisplayName: "Workspace Two", Status: "active"}
	g.Expect(st.Workspaces().Create(ctx, ws2)).To(Succeed())

	for _, ws := range []*models.Workspace{ws1, ws2} {
		g.Expect(st.Policies().Create(ctx, &models.Policy{
			WorkspaceID: ws.ID,
			Name:        "allow-db",
			Action:      "Allow",
			Status:      models.PolicyStatusActive,
		})).To(Succeed())
	}

	// Workspace 1 should only see its own policy.
	policies, total, err := st.Policies().List(ctx, store.PolicyFilter{WorkspaceID: ws1.ID})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(total).To(Equal(int64(1)))
	g.Expect(policies[0].WorkspaceID).To(Equal(ws1.ID))

	// Workspace 2 should only see its own policy.
	policies, total, err = st.Policies().List(ctx, store.PolicyFilter{WorkspaceID: ws2.ID})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(total).To(Equal(int64(1)))
	g.Expect(policies[0].WorkspaceID).To(Equal(ws2.ID))
}
