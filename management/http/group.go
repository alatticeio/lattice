package http

import (
	"github.com/gin-gonic/gin"
	"linkany/management/client"
	"linkany/management/dto"
	"strings"
)

func (s *Server) RegisterGroupRoutes() {
	nodeGroup := s.RouterGroup.Group(PREFIX + "/group")

	// group policy
	nodeGroup.GET("/policy/list", s.authCheck(), s.listGroupPolicies())

	// node group
	nodeGroup.GET("/:id", s.authCheck(), s.GetNodeGroup())
	nodeGroup.POST("/", s.authCheck(), s.createGroup())
	nodeGroup.PUT("/u", s.authCheck(), s.updateGroup())
	nodeGroup.DELETE("/:id", s.authCheck(), s.deleteGroup())
	nodeGroup.GET("/list", s.authCheck(), s.listGroups())
}

func (s *Server) listGroupPolicies() gin.HandlerFunc {
	return func(c *gin.Context) {
		var params dto.GroupPolicyParams
		var err error

		s.logger.Infof("url params: %s", c.Request.URL.Query())
		if err = c.ShouldBindQuery(&params); err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		policies, err := s.groupController.ListGroupPolicies(c, &params)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		WriteOK(c.JSON, policies)
	}
}

// group handler
func (s *Server) GetNodeGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		nodeId := c.Param("id")

		nodeGroup, err := s.groupController.GetNodeGroup(c, nodeId)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		c.JSON(client.Success(nodeGroup))
	}
}

func (s *Server) createGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		var nodeGroupDto dto.NodeGroupDto
		if err := c.ShouldBindJSON(&nodeGroupDto); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}

		token := c.GetHeader("Authorization")
		user, err := s.userController.Get(token)
		nodeGroupDto.CreatedBy = user.Username
		nodeGroupDto.Owner = user.ID
		nodeGroup, err := s.groupController.CreateGroup(c, &nodeGroupDto)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		c.JSON(client.Success(nodeGroup))
	}
}

func (s *Server) updateGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		var nodeGroupDto dto.NodeGroupDto
		if err := c.ShouldBind(&nodeGroupDto); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}

		if nodeGroupDto.NodeArray != "" {
			nodeGroupDto.Nodes = strings.Split(nodeGroupDto.NodeArray, ",")
		}

		if nodeGroupDto.PolicyArray != "" {
			nodeGroupDto.Policies = strings.Split(nodeGroupDto.PolicyArray, ",")
		}

		err := s.groupController.UpdateGroup(c, &nodeGroupDto)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		c.JSON(client.Success(nil))
	}
}

func (s *Server) deleteGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		err := s.groupController.DeleteGroup(c, id)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		c.JSON(client.Success(nil))
	}
}

func (s *Server) listGroups() gin.HandlerFunc {
	return func(c *gin.Context) {
		var params dto.GroupParams
		if err := c.ShouldBindQuery(&params); err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		nodeGroups, err := s.groupController.ListGroups(c, &params)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		WriteOK(c.JSON, nodeGroups)
	}
}
