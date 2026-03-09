package server

import (
	"context"
	"time"
	"wireflow/internal"
	"wireflow/internal/infra"
	"wireflow/management/dto"
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

// 服务端内部逻辑：定时扫描数据库或缓存更新指标
func (s *Server) StartStatusTick() {
	go func() {
		for range time.Tick(30 * time.Second) {
			// 从数据库查出空间利用率
			res, err := s.workspaceController.ListWorkspaces(context.TODO(), &dto.PageRequest{})
			if err != nil {
				s.logger.Error("list workspace error", err)
			}
			for _, item := range res.List {
				// TODO change to real ws
				internal.WorkspaceResourceUsage.WithLabelValues("ws-01", "used", "nodes").Set(float64(item.QuotaUsage))
			}
		}
	}()
}
