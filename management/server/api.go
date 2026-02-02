package server

import (
	"fmt"
	"net/http"
	"wireflow/management/dto"
	"wireflow/management/server/middleware"
	"wireflow/pkg/cmd/network"

	"github.com/gin-gonic/gin"
)

func (s *Server) apiRouter() {
	r := s.Engine
	// 跨域处理（对接 Vite 开发环境）
	s.Use(middleware.CORSMiddleware())

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

	userApi := r.Group("/api/v1/users")
	{
		userApi.POST("/register", s.RegisterUser) //注册用户
		userApi.POST("/login", s.login)           //注册用户
	}

	peerApi := r.Group("/api/v1/peers")
	{
		peerApi.GET("/list", s.listPeers)
		peerApi.PUT("/update", s.updatePeer)
	}

	// 实时状态推送 (WebSocket)
	//r.GET("/ws/status", HandleStatusWS)
}

func (s *Server) ListNetworks(c *gin.Context) {

}

func (s *Server) GetPeers(c *gin.Context) {}

func (s *Server) listTokens() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokens, err := s.networkController.ListTokens(c.Request.Context())
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

// 用户注册
func (s *Server) RegisterUser(c *gin.Context) {
	var req dto.UserDto
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数校验失败: " + err.Error()})
		return
	}

	ctx := c.Request.Context()

	err := s.userController.Register(ctx, req)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "注册失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "注册成功"})
}

func (s *Server) login(c *gin.Context) {
	var req dto.UserDto
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "参数格式错误"})
		return
	}

	token, err := s.userController.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		c.JSON(401, gin.H{"error": err.Error()})
		return
	}

	// 返回给前端
	c.JSON(200, gin.H{
		"message": "登录成功",
		"token":   token,
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
