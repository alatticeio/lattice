//go:build pro

package server

import (
	"wireflow/internal/infra"
	"wireflow/management/server/middleware"
	"wireflow/pkg/utils/resp"

	"github.com/gin-gonic/gin"
)

func (s *Server) monitorRouter() {

	monitorRouter := s.Group("/api/v1/monitor")
	//monitorRouter.Use(dex.AuthMiddleware())
	{
		monitorRouter.GET("/topology", s.topology())
		monitorRouter.GET("/ws-snapshot", middleware.TenantContextMiddleware(), s.workspaceSnapshot())

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
