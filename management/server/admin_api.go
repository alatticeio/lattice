package server

import (
	"wireflow/management/server/middleware"

	"github.com/gin-gonic/gin"
)

func (s *Server) adminRouter() {
	r := s.Engine

	// 只有【系统管理员】才能访问的路由
	adminGroup := r.Group("/api/v1/admin")
	adminGroup.Use(middleware.AuthMiddleware(), AdminOnly())
	{
		adminGroup.POST("/create-namespace", handleCreateNS())
		adminGroup.POST("/promote-user", handlePromoteUser())
	}

	// 【空间管理员】访问的路由
	nsGroup := r.Group("/api/v1/ns/:ns_id")
	nsGroup.Use(middleware.AuthMiddleware(), NamespaceAdminOnly())
	{
		nsGroup.POST("/add-member", handleAddMemberToProject())
	}
}

func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {

	}
}

func NamespaceAdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {}
}

func handleCreateNS() gin.HandlerFunc {
	return func(c *gin.Context) {}
}

func handlePromoteUser() gin.HandlerFunc {
	return func(c *gin.Context) {}
}

func handleAddMemberToProject() gin.HandlerFunc {
	return func(c *gin.Context) {}
}
