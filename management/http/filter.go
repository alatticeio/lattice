package http

import (
	"github.com/gin-gonic/gin"
	"linkany/management/client"
)

func (s *Server) tokenFilter() gin.HandlerFunc {
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

		user, err := s.tokener.Parse(token)
		if err != nil {
			WriteError(c.JSON, "Invalid token")
			c.Abort()
			return
		}

		b, u, err := s.tokener.Verify(user.Username, user.Password)
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

		// put user into context
		c.Set("username", user.Username)
		c.Set("userId", u.ID)

		c.Next()
	}
}
