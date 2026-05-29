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

	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/db/gormstore"
	"github.com/alatticeio/lattice/internal/server/controller"
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/internal/server/models"

	"github.com/glebarez/sqlite"
	. "github.com/onsi/gomega"
	"gorm.io/gorm"
)

// setupPolicyTest creates an in-memory SQLite store for policy controller integration tests.
func setupPolicyTest(t *testing.T) (controller.PolicyController, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	st, err := gormstore.New(db)
	if err != nil {
		t.Fatal(err)
	}
	// Pass nil for resource.Client — only Submit and ListPolicy are tested here,
	// which do not require K8s CRD operations.
	pc := controller.NewPolicyController(nil, st)
	return pc, db
}

func TestPolicyController_Submit(t *testing.T) {
	g := NewWithT(t)
	pc, _ := setupPolicyTest(t)
	ctx := context.Background()

	// Submit a new policy intent (workflow approval path).
	policyDto := &dto.PolicyDto{
		Name:        "allow-frontend-to-api",
		Action:      "Allow",
		Description: "Allow frontend to access API",
		PolicyTypes: []string{"Ingress"},
		PolicySpec: dto.PolicySpec{
			Network: "default",
		},
	}

	policy, err := pc.Submit(ctx, "ws-test", "user-1", "Alice", policyDto)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(policy).NotTo(BeNil())
	g.Expect(policy.Name).To(Equal("allow-frontend-to-api"))
	g.Expect(policy.Action).To(Equal("Allow"))
	g.Expect(policy.Status).To(Equal(models.PolicyStatusPending))
	g.Expect(policy.WorkspaceID).To(Equal("ws-test"))
	g.Expect(policy.CreatedBy).To(Equal("user-1"))
	g.Expect(policy.CreatedByName).To(Equal("Alice"))
	g.Expect(policy.ID).NotTo(BeEmpty())
}

func TestPolicyController_ListPolicy(t *testing.T) {
	g := NewWithT(t)
	pc, _ := setupPolicyTest(t)
	ctx := context.Background()

	// Submit two policies.
	_, err := pc.Submit(ctx, "ws-list", "user-1", "Alice", &dto.PolicyDto{
		Name:        "allow-frontend",
		Action:      "Allow",
		Description: "Frontend access",
		PolicyTypes: []string{"Ingress"},
		PolicySpec: dto.PolicySpec{
			Network: "default",
		},
	})
	g.Expect(err).NotTo(HaveOccurred())

	_, err = pc.Submit(ctx, "ws-list", "user-2", "Bob", &dto.PolicyDto{
		Name:        "deny-external",
		Action:      "Deny",
		Description: "Block external",
		PolicyTypes: []string{"Egress"},
		PolicySpec: dto.PolicySpec{
			Network: "default",
		},
	})
	g.Expect(err).NotTo(HaveOccurred())

	// List policies with workspace context.
	listCtx := context.WithValue(ctx, infra.WorkspaceKey, "ws-list")
	result, err := pc.ListPolicy(listCtx, &dto.PageRequest{
		Page:     1,
		PageSize: 20,
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result).NotTo(BeNil())
	g.Expect(result.Total).To(Equal(int64(2)))
	g.Expect(result.List).To(HaveLen(2))
	g.Expect(result.Page).To(Equal(1))
	g.Expect(result.PageSize).To(Equal(20))
}

func TestPolicyController_ListPolicy_EmptyWorkspace(t *testing.T) {
	g := NewWithT(t)
	pc, _ := setupPolicyTest(t)

	// Submit a policy to a workspace.
	_, err := pc.Submit(context.Background(), "ws-other", "user-1", "Alice", &dto.PolicyDto{
		Name:        "allow-db",
		Action:      "Allow",
		Description: "DB access",
		PolicyTypes: []string{"Ingress"},
		PolicySpec: dto.PolicySpec{
			Network: "default",
		},
	})
	g.Expect(err).NotTo(HaveOccurred())

	// List with a different workspace context should return no results.
	listCtx := context.WithValue(context.Background(), infra.WorkspaceKey, "ws-different")
	result, err := pc.ListPolicy(listCtx, &dto.PageRequest{
		Page:     1,
		PageSize: 20,
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result.Total).To(Equal(int64(0)))
	g.Expect(result.List).To(BeEmpty())
}

func TestPolicyController_ListPolicy_KeywordFilter(t *testing.T) {
	g := NewWithT(t)
	pc, _ := setupPolicyTest(t)

	// Submit several policies.
	policies := []struct {
		name        string
		action      string
		description string
	}{
		{"allow-api", "Allow", "API access"},
		{"allow-db", "Allow", "Database access"},
		{"deny-external", "Deny", "Block external traffic"},
	}
	for _, p := range policies {
		_, err := pc.Submit(context.Background(), "ws-filter", "user-1", "Alice", &dto.PolicyDto{
			Name:        p.name,
			Action:      p.action,
			Description: p.description,
			PolicyTypes: []string{"Ingress"},
			PolicySpec: dto.PolicySpec{
				Network: "default",
			},
		})
		g.Expect(err).NotTo(HaveOccurred())
	}

	// Filter by keyword "api".
	listCtx := context.WithValue(context.Background(), infra.WorkspaceKey, "ws-filter")
	result, err := pc.ListPolicy(listCtx, &dto.PageRequest{
		Page:     1,
		PageSize: 20,
		Keyword:  "api",
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result.Total).To(Equal(int64(1)))
	g.Expect(result.List[0].Name).To(Equal("allow-api"))

	// Filter by keyword "block" (matches description).
	result, err = pc.ListPolicy(listCtx, &dto.PageRequest{
		Page:     1,
		PageSize: 20,
		Keyword:  "block",
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result.Total).To(Equal(int64(1)))
	g.Expect(result.List[0].Name).To(Equal("deny-external"))
}
