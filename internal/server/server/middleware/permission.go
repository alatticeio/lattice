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

package middleware

import (
	"context"
	"strings"

	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/auth"
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/internal/server/permission"
	"github.com/alatticeio/lattice/pkg/utils"
	"github.com/alatticeio/lattice/pkg/utils/resp"

	"github.com/gin-gonic/gin"
)

// Middleware unifies workspace permission enforcement.
// It combines JWT parsing, revocation check, membership/role check,
// and workspace context injection into a single middleware chain.
type Middleware struct {
	checker        permission.Checker
	store          store.Store
	revocationList *auth.RevocationList
}

// NewMiddleware creates a Middleware with a revocation list.
func NewMiddleware(checker permission.Checker, st store.Store, revocationList *auth.RevocationList) *Middleware {
	return &Middleware{checker: checker, store: st, revocationList: revocationList}
}

// WorkspaceAuthMiddleware enforces workspace access control.
// wsID is taken from X-Workspace-Id header (primary) or URL param :id (fallback).
// The middleware parses JWT, checks revocation, verifies membership/role, and injects workspace context.
func (m *Middleware) WorkspaceAuthMiddleware(requiredRole dto.WorkspaceRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Parse JWT from Authorization header.
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			resp.Unauthorized(c, "未授权，请先登录")
			c.Abort()
			return
		}
		tokenString := authHeader[7:]

		claims, err := utils.ParseToken(tokenString)
		if err != nil {
			resp.Unauthorized(c, "无效的 Token")
			c.Abort()
			return
		}

		// 2. Check revocation.
		if m.revocationList != nil && claims.ID != "" && m.revocationList.IsRevoked(claims.ID) {
			resp.Unauthorized(c, "token has been revoked")
			c.Abort()
			return
		}

		// 3. Write user info to Gin context (same as AuthMiddleware).
		c.Set("user_id", claims.Subject)
		c.Set("username", claims.Username)
		c.Set("email", claims.Email)
		c.Set("system_role", claims.SystemRole)
		c.Set("jti", claims.ID)
		c.Set("exp", claims.ExpiresAt.Time)

		// Also inject into Request context.
		ctx := context.WithValue(c.Request.Context(), infra.UserIDKey, claims.Subject)
		ctx = context.WithValue(ctx, infra.SystemRoleKey, claims.SystemRole)
		ctx = context.WithValue(ctx, infra.UsernameKey, claims.Username)
		c.Request = c.Request.WithContext(ctx)

		// 4. Get wsID from header (primary) or URL param (fallback).
		wsID := c.GetHeader("X-Workspace-Id")
		if wsID == "" {
			wsID = c.Param("id")
		}
		if wsID == "" {
			resp.Forbidden(c, "workspace required: set X-Workspace-Id header or include :id in path")
			c.Abort()
			return
		}

		userID := claims.Subject

		// 5. Check platform admin (read from context we just set).
		if claims.SystemRole == string(dto.SystemRolePlatformAdmin) {
			// Platform admins bypass workspace checks.
			// Still inject workspace context if wsID is provided.
			c.Set("workspace_id", wsID)
			reqCtx := context.WithValue(c.Request.Context(), infra.WorkspaceKey, wsID)
			c.Request = c.Request.WithContext(reqCtx)
			c.Next()
			return
		}

		// 6. Check membership and role.
		member, err := m.checker.RequireWorkspaceRole(c.Request.Context(), wsID, userID, requiredRole)
		if err != nil {
			resp.Forbidden(c, "权限不足")
			c.Abort()
			return
		}

		// 7. Inject workspace context and member info.
		c.Set("workspace_id", wsID)
		c.Set("currentTeamMember", member)
		reqCtx := context.WithValue(c.Request.Context(), infra.WorkspaceKey, wsID)
		c.Request = c.Request.WithContext(reqCtx)

		c.Next()
	}
}

// AdminOnly enforces workspace admin role.
func (m *Middleware) AdminOnly() gin.HandlerFunc {
	return m.WorkspaceAuthMiddleware(dto.RoleAdmin)
}

// PlatformAdminOnly enforces system-level platform_admin role (no workspace scope).
func (m *Middleware) PlatformAdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		systemRole := c.GetString("system_role")
		if systemRole != string(dto.SystemRolePlatformAdmin) {
			resp.Forbidden(c, "platform admin only")
			c.Abort()
			return
		}
		c.Next()
	}
}

// IsPlatformAdmin is a helper for use inside handler functions.
func (m *Middleware) IsPlatformAdmin(c *gin.Context) bool {
	return c.GetString("system_role") == string(dto.SystemRolePlatformAdmin)
}
