package server

import (
	"github.com/gin-gonic/gin"
	"linkany/management/client"
)

func (s *Server) authCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check the token
		// If the token is invalid, return 401
		// If the token is valid, continue
		token := c.GetHeader("Authorization")
		if token == "" {
			c.JSON(client.Unauthorized())
			c.Abort()
			return
		}

		b, err := s.tokener.Verify("linkany", "linkany.io", token)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			c.Abort()
			return
		}

		if !b {
			c.JSON(client.Unauthorized())
			c.Abort()
			return
		}

		c.Next()
	}
}
