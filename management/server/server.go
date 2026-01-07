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
	"fmt"
	"strings"
	"wireflow/internal/config"
	"wireflow/internal/core/infra"
	"wireflow/internal/log"
	"wireflow/management/controller"
	"wireflow/management/nats"
	"wireflow/management/resource"

	"gorm.io/gorm"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	PREFIX = "/api/v1/"
)

// Server is the main server struct
type Server struct {
	logger *log.Logger
	listen string
	nats   infra.SignalService

	manager           manager.Manager
	peerController    controller.PeerController
	networkController controller.NetworkController
}

// ServerConfig is the server configuration
type ServerConfig struct {
	Listen          string
	DatabaseService *gorm.DB
	Nats            infra.SignalService
}

// NewServer creates a new server
func NewServer(cfg *ServerConfig) (*Server, error) {
	logger := log.GetLogger("management")
	if config.GlobalConfig.SignalUrl == "" {
		config.GlobalConfig.SignalUrl = fmt.Sprintf("nats://%s:%d", infra.SignalingDomain, infra.DefaultSignalingPort)
		config.WriteConfig("signal-url", config.GlobalConfig.SignalUrl)
	}

	signal, err := nats.NewNatsService(config.GlobalConfig.SignalUrl)
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

	s := &Server{
		logger:            logger,
		listen:            cfg.Listen,
		nats:              signal,
		manager:           mgr,
		peerController:    controller.NewPeerController(client),
		networkController: controller.NewNetworkController(client),
	}

	s.nats.Service("wireflow.signals.register.*", "wireflow_queue", s.Service)
	return s, nil
}

func (s *Server) Start(ctx context.Context) error {
	return s.manager.Start(ctx)
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
