package server

import (
	"fmt"
	"wireflow/management/controller"
	"wireflow/management/dex"
	"wireflow/management/dto"
	"wireflow/management/server/middleware"
	"wireflow/pkg/cmd/network"

	"github.com/gin-gonic/gin"
)

func (s *Server) apiRouter() error {
	r := s.Engine
	// 跨域处理（对接 Vite 开发环境）
	s.Use(middleware.CORSMiddleware())

	dex, err := dex.NewDex(controller.NewTeamController(s.client, s.cfg))
	if err != nil {
		return err
	}
	r.GET("/auth/callback", dex.Login)
	api := r.Group("/api/v1")
	{
		// 网络管理 (Namespace)
		api.POST("/networks", CreateNetwork) // 创建新网络
		api.GET("/networks", s.ListNetworks) // 获取网络列表

		// Token 管理
		api.POST("/networks/:id/tokens", GenerateToken) // 为指定网络生成入网 Token
		api.GET("/tokens", s.listTokens())

		// 节点管理 (Peers)
		api.GET("/networks/:id/peers", s.GetPeers) // 获取该网络下的所有机器
	}

	peerApi := r.Group("/api/v1/peers")
	{
		peerApi.GET("/list", s.listPeers)
		peerApi.PUT("/update", s.updatePeer)
	}

	policyApi := r.Group("/api/v1/policies")
	{
		policyApi.GET("/list", s.listPolicies)
		policyApi.PUT("/update", s.updatePolicy)
		policyApi.POST("/create", s.createPolicy)
		policyApi.DELETE("/delete", s.deletePolicy)
	}

	s.userApi()

	// 实时状态推送 (WebSocket)
	//r.GET("/ws/status", HandleStatusWS)
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
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		tokens, err := s.networkController.ListTokens(c.Request.Context(), &pageParam)
		if err != nil {
			c.JSON(400, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(200, tokens)
	}
}

// 模拟 JWT 或加密 Token 的生成
func GenerateToken(c *gin.Context) {
	//networkID := c.Param("id") // 这里就是 Namespace 的名字

	// 实际商业化建议使用 JWT，包含过期时间和 Namespace 信息
	// 简单实现可以是一个带前缀的随机串
	token := fmt.Sprintf("", network.GenerateNetworkID())

	// 将 Token 存入 Redis 或 K8s Secret，供 Agent 加入时校验
	// SaveTokenToStore(token, networkID)

	c.JSON(200, gin.H{
		"token":       token,
		"expires_in":  86400, // 24小时
		"install_cmd": fmt.Sprintf("curl -sL wireflow.io/i.sh | sh -s -- --token %s", token),
	})
}

func CreateNetwork(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "无效的参数"})
		return
	}

	// 调用 K8s SDK 创建 Namespace
	// err := k8sClient.CreateNamespace(req.Name)

	c.JSON(201, gin.H{
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
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	data, err := s.peerController.ListPeers(c.Request.Context(), &pageParam)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, data)
}

func (s *Server) updatePeer(c *gin.Context) {
	var req dto.PeerDto
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	vo, err := s.peerController.UpdatePeer(c.Request.Context(), &req)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, vo)
}
