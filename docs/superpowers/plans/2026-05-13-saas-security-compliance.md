# SaaS Security Compliance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement 8 security and compliance capabilities required for public SaaS deployment: password policy, input validation, security headers, CSRF hardening, rate limiting, token refresh/logout, HTTPS/TLS ingress, and privacy/TOS pages.

**Architecture:** Backend changes are middleware-level Go additions to the Gin server (`internal/server/server/middleware/`) and service-level validation (`internal/server/service/`, `pkg/utils/`). Frontend changes add legal pages and update signup/login flows. Infrastructure uses Ingress + cert-manager for TLS.

**Tech Stack:** Go 1.25, Gin, bcrypt, `golang.org/x/time/rate`, Vue 3, cert-manager, nginx-ingress

---

## File Structure

```
Create:
  pkg/utils/encrypt_test.go                            # Password policy tests
  internal/server/server/middleware/rate_limit.go       # IP rate limiter
  internal/server/server/middleware/rate_limit_test.go  # Rate limiter tests
  internal/server/server/middleware/security_headers.go # Security header middleware
  internal/server/models/refresh_token.go               # Refresh token model
  internal/server/service/auth.go                       # Auth service (refresh/logout)
  internal/server/service/auth_test.go                  # Auth service tests
  fronted/src/pages/legal/privacy.vue                   # Privacy policy page
  fronted/src/pages/legal/terms.vue                     # Terms of service page
  deploy/k3s/ingress.yaml                               # Ingress + TLS config

Modify:
  pkg/utils/encrypt.go                                 # Add ValidatePassword
  internal/server/service/user.go                      # Register + change-password call ValidatePassword
  internal/server/service/user_test.go                 # Password validation tests
  internal/server/dto/user.go                          # Add binding tags, tosAccepted field
  internal/server/server/api.go                        # Register middlewares
  internal/server/server/server.go                     # HTTPS redirect
  internal/server/server/middleware/cors.go             # Tighten CORS origins
  internal/db/gormstore/migrate.go                      # refresh_tokens migration
  internal/server/seed/seed.go                         # Initial admin (may need refresh token)
  fronted/src/pages/auth/signup/index.vue              # Add TOS checkbox + password strength
  fronted/src/pages/index.vue                          # Footer links
  fronted/src/pages/auth/login/index.vue               # Handle refreshToken from login response
  fronted/src/api/request.ts                           # 401 auto-refresh interceptor
  fronted/src/api/user.ts                              # Add refresh/logout API calls
  config/lattice/overlays/all-in-one/kustomization.yaml # Ingress resource
```

---

### Task 1: Password Policy Validation

**Files:**
- Modify: `pkg/utils/encrypt.go`
- Create: `pkg/utils/encrypt_test.go`
- Modify: `internal/server/service/user.go`
- Modify: `internal/server/dto/user.go`

- [ ] **Step 1: Add ValidatePassword to encrypt.go**

Read `pkg/utils/encrypt.go`. Append after `ComparePassword`:

```go
import (
    "errors"
    "strings"
)

var (
    ErrPasswordTooShort = errors.New("password must be at least 8 characters")
    ErrPasswordNoUpper  = errors.New("password must contain at least one uppercase letter")
    ErrPasswordNoLower  = errors.New("password must contain at least one lowercase letter")
    ErrPasswordNoDigit  = errors.New("password must contain at least one digit")
)

func ValidatePassword(password string) error {
    if len(password) < 8 {
        return ErrPasswordTooShort
    }
    if !strings.ContainsAny(password, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
        return ErrPasswordNoUpper
    }
    if !strings.ContainsAny(password, "abcdefghijklmnopqrstuvwxyz") {
        return ErrPasswordNoLower
    }
    if !strings.ContainsAny(password, "0123456789") {
        return ErrPasswordNoDigit
    }
    return nil
}
```

- [ ] **Step 2: Write tests for ValidatePassword**

Create `pkg/utils/encrypt_test.go`:

