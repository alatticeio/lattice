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
	v1alpha1 "github.com/alatticeio/lattice/api/v1alpha1"
	"github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/agent/log"
	"github.com/alatticeio/lattice/internal/agent/store"
	"github.com/alatticeio/lattice/internal/db"
	"github.com/alatticeio/lattice/internal/license"
	"github.com/alatticeio/lattice/internal/monitor"
	"github.com/alatticeio/lattice/internal/server/auth"
	"github.com/alatticeio/lattice/internal/server/controller"
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/internal/server/llm"
	managementnats "github.com/alatticeio/lattice/internal/server/nats"
	"github.com/alatticeio/lattice/internal/server/permission"
	"github.com/alatticeio/lattice/internal/server/resource"
	"github.com/alatticeio/lattice/internal/server/server/middleware"
	"github.com/alatticeio/lattice/internal/server/service"
	serverutils "github.com/alatticeio/lattice/internal/server/utils"
	"sync"
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
	featureController      controller.FeatureController

	aiService          service.AIService
	intentService      service.IntentService
	peeringService     service.PeeringService
	agentEnrollService service.AgentEnrollService

	agentIsolationService service.AgentIsolationService
	agentRegService       service.AgentRegistrationService
	mcpServerSvc          service.MCPServerService   // nil when K8s unavailable
	agentPolicySvc        service.AgentPolicyService // nil when K8s unavailable

	peerIdentityController controller.PeerIdentityController

	middleware      *middleware.Middleware
	demoLimiter     *middleware.IPRateLimiter
	demoSessions    sync.Map // magic token → demoMagicSession
	revocationList  *auth.RevocationList
	auditService    service.AuditService
	workflowService service.WorkflowService
	authService     service.AuthService

	store           store.Store
	presence        *managementnats.NodePresenceStore
	monitor         *monitor.Monitor
	licenseVerifier license.Verifier
	auditConsumer   interface{ Close() } // PRO: flow audit NATS consumer; nil in community
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

	// ── Weak Dependency 1: NATS Signal Service (optional) ────────────
	// If signaling-url is empty or connection fails, fall back to noop; main process continues.
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

	// ── Weak Dependency 2: K8s Manager (optional) ────────────────────
	// Skipped in non-K8s environments (local dev, CI); does not affect HTTP Server startup.
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

	// Register a Runnable: wait for controller-runtime cache sync to complete,
	// then close cacheReady to signal the HTTP Server it can safely go online.
	var cacheReady chan struct{}
	if mgr != nil {
		cacheReady = make(chan struct{})
		ch := cacheReady
		_ = mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
			// Wait for all Informer Cache to sync
			if !mgr.GetCache().WaitForCacheSync(ctx) {
				return fmt.Errorf("failed to wait for cache sync")
			}
			// Cache synced, notify HTTP Server it can start
			close(ch)
			<-ctx.Done()
			return nil
		}))
	}

	// ── Hard Dependency: Database (returns error on failure, per design) ───
	st, err := db.NewStore(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to init store: %w", err)
	}

	presence := managementnats.NewNodePresenceStore()

	auditSvc := service.NewAuditService(st)
	auditSvc.Start(ctx)

	workflowSvc := service.NewWorkflowService(st)

	// ── Agent Isolation (optional; nil-safe when disabled or no k8s client) ──
	var agentIsolSvc service.AgentIsolationService
	var agentRegSvc service.AgentRegistrationService
	if cfg.AI.AgentIsolation.Enabled && client != nil {
		jwtSecret := cfg.AI.AgentIsolation.JWTSecret
		if jwtSecret == "" {
			jwtSecret = cfg.JWT.Secret
		}
		agentIsolSvc = service.NewAgentIsolationService(
			cfg.AI.AgentIsolation,
			&k8sAgentIdentityReader{c: client.Client},
		)
		agentRegSvc = service.NewAgentRegistrationService(jwtSecret, st, client.Client)
		logger.Info("agent isolation enabled", "mode", cfg.AI.AgentIsolation.EnforcementMode)
		if mgr != nil {
			if err = controller.NewAgentTTLReconciler(mgr.GetClient()).SetupWithManager(mgr); err != nil {
				logger.Warn("failed to setup AgentTTLReconciler", "err", err)
			}
			if err = controller.NewAgentIdentityReconciler(mgr.GetClient()).SetupWithManager(mgr); err != nil {
				logger.Warn("failed to setup AgentIdentityReconciler", "err", err)
			}
		}
	}

	// MCPServer and AgentPolicy services — available whenever K8s client is present.
	var mcpServerSvc service.MCPServerService
	var agentPolicySvc service.AgentPolicyService
	if client != nil {
		mcpServerSvc = service.NewMCPServerService(client.Client)
		agentPolicySvc = service.NewAgentPolicyService(client.Client)
		if mgr != nil {
			if err = controller.NewMCPServerReconciler(mgr.GetClient()).SetupWithManager(mgr); err != nil {
				logger.Warn("failed to setup MCPServerReconciler", "err", err)
			}
		}
	}

	// ── Weak Dependency 3: AI Service (tools always available; Chat/Audit/Debug need LLM) ──
	var aiSvc service.AIService
	var intentSvc service.IntentService
	if cfg.AI.Enabled && cfg.AI.APIKey != "" {
		llmClient, aiErr := llm.NewClient(cfg.AI)
		if aiErr != nil {
			logger.Warn("AI init failed, AI features disabled", "err", aiErr)
		} else {
			aiSvc = service.NewAIServiceWithWorkflow(
				llmClient, st, client, presence,
				cfg.AI.MaxToolCalls,
				workflowSvc,
				cfg.AI.Workflow.AutoApprove,
				agentIsolSvc,
			)
			intentSvc = service.NewIntentService(llmClient, client, st)
			service.SetIntentService(aiSvc, intentSvc)

			// Time-Travel Debug: attach snapshot store to AI service
			service.SetSnapStore(aiSvc, st.NetworkSnapshots())

			// Register SnapshotController (CRD change → snapshot capture)
			if mgr != nil && client != nil {
				snapCtrl := controller.NewSnapshotController(mgr.GetClient(), st.NetworkSnapshots(), st.Workspaces())
				if err = snapCtrl.SetupWithManager(mgr); err != nil {
					logger.Warn("failed to setup snapshot controller", "err", err)
				}
			}

			logger.Info("AI service initialized", "provider", cfg.AI.Provider)
		}
	}
	// Always create tool-capable service even without LLM (Chat/Audit/Debug will return errors).
	if aiSvc == nil {
		aiSvc = service.NewAIServiceWithWorkflow(nil, st, client, presence, 0, workflowSvc, nil, agentIsolSvc)
		logger.Info("AI service in tools-only mode (Chat/Audit/Debug require api-key)")
	}
	// Attach workspace and token controllers for AI workspace/token management tools.
	wsCtrl := controller.NewWorkspaceController(client, st)
	tokCtrl := controller.NewTokenController(client, st)
	service.SetWorkspaceCtrl(aiSvc, wsCtrl)
	service.SetTokenCtrl(aiSvc, tokCtrl)
	service.SetToolSpanRepo(aiSvc, st.ToolSpans())

	// PRO: start flow audit consumer (no-op in community edition).
	flowAuditConsumer := initFlowAuditConsumer(cfg.SignalingURL, st)

	// ── License Verification (Pro validates JWT, Community returns StatusNotFound) ──
	var lv license.Verifier
	if cfg.App.Playground {
		lv = license.NewPlaygroundVerifier()
		logger.Info("playground mode: all Pro features unlocked")
	} else {
		lv = license.NewVerifier("pro")
		var lic *license.License
		var status license.Status
		lic, status, err = lv.Verify()
		if err != nil {
			logger.Warn("license check", "status", status, "err", err)
		} else {
			logger.Info("license valid", "type", lic.Type, "customer", lic.CustomerName,
				"expires", lic.ExpiresAt.Format(time.RFC3339), "features", lic.Features)
		}
	}

	// ── Weak Dependency 4: Monitor (optional) ────────────────────────
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

	demoRL := middleware.NewIPRateLimiter()

	authSvc := service.NewAuthService(st)

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
		peerController:         controller.NewPeerController(client, st, presence, lv),
		networkController:      controller.NewNetworkController(client, st),
		userController:         controller.NewUserController(st),
		policyController:       controller.NewPolicyController(client, st),
		workspaceController:    controller.NewWorkspaceController(client, st),
		memberController:       controller.NewWorkspaceMemberController(st),
		tokenController:        controller.NewTokenController(client, st),
		relayController:        controller.NewRelayController(client, st),
		invitationController:   controller.NewInvitationController(st, string(serverutils.GetJWTSecret())),
		monitorController:      controller.NewMonitorController(cfg.Monitor.Address, st),
		alertController:        controller.NewAlertController(st),
		customMetricController: controller.NewCustomMetricController(st),
		profileController:      controller.NewProfileController(st),
		auditController:        controller.NewAuditController(auditSvc),
		workflowController:     controller.NewWorkflowController(workflowSvc),
		platformController:     controller.NewPlatformController(st),
		featureController:      controller.NewFeatureController(st),
		middleware:             middleware.NewMiddleware(checker, st, revocationList),
		demoLimiter:            demoRL,
		revocationList:         revocationList,
		authService:            authSvc,
		auditService:           auditSvc,
		workflowService:        workflowSvc,
		store:                  st,
		aiService:              aiSvc,
		intentService:          intentSvc,
		peeringService:         service.NewPeeringService(client, st),
		agentEnrollService:     service.NewAgentEnrollService(client),
		agentIsolationService:  agentIsolSvc,
		agentRegService:        agentRegSvc,
		mcpServerSvc:           mcpServerSvc,
		agentPolicySvc:         agentPolicySvc,
		peerIdentityController: controller.NewPeerIdentityController(st),
		licenseVerifier:        lv,
		monitor:                mon,
		auditConsumer:          flowAuditConsumer,
	}

	// initAdmins: runs after DB is ready; only warns on failure, does not block startup.
	if err = s.userController.InitAdmin(context.Background(), config.GlobalConfig.App.InitAdmins); err != nil {
		s.logger.Warn("init admin failed (non-fatal, will retry on next startup)", "err", err)
	} else {
		s.logger.Debug("Init admin success")
	}

	// Playground: auto-create default workspace once K8s cache is ready.
	// Runs in a goroutine because the controller-runtime cache must be synced
	// before workspace creation (createDefaultNetwork uses client.Get via cache).
	if cfg.App.Playground {
		go func() {
			if cacheReady != nil {
				select {
				case <-cacheReady:
				case <-time.After(120 * time.Second):
					s.logger.Warn("playground: timed out waiting for K8s cache, skipping workspace init")
					return
				}
			}
			if wsErr := s.initPlaygroundWorkspace(context.Background()); wsErr != nil {
				s.logger.Warn("playground workspace init failed (non-fatal)", "err", wsErr)
			}
		}()
	}

	// Register workflow executors before starting the router.
	s.registerPolicyExecutor()

	// ── Snapshot retention: prune old snapshots daily ──────────────
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			cutoff := time.Now().Add(-90 * 24 * time.Hour) // 90 days
			n, rErr := st.NetworkSnapshots().DeleteOlderThan(context.Background(), cutoff)
			if rErr != nil {
				logger.Warn("snapshot pruning failed", "err", rErr)
			} else if n > 0 {
				logger.Info("pruned old snapshots", "count", n)
			}
		}
	}()

	s.startDemoCleanup(ctx)

	if err = s.apiRouter(); err != nil {
		return nil, err
	}

	return s, nil
}

