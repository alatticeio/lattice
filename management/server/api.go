package server

import (
	"wireflow/management/dto"

	"github.com/gin-gonic/gin"
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
	nodeApis := s.RouterGroup.Group(PREFIX + "/node")
	nodeApis.POST("/register", s.tokenFilter(), s.register())

}

func (s *Server) RegisterPolicyApis() {
	nodeApis := s.RouterGroup.Group(PREFIX + "/policy/command")
	nodeApis.POST("/list", s.tokenFilter(), s.listUserPolicies())
}

func (s *Server) register() gin.HandlerFunc {
	return func(c *gin.Context) {
		var request dto.PeerDto
		if err := c.ShouldBindJSON(&request); err != nil {
			WriteBadRequest(c.JSON, err.Error())
			return
		}
	}
}

func (s *Server) joinGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
	}
}

func (s *Server) leaveGroup() gin.HandlerFunc {
	return func(c *gin.Context) {
	}
}

func (s *Server) removeGroup() gin.HandlerFunc {
	return func(c *gin.Context) {

	}
}

func (s *Server) addGroup() gin.HandlerFunc {
	return func(c *gin.Context) {

	}
}

// nodes apis
func (s *Server) listUserNodes() gin.HandlerFunc {
	return func(c *gin.Context) {

	}
}
func (s *Server) addLabel() gin.HandlerFunc {
	return func(c *gin.Context) {

	}
}

func (s *Server) showLabel() gin.HandlerFunc {
	return func(c *gin.Context) {
	}
}

func (s *Server) removeLabel() gin.HandlerFunc {
	return func(c *gin.Context) {

		WriteOK(c.JSON, "remove label successfully")
	}
}

func (s *Server) listUserPolicies() gin.HandlerFunc {
	return func(c *gin.Context) {
	}
}
