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

// Server is the main server struct
type Server struct {
	*gin.Engine
	listen            string
	tokener           *utils.Tokener
	userController    *controller.UserController
	peerControlloer   *controller.PeerController
	planController    *controller.PlanController
	supportController *controller.SupportController
	queue             chan *pb.WatchResponse
}

// ServerConfig is the server configuration
type ServerConfig struct {
	Listen          string                `mapstructure: "listen,omitempty"`
	Database        mapper.DatabaseConfig `mapstructure: "database,omitempty"`
	UserController  mapper.UserInterface
	Queue           chan *pb.WatchResponse
	DatabaseService *mapper.DatabaseService
}

// NewServer creates a new server
func NewServer(cfg *ServerConfig) *Server {
	e := gin.Default()
	s := &Server{
		Engine:            e,
		listen:            cfg.Listen,
		userController:    controller.NewUserController(mapper.NewUserMapper(cfg.DatabaseService)),
		peerControlloer:   controller.NewPeerController(mapper.NewPeerMapper(cfg.DatabaseService)),
		planController:    controller.NewPlanController(mapper.NewPlanMapper(cfg.DatabaseService)),
		supportController: controller.NewSupportController(mapper.NewSupportMapper(cfg.DatabaseService)),
		tokener:           utils.NewTokener(),
		queue:             cfg.Queue,
	}
	s.initRoute()

	return s
}

// authCheck checks if the user is authenticated
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

	s.GET("/api/v1/plans", s.authCheck(), s.listPlans())

	//support
	s.GET("/api/v1/supports", s.authCheck(), s.listSupports())
	s.POST("/api/v1/support", s.authCheck(), s.createSupport())
	s.GET("/api/v1/support/:id", s.authCheck(), s.getSupport())
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

func (s *Server) listPlans() gin.HandlerFunc {
	return func(c *gin.Context) {
		plans, err := s.planController.List()
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}

		c.JSON(client.Success(plans))
	}
}

func (s *Server) listSupports() gin.HandlerFunc {
	return func(c *gin.Context) {
		supports, err := s.supportController.List()
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}

		c.JSON(client.Success(supports))
	}
}

func (s *Server) createSupport() gin.HandlerFunc {
	return func(c *gin.Context) {
		var dto dto.SupportDto
		if err := c.ShouldBind(&dto); err != nil {
			c.JSON(client.BadRequest(err))
			return
		}

		support, err := s.supportController.Create(&dto)
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}

		c.JSON(client.Success(support))
	}
}

func (s *Server) getSupport() gin.HandlerFunc {
	return func(c *gin.Context) {
		support, err := s.supportController.Get()
		if err != nil {
			c.JSON(client.InternalServerError(err))
			return
		}

		c.JSON(client.Success(support))
	}
}
