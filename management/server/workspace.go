package server

import (
	"wireflow/management/dto"
	"wireflow/management/server/middleware"
	"wireflow/pkg/utils/resp"

	"github.com/gin-gonic/gin"
)

func (s *Server) workspaceRouter() {
	// 只有【系统管理员】才能访问的路由
	adminGroup := s.Group("/api/v1/workspaces")
	adminGroup.Use(middleware.AuthMiddleware())
	{
		adminGroup.POST("/add", s.handleAddWs())
		adminGroup.GET("/list", s.handleListWs())
	}
}

func (s *Server) handleAddWs() gin.HandlerFunc {
	return func(c *gin.Context) {
		var workspaceDto dto.WorkspaceDto
		if err := c.ShouldBindJSON(&workspaceDto); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}

		workspaceVo, err := s.workspaceController.AddWorkspace(c.Request.Context(), &workspaceDto)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}

		resp.OK(c, workspaceVo)
	}
}

func (s *Server) handleListWs() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req dto.PageRequest

		if err := c.ShouldBindQuery(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}

		res, err := s.workspaceController.ListWorkspaces(c.Request.Context(), &req)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}

		resp.OK(c, res)
	}
}
