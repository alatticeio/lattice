package server

import (
	"github.com/gin-gonic/gin"
)

func (s *Server) authFilter() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}
