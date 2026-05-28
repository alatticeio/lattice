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
	"encoding/base64"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"github.com/alatticeio/lattice/api/v1alpha1"
	"github.com/alatticeio/lattice/internal/agent/infra"
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/pkg/utils"
	"github.com/alatticeio/lattice/pkg/utils/resp"
	"github.com/alatticeio/lattice/pkg/version"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// demoMagicSession is stored in Server.demoSessions keyed by the raw magic token.
type demoMagicSession struct {
	userID               string
	workspaceID          string
	workspaceNamespace   string
	workspaceSlug        string
	workspaceDisplayName string
	expiresAt            time.Time
}

// demoLaunchResponse is returned by POST /api/v1/demo/launch.
type demoLaunchResponse struct {
	WorkspaceID string    `json:"workspace_id"`
	ExpiresAt   time.Time `json:"expires_at"`
	Device1Cmd  string    `json:"device1_cmd"`
	Device2Cmd  string    `json:"device2_cmd"`
	ConsoleURL  string    `json:"console_url"`
}

// sandboxLaunchResponse is returned by POST /api/v1/demo/sandbox/launch.
type sandboxLaunchResponse struct {
	WorkspaceID string    `json:"workspace_id"`
	ExpiresAt   time.Time `json:"expires_at"`
	ServerURL   string    `json:"server_url"`
	Token       string    `json:"token"`
	InstallCmd  string    `json:"install_cmd"`
	ConsoleURL  string    `json:"console_url"`
}

// demoConfigDefaults holds resolved demo configuration with defaults applied.
type demoConfigDefaults struct {
	Enabled          bool
	TTLMinutes       int
	RateLimitPerHour int
}

func (s *Server) demoRouter() {
	s.GET("/api/v1/demo/status", s.handleDemoStatus())
	s.GET("/api/v1/demo/auth", s.handleDemoAuth())
	s.POST("/api/v1/demo/launch",
		s.demoLimiter.Middleware(rate.Limit(s.demoCfg().RateLimitPerHour)/3600, 1),
		s.handleDemoLaunch(),
	)
	s.POST("/api/v1/demo/sandbox/launch",
		s.demoLimiter.Middleware(rate.Limit(s.demoCfg().RateLimitPerHour)/3600, 1),
		s.handleSandboxDemoLaunch(),
	)
}

func (s *Server) demoCfg() *demoConfigDefaults {
	c := s.cfg.App.Demo
	d := &demoConfigDefaults{
		Enabled:          c.Enabled,
		TTLMinutes:       c.TTLMinutes,
		RateLimitPerHour: c.RateLimitPerHour,
	}
	if d.TTLMinutes <= 0 {
		d.TTLMinutes = 60
	}
	if d.RateLimitPerHour <= 0 {
		d.RateLimitPerHour = 5
	}
	return d
}

func (s *Server) handleDemoStatus() gin.HandlerFunc {
	return func(c *gin.Context) {
		resp.OK(c, gin.H{"enabled": s.cfg.App.Demo.Enabled})
	}
}

