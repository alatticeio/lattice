package server

import (
	"strings"
	"wireflow/internal/web"
	"wireflow/management/dex"
	"wireflow/management/dto"
	"wireflow/management/server/middleware"
	"wireflow/management/service"
	"wireflow/pkg/utils/resp"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (s *Server) apiRouter() error {
	// 跨域处理（对接 Vite 开发环境）
	s.Use(middleware.CORSMiddleware())

	// Dex OIDC 为可选依赖：providerUrl 为空时跳过初始化，注册降级 handler。
	if s.cfg.Dex.ProviderUrl != "" {
		dexSvc, err := dex.NewDex(service.NewWorkspaceService(s.client, s.store))
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
		// 网络管理 (Namespace)
		api.POST("/networks", CreateNetwork) // 创建新网络
		api.GET("/networks", s.ListNetworks) // 获取网络列表

		// 节点管理 (Peers)
		api.GET("/networks/peers", middleware.TenantContextMiddleware(), s.GetPeers) // 获取该网络下的所有机器
	}

	tokenApi := s.Group("/api/v1/token")
	{
		// Token 管理
		tokenApi.POST("/generate", middleware.TenantContextMiddleware(), s.generateToken()) // 为指定网络生成入网 Token// Token 管理
		tokenApi.DELETE("/:token", middleware.TenantContextMiddleware(), s.rmToken())
		tokenApi.GET("/list", middleware.TenantContextMiddleware(), s.listTokens())

	}

	peerApi := s.Group("/api/v1/peers")
	{
		peerApi.GET("/list", middleware.TenantContextMiddleware(), s.listPeers)
		peerApi.PUT("/update", s.updatePeer)
	}

	policyApi := s.Group("/api/v1/policies")
	{
		policyApi.GET("/list", middleware.TenantContextMiddleware(), s.listPolicies)
		policyApi.PUT("/update", middleware.TenantContextMiddleware(), s.createOrUpdatePolicy)
		policyApi.POST("/create", middleware.TenantContextMiddleware(), s.createOrUpdatePolicy)
		policyApi.DELETE("/:name", middleware.TenantContextMiddleware(), s.deletePolicy)
	}

	s.userRouter()

	s.workspaceRouter()

	s.monitorRouter()

	s.profileRouter()

	s.dashboardRouter()

	// 实时状态推送 (WebSocket)
	//r.GET("/ws/status", HandleStatusWS)

	// SPA 静态资源：必须最后注册，通过 NoRoute 捕获所有未匹配路径
	web.RegisterHandlers(s.Engine)

	return nil
}

func (s *Server) ListNetworks(c *gin.Context) {

}

func (s *Server) GetPeers(c *gin.Context) {}

func (s *Server) listTokens() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 获取参数
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

// 模拟 JWT 或加密 Token 的生成
func (s *Server) generateToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := s.tokenController.Create(c.Request.Context())
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

	// 调用 K8s SDK 创建 Namespace
	// err := k8sClient.AddWorkspace(req.Name)

	resp.OK(c, gin.H{
		"message": "网络创建成功",
		"id":      req.Name,
	})
}

// 模拟获取所有节点（实际可能来自 wg show 或 内存 Map）
func (s *Server) listPeers(c *gin.Context) {
	// 1. 获取参数
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