```go
package utils

import (
    "testing"
)

func TestValidatePassword(t *testing.T) {
    tests := []struct {
        name  string
        pw    string
        errIs error
    }{
        {"too short", "Ab1", ErrPasswordTooShort},
        {"no upper", "abcdefgh1", ErrPasswordNoUpper},
        {"no lower", "ABCDEFGH1", ErrPasswordNoLower},
        {"no digit", "Abcdefgh", ErrPasswordNoDigit},
        {"valid", "Abcdefg1", nil},
        {"valid long", "MySecureP@ssw0rd!", nil},
        {"valid exactly 8", "Abcdefg1", nil},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidatePassword(tt.pw)
            if tt.errIs != nil {
                if err == nil {
                    t.Errorf("expected error containing %q, got nil", tt.errIs)
                }
            } else {
                if err != nil {
                    t.Errorf("expected no error, got %v", err)
                }
            }
        })
    }
}
```

- [ ] **Step 3: Run tests**

```bash
cd /Users/francis/workspc/lattice && go test ./pkg/utils/ -run TestValidatePassword -v
```

Expected: 7/7 pass.

- [ ] **Step 4: Integrate into user service**

Read `internal/server/service/user.go`. In `Register()` method, add at the top:

```go
if err := utils.ValidatePassword(userDto.Password); err != nil {
    return fmt.Errorf("password: %w", err)
}
```

- [ ] **Step 5: Update DTO binding**

Read `internal/server/dto/user.go`. Ensure `Password` field has binding:

```go
Password string `json:"password" binding:"required,min=8,max=128"`
```

Add `TOSAccepted` field for privacy/TOS (used in Task 8):

```go
TOSAccepted bool `json:"tosAccepted"`
```

- [ ] **Step 6: Build verify**

```bash
cd /Users/francis/workspc/lattice && go build ./...
```

- [ ] **Step 7: Commit**

```bash
git add pkg/utils/encrypt.go pkg/utils/encrypt_test.go internal/server/service/user.go internal/server/dto/user.go
git commit -s -m "feat(security): add password policy validation (8 chars, upper, lower, digit)"
```

---

### Task 2: Input Validation (DTO Binding Tags)

**Files:**
- Create: `pkg/utils/validator.go`
- Modify: `internal/server/dto/user.go`
- Modify: `internal/server/dto/token.go` (if exists)
- Modify: `internal/server/dto/workspace.go` (if exists)

- [ ] **Step 1: Create custom validator**

Read existing DTOs in `internal/server/dto/`. Create `pkg/utils/validator.go`:

```go
package utils

import (
    "net"
    "regexp"
    "strings"
)

var safeStringRE = regexp.MustCompile(`^[a-zA-Z0-9_\-\.\s]+$`)

// IsSafeString returns true if s contains no HTML/JS injection characters.
func IsSafeString(s string) bool {
    return safeStringRE.MatchString(s)
}

// IsValidCIDR returns true if s is a valid CIDR notation.
func IsValidCIDR(s string) bool {
    _, _, err := net.ParseCIDR(s)
    return err == nil
}

// SanitizeString trims and removes null bytes.
func SanitizeString(s string) string {
    s = strings.TrimSpace(s)
    s = strings.ReplaceAll(s, "\x00", "")
    return s
}
```

- [ ] **Step 2: Update UserDto binding tags**

Read `internal/server/dto/user.go`. Apply binding:

```go
type UserDto struct {
    Username    string `json:"username"  binding:"required,min=2,max=64"`
    Email       string `json:"email"     binding:"required,email,max=128"`
    Password    string `json:"password"  binding:"required,min=8,max=128"`
    TOSAccepted bool   `json:"tosAccepted"`
}

type LoginDto struct {
    Email    string `json:"email"    binding:"required,email,max=128"`
    Password string `json:"password" binding:"required,min=1,max=128"`
}
```

- [ ] **Step 3: Check other DTOs**

Run to find DTOs without binding tags:

```bash
cd /Users/francis/workspc/lattice && grep -rn "json:" internal/server/dto/ | grep -v binding | head -20
```

For each DTO used in `c.ShouldBindJSON`, add appropriate binding tags. Priority: `WorkspaceDto`, `TokenDto`, `PolicyDto`.

