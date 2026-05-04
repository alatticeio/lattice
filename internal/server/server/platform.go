package server

import (
	"strings"

	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/internal/server/server/middleware"
	"github.com/alatticeio/lattice/pkg/utils/resp"

	"github.com/gin-gonic/gin"
)

func (s *Server) platformRouter() {
	r := s.Group("/api/v1/settings/platform")
	r.Use(middleware.AuthMiddleware(s.revocationList))
	// Only platform_admin can read/write platform settings
	r.Use(s.middleware.PlatformAdminOnly())
	{
		r.GET("", s.getPlatformSettings())
		r.PUT("", s.updatePlatformSettings())
	}
}

func (s *Server) getPlatformSettings() gin.HandlerFunc {
	return func(c *gin.Context) {
		settings, err := s.platformController.GetSettings(c.Request.Context())
		if err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, settings)
	}
}

func (s *Server) updatePlatformSettings() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req dto.PlatformSettingsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, "invalid request body")
			return
		}
		val := strings.TrimSpace(req.NatsURL)
		if val != "" && !strings.HasPrefix(val, "nats://") && !strings.HasPrefix(val, "nats+tls://") {
			resp.BadRequest(c, "NATS URL must start with nats:// or nats+tls://")
			return
		}
		if err := s.platformController.UpdateSettings(c.Request.Context(), dto.PlatformSettingsRequest{NatsURL: val}); err != nil {
			resp.Error(c, err.Error())
			return
		}
		resp.OK(c, nil)
	}
}
