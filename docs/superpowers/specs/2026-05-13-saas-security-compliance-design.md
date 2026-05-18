# SaaS 安全合规上线设计

> 状态: 设计阶段 | 目标: 公网 SaaS 部署

## 概述

覆盖 8 项必须在公网上线前完成的安全和合规能力：HTTPS/TLS、隐私政策+服务条款、密码策略、Rate Limiting、输入校验、Token 刷新+登出失效、CSRF 保护、安全 Header。

---

## 1. HTTPS/TLS

### 方案

`latticed` 前面加 Ingress Controller（nginx-ingress 或 traefik）+ cert-manager 自动管理 Let's Encrypt 证书。

```
Internet → Ingress (TLS termination) → latticed Service (HTTP :8080)
```

K8s 配置（cert-manager 已部署）：

```yaml
# ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: lattice-api
  namespace: lattice-system
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
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

### 后端改动

`internal/server/server/server.go` 增加 HTTPS 重定向 middleware：

```go
func (s *Server) httpsRedirectMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        if c.Request.Header.Get("X-Forwarded-Proto") == "http" {
            c.Redirect(301, "https://"+c.Request.Host+c.Request.RequestURI)
            c.Abort()
            return
        }
        c.Next()
    }
}
```

### 文件变更

| 文件 | 变更 |
|------|------|
| `deploy/k3s/ingress.yaml` | 新增 Ingress 配置（TLS + cert-manager） |
| `internal/server/server/server.go` | HTTPS 重定向 middleware |
| `config/lattice/overlays/all-in-one/kustomization.yaml` | 增加 Ingress resource |

---

## 2. 隐私政策 + 服务条款

### 方案

前端新增两个静态路由页面，注册页面增加勾选框。

### 前端文件

| 文件 | 说明 |
|------|------|
| `fronted/src/pages/legal/privacy.vue` | 隐私政策页面 |
| `fronted/src/pages/legal/terms.vue` | 服务条款页面 |

路由: `/legal/privacy`, `/legal/terms`

### 注册页改动

`fronted/src/pages/auth/signup/index.vue` 表单增加：

```html
<label class="flex items-start gap-2 text-sm text-muted-foreground">
  <input type="checkbox" v-model="agreedToS" required class="mt-0.5" />
  <span>
    我已阅读并同意
    <a href="/legal/terms" target="_blank" class="text-primary hover:underline">服务条款</a>
    和
    <a href="/legal/privacy" target="_blank" class="text-primary hover:underline">隐私政策</a>
  </span>
</label>
```

### Footer 增加链接

`fronted/src/pages/index.vue` footer 增加:
- `<a href="/legal/privacy">隐私政策</a>`
- `<a href="/legal/terms">服务条款</a>`

### API 记录同意

`POST /api/v1/users/register` 请求增加 `agreedToS: boolean` 字段，存储到 `users` 表的 `tos_accepted_at` 字段。

---

## 3. 密码策略

### 方案

注册和修改密码时强制校验：

| 规则 | 值 |
|------|-----|
| 最小长度 | 8 字符 |
| 必须含大写 | 至少 1 |
| 必须含小写 | 至少 1 |
| 必须含数字 | 至少 1 |
| 允许特殊字符 | 推荐但不强制 |

### 后端

`pkg/utils/encrypt.go` 新增：

```go
var (
    ErrPasswordTooShort  = errors.New("password must be at least 8 characters")
    ErrPasswordNoUpper   = errors.New("password must contain at least one uppercase letter")
    ErrPasswordNoLower   = errors.New("password must contain at least one lowercase letter")
    ErrPasswordNoDigit   = errors.New("password must contain at least one digit")
)

