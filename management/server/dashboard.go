//go:build pro

package server

import (
	"wireflow/pkg/utils/resp"

	"github.com/gin-gonic/gin"
)

// nolint:all
func (s *Server) dashboardRouter() {
	dashApi := s.Group("/api/v1/dashboard")
	{
		dashApi.GET("/overview", s.dashboardOverview())
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