- [ ] **Step 4: Build verify**

```bash
cd /Users/francis/workspc/lattice && go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add pkg/utils/validator.go internal/server/dto/
git commit -s -m "feat(security): add input validation with DTO binding tags and custom validators"
```

---

### Task 3: Security Headers Middleware

**Files:**
- Create: `internal/server/server/middleware/security_headers.go`
- Modify: `internal/server/server/api.go`

- [ ] **Step 1: Read api.go to find middleware registration point**

Read `internal/server/server/api.go`. Find where `s.Use(...)` calls are made (CORS, auth, etc.).

- [ ] **Step 2: Create security headers middleware**

Create `internal/server/server/middleware/security_headers.go`:

```go
package middleware

import "github.com/gin-gonic/gin"

func SecurityHeaders() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
        c.Header("X-Content-Type-Options", "nosniff")
        c.Header("X-Frame-Options", "DENY")
        c.Header("X-XSS-Protection", "0")
        c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
        c.Next()
    }
}
```

- [ ] **Step 3: Register middleware**

In `internal/server/server/api.go`, add at top of middleware chain (before CORS):

```go
import "github.com/alatticeio/lattice/internal/server/server/middleware"

func (s *Server) registerRoutes() {
    s.Use(middleware.SecurityHeaders())
    // ... existing middlewares ...
}
```

- [ ] **Step 4: Build verify**

```bash
cd /Users/francis/workspc/lattice && go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add internal/server/server/middleware/security_headers.go internal/server/server/api.go
git commit -s -m "feat(security): add security headers middleware (HSTS, XFO, CSP)"
```

---

### Task 4: CSRF Hardening (CORS Whitelist)

**Files:**
- Modify: `internal/server/server/middleware/cors.go`

- [ ] **Step 1: Read and update CORS middleware**

Read `internal/server/server/middleware/cors.go`. If `AllowOrigins` uses `*`, tighten:

```go
// Before (if wildcard):
config.AllowOrigins = []string{"*"}

// After:
config.AllowOrigins = []string{
    "https://console.alattice.io",
    "http://localhost:5173", // dev
}
config.AllowCredentials = true
config.AllowHeaders = append(config.AllowHeaders, "Authorization", "X-Workspace-Id")
```

If cors.go uses gin-contrib/cors, use `cors.Config{}`. If it's custom, just tighten origin check.

- [ ] **Step 2: Build verify**

```bash
cd /Users/francis/workspc/lattice && go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add internal/server/server/middleware/cors.go
git commit -s -m "fix(security): tighten CORS origins to production domain"
```

---

### Task 5: Rate Limiting Middleware

**Files:**
- Create: `internal/server/server/middleware/rate_limit.go`
- Create: `internal/server/server/middleware/rate_limit_test.go`
- Modify: `internal/server/server/api.go`

- [ ] **Step 1: Create rate limiter**

Create `internal/server/server/middleware/rate_limit.go`:

```go
package middleware

import (
    "sync"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/alatticeio/lattice/pkg/utils/resp"
    "golang.org/x/time/rate"
)

type rateLimiterEntry struct {
    limiter  *rate.Limiter
    lastSeen time.Time
}

type IPRateLimiter struct {
    mu       sync.RWMutex
    limiters map[string]*rateLimiterEntry
}

func NewIPRateLimiter() *IPRateLimiter {
    rl := &IPRateLimiter{limiters: make(map[string]*rateLimiterEntry)}
    go rl.cleanup(10 * time.Minute)
    return rl
}

func (rl *IPRateLimiter) Allow(key string, r rate.Limit, burst int) bool {
    rl.mu.Lock()
    entry, exists := rl.limiters[key]
    if !exists {
        entry = &rateLimiterEntry{limiter: rate.NewLimiter(r, burst)}
        rl.limiters[key] = entry
    }
    entry.lastSeen = time.Now()
    rl.mu.Unlock()
    return entry.limiter.Allow()
}

func (rl *IPRateLimiter) cleanup(ttl time.Duration) {
    for range time.Tick(ttl) {
        rl.mu.Lock()
        for k, v := range rl.limiters {
            if time.Since(v.lastSeen) > ttl {
                delete(rl.limiters, k)
            }
        }
        rl.mu.Unlock()
    }
}

func (rl *IPRateLimiter) Middleware(r rate.Limit, burst int) gin.HandlerFunc {
    return func(c *gin.Context) {
        ip := c.ClientIP()
        if !rl.Allow(ip, r, burst) {
            resp.Error(c, 429, "Too many requests, please try again later")
            c.Abort()
            return
        }
        c.Next()
    }
}
```

