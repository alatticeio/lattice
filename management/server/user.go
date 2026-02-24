package server

import (
	"net/http"
	"wireflow/management/dto"
	"wireflow/management/server/middleware"
	"wireflow/pkg/utils"
	"wireflow/pkg/utils/resp"

	"github.com/gin-gonic/gin"
)

func (s *Server) userRouter() {

	userApi := s.Group("/api/v1/users")
	//userApi.Use(dex.AuthMiddleware())
	{
		userApi.POST("/register", s.RegisterUser) //注册用户
		userApi.POST("/login", s.login)           //注册用户
		userApi.GET("/getme", middleware.AuthMiddleware(), s.getMe())
		userApi.GET("/list", middleware.AuthMiddleware(), s.listUser())

		userApi.POST("/add", middleware.AuthMiddleware(), s.handleAddUser())
		userApi.DELETE("/:name", middleware.AuthMiddleware(), s.handleDeleteUser())
	}
}

// 用户注册
func (s *Server) RegisterUser(c *gin.Context) {
	var req dto.UserDto
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.BadRequest(c, err.Error())
		return
	}

	ctx := c.Request.Context()

	err := s.userController.Register(ctx, req)

	if err != nil {
		resp.Error(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "注册成功"})
}

func (s *Server) login(c *gin.Context) {
	var req dto.UserDto
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.BadRequest(c, err.Error())
		return
	}

	user, err := s.userController.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		resp.Error(c, err.Error())
		return
	}

	businessToken, err := utils.GenerateBusinessJWT(user.ID, user.Username)
	if err != nil {
		resp.Error(c, err.Error())
		return
	}

	// 返回给前端
	resp.OK(c, map[string]interface{}{
		"user":  user.Username,
		"token": businessToken,
	})
}

func (s *Server) getMe() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetString("user_id")
		if id == "" {
			resp.BadRequest(c, `"ext_id" is empty`)
			return
		}

		user, err := s.userController.GetMe(c.Request.Context(), id)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}

		resp.OK(c, user)
	}
}

func (s *Server) handleAddUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req dto.UserDto
		if err := c.ShouldBindJSON(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}

		err := s.userController.AddUser(c.Request.Context(), &req)

		if err != nil {
			resp.Error(c, err.Error())
			return
		}

		resp.OK(c, nil)
	}
}

func (s *Server) handleDeleteUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		if name == "" {
			resp.BadRequest(c, `"name" is empty`)
			return
		}

		err := s.userController.DeleteUser(c.Request.Context(), name)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}

		resp.OK(c, nil)
	}
}

// listUser list members
func (s *Server) listUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req dto.PageRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			resp.BadRequest(c, err.Error())
			return
		}

		res, err := s.userController.ListUser(c.Request.Context(), &req)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}

		resp.OK(c, res)
	}
}
