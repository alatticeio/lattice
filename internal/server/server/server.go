// Copyright 2026 The Lattice Authors, Inc.
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
	"github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/db"
	"github.com/alatticeio/lattice/internal/monitor"
	"github.com/alatticeio/lattice/internal/server/auth"
	"github.com/alatticeio/lattice/internal/server/controller"
	"github.com/alatticeio/lattice/internal/server/llm"
	managementnats "github.com/alatticeio/lattice/internal/server/nats"
	"github.com/alatticeio/lattice/internal/server/permission"
	"github.com/alatticeio/lattice/internal/server/resource"
	"github.com/alatticeio/lattice/internal/server/server/middleware"
	"github.com/alatticeio/lattice/internal/server/service"
	"github.com/alatticeio/lattice/pkg/utils"
	"time"

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
	cfg    *config.Config

	client            *resource.Client
	manager           manager.Manager
	cacheReady        chan struct{}
	peerController    controller.PeerController
	networkController controller.NetworkController
	userController    controller.UserController
	policyController  controller.PolicyController

	workspaceController  controller.WorkspaceController
	memberController     controller.WorkspaceMemberController
	tokenController      controller.TokenController
	relayController      controller.RelayController
	invitationController controller.InvitationController

	monitorController      controller.MonitorController
	alertController        controller.AlertController
	customMetricController controller.CustomMetricController
	profileController      controller.ProfileController
	auditController        controller.AuditController
	workflowController     controller.WorkflowController
	platformController     controller.PlatformController

	aiService      service.AIService
	peeringService service.PeeringService

	middleware      *middleware.Middleware
	revocationList  *auth.RevocationList
	auditService    service.AuditService
	workflowService service.WorkflowService

	store    store.Store
	presence *managementnats.NodePresenceStore
	monitor  *monitor.Monitor
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
		svc, err := managementnats.NewNatsService(ctx, "lattice-manager", "server", cfg.SignalingURL)
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

	// 注册一个 Runnable：等待 controller-runtime cache 同步完成后，
	// 关闭 cacheReady 通知外部 HTTP Server 可以安全上线。
	var cacheReady chan struct{}
	if mgr != nil {
		cacheReady = make(chan struct{})
		ch := cacheReady
		_ = mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
			// 等待所有 Informer Cache 同步完成
			if !mgr.GetCache().WaitForCacheSync(ctx) {
				return fmt.Errorf("failed to wait for cache sync")
			}
			// Cache 已同步，通知 HTTP Server 可以启动
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

	auditSvc := service.NewAuditService(st)
	auditSvc.Start(ctx)

	workflowSvc := service.NewWorkflowService(st)

	// ── 弱依赖③：AI 服务（APIKey 未配置时降级为 nil）──────────────────────
	var aiSvc service.AIService
	if cfg.AI.Enabled && cfg.AI.APIKey != "" {
		llmClient, aiErr := llm.NewClient(cfg.AI)
		if aiErr != nil {
			logger.Warn("AI init failed, AI features disabled", "err", aiErr)
		} else {
			aiSvc = service.NewAIService(llmClient, st, client, presence, cfg.AI.MaxToolCalls)
			logger.Info("AI service initialized", "provider", cfg.AI.Provider)
		}
	} else {
		logger.Info("AI service disabled (set ai.enabled=true and ai.api-key to enable)")
	}

	// ── 弱依赖④：Monitor（可选）────────────────────────────────────
	var mon *monitor.Monitor
	var heartbeatDB *gorm.DB
	if gs, ok := st.(interface{ DB() *gorm.DB }); ok {
		heartbeatDB = gs.DB()
	}
	mon, monErr := monitor.NewMonitor(cfg.Monitor.Address, st, heartbeatDB)
	if monErr != nil {
		logger.Warn("monitor init failed, monitoring features disabled", "err", monErr)
	} else {
		logger.Info("monitor initialized")
		mon.StartAlertEngine(context.Background())
	}

	revocationList := auth.NewRevocationList()
	revocationList.StartCleanup(5 * time.Minute)

	checker := permission.NewChecker(st, nil)

	s := &Server{
		Engine:                 gin.Default(),
		logger:                 logger,
		listen:                 cfg.Listen,
		nats:                   signal,
		manager:                mgr,
		cacheReady:             cacheReady,
		client:                 client,
		cfg:                    cfg,
		presence:               presence,
		peerController:         controller.NewPeerController(client, st, presence),
		networkController:      controller.NewNetworkController(client, st),
		userController:         controller.NewUserController(st),
		policyController:       controller.NewPolicyController(client, st),
		workspaceController:    controller.NewWorkspaceController(client, st),
		memberController:       controller.NewWorkspaceMemberController(st),
		tokenController:        controller.NewTokenController(client, st),
		relayController:        controller.NewRelayController(client, st),
		invitationController:   controller.NewInvitationController(st, string(utils.GetJWTSecret())),
		monitorController:      controller.NewMonitorController(cfg.Monitor.Address, st),
		alertController:        controller.NewAlertController(st),
		customMetricController: controller.NewCustomMetricController(st),
		profileController:      controller.NewProfileController(st),
		auditController:        controller.NewAuditController(auditSvc),
		workflowController:     controller.NewWorkflowController(workflowSvc),
		platformController:     controller.NewPlatformController(st),
		middleware:             middleware.NewMiddleware(checker, st, revocationList),
		revocationList:         revocationList,
		auditService:           auditSvc,
		workflowService:        workflowSvc,
		store:                  st,
		aiService:              aiSvc,
		peeringService:         service.NewPeeringService(client, st),
		monitor:                mon,
	}

	// initAdmins：DB 已就绪后执行；失败只告警，不阻断启动。
	if err = s.userController.InitAdmin(context.Background(), config.GlobalConfig.App.InitAdmins); err != nil {
		s.logger.Warn("init admin failed (non-fatal, will retry on next startup)", "err", err)
	} else {
		s.logger.Debug("Init admin success")
	}

	// Register workflow executors before starting the router.
	s.registerPolicyExecutor()

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
		// agent ↔ server (peer signaling) — keep these
		"lattice.signals.peer.register":  s.Register,
		"lattice.signals.peer.GetNetMap": s.GetNetMap,
		"lattice.signals.peer.heartbeat": s.Heartbeat,

		// CLI routes removed — CLI now uses HTTP REST
	}

	for route, handler := range routes {
		s.nats.Service(route, "lattice_queue", handler)
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
