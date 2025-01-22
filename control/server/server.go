package server

import (
	"github.com/gin-gonic/gin"
	"linkany/control/client"
	"linkany/control/controller"
	"linkany/control/dto"
	"linkany/control/entity"
	pb "linkany/control/grpc/peer"
	"linkany/control/mapper"
	"linkany/control/utils"
)

type Server struct {
	*gin.Engine
	listen          string
	tokener         *utils.Tokener
	userController  *controller.UserController
	peerControlloer *controller.PeerController
	queue           chan *pb.WatchResponse
}

type ServerConfig struct {
	Listen          string                `mapstructure: "listen,omitempty"`
	Database        mapper.DatabaseConfig `mapstructure: "database,omitempty"`
	UserController  mapper.UserInterface
	Queue           chan *pb.WatchResponse
	DatabaseService *mapper.DatabaseService
}

func NewServer(cfg *ServerConfig) *Server {
	e := gin.Default()
	s := &Server{
		Engine:         e,
		listen:         cfg.Listen,
		userController: controller.NewUserController(mapper.NewUserMapper(cfg.DatabaseService)),
		tokener:        utils.NewTokener(),
		queue:          cfg.Queue,
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

	s.POST("/api/v1/user/register", s.register())
	s.POST("/api/v1/user/login", s.login())
	s.GET("/api/v1/users", s.authCheck(), s.getUsers())
	s.GET("/api/v1/peer/:appId", s.authCheck(), s.getPeerByAppId())
}

func (s *Server) Start() error {
	return s.Run(s.listen)
}

func (s *Server) register() gin.HandlerFunc {
	return func(c *gin.Context) {
		var u dto.UserDto
		if err := c.ShouldBind(&u); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}

		user, err := s.userController.Register(&u)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}

		c.JSON(client.Success(user))
	}
}

func (s *Server) login() gin.HandlerFunc {
	return func(c *gin.Context) {
		var dto dto.UserDto
		var err error
		var token *entity.Token
		if err = c.ShouldBind(&dto); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}

		token, err = s.userController.Login(&dto)
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

func (s *Server) getPeerByAppId() gin.HandlerFunc {
	return func(c *gin.Context) {
		appId := c.Param("appId")
		peer, err := s.peerControlloer.GetByAppId(appId)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}

		c.JSON(client.Success(peer))
	}
}