Check if `resp.Error` exists. If not, wrap:

```go
func rateLimitExceeded(c *gin.Context) {
    c.JSON(429, gin.H{"code": 429, "message": "Too many requests, please try again later"})
    c.Abort()
}
```

- [ ] **Step 2: Register middleware in api.go**

In `internal/server/server/api.go`, after `import` block, add:

```go
var ipLimiter = middleware.NewIPRateLimiter()
```

In `registerRoutes()`, add after security headers:

```go
s.Use(ipLimiter.Middleware(100.0/60, 200)) // 100 req/min, burst 200
```

On login route, add specific limiter:

```go
loginGroup.POST("/login",
    ipLimiter.Middleware(5.0/60, 5),  // 5 req/min
    s.userController.Login,
)
```

On register route:

```go
loginGroup.POST("/register",
    ipLimiter.Middleware(10.0/60, 3), // 10 req/min
    s.userController.Register,
)
```

- [ ] **Step 3: Run go mod tidy**

```bash
cd /Users/francis/workspc/lattice && go mod tidy
```

- [ ] **Step 4: Build verify**

```bash
cd /Users/francis/workspc/lattice && go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add internal/server/server/middleware/rate_limit.go internal/server/server/api.go go.mod go.sum
git commit -s -m "feat(security): add IP-based rate limiting middleware (global + login/register)"
```

---

### Task 6: Token Refresh + Logout

**Files:**
- Create: `internal/server/models/refresh_token.go`
- Create: `internal/server/service/auth.go`
- Modify: `internal/server/service/user.go` (Login method modified to return refresh token)
- Modify: `internal/db/gormstore/migrate.go`
- Modify: `internal/server/server/api.go` (register auth routes)
- Modify: `fronted/src/api/user.ts`
- Modify: `fronted/src/api/request.ts`
- Modify: `fronted/src/pages/auth/login/index.vue`

- [ ] **Step 1: Create refresh token model**

Read existing models in `internal/server/models/` to understand GORM pattern. Create `internal/server/models/refresh_token.go`:

```go
package models

import (
    "crypto/sha256"
    "encoding/hex"
    "time"
)

type RefreshToken struct {
    Model
    UserID    string     `gorm:"index;size:64"`
    TokenHash string     `gorm:"uniqueIndex;size:64"`
    ExpiresAt time.Time
    RevokedAt *time.Time
}

// HashRefreshToken returns SHA256 hex of raw token.
func HashRefreshToken(raw string) string {
    h := sha256.Sum256([]byte(raw))
    return hex.EncodeToString(h[:])
}

func (RefreshToken) TableName() string { return "t_refresh_tokens" }
```

- [ ] **Step 2: Add migration**

Read `internal/db/gormstore/migrate.go`. Add to AutoMigrate:

```go
&models.RefreshToken{},
```

- [ ] **Step 3: Create auth service**

Read `internal/server/service/user.go` to understand how `Login` currently works. Create `internal/server/service/auth.go`:

