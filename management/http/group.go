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
	nodeGroup.GET("/policy/list", s.tokenFilter(), s.listGroupPolicies())
	nodeGroup.DELETE("/:id/policy/:policyId", s.tokenFilter(), s.deleteGroupPolicy())
	nodeGroup.DELETE("/:id/node/:nodeId", s.tokenFilter(), s.deleteGroupNode())

	// node group
	nodeGroup.GET("/:id", s.tokenFilter(), s.GetNodeGroup())
	nodeGroup.POST("/a", s.tokenFilter(), s.createGroup())
	nodeGroup.PUT("/u", s.tokenFilter(), s.updateGroup())
	nodeGroup.DELETE("/:id", s.tokenFilter(), s.authFilter(), s.deleteGroup())
	nodeGroup.GET("/list", s.tokenFilter(), s.listGroups())
	nodeGroup.GET("/q", s.tokenFilter(), s.queryGroups())
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
		gid, err := strconv.ParseUint(groupId, 10, 64)
		if err != nil {
			WriteError(c.JSON, "invalid group id")
			return
		}
		policyId := c.Param("policyId")
		pid, err := strconv.ParseUint(policyId, 10, 64)
		if err != nil {
			WriteError(c.JSON, "invalid policy id")
			return
		}

		err = s.groupController.DeleteGroupPolicy(c, gid, pid)
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
		gid, err := strconv.ParseUint(groupId, 10, 64)
		if err != nil {
			WriteError(c.JSON, "invalid group id")
			return
		}
		nodeId := c.Param("nodeId")
		pid, err := strconv.ParseUint(nodeId, 10, 64)
		if err != nil {
			WriteError(c.JSON, "invalid nodeId id")
			return
		}

		err = s.groupController.DeleteGroupNode(c, gid, pid)
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
		var (
			nodeId uint64
			err    error
		)
		if nodeId, err = strconv.ParseUint(c.Param("id"), 10, 64); err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

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
		if err := c.ShouldBind(&nodeGroupDto); err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		token := c.GetHeader("Authorization")
		user, err := s.userController.Get(c, token)
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
		if err := c.ShouldBindJSON(&nodeGroupDto); err != nil {
			WriteBadRequest(c.JSON, err.Error())
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
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, nil)
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
