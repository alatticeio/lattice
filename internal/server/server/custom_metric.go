package server

import (
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/internal/server/service"
	"github.com/alatticeio/lattice/pkg/utils/resp"

	"github.com/gin-gonic/gin"
)

func (s *Server) customMetricRouter() {
	r := s.Group("/api/v1/metrics/custom")
	r.Use(s.middleware.WorkspaceAuthMiddleware(dto.RoleViewer))
	{
		r.GET("", s.listCustomMetrics())
		r.POST("", s.createCustomMetric())
		r.PUT("/:id", s.updateCustomMetric())
		r.DELETE("/:id", s.deleteCustomMetric())
	}
}

func (s *Server) listCustomMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		wsID := c.GetString("workspace_id")
		if wsID == "" {
			resp.Error(c, "workspace_id required")
			return
		}
		data, err := s.customMetricController.List(c.Request.Context(), wsID)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, data)
	}
}

func (s *Server) createCustomMetric() gin.HandlerFunc {
	return func(c *gin.Context) {
		wsID := c.GetString("workspace_id")
		if wsID == "" {
			resp.Error(c, "workspace_id required")
			return
		}
		createdBy := c.GetString("user_id")
		var req service.CreateCustomMetricRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.Error(c, err.Error())
			return
		}
		data, err := s.customMetricController.Create(c.Request.Context(), wsID, createdBy, req)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, data)
	}
}

func (s *Server) updateCustomMetric() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req service.CreateCustomMetricRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.Error(c, err.Error())
			return
		}
		data, err := s.customMetricController.Update(c.Request.Context(), id, req)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, data)
	}
}

func (s *Server) deleteCustomMetric() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if err := s.customMetricController.Delete(c.Request.Context(), id); err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, nil)
	}
}
