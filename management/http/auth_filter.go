package http

import (
	"linkany/management/utils"

	"github.com/gin-gonic/gin"
)

func (s *Server) authFilter() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check the permission
		// If the permission is invalid, return 403
		// If the permission is valid, continue

		// TODO get role from header

		action := c.GetHeader("action")
		resourceType := c.GetHeader("resource-type")
		resourceId := c.GetHeader("resource-id")
		var resType utils.ResourceType
		switch resourceType {
		case "group":
			resType = utils.Group
		case "policy":
			resType = utils.Policy
		case "node":
			resType = utils.Node
		case "label":
			resType = utils.Label
		default:
			WriteForbidden(c.JSON, "Invalid resource type")
			c.Abort()
			return
		}
		if action != "" {
			b, err := s.accessController.CheckAccess(c, resType, resourceId, action)
			if !b || err != nil {
				WriteForbidden(c.JSON, err.Error())
				c.Abort()
				return
			}
		}

		c.Next()
	}
}
