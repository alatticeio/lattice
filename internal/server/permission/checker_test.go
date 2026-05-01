// Copyright 2024 alatticeio
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

package permission_test

import (
	"context"
	"testing"

	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/db/gormstore"
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/alatticeio/lattice/internal/server/permission"
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

func TestPermissionChecker_PlatformAdminBypass(t *testing.T) {
	st := setupTestStore(t)
	ctx := context.Background()

	admin := &models.User{Model: models.Model{ID: "admin1"}, Email: "admin@test.com", SystemRole: dto.SystemRolePlatformAdmin}
	st.Users().Create(ctx, admin)

	ws := &models.Workspace{Model: models.Model{ID: "ws1"}, Slug: "test", Namespace: "wf-ws1"}
	st.Workspaces().Create(ctx, ws)

	// Admin is NOT a member of ws1
	checker := permission.NewForTest(st, nil)

	// Platform admin should pass even without membership
	member, err := checker.RequireWorkspaceRole(ctx, ws.ID, admin.ID, dto.RoleAdmin)
	if err != nil {
		t.Errorf("expected platform admin to pass, got error: %v", err)
	}
	if member != nil {
		t.Error("expected nil member for platform admin bypass")
	}
}

func TestPermissionChecker_RejectsInsufficientRole(t *testing.T) {
	st := setupTestStore(t)
	ctx := context.Background()

	user := &models.User{Model: models.Model{ID: "u1"}, Email: "user@test.com", SystemRole: dto.SystemRoleUser}
	st.Users().Create(ctx, user)

	ws := &models.Workspace{Model: models.Model{ID: "ws1"}, Slug: "test", Namespace: "wf-ws1"}
	st.Workspaces().Create(ctx, ws)

	st.WorkspaceMembers().AddMember(ctx, &models.WorkspaceMember{
		WorkspaceID: ws.ID,
		UserID:      user.ID,
		Role:        dto.RoleViewer,
		Status:      models.MemberStatusActive,
	})

	checker := permission.NewForTest(st, nil)

	_, err := checker.RequireWorkspaceRole(ctx, ws.ID, user.ID, dto.RoleAdmin)
	if err == nil {
		t.Error("expected viewer to be rejected for admin role requirement")
	}
}

func TestPermissionChecker_PassesSufficientRole(t *testing.T) {
	st := setupTestStore(t)
	ctx := context.Background()

	user := &models.User{Model: models.Model{ID: "u2"}, Email: "user2@test.com", SystemRole: dto.SystemRoleUser}
	st.Users().Create(ctx, user)

	ws := &models.Workspace{Model: models.Model{ID: "ws2"}, Slug: "test2", Namespace: "wf-ws2"}
	st.Workspaces().Create(ctx, ws)

	st.WorkspaceMembers().AddMember(ctx, &models.WorkspaceMember{
		WorkspaceID: ws.ID,
		UserID:      user.ID,
		Role:        dto.RoleAdmin,
		Status:      models.MemberStatusActive,
	})

	checker := permission.NewForTest(st, nil)

	member, err := checker.RequireWorkspaceRole(ctx, ws.ID, user.ID, dto.RoleAdmin)
	if err != nil {
		t.Errorf("expected admin to pass, got error: %v", err)
	}
	if member == nil {
		t.Error("expected member record to be returned")
	}
}

func TestPermissionChecker_RejectsSuspendedMember(t *testing.T) {
	st := setupTestStore(t)
	ctx := context.Background()

	user := &models.User{Model: models.Model{ID: "u3"}, Email: "user3@test.com", SystemRole: dto.SystemRoleUser}
	st.Users().Create(ctx, user)

	ws := &models.Workspace{Model: models.Model{ID: "ws3"}, Slug: "test3", Namespace: "wf-ws3"}
	st.Workspaces().Create(ctx, ws)

	st.WorkspaceMembers().AddMember(ctx, &models.WorkspaceMember{
		WorkspaceID: ws.ID,
		UserID:      user.ID,
		Role:        dto.RoleAdmin,
		Status:      models.MemberStatusSuspended,
	})

	checker := permission.NewForTest(st, nil)

	_, err := checker.RequireWorkspaceRole(ctx, ws.ID, user.ID, dto.RoleAdmin)
	if err == nil {
		t.Error("expected suspended member to be rejected")
	}
}

func TestPermissionChecker_RejectsRemovedMember(t *testing.T) {
	st := setupTestStore(t)
	ctx := context.Background()

	user := &models.User{Model: models.Model{ID: "u4"}, Email: "user4@test.com", SystemRole: dto.SystemRoleUser}
	st.Users().Create(ctx, user)

	ws := &models.Workspace{Model: models.Model{ID: "ws4"}, Slug: "test4", Namespace: "wf-ws4"}
	st.Workspaces().Create(ctx, ws)

	st.WorkspaceMembers().AddMember(ctx, &models.WorkspaceMember{
		WorkspaceID: ws.ID,
		UserID:      user.ID,
		Role:        dto.RoleAdmin,
		Status:      models.MemberStatusRemoved,
	})

	checker := permission.NewForTest(st, nil)

	_, err := checker.RequireWorkspaceRole(ctx, ws.ID, user.ID, dto.RoleAdmin)
	if err == nil {
		t.Error("expected removed member to be rejected")
	}
}
