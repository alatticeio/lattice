package http

import (
	"github.com/gin-gonic/gin"
	"linkany/management/dto"
	"strconv"
)

func (s *Server) RegisterSettingsRoutes() {
	settingsGroup := s.RouterGroup.Group(PREFIX + "/settings")
	settingsGroup.POST("/key/a", s.tokenFilter(), s.newAppKey())
	settingsGroup.DELETE("/key/:id", s.tokenFilter(), s.deleteAppKey())
	settingsGroup.GET("/key/list", s.tokenFilter(), s.listAppKeys())

	settingsGroup.POST("/a", s.tokenFilter(), s.newUserSettings())
}

func (s *Server) newAppKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		err := s.settingsController.NewAppKey(c)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, nil)
	}
}

func (s *Server) newUserSettings() gin.HandlerFunc {
	return func(c *gin.Context) {
		var dto dto.UserSettingsDto
		err := c.ShouldBind(&dto)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		err = s.settingsController.NewUserSettings(c, &dto)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, nil)
	}
}

func (s *Server) deleteAppKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		var err error
		id := c.Param("id")
		keyId, err := strconv.Atoi(id)
		if err != nil {
			WriteError(c.JSON, "invalid key id")
			return
		}
		err = s.settingsController.RemoveAppKey(c, uint(keyId))
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, nil)
	}
}

func (s *Server) listAppKeys() gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			err    error
			params dto.AppKeyParams
		)

		if err = c.ShouldBindQuery(&params); err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		params.UserId = c.Value("userId").(uint)

		vo, err := s.settingsController.ListAppkeys(c, &params)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}
		WriteOK(c.JSON, vo)
	}
}
