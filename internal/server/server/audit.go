package server

import (
	"fmt"
	"strconv"

	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/pkg/utils/resp"

	"github.com/gin-gonic/gin"
)

func (s *Server) auditRouter() {
	// Workspace-scoped audit logs (any workspace member can read their own workspace logs).
	ws := s.Group("/api/v1/workspaces/:id/audit-logs")
	ws.Use(s.middleware.WorkspaceAuthMiddleware(dto.RoleViewer))
	{
		ws.GET("", s.handleListAuditLogs())
	}

	// Platform-level audit logs (platform_admin only).
	platform := s.Group("/api/v1/audit-logs")
	platform.Use(s.middleware.PlatformAdminOnly())
	{
		platform.GET("", s.handleListAuditLogs())
	}
}

func (s *Server) handleListAuditLogs() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Support both workspace-scoped (/workspaces/:id/audit-logs) and global.
		wsID := c.Param("id")
		if wsID == "" {
			wsID = c.Query("workspaceId")
		}

		filter := store.AuditLogFilter{
			WorkspaceID: wsID,
			Action:      c.Query("action"),
			Resource:    c.Query("resource"),
			Status:      c.Query("status"),
			Keyword:     c.Query("keyword"),
			From:        c.Query("from"),
			To:          c.Query("to"),
		}

		if err := bindPage(c, &filter.Page, &filter.PageSize); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}

		result, err := s.auditController.List(c.Request.Context(), filter)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, result)
	}
}

// bindPage reads ?page= and ?pageSize= query params with sensible defaults.
func bindPage(c *gin.Context, page, pageSize *int) error {
	p, ps := 1, 20
	if v := c.Query("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid page: %w", err)
		}
		p = n
	}
	if v := c.Query("pageSize"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid pageSize: %w", err)
		}
		ps = n
	}
	*page = p
	*pageSize = ps
	return nil
}
