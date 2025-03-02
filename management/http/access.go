package http

import (
	"github.com/gin-gonic/gin"
	"linkany/management/client"
	"linkany/management/dto"
	"strconv"
)

func (s *Server) RegisterAccessRoutes() {
	routes := s.RouterGroup.Group(PREFIX + "/access")
	routes.POST("/policy", s.authCheck(), s.createAccessPolicy())
	routes.PUT("/policy", s.authCheck(), s.updateAccessPolicy())
	routes.DELETE("/policy/:policyID", s.authCheck(), s.deleteAccessPolicy())
	routes.GET("/policy/list", s.authCheck(), s.listAccessPolicies())

	// rule
	routes.POST("/rule", s.authCheck(), s.addAccessRule())
	routes.PUT("/rule", s.authCheck(), s.updateAccessRule())
	routes.DELETE("/rule/:ruleID", s.authCheck(), s.deleteAccessRule())
	routes.GET("/rule/list", s.authCheck(), s.listAccessRules())
}

func (s *Server) createAccessPolicy() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req dto.AccessPolicyDto
		if err := c.ShouldBindJSON(&req); err != nil {
			WriteBadRequest(c.JSON, err.Error())
			return
		}

		err := s.accessController.CreatePolicy(c, &req)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, nil)
	}
}

func (s *Server) listAccessPolicies() gin.HandlerFunc {
	return func(c *gin.Context) {
		var params dto.AccessPolicyParams
		if err := c.ShouldBind(&params); err != nil {
			return
		}
		policies, err := s.accessController.ListPolicies(c, &params)
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

		err := s.accessController.UpdatePolicy(c, &req)
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
		err := s.accessController.DeletePolicy(c, uint(id))
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

		err := s.accessController.AddRule(c, &req)
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

		err := s.accessController.UpdateRule(c, &req)
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
		var params dto.AccessPolicyRuleParams
		if err := c.ShouldBind(&params); err != nil {
			WriteBadRequest(c.JSON, err.Error())
			return
		}
		rules, err := s.accessController.ListPolicyRules(c, &params)
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
