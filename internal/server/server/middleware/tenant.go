package middleware

import (
	"context"

	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/pkg/utils/resp"

	"github.com/gin-gonic/gin"
)

// TenantMiddleware enforces workspace membership on requests.
type TenantMiddleware struct {
	store store.Store
}

func NewTenantMiddleware(st store.Store) *TenantMiddleware {
	return &TenantMiddleware{store: st}
}

func (m *TenantMiddleware) Handle() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		wsID := c.GetHeader("X-Workspace-Id")

		// Query the DB for the user's current system_role so that role changes
		// take effect immediately without requiring re-login.
		var systemRole string
		if userID != "" {
			if user, err := m.store.Users().GetByID(c.Request.Context(), userID); err == nil {
				systemRole = string(user.SystemRole)
			}
		}

		// Platform admins may operate without a workspace, or across any workspace.
		if systemRole == string(dto.SystemRolePlatformAdmin) {
			if wsID != "" {
				injectWorkspace(c, wsID, false)
			}
			c.Next()
			return
		}

		if wsID == "" {
			resp.Forbidden(c, "workspace required: set X-Workspace-Id header")
			c.Abort()
			return
		}

		_, err := m.store.WorkspaceMembers().GetMembership(c.Request.Context(), wsID, userID)
		if err != nil {
			resp.Forbidden(c, "not a member of this workspace")
			c.Abort()
			return
		}

		injectWorkspace(c, wsID, true)
		c.Next()
	}
}

func injectWorkspace(c *gin.Context, wsID string, strict bool) {
	ctx := context.WithValue(c.Request.Context(), infra.WorkspaceKey, wsID)
	ctx = context.WithValue(ctx, infra.StrictTenantKey, strict)
	c.Request = c.Request.WithContext(ctx)
}
