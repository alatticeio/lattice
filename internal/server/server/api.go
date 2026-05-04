package server

import (
	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/server/dex"
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/alatticeio/lattice/internal/server/server/middleware"
	"github.com/alatticeio/lattice/internal/server/service"
	"github.com/alatticeio/lattice/internal/web"
	"github.com/alatticeio/lattice/pkg/utils/resp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (s *Server) apiRouter() error {
	// 跨域处理（对接 Vite 开发环境）
	s.Use(middleware.CORSMiddleware())
	// 审计中间件：记录所有非 GET 写操作
	s.Use(middleware.AuditMiddleware(s.auditService))

	// Dex OIDC 为可选依赖：providerUrl 为空时跳过初始化，注册降级 handler。
	if s.cfg.Dex.ProviderUrl != "" {
		dexSvc, err := dex.NewDex(service.NewUserService(s.store))
		if err != nil {
			s.logger.Warn("Dex init failed, /auth/callback will return 503", "err", err)
			s.GET("/auth/callback", func(c *gin.Context) {
				c.JSON(503, gin.H{"error": "Dex OIDC provider not available"})
			})
		} else {
			s.GET("/auth/callback", dexSvc.Login)
		}
	} else {
		s.logger.Warn("dex.providerUrl is empty, Dex OIDC disabled")
		s.GET("/auth/callback", func(c *gin.Context) {
			c.JSON(503, gin.H{"error": "Dex OIDC is not configured"})
		})
	}
	//加入监控
	s.GET("/metrics", gin.WrapH(promhttp.Handler()))
	api := s.Group("/api/v1")
	{
		// 网络管理 (Namespace) — workspace-scoped, requires membership
		netApi := api.Group("")
		netApi.Use(s.middleware.WorkspaceAuthMiddleware(dto.RoleViewer))
		{
			netApi.POST("/networks", CreateNetwork)   // 创建新网络
			netApi.GET("/networks", s.ListNetworks)   // 获取网络列表
			netApi.GET("/networks/peers", s.GetPeers) // 获取该网络下的所有机器
		}
	}

	tokenApi := s.Group("/api/v1/token")
	tokenApi.Use(s.middleware.WorkspaceAuthMiddleware(dto.RoleViewer))
	{
		tokenApi.POST("/generate", s.generateToken())
		tokenApi.DELETE("/:token", s.rmToken())
		tokenApi.GET("/list", s.listTokens())
	}

	peerApi := s.Group("/api/v1/peers")
	peerApi.Use(s.middleware.WorkspaceAuthMiddleware(dto.RoleViewer))
	{
		peerApi.GET("/list", s.listPeers)
		peerApi.PUT("/update", s.updatePeer)
		peerApi.PUT("/:name/disable", s.disablePeer)
		peerApi.PUT("/:name/enable", s.enablePeer)
		peerApi.DELETE("/:name", s.deletePeerHandler)
	}

	policyApi := s.Group("/api/v1/policies")
	policyApi.Use(s.middleware.WorkspaceAuthMiddleware(dto.RoleViewer))
	{
		policyApi.GET("/list", s.listPolicies)
		policyApi.PUT("/update", s.createOrUpdatePolicy)
		policyApi.POST("/create", s.createOrUpdatePolicy)
		policyApi.DELETE("/:name", s.deletePolicy)
	}

	s.userRouter()

	s.workspaceRouter()

	s.relayRouter()

	s.memberRouter()

	s.invitationRouter()

	s.monitorRouter()

	s.alertRouter()

	s.customMetricRouter()

	s.profileRouter()

	s.dashboardRouter()

	s.auditRouter()

	s.workflowRouter()

	s.peeringRouter()

	s.aiRouter()

	s.platformRouter()

	// Discovery — no auth required; returns NATS URL for agent auto-connect.
	api.GET("/discovery", s.handleDiscovery())

	// SPA 静态资源：必须最后注册，通过 NoRoute 捕获所有未匹配路径
	s.logger.Info("Registering SPA static files")
	web.RegisterHandlers(s.Engine)

	return nil
}

func (s *Server) ListNetworks(c *gin.Context) {

}

func (s *Server) GetPeers(c *gin.Context) {}