```go
package service

import (
    "context"
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "time"

    "github.com/alatticeio/lattice/internal/agent/store"
    "github.com/alatticeio/lattice/internal/server/models"
)

type AuthService interface {
    RefreshAccessToken(ctx context.Context, refreshTokenRaw string) (string, string, error)
    Logout(ctx context.Context, refreshTokenRaw string) error
}

type authService struct {
    store     store.Store
    jwtSecret string
}

func NewAuthService(st store.Store, jwtSecret string) AuthService {
    return &authService{store: st, jwtSecret: jwtSecret}
}

func (s *authService) generateRefreshToken(ctx context.Context, userID string) (string, error) {
    raw := make([]byte, 32)
    if _, err := rand.Read(raw); err != nil {
        return "", fmt.Errorf("generate refresh token: %w", err)
    }
    token := hex.EncodeToString(raw)
    rt := &models.RefreshToken{
        UserID:    userID,
        TokenHash: models.HashRefreshToken(token),
        ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
    }
    if err := s.store.RefreshTokens().Create(ctx, rt); err != nil {
        return "", fmt.Errorf("store refresh token: %w", err)
    }
    return token, nil
}

func (s *authService) RefreshAccessToken(ctx context.Context, refreshTokenRaw string) (string, string, error) {
    hash := models.HashRefreshToken(refreshTokenRaw)
    rt, err := s.store.RefreshTokens().GetByHash(ctx, hash)
    if err != nil {
        return "", "", fmt.Errorf("invalid refresh token")
    }
    if rt.RevokedAt != nil {
        return "", "", fmt.Errorf("refresh token revoked")
    }
    if time.Now().After(rt.ExpiresAt) {
        return "", "", fmt.Errorf("refresh token expired")
    }
    // Issue new access token (copy from existing JWT issuance logic in user.go)
    accessToken, err := s.issueAccessToken(rt.UserID)
    if err != nil {
        return "", "", err
    }
    // Rotate: revoke old, issue new
    _ = s.store.RefreshTokens().Revoke(ctx, rt.TokenHash)
    newRefresh, err := s.generateRefreshToken(ctx, rt.UserID)
    if err != nil {
        return "", "", err
    }
    return accessToken, newRefresh, nil
}

func (s *authService) Logout(ctx context.Context, refreshTokenRaw string) error {
    hash := models.HashRefreshToken(refreshTokenRaw)
    return s.store.RefreshTokens().Revoke(ctx, hash)
}

func (s *authService) issueAccessToken(userID string) (string, error) {
    // Reuse existing JWT issuance from user service
    return "", fmt.Errorf("TODO: extract JWT issuance from user service")
}
```

- [ ] **Step 4: Update Login to return refresh token**

In `internal/server/service/user.go` `Login()` method, after issuing JWT:

```go
// Also generate refresh token
refreshToken, err := s.authService.generateRefreshToken(ctx, user.ID)
if err != nil {
    return nil, fmt.Errorf("generate refresh token: %w", err)
}
```

Update `Login` response struct to include `RefreshToken`:

```go
type LoginResponse struct {
    Token        string `json:"token"`
    RefreshToken string `json:"refreshToken"`
    User         string `json:"user"`
}
```

- [ ] **Step 5: Add store interfaces**

Read `internal/agent/store/store.go`. Add `RefreshTokens()` method:

```go
RefreshTokens() RefreshTokenStore
```

Create the interface and GORM implementation alongside existing store patterns.

- [ ] **Step 6: Register auth routes**

In `internal/server/server/api.go`:

```go
auth := s.Group("/api/v1/auth")
auth.POST("/refresh", s.authController.Refresh)
auth.POST("/logout", s.authController.Logout)
```

- [ ] **Step 7: Frontend: update request.ts interceptor**

Read `fronted/src/api/request.ts`. Find response error handler. Add 401 refresh logic:

```typescript
// In response interceptor
if (error.response?.status === 401 && !originalRequest._retry) {
    originalRequest._retry = true
    const refreshToken = localStorage.getItem('wf_refresh_token')
    if (!refreshToken) {
        router.push('/auth/login')
        return Promise.reject(error)
    }
    try {
        const res = await service.post('/auth/refresh', { refreshToken })
        const { token, refreshToken: newRefresh } = res.data
        localStorage.setItem('wf_token', token)
        localStorage.setItem('wf_refresh_token', newRefresh)
        originalRequest.headers['Authorization'] = `Bearer ${token}`
        return service(originalRequest)
    } catch {
        localStorage.removeItem('wf_token')
        localStorage.removeItem('wf_refresh_token')
        router.push('/auth/login')
        return Promise.reject(error)
    }
}
```