func (s *Server) handleDemoLaunch() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := s.demoCfg()
		if !cfg.Enabled {
			c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "demo is disabled"})
			return
		}

		ctx := c.Request.Context()
		ttl := time.Duration(cfg.TTLMinutes) * time.Minute
		expiresAt := time.Now().Add(ttl)

		// 1. Create workspace
		slug := fmt.Sprintf("demo-%d", time.Now().UnixMilli())
		wsVo, err := s.workspaceController.AddWorkspace(ctx, &dto.WorkspaceDto{
			Slug:        slug,
			DisplayName: "Demo Workspace",
		})
		if err != nil {
			resp.Error(c, "failed to create demo workspace: "+err.Error())
			return
		}
		rollback := func() { _ = s.workspaceController.DeleteWorkspace(context.Background(), wsVo.ID) }

		// 2. Mark workspace as demo with expiry
		ws, err := s.store.Workspaces().GetByID(ctx, wsVo.ID)
		if err != nil {
			rollback()
			resp.Error(c, "failed to fetch workspace: "+err.Error())
			return
		}
		ws.IsDemo = true
		ws.ExpiresAt = &expiresAt
		if err = s.store.Workspaces().Update(ctx, ws); err != nil {
			rollback()
			resp.Error(c, "failed to mark demo workspace: "+err.Error())
			return
		}

		// 3. Create enrollment token (limit=2, one per device)
		tokenCtx := context.WithValue(ctx, infra.WorkspaceKey, wsVo.ID)
		expiry := fmt.Sprintf("%dm", cfg.TTLMinutes)
		tokenStr, err := s.tokenController.Create(tokenCtx, &dto.TokenDto{
			Namespace: wsVo.Namespace,
			Limit:     2,
			Expiry:    expiry,
		})
		if err != nil {
			rollback()
			resp.Error(c, "failed to create enrollment token: "+err.Error())
			return
		}

		// 4. Apply allow-all policy so demo devices can reach each other.
		// Must include PeerSelector + Ingress/Egress with network label to match peers,
		// mirroring what e2e createAllowAllPolicy does.
		const demoNetwork = "lattice-default-net"
		networkLabel := fmt.Sprintf("alattice.io/network-%s", demoNetwork)
		peerSel := metav1.LabelSelector{
			MatchLabels: map[string]string{networkLabel: "true"},
		}
		if _, policyErr := s.policyController.ApplyDirect(tokenCtx, wsVo.ID, "", "", &dto.PolicyDto{
			Name:        "demo-allow-all",
			Action:      "Allow",
			PolicyTypes: []string{"Ingress", "Egress"},
			LatticePolicySpec: v1alpha1.LatticePolicySpec{
				Network:      demoNetwork,
				PeerSelector: peerSel,
				Action:       "ALLOW",
				Ingress: []v1alpha1.IngressRule{
					{From: []v1alpha1.PeerSelection{{PeerSelector: &peerSel}}},
				},
				Egress: []v1alpha1.EgressRule{
					{To: []v1alpha1.PeerSelection{{PeerSelector: &peerSel}}},
				},
			},
		}); policyErr != nil {
			s.logger.Warn("demo: failed to apply allow-all policy (non-fatal)", "err", policyErr)
		}

		// 5. Create a temporary demo user and add to the workspace as admin.
		rawBytes := make([]byte, 16)
		if err = utils.GenerateRandomBytes(rawBytes); err != nil {
			rollback()
			resp.Error(c, "failed to generate demo credentials: "+err.Error())
			return
		}
		randSuffix := base64.RawURLEncoding.EncodeToString(rawBytes)[:12]
		demoUsername := fmt.Sprintf("demo-%s", randSuffix)
		demoPassword := base64.RawURLEncoding.EncodeToString(rawBytes) // 22-char random password

		if err = s.userController.Register(ctx, dto.UserDto{
			Username: demoUsername,
			Password: demoPassword,
		}); err != nil {
			rollback()
			resp.Error(c, "failed to create demo user: "+err.Error())
			return
		}

		demoUser, err := s.userController.Login(ctx, demoUsername, demoPassword)
		if err != nil {
			rollback()
			resp.Error(c, "failed to authenticate demo user: "+err.Error())
			return
		}

		if err = s.memberController.Add(ctx, wsVo.ID, demoUser.ID, dto.RoleAdmin); err != nil {
			rollback()
			resp.Error(c, "failed to add demo user to workspace: "+err.Error())
			return
		}

		// 6. Issue magic token (one-time, same TTL as demo workspace).
		magicRaw := make([]byte, 32)
		if err = utils.GenerateRandomBytes(magicRaw); err != nil {
			rollback()
			resp.Error(c, "failed to generate magic token: "+err.Error())
			return
		}
		magicToken := base64.RawURLEncoding.EncodeToString(magicRaw)
		s.demoSessions.Store(magicToken, demoMagicSession{
			userID:               demoUser.ID,
			workspaceID:          wsVo.ID,
			workspaceNamespace:   wsVo.Namespace,
			workspaceSlug:        wsVo.Slug,
			workspaceDisplayName: wsVo.DisplayName,
			expiresAt:            expiresAt,
		})

		// 7. Build curl commands and console URL
		scheme := "https"
		if c.Request.TLS == nil && c.GetHeader("X-Forwarded-Proto") != "https" {
			scheme = "http"
		}
		host := c.Request.Host
		if fwdHost := c.GetHeader("X-Forwarded-Host"); fwdHost != "" {
			host = fwdHost
		}
		serverURL := fmt.Sprintf("%s://%s", scheme, host)
		installURL := fmt.Sprintf("%s/install.sh", serverURL)

		var cmd string
		if isCleanRelease(version.Version) {
			cmd = fmt.Sprintf(
				"curl -fsSL %s | bash -s -- --server %s --token %s --tag %s",
				installURL, serverURL, tokenStr, version.Version,
			)
		} else {
			cmd = fmt.Sprintf(
				"curl -fsSL %s | bash -s -- --server %s --token %s",
				installURL, serverURL, tokenStr,
			)
		}
		consoleURL := fmt.Sprintf("%s/auth/demo?token=%s", serverURL, magicToken)

		resp.OK(c, demoLaunchResponse{
			WorkspaceID: wsVo.ID,
			ExpiresAt:   expiresAt,
			Device1Cmd:  cmd,
			Device2Cmd:  cmd,
			ConsoleURL:  consoleURL,
		})
	}
}

