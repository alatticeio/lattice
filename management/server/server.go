// Copyright 2025 The Wireflow Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"context"
	"encoding/json"
	"wireflow/internal/config"
	"wireflow/internal/infra"
	"wireflow/internal/log"
	"wireflow/management/controller"
	"wireflow/management/database"
	"wireflow/management/nats"
	"wireflow/management/resource"
	"wireflow/pkg/version"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type Handler func(data []byte) ([]byte, error)

// Server is the main server struct.
type Server struct {
	*gin.Engine
	logger *log.Logger
	listen string
	nats   infra.SignalService

	manager           manager.Manager
	peerController    controller.PeerController
	networkController controller.NetworkController
	userController    controller.UserController
}

// ServerConfig is the server configuration.
type ServerConfig struct {
	Listen          string
	DatabaseService *gorm.DB
	Nats            infra.SignalService
}

// NewServer creates a new server.
func NewServer(cfg *ServerConfig) (*Server, error) {
	logger := log.GetLogger("management")

	signal, err := nats.NewNatsService(context.Background(), config.Conf.SignalingURL)
	if err != nil {
		logger.Error("init signal failed", err)
		return nil, err
	}

	mgr, err := resource.NewManager()
	if err != nil {
		logger.Error("init mgr failed", err)
		return nil, err
	}

	client, err := resource.NewClient(signal, mgr)
	if err != nil {
		logger.Error("init client failed", err)
		return nil, err
	}

	database.InitDB("wireflow.db")

	s := &Server{
		logger:            logger,
		listen:            cfg.Listen,
		nats:              signal,
		manager:           mgr,
		peerController:    controller.NewPeerController(client),
		networkController: controller.NewNetworkController(client),
		userController:    controller.NewUserController(),
	}

	routes := map[string]Handler{
		"wireflow.signals.peer.register":       s.Register,
		"wireflow.signals.peer.GetNetMap":      s.GetNetMap,
		"wireflow.signals.service.info":        s.Info,
		"wireflow.signals.service.createToken": s.CreateToken,
	}

	for route, handler := range routes {
		s.nats.Service(route, "wireflow_queue", handler)
	}

	// http
	s.Engine = gin.Default()

	s.apiRouter()

	return s, nil
}

func (s *Server) Start(ctx context.Context) error {
	return s.manager.Start(ctx)
}

func (s *Server) GetManager() manager.Manager {
	return s.manager
}

func (s *Server) Register(content []byte) ([]byte, error) {
	return s.peerController.Register(context.Background(), content)
}

func (s *Server) GetNetMap(content []byte) ([]byte, error) {
	return s.peerController.GetNetmap(context.Background(), content)
}

func (s *Server) Info(content []byte) ([]byte, error) {
	serverInfo := version.Get()
	data, err := json.Marshal(serverInfo)
	return data, err
}

func (s *Server) CreateToken(content []byte) ([]byte, error) {
	return s.peerController.CreateToken(context.Background(), content)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return nil
}
