//go:build pro

package server

import (
	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/internal/server/server/middleware"
	"github.com/alatticeio/lattice/pkg/utils/resp"

	"github.com/gin-gonic/gin"
)

func (s *Server) monitorRouter() {
	monitorRouter := s.Group("/api/v1/monitor")
	monitorRouter.Use(middleware.AuthMiddleware(s.revocationList))
	{
		monitorRouter.GET("/topology", s.topology())
		monitorRouter.GET("/ws-snapshot", s.middleware.WorkspaceAuthMiddleware(dto.RoleViewer), s.workspaceSnapshot())
	}
}

func (s *Server) topology() gin.HandlerFunc {
	return func(c *gin.Context) {
		ve, err := s.monitorController.GetTopologySnapshot(c.Request.Context())
		if err != nil {
			resp.Error(c, "get topoloty falied")
			return
		}

		resp.OK(c, ve)
	}
}

func (s *Server) workspaceSnapshot() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		wsId := ctx.Value(infra.WorkspaceKey).(string)
		ve, err := s.monitorController.GetWorkspaceAggregatedMonitor(ctx, wsId)
		if err != nil {
			resp.Error(c, "get topoloty falied")
			return
		}

		resp.OK(c, ve)
	}
}