// initPlaygroundWorkspace creates a default "My Workspace" on first playground startup.
// It is idempotent: skips silently if any workspace already exists.
func (s *Server) initPlaygroundWorkspace(ctx context.Context) error {
	_, total, err := s.store.Workspaces().List(ctx, "", 1, 1)
	if err != nil {
		return fmt.Errorf("check workspaces: %w", err)
	}
	if total > 0 {
		s.logger.Debug("playground: workspace already exists, skipping init")
		return nil
	}

	admins := config.GlobalConfig.App.InitAdmins
	if len(admins) == 0 {
		return fmt.Errorf("no init_admins configured")
	}
	user, err := s.store.Users().GetByUsername(ctx, admins[0].Username)
	if err != nil {
		return fmt.Errorf("get admin user %q: %w", admins[0].Username, err)
	}

	userCtx := context.WithValue(ctx, infra.UserIDKey, user.ID)
	userCtx = context.WithValue(userCtx, infra.UsernameKey, user.Username)

	ws, err := s.workspaceController.AddWorkspace(userCtx, &dto.WorkspaceDto{
		DisplayName: "My Workspace",
		Slug:        "my-workspace",
	})
	if err != nil {
		return fmt.Errorf("create default workspace: %w", err)
	}
	s.logger.Info("playground: default workspace created", "id", ws.ID, "namespace", ws.Namespace)
	return nil
}

