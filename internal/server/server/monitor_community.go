//go:build !pro

package server

import "github.com/gin-gonic/gin"

func (s *Server) monitorRouter() {
	proOnly := func(c *gin.Context) {
		c.JSON(402, gin.H{"error": "network monitoring requires Lattice Pro — upgrade at https://alattice.io/pro"})
	}
	g := s.Group("/api/v1/monitor")
	g.GET("/topology", proOnly)
	g.GET("/ws-snapshot", proOnly)
}
