package http

import (
	"fmt"
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
	nodeGroup.GET("/group/:id", s.authCheck(), s.GetNodeGroup())
	nodeGroup.POST("/group", s.authCheck(), s.createGroup())
	nodeGroup.PUT("/group/:id", s.authCheck(), s.updateGroup())
	nodeGroup.DELETE("/group/:id", s.authCheck(), s.deleteGroup())
	nodeGroup.GET("/group/list", s.authCheck(), s.listGroups())

	// group member
	nodeGroup.POST("/group/member", s.authCheck(), s.addGroupMember())
	nodeGroup.DELETE("/group/member/:id", s.authCheck(), s.removeGroupMember())
	nodeGroup.PUT("/group/member/:id", s.authCheck(), s.UpdateGroupMember())
	nodeGroup.GET("/group/member/list", s.authCheck(), s.listGroupMembers())

	// Node Label
	nodeGroup.POST("/label", s.authCheck(), s.createNodeTag())
	nodeGroup.PUT("/label", s.authCheck(), s.updateNodeTag())
	nodeGroup.DELETE("/label", s.authCheck(), s.deleteNodeTag())
	nodeGroup.GET("/label/list", s.authCheck(), s.listNodeTags())

	// group node
	nodeGroup.POST("/group/node", s.authCheck(), s.addGroupNode())
	nodeGroup.DELETE("/group/node/:id", s.authCheck(), s.removeGroupNode())
	nodeGroup.GET("/group/node/:id", s.authCheck(), s.getGroupNode())
	nodeGroup.GET("/group/node/list", s.authCheck(), s.listGroupNodes())
}

func (s *Server) getNodeByAppId() gin.HandlerFunc {
	return func(c *gin.Context) {
		appId := c.Param("appId")
		peer, _, err := s.nodeController.GetByAppId(appId, "")
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

func (s *Server) GetNodeGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		nodeId := c.Param("id")

		nodeGroup, err := s.nodeController.GetNodeGroup(c, nodeId)
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
		nodeGroup, err := s.nodeController.CreateGroup(c, &nodeGroupDto)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		c.JSON(client.Success(nodeGroup))
	}
}

func (s *Server) updateGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		groupId := c.Param("id")

		var nodeGroupDto dto.NodeGroupDto
		if err := c.ShouldBind(&nodeGroupDto); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}
		nodeGroupDto.ID = func(str string) uint {
			id, _ := strconv.ParseUint(groupId, 10, 64)
			return uint(id)
		}(groupId)
		err := s.nodeController.UpdateGroup(c, &nodeGroupDto)
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
		err := s.nodeController.DeleteGroup(c, id)
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

		nodeGroups, err := s.nodeController.ListGroups(c, &params)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		WriteOK(c.JSON, nodeGroups)
	}
}

func (s *Server) addGroupMember() gin.HandlerFunc {
	return func(c *gin.Context) {
		var groupMember dto.GroupMemberDto
		if err := c.ShouldBindJSON(&groupMember); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}
		token := c.GetHeader("Authorization")
		user, err := s.userController.Get(token)
		groupMember.CreatedBy = user.Username
		err = s.nodeController.AddGroupMember(c, &groupMember)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		c.JSON(client.Success(nil))
	}
}

func (s *Server) removeGroupMember() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		err := s.nodeController.RemoveGroupMember(c, id)
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
		if err := c.ShouldBindJSON(&params); err != nil {
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

func (s *Server) UpdateGroupMember() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var groupMember dto.GroupMemberDto
		if err := c.ShouldBindJSON(&groupMember); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}

		groupMember.ID, _ = strconv.ParseInt(id, 10, 64)
		err := s.nodeController.UpdateGroupMember(c, &groupMember)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		c.JSON(client.Success(nil))
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

func (s *Server) addGroupNode() gin.HandlerFunc {
	return func(c *gin.Context) {
		var groupNode dto.GroupNodeDto
		if err := c.ShouldBindJSON(&groupNode); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}
		token := c.GetHeader("Authorization")
		user, err := s.userController.Get(token)
		groupNode.CreatedBy = user.Username
		err = s.nodeController.AddGroupNode(c, &groupNode)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		c.JSON(client.Success(nil))
	}
}

func (s *Server) removeGroupNode() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		err := s.nodeController.RemoveGroupNode(c, id)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		c.JSON(client.Success(nil))
	}
}

func (s *Server) listGroupNodes() gin.HandlerFunc {
	return func(c *gin.Context) {
		var params dto.GroupNodeParams
		if err := c.ShouldBindJSON(&params); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}
		fmt.Println("params", params.GroupName)
		nodes, err := s.nodeController.ListGroupNodes(c, &params)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		c.JSON(client.Success(nodes))
	}
}

func (s *Server) getGroupNode() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		member, err := s.nodeController.GetGroupNode(c, id)
		if err != nil {
			WriteBadRequest(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, member)
	}
}
