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

package gormstore_test

import (
	"context"
	"testing"
	"time"

	"github.com/alatticeio/lattice/internal/db/gormstore"
	"github.com/alatticeio/lattice/internal/server/models"

	"github.com/glebarez/sqlite"
	. "github.com/onsi/gomega"
	"gorm.io/gorm"
)

func setupIntentPlanTest(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.IntentPlan{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestIntentPlanRepo_CreateAndGetByID(t *testing.T) {
	g := NewWithT(t)
	db := setupIntentPlanTest(t)
	store, err := gormstore.New(db)
	g.Expect(err).ToNot(HaveOccurred())
	ctx := context.Background()

	plan := &models.IntentPlan{
		ID:          "plan-001",
		WorkspaceID: "ws-test",
		Namespace:   "test-ns",
		Intent:      "allow frontend to api",
		Summary:     "## Changes\n- Create policy allow-frontend-to-api",
		ChangesJSON: `[{"action":"create","resource":"policy/allow-frontend-to-api"}]`,
		RiskLevel:   "low",
		ExpiresAt:   time.Now().Add(10 * time.Minute),
	}
	err = store.IntentPlans().Create(ctx, plan)
	g.Expect(err).ToNot(HaveOccurred())

	got, err := store.IntentPlans().GetByID(ctx, "plan-001")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(got.Intent).To(Equal("allow frontend to api"))
	g.Expect(got.RiskLevel).To(Equal("low"))
}

func TestIntentPlanRepo_GetByID_NotFound(t *testing.T) {
	g := NewWithT(t)
	db := setupIntentPlanTest(t)
	store, err := gormstore.New(db)
	g.Expect(err).ToNot(HaveOccurred())

	_, err = store.IntentPlans().GetByID(context.Background(), "nonexistent")
	g.Expect(err).To(HaveOccurred())
}

func TestIntentPlanRepo_DeleteExpired(t *testing.T) {
	g := NewWithT(t)
	db := setupIntentPlanTest(t)
	store, err := gormstore.New(db)
	g.Expect(err).ToNot(HaveOccurred())
	ctx := context.Background()

	// Expired plan
	err = store.IntentPlans().Create(ctx, &models.IntentPlan{
		ID:        "plan-expired",
		ExpiresAt: time.Now().Add(-time.Hour),
	})
	g.Expect(err).ToNot(HaveOccurred())

	// Valid plan
	err = store.IntentPlans().Create(ctx, &models.IntentPlan{
		ID:        "plan-valid",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	g.Expect(err).ToNot(HaveOccurred())

	err = store.IntentPlans().DeleteExpired(ctx)
	g.Expect(err).ToNot(HaveOccurred())

	// Expired should be gone
	_, err = store.IntentPlans().GetByID(ctx, "plan-expired")
	g.Expect(err).To(HaveOccurred())

	// Valid should remain
	got, err := store.IntentPlans().GetByID(ctx, "plan-valid")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(got.ID).To(Equal("plan-valid"))
}

func TestIntentPlanRepo_Delete(t *testing.T) {
	g := NewWithT(t)
	db := setupIntentPlanTest(t)
	store, err := gormstore.New(db)
	g.Expect(err).ToNot(HaveOccurred())
	ctx := context.Background()

	err = store.IntentPlans().Create(ctx, &models.IntentPlan{ID: "plan-to-delete", ExpiresAt: time.Now().Add(time.Hour)})
	g.Expect(err).ToNot(HaveOccurred())

	err = store.IntentPlans().Delete(ctx, "plan-to-delete")
	g.Expect(err).ToNot(HaveOccurred())

	_, err = store.IntentPlans().GetByID(ctx, "plan-to-delete")
	g.Expect(err).To(HaveOccurred())
}
