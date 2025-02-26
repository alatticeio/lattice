package http

import (
	"github.com/gin-gonic/gin"
	"linkany/management/client"
	"linkany/management/dto"
	"linkany/management/entity"
	"strconv"
)

func (s *Server) RegisterAccessRoutes() {
	routes := s.RouterGroup.Group(PREFIX + "/access")
	routes.POST("/policy", s.authCheck(), s.createAccessPolicy())
}

func (s *Server) createAccessPolicy() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req dto.AccessPolicyDto
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}

		err := s.accessController.CreatePolicy(&entity.AccessPolicy{})
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		WriteOK(c.JSON, nil)
	}
}

func (s *Server) listAccessPolicies() gin.HandlerFunc {
	return func(c *gin.Context) {
		policies, err := s.accessController.ListPolicies()
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}

		c.JSON(client.Success(policies))
	}
}

func (s *Server) updateAccessPolicy() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req dto.AccessPolicyDto
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}

		err := s.accessController.UpdatePolicy(&entity.AccessPolicy{})
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		WriteOK(c.JSON, nil)
	}
}

func (s *Server) deleteAccessPolicy() gin.HandlerFunc {
	return func(c *gin.Context) {
		policyID := c.Param("policyID")
		id, _ := strconv.Atoi(policyID)
		err := s.accessController.DeletePolicy(uint(id))
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		WriteOK(c.JSON, nil)
	}
}

func (s *Server) addAccessRule() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req dto.AccessRuleDto
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}

		err := s.accessController.AddRule(&entity.AccessRule{})
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		WriteOK(c.JSON, nil)
	}
}

func (s *Server) updateAccessRule() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req dto.AccessRuleDto
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}

		err := s.accessController.UpdateRule(&entity.AccessRule{})
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		WriteOK(c.JSON, nil)
	}
}

func (s *Server) deleteAccessRule() gin.HandlerFunc {
	return func(c *gin.Context) {
		ruleID := c.Param("ruleID")
		id, _ := strconv.Atoi(ruleID)
		err := s.accessController.DeleteRule(uint(id))
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		WriteOK(c.JSON, nil)
	}
}

func (s *Server) listAccessRules() gin.HandlerFunc {
	return func(c *gin.Context) {
		policyID := c.Param("policyID")
		id, _ := strconv.Atoi(policyID)
		rules, err := s.accessController.ListPolicyRules(uint(id))
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}

		c.JSON(client.Success(rules))
	}
}

func (s *Server) checkAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		sourceNodeID := c.GetUint("sourceNodeID")
		targetNodeID := c.GetUint("targetNodeID")
		action := c.Query("action")
		allowed, err := s.accessController.CheckAccess(sourceNodeID, targetNodeID, action)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}

		c.JSON(client.Success(allowed))
	}
}
