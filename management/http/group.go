package http

import (
	"github.com/gin-gonic/gin"
	"linkany/management/client"
	"linkany/management/dto"
	"strconv"
	"strings"
)

func (s *Server) RegisterGroupRoutes() {
	nodeGroup := s.RouterGroup.Group(PREFIX + "/group")

	// group policy
	nodeGroup.GET("/policy/list", s.authCheck(), s.listGroupPolicies())
	nodeGroup.DELETE("/:id/policy/:policyId", s.deleteGroupPolicy())
	nodeGroup.DELETE("/:id/node/:nodeId", s.deleteGroupNode())

	// node group
	nodeGroup.GET("/:id", s.authCheck(), s.GetNodeGroup())
	nodeGroup.POST("/a", s.authCheck(), s.createGroup())
	nodeGroup.PUT("/u", s.authCheck(), s.updateGroup())
	nodeGroup.DELETE("/:id", s.authCheck(), s.deleteGroup())
	nodeGroup.GET("/list", s.authCheck(), s.listGroups())
	nodeGroup.GET("/q", s.authCheck(), s.queryGroups())
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

func (s *Server) deleteGroupPolicy() gin.HandlerFunc {
	return func(c *gin.Context) {
		groupId := c.Param("id")
		gid, err := strconv.Atoi(groupId)
		if err != nil {
			WriteError(c.JSON, "invalid group id")
			return
		}
		policyId := c.Param("policyId")
		pid, err := strconv.Atoi(policyId)
		if err != nil {
			WriteError(c.JSON, "invalid policy id")
			return
		}

		err = s.groupController.DeleteGroupPolicy(c, uint(gid), uint(pid))
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		WriteOK(c.JSON, nil)
	}
}

func (s *Server) deleteGroupNode() gin.HandlerFunc {
	return func(c *gin.Context) {
		groupId := c.Param("id")
		gid, err := strconv.Atoi(groupId)
		if err != nil {
			WriteError(c.JSON, "invalid group id")
			return
		}
		nodeId := c.Param("nodeId")
		pid, err := strconv.Atoi(nodeId)
		if err != nil {
			WriteError(c.JSON, "invalid nodeId id")
			return
		}

		err = s.groupController.DeleteGroupNode(c, uint(gid), uint(pid))
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		WriteOK(c.JSON, nil)
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
		nodeGroupDto.Owner = uint64(user.ID)

		if nodeGroupDto.NodeIds != "" {
			nodeGroupDto.NodeIdList = strings.Split(nodeGroupDto.NodeIds, ",")
		}

		if nodeGroupDto.PolicyIds != "" {
			nodeGroupDto.PolicyIdList = strings.Split(nodeGroupDto.PolicyIds, ",")
		}

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

		if nodeGroupDto.NodeIds != "" {
			nodeGroupDto.NodeIdList = strings.Split(nodeGroupDto.NodeIds, ",")
		}

		if nodeGroupDto.PolicyIds != "" {
			nodeGroupDto.PolicyIdList = strings.Split(nodeGroupDto.PolicyIds, ",")
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
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, nil)
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

func (s *Server) queryGroups() gin.HandlerFunc {
	return func(c *gin.Context) {
		var params dto.GroupParams
		if err := c.ShouldBindQuery(&params); err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		nodeGroups, err := s.groupController.QueryGroups(c, &params)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		WriteOK(c.JSON, nodeGroups)
	}
}
