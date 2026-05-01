// Copyright 2026 alatticeio
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

	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/db/gormstore"
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/internal/server/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupTestStore(t *testing.T) store.Store {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Workspace{}, &models.WorkspaceMember{}); err != nil {
		t.Fatal(err)
	}
	st, err := gormstore.New(db)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestSoftRemove_SetsRemovedStatus(t *testing.T) {
	st := setupTestStore(t)
	ctx := context.Background()

	user := &models.User{Model: models.Model{ID: "u1"}, Email: "u1@test.com"}
	if err := st.Users().Create(ctx, user); err != nil {
		t.Fatal(err)
	}

	ws := &models.Workspace{Model: models.Model{ID: "ws1"}, Slug: "test"}
	if err := st.Workspaces().Create(ctx, ws); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	member := &models.WorkspaceMember{
		WorkspaceID: ws.ID,
		UserID:      user.ID,
		Role:        dto.RoleAdmin,
		Status:      models.MemberStatusActive,
		JoinedAt:    &now,
	}
	if err := st.WorkspaceMembers().AddMember(ctx, member); err != nil {
		t.Fatal(err)
	}

	// Soft remove.
	if err := st.WorkspaceMembers().SoftRemove(ctx, ws.ID, user.ID); err != nil {
		t.Fatal(err)
	}

	// Verify status changed.
	got, err := st.WorkspaceMembers().GetMembership(ctx, ws.ID, user.ID)
	if err != nil {
		t.Fatalf("expected member to still exist: %v", err)
	}
	if got.Status != models.MemberStatusRemoved {
		t.Errorf("expected status=%q, got %q", models.MemberStatusRemoved, got.Status)
	}

	// Verify ListMembers excludes removed members.
	list, err := st.WorkspaceMembers().ListMembers(ctx, ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("expected 0 members in list (removed should be excluded), got %d", len(list))
	}
}

func TestSoftRemove_Idempotent(t *testing.T) {
	st := setupTestStore(t)
	ctx := context.Background()

	user := &models.User{Model: models.Model{ID: "u2"}, Email: "u2@test.com"}
	if err := st.Users().Create(ctx, user); err != nil {
		t.Fatal(err)
	}

	ws := &models.Workspace{Model: models.Model{ID: "ws2"}, Slug: "test2"}
	if err := st.Workspaces().Create(ctx, ws); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	member := &models.WorkspaceMember{
		WorkspaceID: ws.ID,
		UserID:      user.ID,
		Role:        dto.RoleMember,
		Status:      models.MemberStatusActive,
		JoinedAt:    &now,
	}
	if err := st.WorkspaceMembers().AddMember(ctx, member); err != nil {
		t.Fatal(err)
	}

	// Soft remove twice should not error.
	if err := st.WorkspaceMembers().SoftRemove(ctx, ws.ID, user.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.WorkspaceMembers().SoftRemove(ctx, ws.ID, user.ID); err != nil {
		t.Fatal(err)
	}

	got, err := st.WorkspaceMembers().GetMembership(ctx, ws.ID, user.ID)
	if err != nil {
		t.Fatalf("expected member to still exist: %v", err)
	}
	if got.Status != models.MemberStatusRemoved {
		t.Errorf("expected status=%q, got %q", models.MemberStatusRemoved, got.Status)
	}
}