func (s *Server) handleSandboxDemoLaunch() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := s.demoCfg()
		if !cfg.Enabled {
			c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "demo is disabled"})
			return
		}

		ctx := c.Request.Context()

		// Optional: re-issue a magic token for an existing sandbox demo workspace.
		// The frontend passes workspace_id when restoring a cached session so that
		// a fresh console_url is obtained after a server restart (magic tokens are
		// stored in memory and lost on restart).
		var req struct {
			WorkspaceID string `json:"workspace_id"`
		}
		_ = c.ShouldBindJSON(&req)

		if req.WorkspaceID != "" {
			if url, ok := s.reissueSandboxMagicToken(c, ctx, req.WorkspaceID); ok {
				resp.OK(c, sandboxLaunchResponse{ConsoleURL: url})
				return
			}
			// Workspace gone or expired — fall through to create a fresh session.
		}

		ttl := time.Duration(cfg.TTLMinutes) * time.Minute
		expiresAt := time.Now().Add(ttl)

		// 1. Create workspace
		slug := fmt.Sprintf("sandbox-demo-%d", time.Now().UnixMilli())
		wsVo, err := s.workspaceController.AddWorkspace(ctx, &dto.WorkspaceDto{
			Slug:        slug,
			DisplayName: "Sandbox Demo Workspace",
		})
		if err != nil {
			resp.Error(c, "failed to create sandbox demo workspace: "+err.Error())
			return
		}
		rollback := func() { _ = s.workspaceController.DeleteWorkspace(context.Background(), wsVo.ID) }

		// 2. Mark workspace as demo with expiry
		ws, err := s.store.Workspaces().GetByID(ctx, wsVo.ID)
		if err != nil {
			rollback()
			resp.Error(c, "failed to fetch workspace: "+err.Error())
			return
		}
		ws.IsDemo = true
		ws.ExpiresAt = &expiresAt
		if err = s.store.Workspaces().Update(ctx, ws); err != nil {
			rollback()
			resp.Error(c, "failed to mark sandbox demo workspace: "+err.Error())
			return
		}

		// 3. Create enrollment token (limit=1, one sandbox agent)
		tokenCtx := context.WithValue(ctx, infra.WorkspaceKey, wsVo.ID)
		expiry := fmt.Sprintf("%dm", cfg.TTLMinutes)
		tokenStr, err := s.tokenController.Create(tokenCtx, &dto.TokenDto{
			Namespace: wsVo.Namespace,
			Limit:     1,
			Expiry:    expiry,
		})
		if err != nil {
			rollback()
			resp.Error(c, "failed to create enrollment token: "+err.Error())
			return
		}

		// 4. Apply allow-all policy
		const demoNetwork = "lattice-default-net"
		networkLabel := fmt.Sprintf("alattice.io/network-%s", demoNetwork)
		peerSel := metav1.LabelSelector{
			MatchLabels: map[string]string{networkLabel: "true"},
		}
		if _, policyErr := s.policyController.ApplyDirect(tokenCtx, wsVo.ID, "", "", &dto.PolicyDto{
			Name:        "demo-allow-all",
			Action:      "Allow",
			PolicyTypes: []string{"Ingress", "Egress"},
			LatticePolicySpec: v1alpha1.LatticePolicySpec{
				Network:      demoNetwork,
				PeerSelector: peerSel,
				Action:       "ALLOW",
				Ingress: []v1alpha1.IngressRule{
					{From: []v1alpha1.PeerSelection{{PeerSelector: &peerSel}}},
				},
				Egress: []v1alpha1.EgressRule{
					{To: []v1alpha1.PeerSelection{{PeerSelector: &peerSel}}},
				},
			},
		}); policyErr != nil {
			s.logger.Warn("sandbox demo: failed to apply allow-all policy (non-fatal)", "err", policyErr)
		}

		// 5. Create demo user
		rawBytes := make([]byte, 16)
		if err = utils.GenerateRandomBytes(rawBytes); err != nil {
			rollback()
			resp.Error(c, "failed to generate demo credentials: "+err.Error())
			return
		}
		randSuffix := base64.RawURLEncoding.EncodeToString(rawBytes)[:12]
		demoUsername := fmt.Sprintf("demo-%s", randSuffix)
		demoPassword := base64.RawURLEncoding.EncodeToString(rawBytes)

		if err = s.userController.Register(ctx, dto.UserDto{
			Username: demoUsername,
			Password: demoPassword,
		}); err != nil {
			rollback()
			resp.Error(c, "failed to create demo user: "+err.Error())
			return
		}

		demoUser, err := s.userController.Login(ctx, demoUsername, demoPassword)
		if err != nil {
			rollback()
			resp.Error(c, "failed to authenticate demo user: "+err.Error())
			return
		}

		if err = s.memberController.Add(ctx, wsVo.ID, demoUser.ID, dto.RoleAdmin); err != nil {
			rollback()
			resp.Error(c, "failed to add demo user to workspace: "+err.Error())
			return
		}

		// 6. Issue magic token
		magicRaw := make([]byte, 32)
		if err = utils.GenerateRandomBytes(magicRaw); err != nil {
			rollback()
			resp.Error(c, "failed to generate magic token: "+err.Error())
			return
		}
		magicToken := base64.RawURLEncoding.EncodeToString(magicRaw)
		s.demoSessions.Store(magicToken, demoMagicSession{
			userID:               demoUser.ID,
			workspaceID:          wsVo.ID,
			workspaceNamespace:   wsVo.Namespace,
			workspaceSlug:        wsVo.Slug,
			workspaceDisplayName: wsVo.DisplayName,
			expiresAt:            expiresAt,
		})

		// 7. Build URLs
		scheme := "https"
		if c.Request.TLS == nil && c.GetHeader("X-Forwarded-Proto") != "https" {
			scheme = "http"
		}
		host := c.Request.Host
		if fwdHost := c.GetHeader("X-Forwarded-Host"); fwdHost != "" {
			host = fwdHost
		}
		serverURL := fmt.Sprintf("%s://%s", scheme, host)
		installURL := fmt.Sprintf("%s/install.sh", serverURL)

		var installCmd string
		if isCleanRelease(version.Version) {
			installCmd = fmt.Sprintf(
				"curl -fsSL %s | bash -s -- --server %s --token %s --tag %s",
				installURL, serverURL, tokenStr, version.Version,
			)
		} else {
			installCmd = fmt.Sprintf(
				"curl -fsSL %s | bash -s -- --server %s --token %s",
				installURL, serverURL, tokenStr,
			)
		}

		consoleURL := fmt.Sprintf("%s/auth/demo?token=%s&redirect=/sandbox", serverURL, magicToken)

		resp.OK(c, sandboxLaunchResponse{
			WorkspaceID: wsVo.ID,
			ExpiresAt:   expiresAt,
			ServerURL:   serverURL,
			Token:       tokenStr,
			InstallCmd:  installCmd,
			ConsoleURL:  consoleURL,
		})
	}
}

