package http

import (
	"github.com/gin-gonic/gin"
	"linkany/management/dto"
)

func (s *Server) RegisterUserRoutes() {
	userGroup := s.RouterGroup.Group(PREFIX + "/user")
	userGroup.POST("/register", s.register())
	userGroup.POST("/login", s.login())
	userGroup.GET("/list", s.authCheck(), s.getUsers())

	// user invite
	userGroup.POST("/invite", s.authCheck(), s.invite())
	userGroup.PUT("/invite/update", s.authCheck(), s.updateInvitation())
	userGroup.GET("/invite", s.authCheck(), s.getInvitation())
	userGroup.GET("/invitations", s.authCheck(), s.listInvitations())
}

// user invite
func (s *Server) invite() gin.HandlerFunc {
	return func(c *gin.Context) {

		var req dto.InviteDto

		// 解析JSON请求体
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid email address or missing field"})
			return
		}

		if err := s.userController.Invite(&req); err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, nil)
	}
}

// get invitation
func (s *Server) getInvitation() gin.HandlerFunc {
	return func(c *gin.Context) {
		userId := c.Query("userId")
		email := c.Query("email")
		invitation, err := s.userController.GetInvitation(userId, email)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, invitation)
	}
}

// update invitation
func (s *Server) updateInvitation() gin.HandlerFunc {
	return func(c *gin.Context) {
		var dto dto.InviteDto
		if err := c.ShouldBindJSON(&dto); err != nil {
			c.JSON(400, gin.H{"error": "Invalid email address or missing field"})
			return
		}
		if err := s.userController.UpdateInvitation(&dto); err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, nil)
	}
}

// list invitations
func (s *Server) listInvitations() gin.HandlerFunc {
	return func(c *gin.Context) {
		invitations, err := s.userController.ListInvitations()
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, invitations)
	}
}