func (s *Server) Start(ctx context.Context) error {
	if s.manager == nil {
		// K8s manager unavailable: block until ctx is cancelled, letting the goroutine exit cleanly.
		<-ctx.Done()
		return nil
	}

	// Register NATS service
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

	// Critical: ensure subscription commands have reached and been processed by the NATS Server
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

	// Sandbox path: enrollment token + client-side public key.
	// Detected by the presence of publicKey in the payload.
	// Regular agents never send publicKey during registration.
	if s.agentRegService != nil {
		var peek dto.PeerDto
		if jsonErr := json.Unmarshal(content, &peek); jsonErr == nil && peek.PublicKey != "" {
			return s.handleSandboxNATSRegister(ctx, peek)
		}
	}

	return s.peerController.Register(ctx, content)
}

// handleSandboxNATSRegister registers a sandbox agent via enrollment token over NATS.
// It delegates to agentRegistrationService.RegisterAgent and returns an infra.Peer
// with the agent JWT in the Token field (no PrivateKey — sandbox owns its own key).
func (s *Server) handleSandboxNATSRegister(ctx context.Context, peer dto.PeerDto) ([]byte, error) {
	result, err := s.agentRegService.RegisterAgent(ctx, service.AgentRegisterRequest{
		EnrollmentToken: peer.Token,
		AgentName:       peer.AppID,
		PublicKey:       peer.PublicKey,
		Sandbox:         v1alpha1.SandboxGVisor,
	})
	if err != nil {
		return nil, err
	}
	// Return infra.Peer with JWT in Token field.
	// PrivateKey is intentionally empty: sandbox generates its own key.
	returnPeer := &infra.Peer{
		Name:         peer.AppID,
		AppID:        peer.AppID,
		Token:        result.JWT,
		EnforcerMode: result.EnforcerMode,
		Tier:         result.Tier,
	}
	data, err := json.Marshal(returnPeer)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (s *Server) GetNetMap(content []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Sandbox agents authenticate with a JWT instead of a LatticeEnrollmentToken CRD.
	// If the token field parses as a valid agent JWT, serve the ConfigMap directly.
	if s.agentRegService != nil && s.client != nil {
		var peer dto.PeerDto
		if jsonErr := json.Unmarshal(content, &peer); jsonErr == nil && peer.Token != "" {
			if claims, jwtErr := s.agentRegService.ValidateAgentJWT(peer.Token); jwtErr == nil {
				msg, err := s.client.GetNetworkMapForPeer(ctx, claims.Namespace, peer.AppID)
				if err != nil {
					return nil, err
				}
				return json.Marshal(msg)
			}
		}
	}

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
	if s.auditConsumer != nil {
		s.auditConsumer.Close()
	}
	return nil
}

func (s *Server) httpsRedirect() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Header.Get("X-Forwarded-Proto") == "http" {
			target := "https://" + c.Request.Host + c.Request.RequestURI
			c.Redirect(301, target)
			c.Abort()
			return
		}
		c.Next()
	}
}
