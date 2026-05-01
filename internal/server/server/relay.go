package server

import (
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/pkg/utils/resp"

	"github.com/gin-gonic/gin"
)

func (s *Server) relayRouter() {
	g := s.Group("/api/v1/settings/relays")
	g.Use(s.middleware.WorkspaceAuthMiddleware(dto.RoleViewer))
	{
		g.GET("", s.listRelays())
		g.POST("", s.createRelay())
		g.PUT("/:id", s.updateRelay())
		g.DELETE("/:id", s.deleteRelay())
		g.POST("/:id/test", s.testRelay())
	}
}

func (s *Server) listRelays() gin.HandlerFunc {
	return func(c *gin.Context) {
		var pageParam dto.PageRequest
		if err := c.ShouldBindQuery(&pageParam); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		result, err := s.relayController.List(c.Request.Context(), &pageParam)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, result)
	}
}

func (s *Server) createRelay() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req dto.RelayDto
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		v, err := s.relayController.Create(c.Request.Context(), &req)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, v)
	}
}

func (s *Server) updateRelay() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			resp.BadRequest(c, "relay id is required")
			return
		}
		var req dto.RelayDto
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		v, err := s.relayController.Update(c.Request.Context(), id, &req)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, v)
	}
}

func (s *Server) deleteRelay() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			resp.BadRequest(c, "relay id is required")
			return
		}
		if err := s.relayController.Delete(c.Request.Context(), id); err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, nil)
	}
}

// testRelay performs a lightweight TCP dial to the relay's TcpUrl and returns
// latency.  It is a best-effort probe — no status update is written here;
// that is left to the reconciler's periodic health-check loop.
func (s *Server) testRelay() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			resp.BadRequest(c, "relay id is required")
			return
		}
		result, err := s.relayController.Test(c.Request.Context(), id)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, result)
	}
}
