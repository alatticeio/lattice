package dex

import (
	"context"
	"net/http"
	"wireflow/management/controller"
	"wireflow/management/model"
	"wireflow/pkg/utils"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

// 1. OIDC 配置
var endpoint = oauth2.Endpoint{
	AuthURL:  "http://localhost:5556/dex/auth",
	TokenURL: "http://localhost:5556/dex/token",
}

var config = oauth2.Config{
	ClientID:     "wireflow-server",     // 必须对应 dex-config.yaml
	ClientSecret: "wireflow-secret-key", // 必须对应 dex-config.yaml
	Endpoint:     endpoint,
	RedirectURL:  "http://localhost:8080/auth/callback",
	Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
}

type Dex struct {
	verifier     *oidc.IDTokenVerifier
	oauth2Config *oauth2.Config

	teamController controller.TeamController
}

func NewDex(teamController controller.TeamController) (*Dex, error) {
	veryfier, err := InitVerifier()
	if err != nil {
		return nil, err
	}
	return &Dex{
		teamController: teamController,
		oauth2Config:   &config,
		verifier:       veryfier,
	}, nil
}

// 2. 登录 Handler
func (d *Dex) Login(c *gin.Context) {
	ctx := c.Request.Context()

	// 1. 获取授权码
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing code"})
		return
	}

	// 2. 使用 OAuth2 配置向 Dex 兑换 Token
	// oauth2Config 是你初始化时定义的变量
	oauth2Token, err := d.oauth2Config.Exchange(ctx, code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to exchange token: " + err.Error()})
		return
	}

	// 3. 解析 ID Token (这是 Dex 返回的用户身份信息)
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No id_token in response"})
		return
	}

	// 4. 验证 Token 并提取 Claims
	idToken, err := d.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify ID Token: " + err.Error()})
		return
	}

	var dexClaims model.WireFlowClaims

	if err := idToken.Claims(&dexClaims); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse claims"})
		return
	}

	// 5. 【核心】同步到你的数据库并初始化 K8s 基础设施
	// 这调用的是我们最初写的 OnboardExternalUser 函数
	user, err := d.teamController.OnboardExternalUser(ctx, dexClaims.Subject, dexClaims.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to onboard user"})
		return
	}

	// 6. 签发你自己的业务 JWT (给前端后续请求使用)
	businessToken, _ := utils.GenerateBusinessJWT(user.ID, user.Email)

	// 7. 返回结果或重定向
	// 私有云部署通常直接重定向回前端 Dashboard，带上 Token
	c.Redirect(http.StatusFound, "http://localhost:5173/login/success?token="+businessToken)
}

func InitVerifier() (*oidc.IDTokenVerifier, error) {
	ctx := context.Background()

	// 1. 创建一个 Provider，它会自动去 http://localhost:5556/dex/.well-known/openid-configuration 获取公钥
	provider, err := oidc.NewProvider(ctx, "http://localhost:5556/dex")
	if err != nil {
		return nil, err
	}

	// 2. 创建 Verifier 配置
	// 它会检查 Token 的发行者是否是 Dex，以及接收者（Audience）是否是你的 wireflow-server
	config := &oidc.Config{
		ClientID: "wireflow-server",
	}

	return provider.Verifier(config), nil
}