func (s *Server) listTokens() gin.HandlerFunc {
	return func(c *gin.Context) {
		var pageParam dto.PageRequest
		err := c.ShouldBindQuery(&pageParam)
		if err != nil {
			resp.BadRequest(c, "invalid params")
			return
		}
		tokens, err := s.networkController.ListTokens(c.Request.Context(), &pageParam)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}

		resp.OK(c, tokens)
	}
}

func (s *Server) generateToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req dto.TokenDto
		// Body is optional for backward compat (Dashboard may POST empty body)
		_ = c.ShouldBindJSON(&req)

		token, err := s.tokenController.Create(c.Request.Context(), &req)
		if err != nil {
			resp.Error(c, err.Error())
			return
		}

		resp.OK(c, map[string]interface{}{
			"token": token,
		})
	}
}

func (s *Server) rmToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Param("token")
		if token == "" {
			resp.Error(c, "token is required")
			return
		}
		err := s.tokenController.Delete(c.Request.Context(), strings.ToLower(token))
		if err != nil {
			resp.Error(c, err.Error())
			return
		}

		resp.OK(c, nil)
	}
}

func CreateNetwork(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.BadRequest(c, "invalid json")
		return
	}

	resp.OK(c, gin.H{
		"message": "网络创建成功",
		"id":      req.Name,
	})
}

func (s *Server) listPeers(c *gin.Context) {
	var pageParam dto.PageRequest
	err := c.ShouldBindQuery(&pageParam)
	if err != nil {
		resp.BadRequest(c, "invalid params")
		return
	}

	data, err := s.peerController.ListPeers(c.Request.Context(), &pageParam)
	if err != nil {
		resp.Error(c, err.Error())
		return
	}

	resp.OK(c, data)
}

func (s *Server) updatePeer(c *gin.Context) {
	var req dto.PeerDto
	err := c.ShouldBindJSON(&req)
	if err != nil {
		resp.BadRequest(c, "invalid params")
		return
	}

	vo, err := s.peerController.UpdatePeer(c.Request.Context(), &req)
	if err != nil {
		resp.Error(c, err.Error())
		return
	}

	resp.OK(c, vo)
}

func (s *Server) peerNamespace(c *gin.Context) (string, error) {
	ctx := c.Request.Context()
	wsID, _ := ctx.Value(infra.WorkspaceKey).(string)
	ws, err := s.store.Workspaces().GetByID(ctx, wsID)
	if err != nil {
		return "", err
	}
	return ws.Namespace, nil
}

func (s *Server) disablePeer(c *gin.Context) {
	name := c.Param("name")
	ns, err := s.peerNamespace(c)
	if err != nil {
		resp.Error(c, err.Error())
		return
	}
	if err := s.peerController.DisablePeer(c.Request.Context(), ns, name); err != nil {
		resp.Error(c, err.Error())
		return
	}
	resp.OK(c, nil)
}

func (s *Server) enablePeer(c *gin.Context) {
	name := c.Param("name")
	ns, err := s.peerNamespace(c)
	if err != nil {
		resp.Error(c, err.Error())
		return
	}
	if err := s.peerController.EnablePeer(c.Request.Context(), ns, name); err != nil {
		resp.Error(c, err.Error())
		return
	}
	resp.OK(c, nil)
}

func (s *Server) deletePeerHandler(c *gin.Context) {
	name := c.Param("name")
	ns, err := s.peerNamespace(c)
	if err != nil {
		resp.Error(c, err.Error())
		return
	}
	if err := s.peerController.DeletePeer(c.Request.Context(), ns, name); err != nil {
		resp.Error(c, err.Error())
		return
	}
	resp.OK(c, nil)
}

func (s *Server) handleDiscovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		natsURL := s.cfg.SignalingURL
		// Prefer DB-stored value (set via platform settings UI)
		if dbURL, err := s.store.SystemConfig().Get(c.Request.Context(), models.ConfigKeyNatsURL); err == nil && dbURL != "" {
			natsURL = dbURL
		}
		if natsURL == "" {
			natsURL = "nats://127.0.0.1:4222"
		}
		resp.OK(c, gin.H{"nats_url": natsURL})
	}
}
