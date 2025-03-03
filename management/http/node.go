package http

import (
	"github.com/gin-gonic/gin"
	"linkany/management/client"
	"linkany/management/dto"
	"strconv"
)

func (s *Server) RegisterNodeRoutes() {
	nodeGroup := s.RouterGroup.Group(PREFIX + "/node")
	nodeGroup.GET("/appId/:appId", s.authCheck(), s.getNodeByAppId())
	nodeGroup.POST("/", s.authCheck(), s.createNode())
	nodeGroup.PUT("/", s.authCheck(), s.updateNode())
	nodeGroup.DELETE("/", s.authCheck(), s.deleteNode())
	nodeGroup.GET("/list", s.authCheck(), s.listNodes())

	// node group
	nodeGroup.POST("/group", s.authCheck(), s.createNodeGroup())
	nodeGroup.PUT("/group/:id", s.authCheck(), s.updateNodeGroup())
	nodeGroup.DELETE("/group/:id", s.authCheck(), s.deleteNodeGroup())
	nodeGroup.GET("/group/list", s.authCheck(), s.listNodeGroups())

	// group member
	nodeGroup.POST("/group/member", s.authCheck(), s.addGroupMember())
	nodeGroup.DELETE("/group/member/:memberID", s.authCheck(), s.removeGroupMember())
	nodeGroup.GET("/group/member/:memberID", s.authCheck(), s.getGroupMember())
	nodeGroup.GET("/group/member/list/:groupID", s.authCheck(), s.listGroupMembers())

	// Node Label
	nodeGroup.POST("/label", s.authCheck(), s.createNodeTag())
	nodeGroup.PUT("/label", s.authCheck(), s.updateNodeTag())
	nodeGroup.DELETE("/label", s.authCheck(), s.deleteNodeTag())
	nodeGroup.GET("/label/list", s.authCheck(), s.listNodeTags())

}

func (s *Server) getNodeByAppId() gin.HandlerFunc {
	return func(c *gin.Context) {
		appId := c.Param("appId")
		peer, err := s.nodeController.GetByAppId(appId)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		c.JSON(client.Success(peer))
	}
}

func (s *Server) createNode() gin.HandlerFunc {
	return func(c *gin.Context) {
		var peerDto dto.PeerDto
		if err := c.ShouldBindJSON(&peerDto); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}

		peer, err := s.nodeController.Registry(&peerDto)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		c.JSON(client.Success(peer))
	}
}

func (s *Server) listNodes() gin.HandlerFunc {
	return func(c *gin.Context) {
		params := &dto.QueryParams{}
		if err := c.ShouldBindQuery(params); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}

		peers, err := s.nodeController.List(params)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		c.JSON(client.Success(peers))
	}
}

func (s *Server) updateNode() gin.HandlerFunc {
	return func(c *gin.Context) {
		var peerDto dto.PeerDto
		if err := c.ShouldBindJSON(&peerDto); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}

		peer, err := s.nodeController.Update(&peerDto)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		c.JSON(client.Success(peer))
	}
}

func (s *Server) deleteNode() gin.HandlerFunc {
	return func(c *gin.Context) {
		var peerDto dto.PeerDto
		if err := c.ShouldBindJSON(&peerDto); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}

		err := s.nodeController.Delete(&peerDto)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		c.JSON(client.Success(nil))
	}
}

func (s *Server) createNodeGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		var nodeGroupDto dto.NodeGroupDto
		if err := c.ShouldBindJSON(&nodeGroupDto); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}

		nodeGroup, err := s.nodeController.CreateGroup(c, &nodeGroupDto)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		c.JSON(client.Success(nodeGroup))
	}
}

func (s *Server) updateNodeGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var nodeGroupDto dto.NodeGroupDto
		if err := c.ShouldBindJSON(&nodeGroupDto); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}
		idInt, _ := strconv.Atoi(id)
		nodeGroupDto.ID = uint(idInt)
		err := s.nodeController.UpdateGroup(c, &nodeGroupDto)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		c.JSON(client.Success(nil))
	}
}

func (s *Server) deleteNodeGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		err := s.nodeController.DeleteGroup(c, id)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		c.JSON(client.Success(nil))
	}
}

func (s *Server) listNodeGroups() gin.HandlerFunc {
	return func(c *gin.Context) {
		var params dto.GroupParams
		if err := c.ShouldBindQuery(params); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}
		nodeGroups, err := s.nodeController.ListGroups(c, &params)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		c.JSON(client.Success(nodeGroups))
	}
}

func (s *Server) addGroupMember() gin.HandlerFunc {
	return func(c *gin.Context) {
		var groupMember dto.GroupMemberDto
		if err := c.ShouldBindJSON(&groupMember); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}

		err := s.nodeController.AddGroupMember(&groupMember)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		c.JSON(client.Success(nil))
	}
}

func (s *Server) removeGroupMember() gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID := c.Param("memberID")
		err := s.nodeController.RemoveGroupMember(memberID)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		c.JSON(client.Success(nil))
	}
}

func (s *Server) listGroupMembers() gin.HandlerFunc {
	return func(c *gin.Context) {
		var params dto.GroupMemberParams
		if err := c.ShouldBindQuery(&params); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}
		members, err := s.nodeController.ListGroupMembers(c, &params)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		c.JSON(client.Success(members))
	}
}

func (s *Server) getGroupMember() gin.HandlerFunc {
	return func(c *gin.Context) {
		memberID := c.Param("memberID")
		member, err := s.nodeController.GetGroupMember(memberID)
		if err != nil {
			WriteBadRequest(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, member)
	}
}

// Node Label
func (s *Server) createNodeTag() gin.HandlerFunc {
	return func(c *gin.Context) {
		var tagDto dto.TagDto
		if err := c.ShouldBindJSON(&tagDto); err != nil {
			WriteBadRequest(c.JSON, err.Error())
			return
		}

		tagDto.Username = c.GetHeader("username")

		tag, err := s.nodeController.CreateTag(c, &tagDto)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, tag)
	}
}

func (s *Server) updateNodeTag() gin.HandlerFunc {
	return func(c *gin.Context) {
		var tagDto dto.TagDto
		if err := c.ShouldBindJSON(&tagDto); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}

		err := s.nodeController.UpdateTag(c, &tagDto)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		WriteOK(c.JSON, nil)
	}
}

func (s *Server) deleteNodeTag() gin.HandlerFunc {
	return func(c *gin.Context) {
		var tagDto dto.TagDto
		if err := c.ShouldBindJSON(&tagDto); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}

		err := s.nodeController.DeleteTag(c, uint64(tagDto.ID))
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, err.Error())
	}
}

func (s *Server) listNodeTags() gin.HandlerFunc {
	return func(c *gin.Context) {
		var params dto.LabelParams
		if err := c.ShouldBindJSON(&params); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}

		vo, err := s.nodeController.ListTags(c, &params)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		WriteOK(c.JSON, vo)
	}
}
