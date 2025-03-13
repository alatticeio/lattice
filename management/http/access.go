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
	routes.GET("/policy/page", s.authCheck(), s.listAccessPolicies())

	routes.GET("/policy/list", s.authCheck(), s.listPolicies())

	// rule
	routes.GET("/rule/:ruleID", s.authCheck(), s.getRule())
	routes.POST("/rule", s.authCheck(), s.addAccessRule())
	routes.PUT("/rule", s.authCheck(), s.updateAccessRule())
	routes.DELETE("/rule/:ruleID", s.authCheck(), s.deleteAccessRule())
	// policy rule
	routes.GET("/policy/rules", s.authCheck(), s.listAccessRules())

	//permissions
	routes.GET("/permissions", s.authCheck(), s.listPermissions())
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
		var err error

		s.logger.Infof("url params: %s", c.Request.URL.Query())
		if err = c.ShouldBindQuery(&params); err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		policies, err := s.accessController.ListPagePolicies(c, &params)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		WriteOK(c.JSON, policies)
	}
}

func (s *Server) listPolicies() gin.HandlerFunc {
	return func(c *gin.Context) {
		var params dto.AccessPolicyParams
		var err error

		s.logger.Infof("url params: %s", c.Request.URL.Query())
		if err = c.ShouldBindQuery(&params); err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		policies, err := s.accessController.ListPolicies(c, &params)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		WriteOK(c.JSON, policies)
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
		var err error

		if err = c.ShouldBindQuery(&params); err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		rules, err := s.accessController.ListPolicyRules(c, &params)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		WriteOK(c.JSON, rules)
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

func (s *Server) getRule() gin.HandlerFunc {
	return func(c *gin.Context) {
		ruleID := c.Param("ruleID")
		id, _ := strconv.Atoi(ruleID)
		rule, err := s.accessController.GetRule(c, int64(id))
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		WriteOK(c.JSON, rule)
	}
}

func (s *Server) listPermissions() gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			params dto.PermissionParams
			err    error
		)
		if err = c.ShouldBindQuery(&params); err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		permissions, err := s.accessController.ListPermissions(c, &params)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		WriteOK(c.JSON, permissions)
	}
}
