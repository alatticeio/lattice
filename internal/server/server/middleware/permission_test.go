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

package middleware_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/db/gormstore"
	"github.com/alatticeio/lattice/internal/server/auth"
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/alatticeio/lattice/internal/server/permission"
	mw "github.com/alatticeio/lattice/internal/server/server/middleware"
	"github.com/alatticeio/lattice/pkg/utils"
	"github.com/alatticeio/lattice/pkg/utils/resp"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupPermissionTest(t *testing.T) (*gin.Engine, store.Store, *mw.Middleware) {
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
	checker := permission.NewForTest(st, nil)
	middleware := mw.NewMiddleware(checker, st, nil)

	engine := gin.New()
	return engine, st, middleware
}

func setupWithRevocation(t *testing.T) (*gin.Engine, store.Store, *mw.Middleware, *auth.RevocationList) {
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
	rl := auth.NewRevocationList()
	checker := permission.NewForTest(st, nil)
	middleware := mw.NewMiddleware(checker, st, rl)

	engine := gin.New()
	return engine, st, middleware, rl
}

func makeTestToken(t *testing.T, userID, email, username, systemRole string) string {
	t.Helper()
	token, err := utils.GenerateBusinessJWT(userID, email, username, systemRole)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func TestWorkspaceAuthMiddleware_PassesForAdmin(t *testing.T) {
	engine, st, middleware := setupPermissionTest(t)
	ctx := context.Background()

	user := &models.User{Model: models.Model{ID: "u-admin"}, Email: "admin@test.com", SystemRole: dto.SystemRoleUser}
	st.Users().Create(ctx, user)

	ws := &models.Workspace{Model: models.Model{ID: "ws-admin"}, Slug: "test-admin", Namespace: "wf-ws-admin"}
	st.Workspaces().Create(ctx, ws)

	st.WorkspaceMembers().AddMember(ctx, &models.WorkspaceMember{
		WorkspaceID: ws.ID,
		UserID:      user.ID,
		Role:        dto.RoleAdmin,
		Status:      models.MemberStatusActive,
	})

	token := makeTestToken(t, user.ID, user.Email, user.Username, string(user.SystemRole))

	engine.GET("/api/v1/workspaces/:id/test", middleware.WorkspaceAuthMiddleware(dto.RoleAdmin), func(c *gin.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api/v1/workspaces/"+ws.ID+"/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Workspace-Id", ws.ID)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Errorf("expected 200 'ok' for admin, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestWorkspaceAuthMiddleware_RejectsViewerForAdminRoute(t *testing.T) {
	engine, st, middleware := setupPermissionTest(t)
	ctx := context.Background()

	user := &models.User{Model: models.Model{ID: "u-viewer"}, Email: "viewer@test.com", SystemRole: dto.SystemRoleUser}
	st.Users().Create(ctx, user)

	ws := &models.Workspace{Model: models.Model{ID: "ws-viewer"}, Slug: "test-viewer", Namespace: "wf-ws-viewer"}
	st.Workspaces().Create(ctx, ws)

	st.WorkspaceMembers().AddMember(ctx, &models.WorkspaceMember{
		WorkspaceID: ws.ID,
		UserID:      user.ID,
		Role:        dto.RoleViewer,
		Status:      models.MemberStatusActive,
	})

	token := makeTestToken(t, user.ID, user.Email, user.Username, string(user.SystemRole))

	engine.GET("/api/v1/workspaces/:id/test", middleware.WorkspaceAuthMiddleware(dto.RoleAdmin), func(c *gin.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api/v1/workspaces/"+ws.ID+"/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Workspace-Id", ws.ID)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	var r resp.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatalf("expected JSON response, got: %s", rec.Body.String())
	}
	if r.Code != http.StatusForbidden {
		t.Errorf("expected code 403 for viewer on admin route, got %d", r.Code)
	}
}

func TestWorkspaceAuthMiddleware_PlatformAdminBypassesWorkspaceCheck(t *testing.T) {
	engine, st, middleware := setupPermissionTest(t)
	ctx := context.Background()

	user := &models.User{Model: models.Model{ID: "u-platform"}, Email: "platform@test.com", SystemRole: dto.SystemRolePlatformAdmin}
	st.Users().Create(ctx, user)

	ws := &models.Workspace{Model: models.Model{ID: "ws-platform"}, Slug: "test-platform", Namespace: "wf-ws-platform"}
	st.Workspaces().Create(ctx, ws)

	token := makeTestToken(t, user.ID, user.Email, user.Username, string(user.SystemRole))

	engine.GET("/api/v1/workspaces/:id/test", middleware.WorkspaceAuthMiddleware(dto.RoleAdmin), func(c *gin.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api/v1/workspaces/"+ws.ID+"/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Workspace-Id", ws.ID)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Errorf("expected 200 'ok' for platform admin, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestAdminOnly_DelegatesToWorkspaceAuth(t *testing.T) {
	engine, st, middleware := setupPermissionTest(t)
	ctx := context.Background()

	user := &models.User{Model: models.Model{ID: "u-member"}, Email: "member@test.com", SystemRole: dto.SystemRoleUser}
	st.Users().Create(ctx, user)

	ws := &models.Workspace{Model: models.Model{ID: "ws-member"}, Slug: "test-member", Namespace: "wf-ws-member"}
	st.Workspaces().Create(ctx, ws)

	st.WorkspaceMembers().AddMember(ctx, &models.WorkspaceMember{
		WorkspaceID: ws.ID,
		UserID:      user.ID,
		Role:        dto.RoleMember,
		Status:      models.MemberStatusActive,
	})

	token := makeTestToken(t, user.ID, user.Email, user.Username, string(user.SystemRole))

	engine.GET("/api/v1/workspaces/:id/test", middleware.AdminOnly(), func(c *gin.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api/v1/workspaces/"+ws.ID+"/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Workspace-Id", ws.ID)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	var r resp.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatalf("expected JSON response, got: %s", rec.Body.String())
	}
	if r.Code != http.StatusForbidden {
		t.Errorf("expected code 403 for member on AdminOnly route, got %d", r.Code)
	}
}

func TestWorkspaceAuthMiddleware_WsIDFromURLParam(t *testing.T) {
	engine, st, middleware := setupPermissionTest(t)
	ctx := context.Background()

	user := &models.User{Model: models.Model{ID: "u-param"}, Email: "param@test.com", SystemRole: dto.SystemRoleUser}
	st.Users().Create(ctx, user)

	ws := &models.Workspace{Model: models.Model{ID: "ws-param"}, Slug: "test-param"}
	st.Workspaces().Create(ctx, ws)

	st.WorkspaceMembers().AddMember(ctx, &models.WorkspaceMember{
		WorkspaceID: ws.ID, UserID: user.ID, Role: dto.RoleViewer, Status: models.MemberStatusActive,
	})

	token := makeTestToken(t, user.ID, user.Email, user.Username, string(user.SystemRole))

	engine.GET("/api/v1/workspaces/:id/test", middleware.WorkspaceAuthMiddleware(dto.RoleViewer), func(c *gin.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api/v1/workspaces/"+ws.ID+"/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	// NO X-Workspace-Id header

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Errorf("expected 200 with wsID from URL param, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestWorkspaceAuthMiddleware_HeaderPriorityOverParam(t *testing.T) {
	engine, st, middleware := setupPermissionTest(t)
	ctx := context.Background()

	user := &models.User{Model: models.Model{ID: "u-priority"}, Email: "prio@test.com", SystemRole: dto.SystemRoleUser}
	st.Users().Create(ctx, user)

	ws1 := &models.Workspace{Model: models.Model{ID: "ws-header"}, Slug: "ws1"}
	ws2 := &models.Workspace{Model: models.Model{ID: "ws-param2"}, Slug: "ws2"}
	st.Workspaces().Create(ctx, ws1)
	st.Workspaces().Create(ctx, ws2)

	st.WorkspaceMembers().AddMember(ctx, &models.WorkspaceMember{
		WorkspaceID: ws1.ID, UserID: user.ID, Role: dto.RoleAdmin, Status: models.MemberStatusActive,
	})
	st.WorkspaceMembers().AddMember(ctx, &models.WorkspaceMember{
		WorkspaceID: ws2.ID, UserID: user.ID, Role: dto.RoleViewer, Status: models.MemberStatusActive,
	})

	token := makeTestToken(t, user.ID, user.Email, user.Username, string(user.SystemRole))

	engine.GET("/api/v1/workspaces/:id/test", middleware.AdminOnly(), func(c *gin.Context) {
		wsID := c.GetString("workspace_id")
		c.String(200, wsID)
	})

	req := httptest.NewRequest("GET", "/api/v1/workspaces/"+ws2.ID+"/test", nil) // param = ws2
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Workspace-Id", ws1.ID) // header = ws1

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || rec.Body.String() != ws1.ID {
		t.Errorf("expected header wsID %s to win over param %s, got %d %s", ws1.ID, ws2.ID, rec.Code, rec.Body.String())
	}
}

func TestWorkspaceAuthMiddleware_MissingWsID(t *testing.T) {
	engine, _, middleware := setupPermissionTest(t)

	user := &models.User{Model: models.Model{ID: "u-nows"}, Email: "nows@test.com", SystemRole: dto.SystemRoleUser}
	token := makeTestToken(t, user.ID, user.Email, user.Username, string(user.SystemRole))

	engine.GET("/api/v1/workspaces/:id/test", middleware.WorkspaceAuthMiddleware(dto.RoleViewer), func(c *gin.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api/v1/workspaces//test", nil) // empty :id param
	req.Header.Set("Authorization", "Bearer "+token)
	// NO X-Workspace-Id header

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	var r resp.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatalf("expected JSON response, got: %s", rec.Body.String())
	}
	if r.Code != http.StatusForbidden {
		t.Errorf("expected 403 for missing wsID, got %d", r.Code)
	}
}

func TestWorkspaceAuthMiddleware_InjectsUserContext(t *testing.T) {
	engine, st, middleware := setupPermissionTest(t)
	ctx := context.Background()

	user := &models.User{Model: models.Model{ID: "u-ctx"}, Email: "ctx@test.com", Username: "ctxuser", SystemRole: dto.SystemRoleUser}
	st.Users().Create(ctx, user)

	ws := &models.Workspace{Model: models.Model{ID: "ws-ctx"}, Slug: "test-ctx"}
	st.Workspaces().Create(ctx, ws)

	st.WorkspaceMembers().AddMember(ctx, &models.WorkspaceMember{
		WorkspaceID: ws.ID, UserID: user.ID, Role: dto.RoleViewer, Status: models.MemberStatusActive,
	})

	token := makeTestToken(t, user.ID, user.Email, user.Username, string(user.SystemRole))

	var gotUserID, gotEmail, gotUsername, gotSystemRole string
	engine.GET("/api/v1/workspaces/:id/test", middleware.WorkspaceAuthMiddleware(dto.RoleViewer), func(c *gin.Context) {
		gotUserID = c.GetString("user_id")
		gotEmail = c.GetString("email")
		gotUsername = c.GetString("username")
		gotSystemRole = c.GetString("system_role")
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api/v1/workspaces/"+ws.ID+"/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if gotUserID != user.ID {
		t.Errorf("expected user_id=%q, got %q", user.ID, gotUserID)
	}
	if gotEmail != user.Email {
		t.Errorf("expected email=%q, got %q", user.Email, gotEmail)
	}
	if gotUsername != user.Username {
		t.Errorf("expected username=%q, got %q", user.Username, gotUsername)
	}
	if gotSystemRole != string(dto.SystemRoleUser) {
		t.Errorf("expected system_role=%q, got %q", dto.SystemRoleUser, gotSystemRole)
	}
}

func TestWorkspaceAuthMiddleware_InjectsWorkspaceContext(t *testing.T) {
	engine, st, middleware := setupPermissionTest(t)
	ctx := context.Background()

	user := &models.User{Model: models.Model{ID: "u-wsctx"}, Email: "wsctx@test.com", SystemRole: dto.SystemRoleUser}
	st.Users().Create(ctx, user)

	ws := &models.Workspace{Model: models.Model{ID: "ws-wsctx"}, Slug: "test-wsctx"}
	st.Workspaces().Create(ctx, ws)

	st.WorkspaceMembers().AddMember(ctx, &models.WorkspaceMember{
		WorkspaceID: ws.ID, UserID: user.ID, Role: dto.RoleViewer, Status: models.MemberStatusActive,
	})

	token := makeTestToken(t, user.ID, user.Email, user.Username, string(user.SystemRole))

	var gotWsID string
	var gotWsCtx string
	var gotMember any
	engine.GET("/api/v1/workspaces/:id/test", middleware.WorkspaceAuthMiddleware(dto.RoleViewer), func(c *gin.Context) {
		gotWsID = c.GetString("workspace_id")
		gotWsCtx, _ = c.Request.Context().Value(infra.WorkspaceKey).(string)
		gotMember, _ = c.Get("currentTeamMember")
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api/v1/workspaces/"+ws.ID+"/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Workspace-Id", ws.ID)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if gotWsID != ws.ID {
		t.Errorf("expected workspace_id=%q, got %q", ws.ID, gotWsID)
	}
	if gotWsCtx != ws.ID {
		t.Errorf("expected infra.WorkspaceKey=%q, got %q", ws.ID, gotWsCtx)
	}
	if gotMember == nil {
		t.Error("expected currentTeamMember to be set")
	}
}

func TestWorkspaceAuthMiddleware_RejectsNonMember(t *testing.T) {
	engine, st, middleware := setupPermissionTest(t)
	ctx := context.Background()

	user := &models.User{Model: models.Model{ID: "u-nonmem"}, Email: "nonmem@test.com", SystemRole: dto.SystemRoleUser}
	st.Users().Create(ctx, user)

	ws := &models.Workspace{Model: models.Model{ID: "ws-nonmem"}, Slug: "test-nonmem"}
	st.Workspaces().Create(ctx, ws)

	// User is NOT a member of ws.
	token := makeTestToken(t, user.ID, user.Email, user.Username, string(user.SystemRole))

	engine.GET("/api/v1/workspaces/:id/test", middleware.WorkspaceAuthMiddleware(dto.RoleViewer), func(c *gin.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api/v1/workspaces/"+ws.ID+"/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Workspace-Id", ws.ID)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	var r resp.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatalf("expected JSON response, got: %s", rec.Body.String())
	}
	if r.Code != http.StatusForbidden {
		t.Errorf("expected 403 for non-member, got %d", r.Code)
	}
}

func TestWorkspaceAuthMiddleware_RejectsSuspendedMember(t *testing.T) {
	engine, st, middleware := setupPermissionTest(t)
	ctx := context.Background()

	user := &models.User{Model: models.Model{ID: "u-sus"}, Email: "sus@test.com", SystemRole: dto.SystemRoleUser}
	st.Users().Create(ctx, user)

	ws := &models.Workspace{Model: models.Model{ID: "ws-sus"}, Slug: "test-sus"}
	st.Workspaces().Create(ctx, ws)

	st.WorkspaceMembers().AddMember(ctx, &models.WorkspaceMember{
		WorkspaceID: ws.ID, UserID: user.ID, Role: dto.RoleAdmin, Status: models.MemberStatusSuspended,
	})

	token := makeTestToken(t, user.ID, user.Email, user.Username, string(user.SystemRole))

	engine.GET("/api/v1/workspaces/:id/test", middleware.WorkspaceAuthMiddleware(dto.RoleViewer), func(c *gin.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api/v1/workspaces/"+ws.ID+"/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Workspace-Id", ws.ID)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	var r resp.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatalf("expected JSON response, got: %s", rec.Body.String())
	}
	if r.Code != http.StatusForbidden {
		t.Errorf("expected 403 for suspended member, got %d", r.Code)
	}
}

func TestWorkspaceAuthMiddleware_ViewerPassesRoleViewerRoute(t *testing.T) {
	engine, st, middleware := setupPermissionTest(t)
	ctx := context.Background()

	user := &models.User{Model: models.Model{ID: "u-viewer2"}, Email: "viewer2@test.com", SystemRole: dto.SystemRoleUser}
	st.Users().Create(ctx, user)

	ws := &models.Workspace{Model: models.Model{ID: "ws-viewer2"}, Slug: "test-viewer2"}
	st.Workspaces().Create(ctx, ws)

	st.WorkspaceMembers().AddMember(ctx, &models.WorkspaceMember{
		WorkspaceID: ws.ID, UserID: user.ID, Role: dto.RoleViewer, Status: models.MemberStatusActive,
	})

	token := makeTestToken(t, user.ID, user.Email, user.Username, string(user.SystemRole))

	engine.GET("/api/v1/workspaces/:id/test", middleware.WorkspaceAuthMiddleware(dto.RoleViewer), func(c *gin.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api/v1/workspaces/"+ws.ID+"/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Workspace-Id", ws.ID)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || rec.Body.String() != "ok" {
		t.Errorf("expected 200 'ok' for viewer on RoleViewer route, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestWorkspaceAuthMiddleware_RejectsRevokedToken(t *testing.T) {
	engine, st, middleware, rl := setupWithRevocation(t)
	ctx := context.Background()

	user := &models.User{Model: models.Model{ID: "u-revoke"}, Email: "revoke@test.com", SystemRole: dto.SystemRoleUser}
	st.Users().Create(ctx, user)

	ws := &models.Workspace{Model: models.Model{ID: "ws-revoke"}, Slug: "test-revoke"}
	st.Workspaces().Create(ctx, ws)

	st.WorkspaceMembers().AddMember(ctx, &models.WorkspaceMember{
		WorkspaceID: ws.ID, UserID: user.ID, Role: dto.RoleViewer, Status: models.MemberStatusActive,
	})

	token := makeTestToken(t, user.ID, user.Email, user.Username, string(user.SystemRole))
	claims, err := utils.ParseToken(token)
	if err != nil {
		t.Fatal(err)
	}

	// Revoke the token.
	rl.Revoke(claims.ID, claims.ExpiresAt.Time)

	engine.GET("/api/v1/workspaces/:id/test", middleware.WorkspaceAuthMiddleware(dto.RoleViewer), func(c *gin.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api/v1/workspaces/"+ws.ID+"/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Workspace-Id", ws.ID)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	var r resp.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatalf("expected JSON response, got: %s", rec.Body.String())
	}
	if r.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for revoked token, got %d", r.Code)
	}
}

func TestWorkspaceAuthMiddleware_RejectsMissingToken(t *testing.T) {
	engine, _, middleware := setupPermissionTest(t)

	engine.GET("/api/v1/workspaces/:id/test", middleware.WorkspaceAuthMiddleware(dto.RoleViewer), func(c *gin.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api/v1/workspaces/ws123/test", nil)
	// NO Authorization header

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	var r resp.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatalf("expected JSON response, got: %s", rec.Body.String())
	}
	if r.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing token, got %d", r.Code)
	}
}

func TestWorkspaceAuthMiddleware_RejectsInvalidToken(t *testing.T) {
	engine, _, middleware := setupPermissionTest(t)

	engine.GET("/api/v1/workspaces/:id/test", middleware.WorkspaceAuthMiddleware(dto.RoleViewer), func(c *gin.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/api/v1/workspaces/ws123/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token-here")

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	var r resp.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatalf("expected JSON response, got: %s", rec.Body.String())
	}
	if r.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for invalid token, got %d", r.Code)
	}
}

func TestPlatformAdminOnly_AllowsPlatformAdmin(t *testing.T) {
	rl := auth.NewRevocationList()
	engine, _, mwInst, _ := setupWithRevocation(t)

	// Create a platform admin token.
	token := makeTestToken(t, "u-pa", "pa@test.com", "pa", string(dto.SystemRolePlatformAdmin))

	// Use AuthMiddleware first, then PlatformAdminOnly.
	engine.GET("/test", mw.AuthMiddleware(rl), mwInst.PlatformAdminOnly(), func(c *gin.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for platform admin, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestPlatformAdminOnly_RejectsRegularUser(t *testing.T) {
	rl := auth.NewRevocationList()
	engine, _, mwInst, _ := setupWithRevocation(t)

	token := makeTestToken(t, "u-regular", "reg@test.com", "reg", "")

	engine.GET("/test", mw.AuthMiddleware(rl), mwInst.PlatformAdminOnly(), func(c *gin.Context) {
		c.String(200, "ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	var r resp.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatalf("expected JSON, got %s", rec.Body.String())
	}
	if r.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", r.Code)
	}
}
