package http

import (
	"gorm.io/gorm"
	"linkany/internal"
	"linkany/management/client"
	"linkany/management/controller"
	"linkany/management/db"
	"linkany/management/dto"
	"linkany/management/entity"
	"linkany/pkg/log"
	"linkany/pkg/redis"

	"github.com/gin-gonic/gin"
)

const (
	PREFIX = "/api/v1/"
)

// Server is the main server struct
type Server struct {
	*gin.Engine
	logger            *log.Logger
	listen            string
	tokenController   *controller.TokenController
	userController    *controller.UserController
	nodeController    *controller.NodeController
	planController    *controller.PlanController
	supportController *controller.SupportController
	accessController  *controller.AccessController
	groupController   *controller.GroupController
	sharedController  *controller.SharedController

	settingsController *controller.SettingsController

	manager *internal.WatchManager
}

// ServerConfig is the server configuration
type ServerConfig struct {
	Listen          string            `mapstructure: "listen,omitempty"`
	Database        db.DatabaseConfig `mapstructure: "database,omitempty"`
	DatabaseService *gorm.DB
	Rdb             *redis.Client
}

// NewServer creates a new server
func NewServer(cfg *ServerConfig) *Server {
	e := gin.Default()
	s := &Server{
		logger:         log.NewLogger(log.Loglevel, "mgt-server"),
		Engine:         e,
		listen:         cfg.Listen,
		userController: controller.NewUserController(cfg.DatabaseService, cfg.Rdb),
		nodeController: controller.NewPeerController(cfg.DatabaseService),
		//planController:     controller.NewPlanController(service.NewPlanService(cfg.DatabaseService)),
		//supportController:  controller.NewSupportController(service.NewSupportMapper(cfg.DatabaseService)),
		accessController:   controller.NewAccessController(cfg.DatabaseService),
		groupController:    controller.NewGroupController(cfg.DatabaseService),
		sharedController:   controller.NewSharedController(cfg.DatabaseService),
		settingsController: controller.NewSettingsController(cfg.DatabaseService),
		tokenController:    controller.NewTokenController(cfg.DatabaseService),
		manager:            internal.NewWatchManager(),
	}
	s.initRoute()

	return s
}

// tokenFilter checks if the user is authenticated
func (s *Server) initRoute() {

	// register user router
	s.RegisterUserRoutes()
	s.RegisterNodeRoutes()
	s.RegisterAccessRoutes()
	s.RegisterGroupRoutes()
	s.RegisterSharedRoutes()
	s.RegisterSettingsRoutes()

	s.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	s.GET("/api/v1/plans", s.tokenFilter(), s.listPlans())

	//support
	s.GET("/api/v1/supports", s.tokenFilter(), s.listSupports())
	s.POST("/api/v1/support", s.tokenFilter(), s.createSupport())
	s.GET("/api/v1/support/:id", s.tokenFilter(), s.getSupport())
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

		user, err := s.userController.Register(c, &u)
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

		token, err = s.userController.Login(c, &dto)
		if err != nil {
			WriteError(c.JSON, err.Error())
			return
		}

		WriteOK(c.JSON, token)
	}
}

func (s *Server) getUsers() gin.HandlerFunc {
	return func(context *gin.Context) {

	}
}

func (s *Server) listPlans() gin.HandlerFunc {
	return func(c *gin.Context) {
		//plans, err := s.planController.GetNetMap()
		//if err != nil {
		//	c.JSON(client.InternalServerError(err))
		//	return
		//}
		//
		//c.JSON(client.Success(plans))
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
