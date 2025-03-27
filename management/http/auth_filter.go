package http

import (
	"github.com/gin-gonic/gin"
	"linkany/management/utils"
)

func (s *Server) authFilter() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check the permission
		// If the permission is invalid, return 403
		// If the permission is valid, continue

		action := c.GetHeader("action")
		resourceType := c.GetHeader("resourceType")
		resourceId := c.GetInt("resourceId")
		var resType utils.ResourceType
		switch resourceType {
		case "group":
			resType = utils.Group
		}
		if action != "" {
			b, err := s.accessController.CheckAccess(c, resType, uint(resourceId), action)
			if !b || err != nil {
				WriteError(c.JSON, err.Error())
				c.Abort()
				return
			}
		}

		c.Next()
	}
}
