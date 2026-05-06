//go:build !pro

package server

import (
	"github.com/alatticeio/lattice/pkg/utils/resp"

	"github.com/gin-gonic/gin"
)

func (s *Server) monitorRouter() {
	proOnly := func(c *gin.Context) {
		resp.PaymentRequired(c, "network monitoring requires Lattice Pro — upgrade at https://alattice.io/pro")
	}
	g := s.Group("/api/v1/monitor")
	g.GET("/topology", proOnly)
	g.GET("/ws-snapshot", proOnly)
}
