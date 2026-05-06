//go:build !pro

package server

import (
	"github.com/alatticeio/lattice/pkg/utils/resp"

	"github.com/gin-gonic/gin"
)

func (s *Server) dashboardRouter() {
	s.GET("/api/v1/dashboard/overview", func(c *gin.Context) {
		resp.PaymentRequired(c, "dashboard analytics requires Lattice Pro — upgrade at https://alattice.io/pro")
	})
}
