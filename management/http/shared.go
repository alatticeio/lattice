package http

import (
	"linkany/management/client"
	"linkany/management/dto"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func (s *Server) RegisterSharedRoutes() {
	userGroup := s.RouterGroup.Group(PREFIX + "/shared")
	userGroup.DELETE("/invite/:inviteId/label/:labelId", s.tokenFilter(), s.deleteSharedLabel())
	userGroup.DELETE("/invite/:inviteId/group/:groupId", s.tokenFilter(), s.deleteSharedGroup())
	userGroup.DELETE("/invite/:inviteId/node/:nodeId", s.tokenFilter(), s.deleteSharedNode())
	userGroup.DELETE("/invite/:inviteId/policy/:policyId", s.tokenFilter(), s.deleteSharedPolicy())

	// add node to group
	userGroup.POST("/invite/:inviteId/group/:groupId/node/:nodeId", s.tokenFilter(), s.authFilter(), s.addNodeToGroup())
	userGroup.POST("/invite/:inviteId/group/:groupId/policy/:policyId", s.tokenFilter(), s.authFilter(), s.addPolicyToGroup())

	// list
	userGroup.GET("/group/list", s.tokenFilter(), s.listSharedGroups())
	userGroup.GET("/node/list", s.tokenFilter(), s.listSharedNodes())
	userGroup.GET("/policy/list", s.tokenFilter(), s.listSharedPolicies())
	userGroup.GET("/label/list", s.tokenFilter(), s.listSharedLabels())

	// update
	userGroup.PUT("/node/:nodeId", s.tokenFilter(), s.authFilter(), s.updateNode())
	// userGroup.POST("/invite/:inviteId/group/:groupId/label/:labelId", s.addLabelToGroup())
	// userGroup.POST("/invite/:inviteId/group/:groupId", s.addGroup())
	// userGroup.POST("/invite/:inviteId/label/:labelId", s.addLabel())
	// userGroup.POST("/invite/:inviteId/node/:nodeId", s.addNode())
	// userGroup.POST("/invite/:inviteId/policy/:policyId", s.addPolicy())
	// userGroup.POST("/invite/:inviteId", s.addInvite())
	// userGroup.GET("/invite/:inviteId", s.getInvite())
	// userGroup.GET("/invite/:inviteId/label", s.getSharedLabels())
	// userGroup.GET("/invite/:inviteId/group", s.getSharedGroups())

	// // update group node
	// userGroup.PUT("/invite/:inviteId/group/:groupId/node/:nodeId", s.updateGroupNode())
	// userGroup.PUT("/invite/:inviteId/group/:groupId", s.updateGroup())
	// userGroup.PUT("/invite/:inviteId/label/:labelId", s.updateLabel())
	// userGroup.PUT("/invite/:inviteId/node/:nodeId", s.updateNode())
	// userGroup.PUT("/invite/:inviteId/policy/:policyId", s.updatePolicy())
	// userGroup.PUT("/invite/:inviteId", s.updateInvite())
	// userGroup.GET("/invite/:inviteId/node", s.getSharedNodes())
	// userGroup.GET("/invite/:inviteId/policy", s.getSharedPolicies())
	// userGroup.GET("/invite/:inviteId/group/:groupId/node", s.getGroupNodes())

}

func (s *Server) deleteSharedLabel() gin.HandlerFunc {
	return func(c *gin.Context) {
		ivd := c.Param("inviteId")
		inviteId, err := strconv.Atoi(ivd)
		if err != nil {
			WriteError(c.JSON, "invalid invite id")
			return
		}
		lid := c.Param("labelId")
		labelId, err := strconv.Atoi(lid)
		if err != nil {
			WriteError(c.JSON, "invalid label id")
			return
		}
		err = s.sharedController.DeleteSharedLabel(c, uint(inviteId), uint(labelId))
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, nil)
	}
}

func (s *Server) deleteSharedGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		ivd := c.Param("inviteId")
		inviteId, err := strconv.Atoi(ivd)
		if err != nil {
			WriteError(c.JSON, "invalid invite id")
			return
		}
		gid := c.Param("groupId")
		groupId, err := strconv.Atoi(gid)
		if err != nil {
			WriteError(c.JSON, "invalid group id")
			return
		}
		err = s.sharedController.DeleteSharedGroup(c, uint(inviteId), uint(groupId))
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, nil)
	}
}

