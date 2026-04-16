//go:build !pro

package server

import "github.com/gin-gonic/gin"

func (s *Server) dashboardRouter() {
	s.GET("/api/v1/dashboard/overview", func(c *gin.Context) {
		c.JSON(402, gin.H{"error": "dashboard analytics requires Wireflow Pro — upgrade at https://wireflow.run/pro"})
	})
}
