package server

import (
	"github.com/gin-gonic/gin"
	"linkany/control/entity"
	"linkany/control/service"
)

type Server struct {
	*gin.Engine
	listen      string
	userService service.UserInterface
}

type ServerConfig struct {
	Listen      string
	UserService service.UserInterface
}

func NewServer(cfg *ServerConfig) *Server {
	e := gin.Default()
	return &Server{Engine: e, listen: cfg.Listen, userService: cfg.UserService}
}

func (s *Server) initRoute() {
	s.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	s.POST("/api/v1/login", s.login())
}

func (s *Server) Start() error {
	return s.Run(s.listen)
}

func (s *Server) login() gin.HandlerFunc {
	return func(c *gin.Context) {
		var u entity.User
		var err error
		var token entity.Token
		if err = c.ShouldBind(&u); err != nil {
			c.JSON(400, gin.H{
				"error": err.Error(),
			})
			return
		}

		token, err = s.userService.Login(&u)
		if err != nil {
			c.JSON(500, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(200, token)
	}
}
