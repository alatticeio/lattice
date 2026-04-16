//go:build !pro

package server

import "github.com/gin-gonic/gin"

func (s *Server) monitorRouter() {
	proOnly := func(c *gin.Context) {
		c.JSON(402, gin.H{"error": "network monitoring requires Wireflow Pro — upgrade at https://wireflow.run/pro"})
	}
	g := s.Group("/api/v1/monitor")
	g.GET("/topology", proOnly)
	g.GET("/ws-snapshot", proOnly)
}
