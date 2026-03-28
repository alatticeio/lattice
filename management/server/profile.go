package server

import (
	"net/http"
	"wireflow/internal/infra"
	"wireflow/management/dto"
	"wireflow/management/server/middleware"
	"wireflow/pkg/utils/resp"

	"github.com/gin-gonic/gin"
)

func (s *Server) profileRouter() {
	profileApi := s.Group("/api/v1/profile")
	//userApi.Use(dex.AuthMiddleware())
	{
		profileApi.POST("/getProfile", middleware.AuthMiddleware(), s.getProfile())
		profileApi.PUT("/updateProfile", middleware.AuthMiddleware(), s.updateProfile())
	}
}

func (s *Server) getProfile() gin.HandlerFunc {
	return func(c *gin.Context) {
		userId := c.Request.Context().Value(infra.UserIDKey).(string)
		response, err := s.profileController.GetProfile(c.Request.Context(), userId)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}

		c.JSON(http.StatusOK, response)
	}
}

func (s *Server) updateProfile() gin.HandlerFunc {
	return func(c *gin.Context) {
		userId := c.Request.Context().Value(infra.UserIDKey).(string)
		var req dto.UpdateSettingsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}

		err := s.profileController.UpdateProfile(c.Request.Context(), userId, req)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}

		resp.OK(c, nil)
	}
}