func ValidatePassword(password string) error {
    if len(password) < 8 {
        return ErrPasswordTooShort
    }
    hasUpper := strings.ContainsAny(password, "ABCDEFGHIJKLMNOPQRSTUVWXYZ")
    hasLower := strings.ContainsAny(password, "abcdefghijklmnopqrstuvwxyz")
    hasDigit := strings.ContainsAny(password, "0123456789")
    if !hasUpper { return ErrPasswordNoUpper }
    if !hasLower { return ErrPasswordNoLower }
    if !hasDigit { return ErrPasswordNoDigit }
    return nil
}
```

### 调用点

- `internal/server/service/user.go` `Register()` 方法开头调用 `ValidatePassword`
- `POST /api/v1/users/change-password` 同样调用

### 前端

注册和修改密码表单加实时密码强度指示器（弱/中/强）。

---

## 4. Rate Limiting

### 方案

Gin middleware，分两层限流：
- **全局**：100 req/min per IP
- **登录接口**：5 req/min per IP
- **敏感接口**（注册、改密）：10 req/min per IP

使用内存 token bucket（`golang.org/x/time/rate`），不引入 Redis。

### 新增文件

`internal/server/server/middleware/rate_limit.go`：

```go
package middleware

import (
    "sync"
    "time"
    "golang.org/x/time/rate"
    "github.com/gin-gonic/gin"
    "github.com/alatticeio/lattice/pkg/utils/resp"
)

type IPRateLimiter struct {
    mu       sync.RWMutex
    limiters map[string]*rateLimiterEntry
}

