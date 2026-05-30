package server

import (
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/internal/server/service"
	"github.com/alatticeio/lattice/pkg/utils/resp"

	"github.com/gin-gonic/gin"
)

func (s *Server) peerIdentityRouter() {
	r := s.Group("/api/v1/peer-identities")
	r.Use(s.middleware.WorkspaceAuthMiddleware(dto.RoleViewer))
	{
		r.GET("", s.listPeerIdentities())
		r.POST("", s.createPeerIdentity())
		r.GET("/:id", s.getPeerIdentity())
		r.PUT("/:id", s.updatePeerIdentity())
		r.DELETE("/:id", s.deletePeerIdentity())
	}
}

func (s *Server) listPeerIdentities() gin.HandlerFunc {
	return func(c *gin.Context) {
		wsID := c.GetString("workspace_id")
		if wsID == "" {
			resp.Error(c, "workspace_id required")
			return
		}
		data, err := s.peerIdentityController.List(c.Request.Context(), wsID)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, data)
	}
}

func (s *Server) createPeerIdentity() gin.HandlerFunc {
	return func(c *gin.Context) {
		wsID := c.GetString("workspace_id")
		if wsID == "" {
			resp.Error(c, "workspace_id required")
			return
		}
		var req service.CreatePeerIdentityRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.Error(c, err.Error())
			return
		}
		data, err := s.peerIdentityController.Create(c.Request.Context(), wsID, req)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, data)
	}
}

func (s *Server) getPeerIdentity() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		data, err := s.peerIdentityController.Get(c.Request.Context(), id)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, data)
	}
}

func (s *Server) updatePeerIdentity() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req service.CreatePeerIdentityRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.Error(c, err.Error())
			return
		}
		data, err := s.peerIdentityController.Update(c.Request.Context(), id, req)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, data)
	}
}

func (s *Server) deletePeerIdentity() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if err := s.peerIdentityController.Delete(c.Request.Context(), id); err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, nil)
	}
}
