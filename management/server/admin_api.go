package server

import (
	"wireflow/management/server/middleware"

	"github.com/gin-gonic/gin"
)

// nolint:all
func (s *Server) adminRouter() {
	// 只有【系统管理员】才能访问的路由
	adminGroup := s.Group("/api/v1/admin")
	adminGroup.Use(middleware.AuthMiddleware(), middleware.AdminOnly())
	{
		adminGroup.POST("/promote-user", handlePromoteUser())
		adminGroup.POST("/create-user", handleCreateUser())
	}

	// 【空间管理员】访问的路由
	nsGroup := s.Group("/api/v1/ns/:ns_id")
	nsGroup.Use(middleware.AuthMiddleware(), NamespaceAdminOnly())
	{
		nsGroup.POST("/add-member", handleAddMemberToProject())
	}
}

func NamespaceAdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {}
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
