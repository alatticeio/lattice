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
	nodeGroup.POST("/a", s.authCheck(), s.createAppId())
	nodeGroup.POST("/", s.authCheck(), s.createNode())
	nodeGroup.PUT("/", s.authCheck(), s.updateNode())
	nodeGroup.DELETE("/:appId", s.authCheck(), s.deleteNode())
	nodeGroup.GET("/list", s.authCheck(), s.listNodes())

	// group member
	nodeGroup.POST("/group/member", s.authCheck(), s.addGroupMember())
	nodeGroup.DELETE("/group/member/:id", s.authCheck(), s.removeGroupMember())
	nodeGroup.PUT("/group/member/:id", s.authCheck(), s.UpdateGroupMember())
	nodeGroup.GET("/group/member/list", s.authCheck(), s.listGroupMembers())

	// Label
	nodeGroup.POST("/label", s.authCheck(), s.createLabel())
	nodeGroup.PUT("/label", s.authCheck(), s.updateLabel())
	nodeGroup.DELETE("/label", s.authCheck(), s.deleteLabel())
	nodeGroup.GET("/label/list", s.authCheck(), s.listLabel())

	// group node
	nodeGroup.POST("/group/node", s.authCheck(), s.addGroupNode())
	nodeGroup.DELETE("/group/node/:id", s.authCheck(), s.removeGroupNode())
	nodeGroup.GET("/group/node/:id", s.authCheck(), s.getGroupNode())
	nodeGroup.GET("/group/node/list", s.authCheck(), s.listGroupNodes())

	// node label
	nodeGroup.POST("/label/node", s.authCheck(), s.addNodeLabel())
	nodeGroup.DELETE("/label/node", s.authCheck(), s.removeNodeLabel())
	nodeGroup.GET("/label/node/list", s.authCheck(), s.listNodeLabels())
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
		var peerDto dto.NodeDto
		if err := c.ShouldBind(&peerDto); err != nil {
			WriteBadRequest(c.JSON, err.Error())
			return
		}

		peer, err := s.nodeController.Registry(&peerDto)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		c.JSON(client.Success(peer))
	}
}

func (s *Server) createAppId() gin.HandlerFunc {
	return func(c *gin.Context) {
		peer, err := s.nodeController.CreateAppId(c)
		if err != nil {
			WriteError(c.JSON, err.Error())
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

		nodes, err := s.nodeController.ListNodes(params)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, nodes)
	}
}

func (s *Server) updateNode() gin.HandlerFunc {
	return func(c *gin.Context) {
		var peerDto dto.NodeDto
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
		appId := c.Param("appId")

		err := s.nodeController.Delete(c, appId)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}
		c.JSON(client.Success(nil))
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
		id := c.Query("id")
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
func (s *Server) createLabel() gin.HandlerFunc {
	return func(c *gin.Context) {
		var tagDto dto.TagDto
		if err := c.ShouldBindJSON(&tagDto); err != nil {
			WriteBadRequest(c.JSON, err.Error())
			return
		}
		token := c.GetHeader("Authorization")
		user, err := s.userController.Get(token)
		tagDto.OwnerId = uint64(user.ID)
		tagDto.CreatedBy = user.Username

		tag, err := s.nodeController.CreateLabel(c, &tagDto)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, tag)
	}
}

func (s *Server) updateLabel() gin.HandlerFunc {
	return func(c *gin.Context) {
		var tagDto dto.TagDto
		if err := c.ShouldBindJSON(&tagDto); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}
		token := c.GetHeader("Authorization")
		user, err := s.userController.Get(token)
		tagDto.UpdatedBy = user.Username

		err = s.nodeController.UpdateLabel(c, &tagDto)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		WriteOK(c.JSON, nil)
	}
}

func (s *Server) deleteLabel() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Query("id")

		err := s.nodeController.DeleteLabel(c, id)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, nil)
	}
}

func (s *Server) listLabel() gin.HandlerFunc {
	return func(c *gin.Context) {
		var params dto.LabelParams
		if err := c.ShouldBindJSON(&params); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}

		vo, err := s.nodeController.ListLabel(c, &params)
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
		id := c.Query("id")
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

func (s *Server) addNodeLabel() gin.HandlerFunc {
	return func(c *gin.Context) {
		var nodeLabel dto.NodeLabelDto
		if err := c.ShouldBindJSON(&nodeLabel); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}
		token := c.GetHeader("Authorization")
		user, err := s.userController.Get(token)
		nodeLabel.CreatedBy = user.Username
		err = s.nodeController.AddNodeLabel(c, &nodeLabel)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, nil)
	}
}

func (s *Server) removeNodeLabel() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Query("id")
		err := s.nodeController.RemoveNodeLabel(c, id)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, nil)
	}
}

func (s *Server) listNodeLabels() gin.HandlerFunc {
	return func(c *gin.Context) {
		var params dto.NodeLabelParams
		if err := c.ShouldBindJSON(&params); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}
		nodeLabels, err := s.nodeController.ListNodeLabels(c, &params)
		if err != nil {
			WriteBadRequest(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, nodeLabels)
	}
}
