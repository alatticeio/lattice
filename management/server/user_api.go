package server

import (
	"net/http"
	"wireflow/management/dex"
	"wireflow/management/dto"

	"github.com/gin-gonic/gin"
)

func (s *Server) userApi() {
	r := s.Engine

	userApi := r.Group("/api/v1/users")
	userApi.Use(dex.AuthMiddleware())
	{
		userApi.POST("/register", s.RegisterUser) //注册用户
		userApi.POST("/login", s.login)           //注册用户
		userApi.GET("/getme", s.getMe())
	}
}

// 用户注册
func (s *Server) RegisterUser(c *gin.Context) {
	var req dto.UserDto
	if err := c.ShouldBindJSON(&req); err != nil {
		WriteBadRequest(c.JSON, err.Error())
		return
	}

	ctx := c.Request.Context()

	err := s.userController.Register(ctx, req)

	if err != nil {
		WriteError(c.JSON, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "注册成功"})
}

func (s *Server) login(c *gin.Context) {
	var req dto.UserDto
	if err := c.ShouldBindJSON(&req); err != nil {
		WriteBadRequest(c.JSON, err.Error())
		return
	}

	token, err := s.userController.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		WriteError(c.JSON, err.Error())
		return
	}

	// 返回给前端
	c.JSON(200, gin.H{
		"message": "登录成功",
		"token":   token,
	})
}

func (s *Server) getMe() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetString("userID")
		if id == "" {
			WriteBadRequest(c.JSON, `"ext_id" is empty`)
			return
		}

		user, err := s.userController.GetMe(c.Request.Context(), id)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		c.JSON(200, user)
	}
}
