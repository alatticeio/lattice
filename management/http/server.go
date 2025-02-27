package http

import (
	"github.com/gin-gonic/gin"
	"linkany/management/client"
	"linkany/management/controller"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/management/service"
	"linkany/management/utils"
	"linkany/pkg/redis"
)

const (
	PREFIX = "/api/v1/"
)

// Server is the main server struct
type Server struct {
	*gin.Engine
	listen            string
	tokener           *utils.Tokener
	userController    *controller.UserController
	nodeController    *controller.NodeController
	planController    *controller.PlanController
	supportController *controller.SupportController
	accessController  *controller.AccessController
}

// ServerConfig is the server configuration
type ServerConfig struct {
	Listen          string                 `mapstructure: "listen,omitempty"`
	Database        service.DatabaseConfig `mapstructure: "database,omitempty"`
	DatabaseService *service.DatabaseService
	Rdb             *redis.Client
}

// NewServer creates a new server
func NewServer(cfg *ServerConfig) *Server {
	e := gin.Default()
	s := &Server{
		Engine:            e,
		listen:            cfg.Listen,
		userController:    controller.NewUserController(service.NewUserService(cfg.DatabaseService, cfg.Rdb)),
		nodeController:    controller.NewPeerController(service.NewNodeService(cfg.DatabaseService)),
		planController:    controller.NewPlanController(service.NewPlanService(cfg.DatabaseService)),
		supportController: controller.NewSupportController(service.NewSupportMapper(cfg.DatabaseService)),
		tokener:           utils.NewTokener(),
	}
	s.initRoute()

	return s
}

// authCheck checks if the user is authenticated
func (s *Server) initRoute() {

	// register user router
	s.RegisterUserRoutes()
	s.RegisterNodeRoutes()
	s.RegisterAccessRoutes()

	s.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

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
