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
	nodeGroup.GET("/appId/:appId", s.tokenFilter(), s.getNodeByAppId())
	nodeGroup.POST("/a", s.tokenFilter(), s.createAppId())
	nodeGroup.POST("/", s.tokenFilter(), s.createNode())
	nodeGroup.PUT("/u", s.tokenFilter(), s.updateNode())
	nodeGroup.DELETE("/:appId", s.tokenFilter(), s.deleteNode())
	nodeGroup.GET("/list", s.tokenFilter(), s.listNodes())
	nodeGroup.GET("/q", s.tokenFilter(), s.queryNodes())

	// group member
	nodeGroup.POST("/group/member", s.tokenFilter(), s.addGroupMember())
	nodeGroup.DELETE("/group/member/:id", s.tokenFilter(), s.removeGroupMember())
	nodeGroup.PUT("/group/member/:id", s.tokenFilter(), s.UpdateGroupMember())
	nodeGroup.GET("/group/member/list", s.tokenFilter(), s.listGroupMembers())

	// Label
	nodeGroup.POST("/label", s.tokenFilter(), s.createLabel())
	nodeGroup.PUT("/label", s.tokenFilter(), s.updateLabel())
	nodeGroup.DELETE("/label", s.tokenFilter(), s.deleteLabel())
	nodeGroup.GET("/label/list", s.tokenFilter(), s.listLabel())
	nodeGroup.GET("/label", s.tokenFilter(), s.getLabel())
	nodeGroup.GET("/label/q", s.tokenFilter(), s.queryLabels())

	// group node
	nodeGroup.POST("/group/node", s.tokenFilter(), s.addGroupNode())
	nodeGroup.DELETE("/group/node/:id", s.tokenFilter(), s.removeGroupNode())
	nodeGroup.GET("/group/node/:id", s.tokenFilter(), s.getGroupNode())
	nodeGroup.GET("/group/node/list", s.tokenFilter(), s.listGroupNodes())
	nodeGroup.GET("/group/node/q", s.tokenFilter(), s.queryNodes())

	// node label
	nodeGroup.POST("/label/node", s.tokenFilter(), s.addNodeLabel())
	nodeGroup.DELETE("/label/node", s.tokenFilter(), s.removeNodeLabel())
	nodeGroup.GET("/label/node/list", s.tokenFilter(), s.listNodeLabels())

}

func (s *Server) getNodeByAppId() gin.HandlerFunc {
	return func(c *gin.Context) {
		appId := c.Param("appId")
		node, err := s.nodeController.GetByAppId(c, appId)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, node)
	}
}

func (s *Server) createNode() gin.HandlerFunc {
	return func(c *gin.Context) {
		var peerDto dto.NodeDto
		if err := c.ShouldBind(&peerDto); err != nil {
			WriteBadRequest(c.JSON, err.Error())
			return
		}

		node, err := s.nodeController.Registry(c, &peerDto)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, node)
	}
}

func (s *Server) createAppId() gin.HandlerFunc {
	return func(c *gin.Context) {
		node, err := s.nodeController.CreateAppId(c)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, node)
	}
}

func (s *Server) listNodes() gin.HandlerFunc {
	return func(c *gin.Context) {
		params := &dto.QueryParams{}
		if err := c.ShouldBindQuery(params); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}
		token := c.GetHeader("Authorization")
		user, err := s.userController.Get(c, token)
		params.UserId = user.ID

		nodes, err := s.nodeController.ListNodes(c, params)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, nodes)
	}
}

func (s *Server) queryNodes() gin.HandlerFunc {
	return func(c *gin.Context) {
		params := &dto.QueryParams{}
		if err := c.ShouldBindQuery(params); err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		nodes, err := s.nodeController.QueryNodes(c, params)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, nodes)
	}
}

func (s *Server) updateNode() gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			nodeDto dto.NodeDto
			err     error
		)
		if err = c.ShouldBind(&nodeDto); err != nil {
			WriteBadRequest(c.JSON, err.Error())
			return
		}

		err = s.nodeController.Update(c, &nodeDto)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, nil)
	}
}

