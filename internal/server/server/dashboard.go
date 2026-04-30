//go:build pro

package server

import (
	"github.com/alatticeio/lattice/internal/server/server/middleware"
	"github.com/alatticeio/lattice/pkg/utils/resp"

	"github.com/gin-gonic/gin"
)

func (s *Server) dashboardRouter() {
	// Global dashboard — platform_admin only
	dashApi := s.Group("/api/v1/dashboard")
	dashApi.Use(middleware.AuthMiddleware())
	{
		dashApi.GET("/overview", s.dashboardOverview())
	}

	// Workspace-scoped dashboard — any workspace member
	wsApi := s.Group("/api/v1/workspaces/:id/dashboard")
	wsApi.Use(middleware.AuthMiddleware())
	{
		wsApi.GET("", s.workspaceDashboard())
	}
}

func (s *Server) dashboardOverview() gin.HandlerFunc {
	return func(c *gin.Context) {
		data, err := s.monitorController.GetGlobalDashboard(c.Request.Context())
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, data)
	}
}

func (s *Server) workspaceDashboard() gin.HandlerFunc {
	return func(c *gin.Context) {
		wsID := c.Param("id")
		data, err := s.monitorController.GetWorkspaceDashboard(c.Request.Context(), wsID)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, data)
	}
}