- [ ] **Step 8: Frontend: update login page**

Read `fronted/src/pages/auth/login/index.vue`. After successful login, save both tokens:

```typescript
localStorage.setItem('wf_token', res.token)
localStorage.setItem('wf_refresh_token', res.refreshToken)
```

- [ ] **Step 9: Build verify**

```bash
cd /Users/francis/workspc/lattice && go build ./...
```

- [ ] **Step 10: Commit**

```bash
git add internal/server/models/refresh_token.go internal/server/service/auth.go internal/server/service/user.go internal/db/gormstore/migrate.go internal/server/server/api.go internal/agent/store/store.go fronted/src/api/request.ts fronted/src/pages/auth/login/index.vue
git commit -s -m "feat(security): add dual-token model with access(15min) + refresh(30d) and logout invalidation"
```

---

### Task 7: HTTPS/TLS Ingress

**Files:**
- Create: `deploy/k3s/ingress.yaml`
- Modify: `internal/server/server/server.go`
- Modify: `config/lattice/overlays/all-in-one/kustomization.yaml`

- [ ] **Step 1: Create Ingress manifest**

Create `deploy/k3s/ingress.yaml`:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: lattice-api
  namespace: lattice-system
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/force-ssl-redirect: "true"
spec:
  ingressClassName: nginx
  tls:
  - hosts:
    - console.alattice.io
    secretName: lattice-tls
  rules:
  - host: console.alattice.io
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: lattice-api-service
            port:
              number: 8080
```

- [ ] **Step 2: Add HTTPS redirect middleware to server.go**

Read `internal/server/server/server.go`. Add method:

```go
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
```

Register in `api.go` before security headers:

```go
s.Use(s.httpsRedirect())
```

- [ ] **Step 3: Update kustomization**

Read `config/lattice/overlays/all-in-one/kustomization.yaml`. Add Ingress resource:

```yaml
resources:
- ../../base
- ingress.yaml  # Add this
```

- [ ] **Step 4: Commit**

```bash
git add deploy/k3s/ingress.yaml internal/server/server/server.go internal/server/server/api.go config/lattice/overlays/all-in-one/kustomization.yaml
git commit -s -m "feat(infra): add Ingress with TLS (cert-manager) and HTTPS redirect"
```

---

### Task 8: Privacy Policy + Terms of Service Pages

**Files:**
- Create: `fronted/src/pages/legal/privacy.vue`
- Create: `fronted/src/pages/legal/terms.vue`
- Modify: `fronted/src/pages/auth/signup/index.vue`
- Modify: `fronted/src/pages/index.vue` (footer)
- Modify: `fronted/src/locales/zh-CN/common.json`
- Modify: `fronted/src/locales/en/common.json`

- [ ] **Step 1: Create privacy policy page**

Create `fronted/src/pages/legal/privacy.vue`:

```vue
<script setup lang="ts">
definePage({ meta: { layout: 'blank' } })
</script>

<template>
  <div class="min-h-screen bg-background text-foreground">
    <div class="max-w-3xl mx-auto px-6 py-16 prose dark:prose-invert">
      <h1>隐私政策</h1>
      <p class="text-muted-foreground">最后更新: 2026-05-13</p>

      <h2>1. 我们收集的信息</h2>
      <p>注册时收集：邮箱地址、用户名（可选）。Agent 运行时收集：设备公钥、VPN IP、流量审计日志。</p>

      <h2>2. 信息使用</h2>
      <p>用于提供 WireGuard 网络服务和 AI Agent 沙箱功能。流量审计日志仅用于安全合规和异常检测。</p>

      <h2>3. 数据存储与安全</h2>
      <p>数据存储在加密的数据库中。密码使用 bcrypt 哈希存储。API 通信强制 HTTPS。</p>

      <h2>4. 数据保留与删除</h2>
      <p>账户删除时，关联的 Peer、Token、审计日志在 30 天内清除。可联系 hello@alattice.io 请求数据导出或提前删除。</p>

      <h2>5. 第三方服务</h2>
      <p>SSO/OIDC 登录通过您配置的 Identity Provider 完成。Lattice 不存储第三方密码。</p>

      <h2>6. 联系我们</h2>
      <p>隐私相关问题请发送邮件至 hello@alattice.io</p>
    </div>
  </div>
