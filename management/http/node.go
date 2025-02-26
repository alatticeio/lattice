package http

import (
	"github.com/gin-gonic/gin"
	"linkany/management/client"
	"linkany/management/dto"
	"linkany/management/service"
)

func (s *Server) RegisterNodeRoutes() {
	nodeGroup := s.RouterGroup.Group(PREFIX + "/node")
	nodeGroup.GET("/:appId", s.authCheck(), s.getNodeByAppId())
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
		params := &service.QueryParams{}
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

		nodeGroup, err := s.nodeController.CreateGroup(&nodeGroupDto)
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

		err := s.nodeController.UpdateGroup(id, &nodeGroupDto)
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
		err := s.nodeController.DeleteGroup(id)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		c.JSON(client.Success(nil))
	}
}

func (s *Server) listNodeGroups() gin.HandlerFunc {
	return func(c *gin.Context) {
		nodeGroups, err := s.nodeController.ListGroups()
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		c.JSON(client.Success(nodeGroups))
	}
}

func (s *Server) addGroupMember() gin.HandlerFunc {
	return func(c *gin.Context) {
		var groupMember dto.GroupMember
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
		groupID := c.Param("groupID")
		members, err := s.nodeController.ListGroupMembers(groupID)
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
			c.JSON(client.InternalServerError(err))
			return
		}
		c.JSON(client.Success(member))
	}
}