// handleDemoAuth validates a magic token and returns a short-lived JWT + refresh token.
// The token is one-time use and is deleted from the session map on success.
func (s *Server) handleDemoAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Query("token")
		if token == "" {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "token is required"})
			return
		}

		val, ok := s.demoSessions.Load(token)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "invalid or expired demo token"})
			return
		}
		session := val.(demoMagicSession)

		if time.Now().After(session.expiresAt) {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "demo session has expired"})
			return
		}

		user, err := s.store.Users().GetByID(c.Request.Context(), session.userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "demo user not found"})
			return
		}

		ttl := time.Until(session.expiresAt)
		accessToken, err := utils.GenerateBusinessJWTWithDuration(
			user.ID, user.Email, user.Username, string(user.SystemRole), ttl,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "failed to issue token"})
			return
		}

		rawRefreshToken, err := s.generateRefreshToken(c, user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "failed to issue refresh token"})
			return
		}

		resp.OK(c, gin.H{
			"token":        accessToken,
			"refreshToken": rawRefreshToken,
			"workspace": gin.H{
				"id":          session.workspaceID,
				"namespace":   session.workspaceNamespace,
				"slug":        session.workspaceSlug,
				"displayName": session.workspaceDisplayName,
			},
		})
	}
}

// reissueSandboxMagicToken issues a fresh magic token for an existing, unexpired sandbox
// demo workspace. It creates a new temporary demo user, adds them to the workspace, and
// stores the magic session. Returns the console URL and true on success.
func (s *Server) reissueSandboxMagicToken(c *gin.Context, ctx context.Context, workspaceID string) (string, bool) {
	ws, err := s.store.Workspaces().GetByID(ctx, workspaceID)
	if err != nil || !ws.IsDemo || ws.ExpiresAt == nil || ws.ExpiresAt.Before(time.Now()) {
		return "", false
	}

	// Create a new demo user for this workspace.
	rawBytes := make([]byte, 16)
	if err = utils.GenerateRandomBytes(rawBytes); err != nil {
		return "", false
	}
	randSuffix := base64.RawURLEncoding.EncodeToString(rawBytes)[:12]
	demoUsername := fmt.Sprintf("demo-%s", randSuffix)
	demoPassword := base64.RawURLEncoding.EncodeToString(rawBytes)

	if err = s.userController.Register(ctx, dto.UserDto{Username: demoUsername, Password: demoPassword}); err != nil {
		return "", false
	}
	demoUser, err := s.userController.Login(ctx, demoUsername, demoPassword)
	if err != nil {
		return "", false
	}
	if err = s.memberController.Add(ctx, ws.ID, demoUser.ID, dto.RoleAdmin); err != nil {
		return "", false
	}

	// Issue magic token.
	magicRaw := make([]byte, 32)
	if err = utils.GenerateRandomBytes(magicRaw); err != nil {
		return "", false
	}
	magicToken := base64.RawURLEncoding.EncodeToString(magicRaw)
	s.demoSessions.Store(magicToken, demoMagicSession{
		userID:               demoUser.ID,
		workspaceID:          ws.ID,
		workspaceNamespace:   ws.Namespace,
		workspaceSlug:        ws.Slug,
		workspaceDisplayName: ws.DisplayName,
		expiresAt:            *ws.ExpiresAt,
	})

	scheme := "https"
	if c.Request.TLS == nil && c.GetHeader("X-Forwarded-Proto") != "https" {
		scheme = "http"
	}
	host := c.Request.Host
	if fwdHost := c.GetHeader("X-Forwarded-Host"); fwdHost != "" {
		host = fwdHost
	}
	serverURL := fmt.Sprintf("%s://%s", scheme, host)
	return fmt.Sprintf("%s/auth/demo?token=%s&redirect=/sandbox", serverURL, magicToken), true
}

