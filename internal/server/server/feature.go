package server

import (
	"github.com/alatticeio/lattice/internal/server/server/middleware"
	"github.com/alatticeio/lattice/pkg/utils/resp"

	"github.com/gin-gonic/gin"
)

func (s *Server) featureRouter() {
	// GET is available to all authenticated users.
	readGroup := s.Group("/api/v1/features")
	readGroup.Use(middleware.AuthMiddleware(s.revocationList))
	{
		readGroup.GET("", s.listFeatures())
	}

	// PUT is platform-admin only.
	writeGroup := s.Group("/api/v1/features")
	writeGroup.Use(middleware.AuthMiddleware(s.revocationList))
	writeGroup.Use(s.middleware.PlatformAdminOnly())
	{
		writeGroup.PUT("/:key", s.updateFeature())
	}
}

func (s *Server) listFeatures() gin.HandlerFunc {
	return func(c *gin.Context) {
		flags, err := s.featureController.ListFlags(c.Request.Context())
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, flags)
	}
}

func (s *Server) updateFeature() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.Param("key")
		if key == "" {
			resp.BadRequest(c, "feature key is required")
			return
		}

		var req struct {
			Enabled bool `json:"enabled"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, "invalid request body")
			return
		}

		if err := s.featureController.UpdateFlag(c.Request.Context(), key, req.Enabled); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}
		resp.OK(c, nil)
	}
}
