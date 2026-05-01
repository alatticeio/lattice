package server

import (
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/internal/server/server/middleware"
	"github.com/alatticeio/lattice/pkg/utils/resp"

	"github.com/gin-gonic/gin"
)

func (s *Server) workspaceRouter() {
	workspaceGroup := s.Group("/api/v1/workspaces")
	workspaceGroup.Use(middleware.AuthMiddleware(s.revocationList))
	{
		workspaceGroup.POST("/add", s.handleAddWs())
		workspaceGroup.GET("/list", s.handleListWs())
		workspaceGroup.PUT("/:id", s.middleware.AdminOnly(), s.handleUpdateWs())
		workspaceGroup.DELETE("/:id", s.middleware.AdminOnly(), s.handleDeleteWs())
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

func (s *Server) handleUpdateWs() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			resp.BadRequest(c, "id is required")
			return
		}
		var workspaceDto dto.WorkspaceDto
		if err := c.ShouldBindJSON(&workspaceDto); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		vo, err := s.workspaceController.UpdateWorkspace(c.Request.Context(), id, &workspaceDto)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, vo)
	}
}

func (s *Server) handleDeleteWs() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			resp.BadRequest(c, "id is required")
			return
		}

		err := s.workspaceController.DeleteWorkspace(c.Request.Context(), id)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}

		resp.OK(c, "ok")
	}
}
