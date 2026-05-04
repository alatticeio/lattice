# Platform Settings — NATS URL 配置功能设计

## 概述

为 Lattice 管理 UI 增加平台级设置页面，使管理员可以通过 UI 配置 NATS 信令服务器地址，
替代仅能通过配置文件/环境变量修改的方式。采用 key-value 架构以支持后续其他平台配置项的扩展。

## 动机

生产环境中 NATS 通常作为独立集群部署，与 `latticed` 分离。当前 NATS URL 只能通过
`lattice.yaml` 或 `LATTICE_SIGNALING_URL` 环境变量配置，管理员无法在 UI 上查看或修改。
Agent 通过 `/api/v1/discovery` 获取 NATS 地址，因此服务端需要一个可运行时修改的配置入口。

## 后端设计

### 1. 数据模型

新增 `SystemConfig` 模型，采用 key-value 结构：

```go
// internal/server/models/system_config.go
type SystemConfig struct {
    Key        string         `gorm:"primaryKey;type:varchar(128);not null" json:"key"`
    Value      string         `gorm:"type:text" json:"value"`
    CreatedAt  time.Time      `json:"created_at"`
    UpdatedAt  time.Time      `json:"updated_at"`
}

func (SystemConfig) TableName() string { return "la_system_config" }
```

预定义的 key 常量：

```go
const (
    ConfigKeyNatsURL = "nats_url"
)
```

### 2. Store 接口

新增 `SystemConfigRepository`：

```go
// internal/agent/store/store.go
type SystemConfigRepository interface {
    Get(ctx context.Context, key string) (string, error)
    Set(ctx context.Context, key string, value string) error
    SetMulti(ctx context.Context, kv map[string]string) error
    GetAll(ctx context.Context) (map[string]string, error)
}
```

在 `Store` 接口中新增 `SystemConfig() SystemConfigRepository` 方法，
在 `GormStore` 中实现，并在 `migrate()` 中注册 `&models.SystemConfig{}`。

### 3. API 路由

新增平台设置 API，**需要 platform_admin 权限**：

```
GET  /api/v1/settings/platform  → 获取所有平台设置
PUT  /api/v1/settings/platform  → 更新平台设置
```

**GET 响应示例：**

```json
{
  "code": 200,
  "data": {
    "nats_url": "nats://nats-cluster:4222"
  }
}
```

**PUT 请求体示例：**

```json
{
  "nats_url": "nats://nats-cluster:4222"
}
```

### 4. Discovery 集成

修改 `handleDiscovery()` 使其优先读取 DB 中的 NATS URL，
未设置时回退到 `cfg.SignalingURL`：

```go
func (s *Server) handleDiscovery() gin.HandlerFunc {
    return func(c *gin.Context) {
        natsURL := s.cfg.SignalingURL
        if dbURL, err := s.store.SystemConfig().Get(c.Request.Context(), models.ConfigKeyNatsURL); err == nil && dbURL != "" {
            natsURL = dbURL
        }
        if natsURL == "" {
            natsURL = "nats://127.0.0.1:4222"
        }
        resp.OK(c, gin.H{"nats_url": natsURL})
    }
}
```

### 5. 文件清单（后端）

| 文件 | 操作 |
|------|------|
| `internal/server/models/system_config.go` | 新建 — Model 定义 |
| `internal/agent/store/store.go` | 修改 — 新增 `SystemConfigRepository` 接口和 `Store.SystemConfig()` |
| `internal/db/gormstore/system_config.go` | 新建 — GORM 实现 |
| `internal/db/gormstore/store.go` | 修改 — 注册 `SystemConfigRepository` |
| `internal/db/gormstore/migrate.go` | 修改 — 注册 AutoMigrate |
| `internal/server/dto/platform.go` | 新建 — 请求/响应 DTO |
| `internal/server/controller/platform.go` | 新建 — Controller |
| `internal/server/service/platform.go` | 新建 — Service 层 |
| `internal/server/server/platform.go` | 新建 — API 路由注册 |
| `internal/server/server/api.go` | 修改 — 调用 `s.platformRouter()` |

## 前端设计

### 1. 页面

新增 `fronted/src/pages/settings/platform/index.vue`：

- 标题："Platform Settings" / "平台设置"
- 表单区域：
  - **NATS URL** 输入框，占满宽度
  - 前缀校验 `nats://` 或 `nats+tls://`
  - 保存按钮（带加载状态）
  - 成功/失败 toast 提示
- 布局风格与现有 settings 页面一致（`settings/relays/`, `settings/audit/`）

### 2. 侧边栏导航

在 Platform Admin 分组中新增"Platform Settings"入口（仅 platform_admin 可见）:

```typescript
// fronted/src/components/app-sidebar/AppSidebar.vue
{
  title: t('common.nav.group.platform'),
  url: "#",
  icon: ShieldCheck,
  items: [
    { title: t('common.nav.platformSettings'), url: "/settings/platform" },
    // ... existing items
  ],
}
```

各页面通过 `definePage` 的 `meta.titleKey` 实现 i18n 标题。

### 3. API 层

新增 `fronted/src/api/platform.ts`：

```typescript
export const getPlatformSettings = () => request.get('/settings/platform')
export const updatePlatformSettings = (data: { nats_url: string }) => request.put('/settings/platform', data)
```

### 4. 文件清单（前端）

| 文件 | 操作 |
|------|------|
| `fronted/src/pages/settings/platform/index.vue` | 新建 — 平台设置页面 |
| `fronted/src/api/platform.ts` | 新建 — API 调用 |
| `fronted/src/locales/*/settings.json` | 修改 — 新增 i18n 文案 |

## 边界情况处理

| 场景 | 行为 |
|------|------|
| NATS URL 为空字符串 | 保存时拒绝，提示"URL 不能为空" |
| URL 格式不合法（无 `nats://` 前缀） | 前端校验 + 后端校验，提示格式要求 |
| DB 查询失败 | discovery 静默回退到 `cfg.SignalingURL`，不阻塞 agent 连接 |
| platform_admin 以外的角色访问 | 返回 403 Forbidden |
| PUT 空 body | 不做任何更新，返回当前值 |
| 并发写入 | key-value 行级锁由数据库保证，无并发问题 |

## 未来扩展

key-value 架构天然支持新增配置项，后续只需：

1. 在 `models` 中定义新 key 常量
2. 在 DTO 中添加字段
3. 前端表单添加对应输入项

无需修改表结构、Store 接口或 API 路由。

## 实施顺序

1. 后端 Model + Store 层（含 migration）
2. 后端 Service + Controller 层
3. 后端 API 路由 + Discovery 集成
4. 前端 API 层
5. 前端页面
6. 端到端验证
