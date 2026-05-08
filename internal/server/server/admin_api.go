package server

import (
	"github.com/alatticeio/lattice/internal/server/server/middleware"

	"github.com/gin-gonic/gin"
)

// nolint:all
func (s *Server) adminRouter() {
	// Routes accessible only by [System Administrators]
	adminGroup := s.Group("/api/v1/admin")
	adminGroup.Use(middleware.AuthMiddleware(s.revocationList), s.middleware.PlatformAdminOnly())
	{
		adminGroup.POST("/promote-user", handlePromoteUser())
		adminGroup.POST("/create-user", handleCreateUser())
	}

	// Routes accessible by [Workspace Administrators]
	nsGroup := s.Group("/api/v1/ns/:ns_id")
	nsGroup.Use(middleware.AuthMiddleware(s.revocationList), s.middleware.AdminOnly())
	{
		nsGroup.POST("/add-member", handleAddMemberToProject())
	}
}

// nolint:all
func handlePromoteUser() gin.HandlerFunc {
	return func(c *gin.Context) {}
}

// nolint:all
func handleAddMemberToProject() gin.HandlerFunc {
	return func(c *gin.Context) {}
}

// nolint:all
func handleCreateUser() gin.HandlerFunc {
	return func(c *gin.Context) {

	}
}