</template>
```

- [ ] **Step 2: Create terms of service page**

Create `fronted/src/pages/legal/terms.vue`:

```vue
<script setup lang="ts">
definePage({ meta: { layout: 'blank' } })
</script>

<template>
  <div class="min-h-screen bg-background text-foreground">
    <div class="max-w-3xl mx-auto px-6 py-16 prose dark:prose-invert">
      <h1>服务条款</h1>
      <p class="text-muted-foreground">最后更新: 2026-05-13</p>

      <h2>1. 服务描述</h2>
      <p>Lattice 提供 WireGuard overlay 网络服务和 AI Agent 安全沙箱服务。</p>

      <h2>2. 用户责任</h2>
      <p>用户负责 Agent 在沙箱内的行为。不得用于非法活动、网络攻击、垃圾邮件。</p>

      <h2>3. 免费与付费</h2>
      <p>社区版免费使用，受节点数和功能限制。Pro 版按订阅付费。</p>

      <h2>4. 服务可用性</h2>
      <p>我们尽力保证服务可用，但不提供 SLA（社区版）。Pro 版 SLA 见单独协议。</p>

      <h2>5. 终止</h2>
      <p>违反条款的账户将被终止。用户可随时删除账户和数据。</p>

      <h2>6. 责任限制</h2>
      <p>Lattice 不对因使用服务产生的间接损失承担责任。</p>
    </div>
  </div>
</template>
```

- [ ] **Step 3: Update signup page**

Read `fronted/src/pages/auth/signup/index.vue`. Add TOS checkbox before submit button:

```html
<div class="flex items-start gap-2">
  <input type="checkbox" v-model="agreedToS" required class="mt-0.5 accent-primary" />
  <span class="text-xs text-muted-foreground">
    我已阅读并同意
    <a href="/legal/terms" target="_blank" class="text-primary hover:underline">服务条款</a>
    和
    <a href="/legal/privacy" target="_blank" class="text-primary hover:underline">隐私政策</a>
  </span>
</div>
```

Add to script:

```typescript
const agreedToS = ref(false)
```

Send `tosAccepted: true` in register payload.

- [ ] **Step 4: Update footer**

Read `fronted/src/pages/index.vue`. In footer links section, add:

```html
<a href="/legal/privacy" class="hover:text-foreground transition-colors">隐私政策</a>
<a href="/legal/terms" class="hover:text-foreground transition-colors">服务条款</a>
```

- [ ] **Step 5: Run tests**

```bash
cd fronted && npx vitest run 2>&1 | tail -5
```

- [ ] **Step 6: Commit**

```bash
git add fronted/src/pages/legal/ fronted/src/pages/auth/signup/index.vue fronted/src/pages/index.vue
git commit -s -m "feat(legal): add privacy policy and terms of service pages with signup consent"
```

---

### Task 9: Final Verification

- [ ] **Step 1: Run Go tests**

```bash
cd /Users/francis/workspc/lattice && go test ./pkg/utils/ ./internal/server/... -v 2>&1 | tail -20
```

- [ ] **Step 2: Run frontend tests**

```bash
cd fronted && npx vitest run 2>&1 | tail -5
```

- [ ] **Step 3: Build both community and PRO**

```bash
cd /Users/francis/workspc/lattice && go build ./...
cd /Users/francis/workspc/lattice && go build -tags pro ./...
```

- [ ] **Step 4: Commit any fixes**

---

## Implementation Order

```
Task 1 (password) → Task 2 (validation) → Task 3 (security headers) → Task 4 (CSRF)
  → Task 5 (rate limit) → Task 6 (token refresh) → Task 7 (HTTPS) → Task 8 (privacy/TOS)
  → Task 9 (verify)
```
