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
	"fmt"
	"time"
	"wireflow/internal/config"
	"wireflow/internal/db"
	"wireflow/internal/infra"
	"wireflow/internal/log"
	"wireflow/internal/store"
	"wireflow/management/controller"
	managementnats "wireflow/management/nats"
	"wireflow/management/resource"
	"wireflow/pkg/version"

	"github.com/gin-gonic/gin"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type Handler func(data []byte) ([]byte, error)

// Server is the main server struct.
type Server struct {
	*gin.Engine
	logger *log.Logger
	listen string
	nats   infra.SignalService
	cfg    *config.Config

	client            *resource.Client
	manager           manager.Manager
	cacheReady        chan struct{}
	peerController    controller.PeerController
	networkController controller.NetworkController
	userController    controller.UserController
	policyController  controller.PolicyController

	workspaceController controller.WorkspaceController
	tokenController     controller.TokenController

	monitorController controller.MonitorController
	profileController controller.ProfileController

	store    store.Store
	presence *managementnats.NodePresenceStore
}

// ServerConfig is the server configuration.
type ServerConfig struct {
	Cfg  *config.Config
	Nats infra.SignalService
}

// NewServer creates a new server.
func NewServer(ctx context.Context, serverConfig *ServerConfig) (*Server, error) {
	logger := log.GetLogger("management")
	cfg := serverConfig.Cfg

	// ── 弱依赖①：NATS 信令服务（可选）──────────────────────────────
	// 若 signaling-url 为空或连接失败，降级为 noop，主进程继续启动。
	var signal infra.SignalService
	if cfg.SignalingURL == "" {
		logger.Warn("signaling-url is empty, NATS signal service disabled")
		signal = managementnats.NewNoopSignalService()
	} else {
		svc, err := managementnats.NewNatsService(ctx, "wireflow-manager", "server", cfg.SignalingURL)
		if err != nil {
			logger.Warn("NATS init failed, falling back to noop signal service", "url", cfg.SignalingURL, "err", err)
			signal = managementnats.NewNoopSignalService()
		} else {
			signal = svc
		}
	}

	// ── 弱依赖②：K8s Manager（可选）────────────────────────────────
	// 非 K8s 环境（本地开发、CI）下跳过，不影响 HTTP Server 启动。
	var mgr manager.Manager
	var client *resource.Client
	k8sMgr, err := resource.NewManager()
	if err != nil {
		logger.Warn("K8s manager init failed, running without controller-runtime", "err", err)
	} else {
		mgr = k8sMgr
		k8sClient, cerr := resource.NewClient(signal, mgr)
		if cerr != nil {
			logger.Warn("K8s client init failed, running without K8s CRD support", "err", cerr)
		} else {
			client = k8sClient
		}
	}

	// 注册一个 Runnable：controller-runtime 在 cache 同步完成后才会启动 Runnable，
	// 关闭 cacheReady 通知外部 HTTP Server 可以安全上线。
	var cacheReady chan struct{}
	if mgr != nil {
		cacheReady = make(chan struct{})
		ch := cacheReady
		_ = mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
			close(ch)
			<-ctx.Done()
			return nil
		}))
	}

	// ── 强依赖：数据库（失败时返回错误，符合设计约束）───────────
	st, err := db.NewStore(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to init store: %w", err)
	}

	presence := managementnats.NewNodePresenceStore()

	s := &Server{
		Engine:              gin.Default(),
		logger:              logger,
		listen:              cfg.Listen,
		nats:                signal,
		manager:             mgr,
		cacheReady:          cacheReady,
		client:              client,
		cfg:                 cfg,
		presence:            presence,
		peerController:      controller.NewPeerController(client, st, presence),
		networkController:   controller.NewNetworkController(client, st),
		userController:      controller.NewUserController(st),
		policyController:    controller.NewPolicyController(client, st),
		workspaceController: controller.NewWorkspaceController(client, st),
		tokenController:     controller.NewTokenController(client, st),
		monitorController:   controller.NewMonitorController(cfg.Monitor.Address),
		profileController:   controller.NewProfileController(st),
		store:               st,
	}

	// initAdmins：DB 已就绪后执行；失败只告警，不阻断启动。
	if err = s.userController.InitAdmin(context.Background(), config.GlobalConfig.App.InitAdmins); err != nil {
		s.logger.Warn("init admin failed (non-fatal, will retry on next startup)", "err", err)
	} else {
		s.logger.Debug("Init admin success")
	}

	if err = s.apiRouter(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Server) Start(ctx context.Context) error {
	if s.manager == nil {
		// K8s manager 不可用，阻塞直到 ctx 取消，保持 goroutine 正常退出。
		<-ctx.Done()
		return nil
	}

	//注册nats service
	routes := map[string]Handler{
		"wireflow.signals.peer.register":       s.Register,
		"wireflow.signals.peer.GetNetMap":      s.GetNetMap,
		"wireflow.signals.peer.heartbeat":      s.Heartbeat,
		"wireflow.signals.service.info":        s.Info,
		"wireflow.signals.service.createToken": s.CreateToken,
	}

	for route, handler := range routes {
		s.nats.Service(route, "wireflow_queue", handler)
	}

	// 关键：确保订阅指令已经到达并被 NATS Server 处理
	if err := s.nats.Flush(); err != nil {
		s.logger.Error("NATS subscription sync failed", err)
	}

	return s.manager.Start(ctx)
}

func (s *Server) GetManager() manager.Manager {
	return s.manager
}

// CacheReady returns a channel that is closed once the controller-runtime cache
// has fully synced. Returns nil if no K8s manager is available.
func (s *Server) CacheReady() <-chan struct{} {
	return s.cacheReady
}

func (s *Server) Register(content []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.peerController.Register(ctx, content)
}

func (s *Server) GetNetMap(content []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.peerController.GetNetmap(ctx, content)
}

func (s *Server) Info(content []byte) ([]byte, error) {
	serverInfo := version.Get()
	data, err := json.Marshal(serverInfo)
	return data, err
}

func (s *Server) CreateToken(content []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return s.peerController.CreateToken(ctx, content)
}

// Heartbeat handles periodic heartbeat requests from agent nodes and updates
// the in-memory presence store so ListPeers can report real-time online status.
func (s *Server) Heartbeat(content []byte) ([]byte, error) {
	var payload struct {
		AppID string `json:"appId"`
	}
	if err := json.Unmarshal(content, &payload); err != nil {
		return nil, err
	}
	if payload.AppID != "" {
		s.presence.Update(payload.AppID)
	}
	return []byte{}, nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return nil
}
