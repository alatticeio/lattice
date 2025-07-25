package http

import (
	"github.com/gin-gonic/gin"
	"linkany/management/dto"
	"linkany/management/vo"
)

func (s *Server) RegisterApis() {
	s.RegisterGroupApis()
	s.RegisterNodeApis()
	s.RegisterPolicyApis()
}

func (s *Server) RegisterGroupApis() {
	groupApis := s.RouterGroup.Group(PREFIX + "/group")
	groupApis.POST("/join", s.tokenFilter(), s.joinGroup())
	groupApis.POST("/leave", s.tokenFilter(), s.leaveGroup())
	groupApis.POST("/remove", s.tokenFilter(), s.removeGroup())
	groupApis.POST("/add", s.tokenFilter(), s.addGroup())
}

func (s *Server) RegisterNodeApis() {
	nodeApis := s.RouterGroup.Group(PREFIX + "/node/command")
	nodeApis.POST("/list", s.tokenFilter(), s.listUserNodes())
	nodeApis.POST("/label/add", s.tokenFilter(), s.addLabel())
	nodeApis.POST("/label/show", s.tokenFilter(), s.showLabel())
	nodeApis.POST("/label/rm", s.tokenFilter(), s.removeLabel())
}

func (s *Server) RegisterPolicyApis() {
	nodeApis := s.RouterGroup.Group(PREFIX + "/policy/command")
	nodeApis.POST("/list", s.tokenFilter(), s.listUserPolicies())
}

func (s *Server) joinGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		var params dto.ApiCommandParams
		if err := c.ShouldBindJSON(&params); err != nil {
			WriteBadRequest(c.JSON, "invalid request: "+err.Error())
			return
		}

		if err := s.groupController.JoinGroup(c, &params); err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		WriteOK(c.JSON, "joined group successfully")
	}
}

func (s *Server) leaveGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		var params dto.ApiCommandParams
		if err := c.ShouldBindJSON(&params); err != nil {
			WriteBadRequest(c.JSON, "invalid request: "+err.Error())
			return
		}

		if err := s.groupController.LeaveGroup(c, &params); err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		WriteOK(c.JSON, "left group successfully")
	}
}

func (s *Server) removeGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		var params dto.ApiCommandParams
		if err := c.ShouldBindJSON(&params); err != nil {
			WriteBadRequest(c.JSON, "invalid request: "+err.Error())
			return
		}

		if err := s.groupController.RemoveGroup(c, &params); err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		WriteOK(c.JSON, "remove group successfully")
	}
}

func (s *Server) addGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
		var params dto.ApiCommandParams
		if err := c.ShouldBindJSON(&params); err != nil {
			WriteBadRequest(c.JSON, "invalid request: "+err.Error())
			return
		}

		if err := s.groupController.AddGroup(c, &params); err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		WriteOK(c.JSON, "add group successfully")
	}
}

// nodes apis
func (s *Server) listUserNodes() gin.HandlerFunc {
	return func(c *gin.Context) {
		var params dto.ApiCommandParams
		if err := c.ShouldBindJSON(&params); err != nil {
			WriteBadRequest(c.JSON, "invalid request: "+err.Error())
			return
		}

		nodes, err := s.nodeController.ListUserNodes(c, &params)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		WriteOK(c.JSON, nodes)
	}
}
func (s *Server) addLabel() gin.HandlerFunc {
	return func(c *gin.Context) {
		var params dto.ApiCommandParams
		if err := c.ShouldBindJSON(&params); err != nil {
			WriteBadRequest(c.JSON, "invalid request: "+err.Error())
			return
		}

		if err := s.nodeController.AddLabel(c, &params); err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		WriteOK(c.JSON, "add label successfully")
	}
}

func (s *Server) showLabel() gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			params dto.ApiCommandParams
			labels []vo.NodeLabelVo
			err    error
		)
		if err = c.ShouldBindJSON(&params); err != nil {
			WriteBadRequest(c.JSON, "invalid request: "+err.Error())
			return
		}

		if labels, err = s.nodeController.ShowLabel(c, &params); err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		WriteOK(c.JSON, labels)
	}
}

func (s *Server) removeLabel() gin.HandlerFunc {
	return func(c *gin.Context) {
		var params dto.ApiCommandParams
		if err := c.ShouldBindJSON(&params); err != nil {
			WriteBadRequest(c.JSON, "invalid request: "+err.Error())
			return
		}

		if err := s.nodeController.RemoveLabel(c, &params); err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		WriteOK(c.JSON, "remove label successfully")
	}
}

func (s *Server) listUserPolicies() gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			params dto.ApiCommandParams
			labels []vo.AccessPolicyVo
			err    error
		)
		if err = c.ShouldBindJSON(&params); err != nil {
			WriteBadRequest(c.JSON, "invalid request: "+err.Error())
			return
		}

		if labels, err = s.accessController.ListUserPolicies(c, &params); err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		WriteOK(c.JSON, labels)
	}
}
