package server

import (
	"github.com/gin-gonic/gin"
	"linkany/control/client"
	"linkany/control/controller"
	"linkany/control/entity"
	"linkany/control/mapper"
	"linkany/control/utils"
)

type Server struct {
	*gin.Engine
	listen         string
	tokener        *utils.Tokener
	userController *controller.UserController
}

type ServerConfig struct {
	Listen         string                `mapstructure: "listen,omitempty"`
	Database       mapper.DatabaseConfig `mapstructure: "database,omitempty"`
	UserController mapper.UserInterface
}

func NewServer(cfg *ServerConfig) *Server {
	e := gin.Default()
	dbService := mapper.NewDatabaseService(&cfg.Database)
	s := &Server{
		Engine:         e,
		listen:         cfg.Listen,
		userController: controller.NewUserController(mapper.NewUserMapper(dbService)),
		tokener:        utils.NewTokener(),
	}
	s.initRoute()

	return s
}

func (s *Server) initRoute() {
	s.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	s.POST("/api/v1/user/login", s.login())
	s.GET("/api/v1/users", s.authCheck(), s.getUsers())
}

func (s *Server) Start() error {
	return s.Run(s.listen)
}

func (s *Server) login() gin.HandlerFunc {
	return func(c *gin.Context) {
		var u entity.User
		var err error
		var token *entity.Token
		if err = c.ShouldBind(&u); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}

		token, err = s.userController.Login(&u)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}

		c.JSON(client.Success(token))
	}
}

func (s *Server) getUsers() gin.HandlerFunc {
	return func(context *gin.Context) {

	}
}
