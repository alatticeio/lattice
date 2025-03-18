package http

import (
	"linkany/management/dto"
	"linkany/management/utils"
	"strconv"

	"github.com/gin-gonic/gin"
)

func (s *Server) RegisterUserRoutes() {
	userGroup := s.RouterGroup.Group(PREFIX + "/user")
	userGroup.POST("/register", s.register())
	userGroup.POST("/login", s.login())
	userGroup.GET("/list", s.authCheck(), s.getUsers())
	userGroup.GET("/info", s.authCheck(), s.getUserInfo())

	// user invite
	userGroup.POST("/invite/a", s.authCheck(), s.invite())
	userGroup.PUT("/invite/c/:id", s.authCheck(), s.cancelInvite())
	userGroup.DELETE("/invite/d/:id", s.authCheck(), s.deleteInvite())
	userGroup.PUT("/invite/u", s.authCheck(), s.updateInvitation())
	userGroup.GET("/invite/g", s.authCheck(), s.getInvitation())
	userGroup.GET("/invite/list", s.authCheck(), s.listInvites())

	// user invitation
	userGroup.GET("/invitation/list", s.authCheck(), s.listInvitations())
	userGroup.PUT("/invitation/u", s.authCheck(), s.updateInvitation())
	userGroup.PUT("/invitation/r/:inviteId", s.authCheck(), s.rejectInvitation())
	userGroup.PUT("/invitation/a/:inviteId", s.authCheck(), s.acceptInvitation())
}

func (s *Server) getUserInfo() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		user, err := s.userController.Get(token)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, user)
	}
}

// user invite
func (s *Server) invite() gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			req dto.InviteDto
			err error
		)

		// 解析JSON请求体
		if err := c.ShouldBindJSON(&req); err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		username := c.GetString("username")
		req.InviterName = username

		if req.GroupIds != "" {
			req.GroupIdList, err = utils.Splits(req.GroupIds, ",")
			if err != nil {
				s.logger.Errorf("%v", err)
				WriteError(c.JSON, err.Error())
				return
			}
		}

		if req.NodeIds != "" {
			req.NodeIdList, err = utils.Splits(req.NodeIds, ",")
			if err != nil {
				WriteError(c.JSON, err.Error())
				return
			}
		}

		if req.PolicyIds != "" {
			req.PolicyIdList, err = utils.Splits(req.PolicyIds, ",")
			if err != nil {
				WriteError(c.JSON, err.Error())
				return
			}
		}

		if req.LabelIds != "" {
			req.LabelIdList, err = utils.Splits(req.LabelIds, ",")
			if err != nil {
				WriteError(c.JSON, err.Error())
				return
			}

		}

		if req.PermissionIds != "" {
			req.PermissionIdList, err = utils.Splits(req.PermissionIds, ",")
			if err != nil {
				WriteError(c.JSON, err.Error())
				return
			}
		}

		if err := s.userController.Invite(&req); err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, nil)
	}
}

// cancel invite cancel
func (s *Server) cancelInvite() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if err := s.userController.CancelInvite(id); err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, nil)
	}
}

// delete invite cancel
func (s *Server) deleteInvite() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if err := s.userController.DeleteInvite(id); err != nil {
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
		var dto dto.InvitationDto
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

func (s *Server) rejectInvitation() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("inviteId")
		uid, err := strconv.Atoi(id)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		if err := s.userController.RejectInvitation(uint(uid)); err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, nil)
	}
}

func (s *Server) acceptInvitation() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("inviteId")
		uid, err := strconv.Atoi(id)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		if err := s.userController.AcceptInvitation(uint(uid)); err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, nil)
	}
}

// list invitations
func (s *Server) listInvitations() gin.HandlerFunc {
	return func(c *gin.Context) {
		var params dto.InvitationParams
		var err error
		if err = c.ShouldBindQuery(&params); err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		invitations, err := s.userController.ListUserInvitations(&params)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, invitations)
	}
}

func (s *Server) listInvites() gin.HandlerFunc {
	return func(c *gin.Context) {
		var params dto.InvitationParams
		var err error
		if err = c.ShouldBindQuery(&params); err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		invites, err := s.userController.ListUserInvites(&params)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, invites)
	}
}