type rateLimiterEntry struct {
    limiter  *rate.Limiter
    lastSeen time.Time
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

// RateLimit returns a Gin middleware with the given rate and burst.
func (rl *IPRateLimiter) RateLimit(r rate.Limit, burst int) gin.HandlerFunc {
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

### 接入

`internal/server/server/api.go`：

```go
var ipLimiter = middleware.NewIPRateLimiter()

func (s *Server) registerRoutes() {
    // 全局
    s.Use(ipLimiter.RateLimit(100, 200)) // 100 req/s, burst 200

    // 登录接口更严格
    loginGroup := s.Group("/api/v1/users")
    loginGroup.POST("/login", ipLimiter.RateLimit(5.0/60, 5), s.userController.Login)

    // 注册
    loginGroup.POST("/register", ipLimiter.RateLimit(10.0/60, 3), s.userController.Register)
}
```

---

## 5. 输入校验

### 方案

全 API 统一参数校验。Gin 用 `binding` tag + 自定义 validator。

### 改动

所有 `dto` struct 补齐 `binding` tag：

```go
type UserDto struct {
    Username string `json:"username" binding:"required,min=2,max=64"`
    Email    string `json:"email"    binding:"required,email,max=128"`
    Password string `json:"password" binding:"required,min=8,max=128"`
}
```

新增自定义 validator `pkg/utils/validator.go`：
- `safe_string`：无 HTML/JS 注入字符
- `cidr`：合法 CIDR 格式

### 后端统一错误处理

Gin 的 `binding` 错误返回统一格式：

```json
{
  "code": 400,
  "message": "参数校验失败",
  "errors": [
    { "field": "email", "message": "must be a valid email address" }
  ]
}
```

---

## 6. Token 刷新 + 登出失效

跳过此项（用户选的是 7，但列表中没有 6）。

---

## 7. Token 刷新 + 登出失效

### 当前问题

JWT 签发后有效期 30 天，无法中途撤销（除非加入撤销列表，但当前撤销列表功能不完整）。用户登出后，已签发的 JWT 仍可继续使用。

### 方案

**双 token 模型**：

| Token | 有效期 | 用途 |
|-------|--------|------|
| Access Token | 15 分钟 | 调用 API |
| Refresh Token | 30 天 | 刷新 access token |

**流程**：

```
用户登录 → 返回 { accessToken, refreshToken }
accessToken 过期 → POST /api/v1/auth/refresh { refreshToken } → 新 accessToken
用户登出 → refreshToken 加入 DB 撤销列表，accessToken 依靠短 TTL 自然过期
```

### 后端

`internal/server/models/refresh_token.go`：

```go
type RefreshToken struct {
    Model
    UserID    string    `gorm:"index;size:64"`
    TokenHash string    `gorm:"uniqueIndex;size:64"` // SHA256( raw_token )
    ExpiresAt time.Time
    RevokedAt *time.Time
}
```

`POST /api/v1/auth/refresh`：
1. 接收 `refreshToken`
2. SHA256 → 查 `RefreshToken` 表
3. 未过期、未撤销 → 签发新 accessToken → 返回

`POST /api/v1/auth/logout`：
1. 接收 `refreshToken`
2. 标记 `RevokedAt = now`
3. 前端清除本地 token

### 前端

`request.ts` 拦截器：401 时自动调 `/api/v1/auth/refresh`，刷新成功重试原请求；刷新失败跳登录页。

### 兼容

旧的 30 天 JWT 暂保留，`accessToken` 优先。过渡期两个都接受。

---

## 8. CSRF 保护

### 方案

API 使用 `Authorization: Bearer` header，天然不受 CSRF 影响（浏览器不会自动附加自定义 header）。前端 SPA 不会发送 cookie-based auth，所以：

→ **无需额外 CSRF middleware。** 只需确保不使用 cookie 传递 token。

### 确认项

| 检查点 | 结论 |
|--------|------|
| Token 是否通过 `Authorization` header 传递 | ✅ request.ts 已实现 |
| 是否有 cookie-based session | ❌ 没有 |
| 是否使用 `SameSite` cookie | N/A |

### 预防措施

在 `cors.go` 中确保 CORS 跨域白名单：

```go
config.AllowOrigins = []string{"https://console.alattice.io"} // 不可以用 *
```

---

## 9. 安全 Header

### 方案

Gin middleware 统一注入。

`internal/server/server/middleware/security_headers.go`：

```go
func SecurityHeaders() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
        c.Header("X-Content-Type-Options", "nosniff")
        c.Header("X-Frame-Options", "DENY")
        c.Header("X-XSS-Protection", "0") // deprecated but safe to set to 0
        c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
        c.Header("Content-Security-Policy",
            "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; connect-src 'self'")
        c.Next()
    }
}
```

`api.go` 中全局注册：

```go
s.Use(middleware.SecurityHeaders())
```

---

## 文件变更汇总

### 新增

| 文件 | 说明 |
|------|------|
| `internal/server/server/middleware/rate_limit.go` | IP 级别 rate limiter |
| `internal/server/server/middleware/security_headers.go` | 安全 header 注入 |
| `internal/server/models/refresh_token.go` | Refresh Token 模型 |
| `pkg/utils/validator.go` | 自定义参数验证器 |
| `fronted/src/pages/legal/privacy.vue` | 隐私政策页 |
| `fronted/src/pages/legal/terms.vue` | 服务条款页 |
| `deploy/k3s/ingress.yaml` | Ingress + TLS |

### 修改

| 文件 | 变更 |
|------|------|
| `internal/server/server/api.go` | 注册 rate limit + security header middleware |
| `internal/server/server/server.go` | HTTPS redirect middleware |
| `internal/server/service/user.go` | 密码策略校验、token 刷新、登出失效 |
| `pkg/utils/encrypt.go` | `ValidatePassword` 函数 |
| `internal/server/service/auth.go` | Token refresh + logout API |
| `internal/db/gormstore/migrate.go` | refresh_tokens 表迁移 |
| `internal/server/dto/user.go` | DTO binding tag 补齐 |
| `fronted/src/pages/auth/signup/index.vue` | 同意条款勾选框 + 密码强度指示 |
| `fronted/src/pages/index.vue` | Footer 增加隐私/条款链接 |
| `fronted/src/api/request.ts` | 401 自动刷新 token |
| `internal/server/server/middleware/cors.go` | CORS 白名单 |
| `config/lattice/overlays/all-in-one/kustomization.yaml` | Ingress resource |

---

## 实现顺序

1. **密码策略** → `ValidatePassword` + 注册/改密集成
2. **输入校验** → DTO binding tag 补齐
3. **安全 Header** → middleware 一行注册
4. **CSRF** → CORS 白名单收紧（确认即可）
5. **Rate Limiting** → middleware + 接口分级
6. **Token 刷新 + 登出** → 双 token 模型 + 前端拦截器
7. **HTTPS/TLS** → Ingress + cert-manager
8. **隐私政策/条款** → 静态页面 + 注册勾选

---

*最后更新: 2026-05-13*