func (s *Server) deleteNode() gin.HandlerFunc {
	return func(c *gin.Context) {
		appId := c.Param("appId")

		err := s.nodeController.Delete(c, appId)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, nil)
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
		user, err := s.userController.Get(c, token)
		groupMember.CreatedBy = user.Username
		err = s.nodeController.AddGroupMember(c, &groupMember)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, nil)
	}
}

func (s *Server) removeGroupMember() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Query("id")
		if id == "" {
			WriteError(c.JSON, "id is required")
			return
		}
		gid, err := strconv.ParseUint(id, 10, 64)
		if err != nil {
			WriteError(c.JSON, "invalid id")
			return
		}
		err = s.nodeController.RemoveGroupMember(c, gid)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, nil)
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
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, members)
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

		groupMember.ID, _ = strconv.ParseUint(id, 10, 64)
		err := s.nodeController.UpdateGroupMember(c, &groupMember)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, nil)
	}
}

// Node Label
func (s *Server) createLabel() gin.HandlerFunc {
	return func(c *gin.Context) {
		var tagDto dto.TagDto
		if err := c.ShouldBindJSON(&tagDto); err != nil {
			fmt.Println("err", err.Error())
			WriteBadRequest(c.JSON, err.Error())
			return
		}
		fmt.Println("label:", tagDto)
		token := c.GetHeader("Authorization")
		user, err := s.userController.Get(c, token)
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
		user, err := s.userController.Get(c, token)
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
		labelId, err := strconv.ParseUint(id, 10, 64)
		if err != nil {
			WriteError(c.JSON, "invalid id")
			return
		}
		err = s.nodeController.DeleteLabel(c, labelId)
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
		if err := c.ShouldBindQuery(&params); err != nil {
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

func (s *Server) getLabel() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		labelId, err := strconv.ParseUint(id, 10, 64)
		if err != nil {
			WriteError(c.JSON, "invalid id")
			return
		}
		label, err := s.nodeController.GetLabel(c, labelId)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		WriteOK(c.JSON, label)
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
		user, err := s.userController.Get(c, token)
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
		groupNodeId, err := strconv.ParseUint(id, 10, 64)
		if err != nil {
			WriteError(c.JSON, "invalid id")
			return
		}
		err = s.nodeController.RemoveGroupNode(c, groupNodeId)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, nil)
	}
}

func (s *Server) listGroupNodes() gin.HandlerFunc {
	return func(c *gin.Context) {
		var params dto.GroupNodeParams
		if err := c.ShouldBindQuery(&params); err != nil {
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
		groupNodeId, err := strconv.ParseUint(id, 10, 64)
		if err != nil {
			WriteError(c.JSON, "invalid id")
			return
		}
		member, err := s.nodeController.GetGroupNode(c, groupNodeId)
		if err != nil {
			WriteBadRequest(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, member)
	}
}

func (s *Server) addNodeLabel() gin.HandlerFunc {
	return func(c *gin.Context) {
		var nodeLabel dto.NodeLabelUpdateReq
		if err := c.ShouldBind(&nodeLabel); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}
		token := c.GetHeader("Authorization")
		user, err := s.userController.Get(c, token)

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
		nodeId := c.Query("nodeId")
		labelId := c.Query("labelId")

		nodeIdUint, err := strconv.ParseUint(nodeId, 10, 64)
		if err != nil {
			WriteError(c.JSON, "invalid node id")
			return
		}

		labelIdUint, err := strconv.ParseUint(labelId, 10, 64)
		if err != nil {
			WriteError(c.JSON, "invalid label id")
			return
		}

		err = s.nodeController.RemoveNodeLabel(c, nodeIdUint, labelIdUint)
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

func (s *Server) queryLabels() gin.HandlerFunc {
	return func(c *gin.Context) {
		var params dto.LabelParams
		if err := c.ShouldBindQuery(&params); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}
		nodeLabels, err := s.nodeController.QueryLabels(c, &params)
		if err != nil {
			WriteBadRequest(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, nodeLabels)
	}
}
