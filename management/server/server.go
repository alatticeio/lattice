package server

import (
	"context"
	"fmt"
	"strings"
	"wireflow/internal/config"
	"wireflow/internal/core/infra"
	"wireflow/internal/log"
	"wireflow/management/controller"
	"wireflow/management/nats"
	"wireflow/management/resource"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/gin-gonic/gin"
)

const (
	PREFIX = "/api/v1/"
)

// Server is the main server struct
type Server struct {
	ctx context.Context
	*gin.Engine
	logger *log.Logger
	listen string
	nats   infra.SignalService

	peerController    controller.PeerController
	networkController controller.NetworkController
}

// ServerConfig is the server configuration
type ServerConfig struct {
	Listen          string
	DatabaseService *gorm.DB
	Rdb             *redis.Client
	Nats            infra.SignalService
}

// NewServer creates a new server
func NewServer(cfg *ServerConfig) (*Server, error) {
	e := gin.Default()
	if config.GlobalConfig.SignalUrl == "" {
		config.GlobalConfig.SignalUrl = fmt.Sprintf("nats://%s:%d", infra.SignalingDomain, infra.DefaultSignalingPort)
		config.WriteConfig("signal-url", config.GlobalConfig.SignalUrl)
	}

	signal, err := nats.NewNatsService(config.GlobalConfig.SignalUrl)
	if err != nil {
		return nil, err
	}

	client, err := resource.NewClient()
	if err != nil {
		return nil, err
	}

	s := &Server{
		logger:            log.NewLogger(log.Loglevel, "ctrl-server"),
		Engine:            e,
		listen:            cfg.Listen,
		nats:              signal,
		peerController:    controller.NewPeerController(client),
		networkController: controller.NewNetworkController(client),
	}

	s.nats.Service("wireflow.signals.register.*", "wireflow_queue", s.Service)
	//启动informer
	s.initRoute()
	s.logger.Infof("listening on %s", cfg.Listen)
	return s, nil
}

// tokenFilter checks if the user is authenticated
func (s *Server) initRoute() {
	// register user router

	// register api
	s.RegisterApis()

	s.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
}

func (s *Server) Service(subject string, content []byte) ([]byte, error) {
	ctx := context.Background()
	action := getAction(subject)
	switch action {
	case "GetNetMap":
		return s.peerController.GetNetmap(context.Background(), content)
	case "register":
		return s.peerController.Register(ctx, content)
	case "create_network":
		return s.networkController.CreateNetwork(ctx, content)
	case "join_network", "add_network":
		if err := s.networkController.JoinNetwork(ctx, content); err != nil {
			return nil, err
		}

		return nil, nil
	case "leave_network", "rm_network":
		if err := s.networkController.LeaveNetwork(ctx, content); err != nil {
			return nil, err
		}

		return nil, nil
	}
	return nil, nil
}

func getAction(subject string) string {
	index := strings.LastIndex(subject, ".")
	return subject[index+1:]
}