// startDemoCleanup launches a background goroutine that deletes expired demo workspaces every 5 minutes.
func (s *Server) startDemoCleanup(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.sweepExpiredDemos(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}

// isCleanRelease reports whether v is a clean semver release tag (e.g. "v1.2.3").
var cleanReleaseRe = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

func isCleanRelease(v string) bool {
	return cleanReleaseRe.MatchString(v)
}

func (s *Server) sweepExpiredDemos(ctx context.Context) {
	// Purge expired magic tokens from the in-memory session map.
	now := time.Now()
	s.demoSessions.Range(func(key, value any) bool {
		if sess, ok := value.(demoMagicSession); ok && now.After(sess.expiresAt) {
			s.demoSessions.Delete(key)
		}
		return true
	})

	expired, err := s.store.Workspaces().ListExpiredDemos(ctx, now)
	if err != nil {
		s.logger.Warn("demo cleanup: list expired demos failed", "err", err)
		return
	}
	for _, ws := range expired {
		if err := s.workspaceController.DeleteWorkspace(ctx, ws.ID); err != nil {
			s.logger.Warn("demo cleanup: delete workspace failed", "id", ws.ID, "err", err)
		} else {
			s.logger.Info("demo cleanup: deleted expired demo workspace", "id", ws.ID, "namespace", ws.Namespace)
		}
	}
}
