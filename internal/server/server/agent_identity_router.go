package server

import (
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/internal/server/service"
	"github.com/alatticeio/lattice/pkg/utils/resp"

	"github.com/gin-gonic/gin"
)

func (s *Server) agentIdentityRouter() {
	r := s.Group("/api/v1/agent-identities")
	r.Use(s.middleware.WorkspaceAuthMiddleware(dto.RoleViewer))
	{
		r.GET("", s.listAgentIdentities())
		r.POST("", s.createAgentIdentity())
		r.GET("/:id", s.getAgentIdentity())
		r.PUT("/:id", s.updateAgentIdentity())
		r.DELETE("/:id", s.deleteAgentIdentity())
	}
}

func (s *Server) listAgentIdentities() gin.HandlerFunc {
	return func(c *gin.Context) {
		wsID := c.GetString("workspace_id")
		if wsID == "" {
			resp.Error(c, "workspace_id required")
			return
		}
		data, err := s.agentIdentityServerController.List(c.Request.Context(), wsID)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, data)
	}
}

func (s *Server) createAgentIdentity() gin.HandlerFunc {
	return func(c *gin.Context) {
		wsID := c.GetString("workspace_id")
		if wsID == "" {
			resp.Error(c, "workspace_id required")
			return
		}
		var req service.CreateAgentIdentityRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.Error(c, err.Error())
			return
		}
		data, err := s.agentIdentityServerController.Create(c.Request.Context(), wsID, req)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, data)
	}
}

func (s *Server) getAgentIdentity() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		data, err := s.agentIdentityServerController.Get(c.Request.Context(), id)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, data)
	}
}

func (s *Server) updateAgentIdentity() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req service.CreateAgentIdentityRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.Error(c, err.Error())
			return
		}
		data, err := s.agentIdentityServerController.Update(c.Request.Context(), id, req)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, data)
	}
}

func (s *Server) deleteAgentIdentity() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if err := s.agentIdentityServerController.Delete(c.Request.Context(), id); err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, nil)
	}
}