func (s *Server) deleteSharedNode() gin.HandlerFunc {
	return func(c *gin.Context) {
		ivd := c.Param("inviteId")
		inviteId, err := strconv.Atoi(ivd)
		if err != nil {
			WriteError(c.JSON, "invalid invite id")
			return
		}
		nid := c.Param("nodeId")
		nodeId, err := strconv.Atoi(nid)
		if err != nil {
			WriteError(c.JSON, "invalid node id")
			return
		}
		err = s.sharedController.DeleteSharedNode(c, uint(inviteId), uint(nodeId))
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, nil)
	}
}

func (s *Server) deleteSharedPolicy() gin.HandlerFunc {
	return func(c *gin.Context) {
		ivd := c.Param("inviteId")
		inviteId, err := strconv.Atoi(ivd)
		if err != nil {
			WriteError(c.JSON, "invalid invite id")
			return
		}
		pid := c.Param("policyId")
		policyId, err := strconv.Atoi(pid)
		if err != nil {
			WriteError(c.JSON, "invalid policy id")
			return
		}
		err = s.sharedController.DeleteSharedPolicy(c, uint(inviteId), uint(policyId))
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, nil)
	}
}

func (s *Server) addNodeToGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		var nodeGroupDto dto.NodeGroupDto
		if err := c.ShouldBind(&nodeGroupDto); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}

		if nodeGroupDto.NodeIds != "" {
			nodeGroupDto.NodeIdList = strings.Split(nodeGroupDto.NodeIds, ",")
		}

		// if nodeGroupDto.PolicyIds != "" {
		// 	nodeGroupDto.PolicyIdList = strings.Split(nodeGroupDto.PolicyIds, ",")
		// }

		err := s.sharedController.AddNodeToGroup(c, &nodeGroupDto)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		c.JSON(client.Success(nil))
	}
}

func (s *Server) removeNodeFromGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		var nodeGroupDto dto.NodeGroupDto
		if err := c.ShouldBind(&nodeGroupDto); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}

		if nodeGroupDto.NodeIds != "" {
			nodeGroupDto.NodeIdList = strings.Split(nodeGroupDto.NodeIds, ",")
		}

		err := s.groupController.UpdateGroup(c, &nodeGroupDto)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		c.JSON(client.Success(nil))
	}
}

func (s *Server) addPolicyToGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		var nodeGroupDto dto.NodeGroupDto
		if err := c.ShouldBind(&nodeGroupDto); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}

		// if nodeGroupDto.NodeIds != "" {
		// 	nodeGroupDto.NodeIdList = strings.Split(nodeGroupDto.NodeIds, ",")
		// }

		if nodeGroupDto.PolicyIds != "" {
			nodeGroupDto.PolicyIdList = strings.Split(nodeGroupDto.PolicyIds, ",")
		}

		err := s.sharedController.AddPolicyToGroup(c, &nodeGroupDto)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		c.JSON(client.Success(nil))
	}
}

// list
func (s *Server) listSharedGroups() gin.HandlerFunc {
	return func(c *gin.Context) {
		var params dto.SharedGroupParams
		if err := c.ShouldBindQuery(&params); err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		pageVo, err := s.sharedController.ListGroups(c, &params)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, pageVo)
	}
}

func (s *Server) listSharedNodes() gin.HandlerFunc {
	return func(c *gin.Context) {
		var params dto.SharedNodeParams

		if err := c.ShouldBindQuery(&params); err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		pageVo, err := s.sharedController.ListNodes(c, &params)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, pageVo)
	}
}

func (s *Server) listSharedLabels() gin.HandlerFunc {
	return func(c *gin.Context) {
		var params dto.SharedLabelParams

		if err := c.ShouldBindQuery(&params); err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		pageVo, err := s.sharedController.ListLabels(c, &params)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, pageVo)
	}
}

func (s *Server) listSharedPolicies() gin.HandlerFunc {
	return func(c *gin.Context) {
		var params dto.SharedPolicyParams

		if err := c.ShouldBindQuery(&params); err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		pageVo, err := s.sharedController.ListPolicies(c, &params)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, pageVo)
	}
}
