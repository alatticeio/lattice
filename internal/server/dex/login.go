//go:build pro

package dex

import (
	"context"
	"fmt"
	"net/http"

	"github.com/alatticeio/lattice/internal/agent/config"
	"github.com/alatticeio/lattice/internal/server/models"
	"github.com/alatticeio/lattice/internal/server/service"
	serverutils "github.com/alatticeio/lattice/internal/server/utils"
	"github.com/alatticeio/lattice/pkg/utils/resp"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

// 1. OIDC configuration
var endpoint = oauth2.Endpoint{
	AuthURL:  "http://lattice-dex.lattice-system.svc.cluster.local:5556/dex/auth",
	TokenURL: "http://lattice-dex.lattice-system.svc.cluster.local:5556/dex/token",
}

var oauth2Config = oauth2.Config{
	ClientID:     "lattice-server",     // Must match dex-oauth2Config.yaml
	ClientSecret: "lattice-secret-key", // Must match dex-oauth2Config.yaml
	Endpoint:     endpoint,
	RedirectURL:  "http://localhost:8080/auth/callback",
	Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
}

type Dex struct {
	verifier     *oidc.IDTokenVerifier
	oauth2Config *oauth2.Config

	userService service.UserService
}

func NewDex(userService service.UserService) (*Dex, error) {
	veryfier, err := InitVerifier()
	if err != nil {
		return nil, err
	}
	return &Dex{
		userService:  userService,
		oauth2Config: &oauth2Config,
		verifier:     veryfier,
	}, nil
}

// 2. Login handler
func (d *Dex) Login(c *gin.Context) {
	ctx := c.Request.Context()

	// 1. Get the authorization code
	code := c.Query("code")
	if code == "" {
		resp.BadRequest(c, "Missing code")
		return
	}

	// 2. Use OAuth2 config to exchange the token with Dex
	// oauth2Config is the variable you initialized earlier
	oauth2Token, err := d.oauth2Config.Exchange(ctx, code)
	if err != nil {
		resp.Error(c, fmt.Sprintf("Failed to exchange token: %v", err))
		return
	}

	// 3. Parse the ID Token (this is the user identity information returned by Dex)
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		resp.Error(c, "No id_token in response")
		return
	}

	// 4. Verify the token and extract claims
	idToken, err := d.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		resp.Error(c, fmt.Sprintf("Failed to verify ID Token: %v ", err))
		return
	}

	var dexClaims models.LatticeClaims

	if err = idToken.Claims(&dexClaims); err != nil {
		resp.Error(c, "Failed to parse claims")
		return
	}

	//// 5. [Core] Sync to your database and initialize K8s infrastructure
	//// This calls the OnboardExternalUser function we initially wrote
	//user, err := d.workspaceService.OnboardExternalUser(ctx, dexClaims.Subject, dexClaims.Name)
	//if err != nil {
	//	c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to onboard user"})
	//	return
	//}

	user, err := d.userService.OnboardExternalUser(ctx, "dex", dexClaims.Subject, dexClaims.Email, config.GlobalConfig.Dex.AdminEmails)
	if err != nil {
		resp.Error(c, fmt.Sprintf("Failed to get user: %v", err))
		return
	}

	// 6. Issue your own business JWT (for subsequent frontend requests)
	businessToken, _ := serverutils.GenerateBusinessJWT(user.ID, user.Email, user.Username, string(user.SystemRole))

	// 7. Return result or redirect
	// Private cloud deployments typically redirect directly to the frontend Dashboard with the token
	c.Redirect(http.StatusFound, "http://localhost:5173/login/success?token="+businessToken)
}

func InitVerifier() (*oidc.IDTokenVerifier, error) {
	ctx := context.Background()

	// 1. Create a Provider, which will automatically fetch the public key from http://localhost:5556/dex/.well-known/openid-configuration
	provider, err := oidc.NewProvider(ctx, config.GlobalConfig.Dex.ProviderUrl)
	if err != nil {
		return nil, err
	}

	// 2. Create Verifier configuration
	// It checks whether the Token's issuer is Dex and whether the audience is your lattice-server
	cfg := &oidc.Config{
		ClientID: "lattice-server",
	}

	return provider.Verifier(cfg), nil
}
