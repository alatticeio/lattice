# Token 体系增强设计

> 状态: 设计阶段 | 关联: `2026-05-13-infrastructure-e2e-testing-design.md`

## 概述

在现有 Join Token（标准 Agent 入网凭证）和 Enrollment Token（Sandbox Agent 入网凭证）基础上，增强三个方面：
1. **扫码/一键入网**：`lattice://` URL Scheme + QR 码，设备扫描即可自动配置并接入
2. **Token 生命周期管理**：补齐 TTL 标准化、使用跟踪、过期清理、撤销机制、审计日志
3. **用户自定义 Tool**：用户可注册自己的 MCP tool 到平台，创建 token 时从平台内置 + 用户自定义 tool 池中勾选

不改变两套 token 的存储模型（Join Token 走 K8s CRD，Enrollment Token 走 SQL）。

### SSO 概要

当前 SSO 基于 Dex OIDC（`internal/server/dex/`），支持 OIDC 登录。用户身份通过 Dex ID Token 验证，JWT 签发绑定 OIDC subject。邀请系统（`InvitationService`）通过 HMAC-signed 链接邀请用户加入 workspace。

SSO 与 token 融合的三大场景：

| 场景 | 描述 |
|------|------|
| **SSO 用户创建 token** | SSO 登录的管理员创建 agent token，token 记录 `CreatedBy`（SSO user ID） |
| **SSO + 邀请入网** | 邀请链接包含 SSO 上下文 → 用户 SSO 登录 → 自动创建 agent token → 一键入网 |
| **SSO + 扫码入网** | QR 码指向 SSO 登录页 → 浏览器 SSO 认证 → 回调生成专属 token → CLI 自动配置 |
| **`lattice login` 双重 token** | CLI login 一次认证，同时返回管理 session JWT + agent join token，agent 立即可入网 |

---

## 一、扫码入网

### 1.1 URL Scheme

```
lattice://join?token=<token>&server=<server_url>&name=<name>
```

| 参数 | 必填 | 说明 |
|------|------|------|
| `token` | 是 | Join token 或 Enrollment token（长字符串） |
| `server` | 是 | 控制面 API URL |
| `name` | 否 | 提示的 peer name，用户可覆盖 |

示例：

```
lattice://join?token=a1b2c3d4e5f6...&server=https://console.alattice.io&name=my-agent
```

### 1.2 QR 码生成

前端 token 管理页面，每个有效 token 旁增加"QR 码"按钮。点击后弹出 Dialog 显示：

- QR 码（使用 `qrcode` 库在前端生成，canvas 渲染）
- 对应的 `lattice://join?...` 链接文本，支持一键复制
- 有效期提示

QR 码尺寸约 300×300 px，足够容纳一个 500 字符的 URL（`lattice://` scheme + 64 字符 hex token + server URL）。

### 1.3 CLI `lattice join` 命令

新增 `cmd/lattice/cmd/join.go`：

```
$ lattice join "lattice://join?token=a1b2c3&server=https://console.alattice.io&name=my-agent"

  → 解析 URL
  → 写入 lattice.yaml:
      server_url: https://console.alattice.io
      auth_token: a1b2c3  (或 agent.enrollment_token: a1b2c3)
      agent.name: my-agent
  → lattice up
```

**实现**：

```go
// cmd/lattice/cmd/join.go
type JoinURL struct {
    Token  string
    Server string
    Name   string
}

func parseJoinURL(raw string) (*JoinURL, error) {
    u, err := url.Parse(raw)
    if err != nil || u.Scheme != "lattice" || u.Host != "join" {
        return nil, fmt.Errorf("invalid join URL: expected lattice://join?token=...&server=...")
    }
    q := u.Query()
    return &JoinURL{
        Token:  q.Get("token"),
        Server: q.Get("server"),
        Name:   q.Get("name"),
    }, nil
}
```

`lattice join` 支持两种入参：
1. **完整 URL**：`lattice join "lattice://join?token=...&server=..."`
2. **短码**（远期）：`lattice join <short-code>` → CLI 调 `GET /api/v1/token/resolve?code=<short-code>` 获取完整信息

### 1.4 前端 QR 码展示

**位置**：`/manage/tokens` 页面，Token 列表的 Actions 列增加"QR"按钮。

**交互**：
1. 点击 QR 按钮 → 弹出 `Dialog`
2. Dialog 内容：
   - QR 码 canvas（`qrcode` npm 包）
   - 复制按钮（复制 `lattice://join?...` URL）
   - Download 按钮（下载 QR 码 PNG）
   - 提示文字："用 Lattice CLI 扫描或复制链接到终端执行 `lattice join <url>`"

**依赖**：新增 `qrcode` npm 依赖（`pnpm add qrcode`）。

---

## 二、Token 生命周期增强

### 2.1 TTL 标准化

**当前问题**：Join token 的 `Expiry` 用字符串 `"168h"` 存储；Enrollment token 用 `time.Duration`。

**改进**：两者统一使用 Go `time.Duration` 的秒数（int64），前端统一展示。

Join token CRD 的 `Expiry` 字段保持不变（`metav1.Time`），改为通过 `TokenDto.Expiry` 传入秒数而非字符串：

```go
// dto/token.go
type TokenDto struct {
    Namespace  string `json:"namespace"`
    Name       string `json:"name"`
    ExpirySecs int64  `json:"expirySecs"` // TTL in seconds, default 604800 (7d)
    Limit      int    `json:"limit"`
}
```

前端创建表单中 TTL 选择：`1h` / `6h` / `24h` / `7d` / `30d` / `永久`。

### 2.2 使用跟踪

前端 token 列表增加"已用/限额"列，展示 `UsedCount / UsageLimit`：

```
Token          | 命名空间  | 已用/限额 | 过期时间           | 状态
a1b2c3...      | lattice-system | 3/5     | 2026-05-20 14:00 | 有效
```

对于 Enrollment token（一次性），展示 `✓ 已使用` 或 `— 未使用`。

### 2.3 过期自动清理

- **Join token (CRD)**：已有 `TokenReconciler` 标记 `Phase=Expired`。过期 token 保留 7 天后自动删除（增加 `spec.expiry + 7d` 的 cleanup 逻辑）。
- **Enrollment token (SQL)**：增加 `DeleteExpired` 定期清理 goroutine（每 1h 运行一次）。

### 2.4 撤销机制

**Join token**：当前通过 CRD Delete 实现"撤销"。增强为 soft-delete：

```go
// LatticeEnrollmentTokenStatus 增加字段
type LatticeEnrollmentTokenStatus struct {
    // ... existing ...
    RevokedAt *metav1.Time `json:"revokedAt,omitempty"`
    RevokedBy string       `json:"revokedBy,omitempty"`
}
```

撤销后 token 仍可查询（用于审计），但新 agent 无法使用（注册时检查 `RevokedAt != nil`）。

**Enrollment token (SQL)**：`la_agent_enrollment_tokens` 表增加 `revoked_at` 和 `revoked_by` 字段。撤销 API 不再物理删除，改为标记。

### 2.5 审计日志

Token 生命周期事件写入 `t_audit_log`（复用现有表 + `action` 字段）：

| 事件 | action | resource | detail |
|------|--------|----------|--------|
| 创建 token | `token.create` | `token:<name>` | `{expiry, limit, namespace}` |
| 使用 token | `token.use` | `token:<name>` | `{peer_name, peer_ip}` |
| 过期 token | `token.expire` | `token:<name>` | `{used_count, usage_limit}` |
| 撤销 token | `token.revoke` | `token:<name>` | `{revoked_by}` |

---

## 三、SSO 与 Token 融合

### 3.1 场景一：SSO 用户创建 Token

SSO 登录 → JWT → 创建 token（记录 `CreatedBy` = SSO user ID）。

**当前状态**：Token 创建已记录 `CreatedBy`（Enrollment token 的 `models.AgentEnrollmentToken.CreatedBy`）。Join token 通过 K8s CRD 创建，无 `CreatedBy` 字段。

**增强**：
- Join token CRD `LatticeEnrollmentTokenSpec` 增加 `CreatedBy` 和 `CreatedByEmail` 字段
- 前端 token 列表展示创建者信息（SSO identity）
- 审计日志关联 SSO user

### 3.2 场景二：SSO + 邀请入网

用户 A（SSO 登录）邀请用户 B 加入 workspace。用户 B 通过 SSO 登录后自动获得 agent token。

**流程**：

```
1. 用户 A 在 Console → Members → Invite
   输入用户 B 的 email + 选择 role
   → POST /api/v1/invitations/create

2. 用户 B 收到邀请链接（邮件 / 链接复制）
   → https://console.alattice.io/invite/<hmac_token>

3. 用户 B 打开链接 → SSO 登录（Dex OIDC）
   → 验证 OIDC identity → 寻找或创建 Lattice 用户
   → Accept invitation（加入 workspace member）
   → 自动生成用户的 agent token
   → 返回 token + lattice://join URL + QR 码

4. 用户 B 在终端执行
   → lattice join "lattice://join?token=...&server=..."
   → 自动配置入网
```

**关键设计**：

- 邀请链接 `https://console.alattice.io/invite/<token>` 在 SSO 回调后，若邀请仍有效，自动生成一个绑定到该用户 SSO identity 的 agent token
- 一次邀请 → 一个专属 token → 一次使用（one-time）
- Token 的 `CreatedBy` 记录为 SSO identity，`UsageLimit=1`
- 前端展示："用户 B 已接受邀请，agent token 已生成"

**新增 API**：

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/invitations/accept-and-provision` | SSO 登录后接受邀请 + 自动生成 agent token |

**请求**：
```json
{
  "invitationToken": "<hmac_token>",
  "agentName": "laptop-b"
}
```

**响应**：
```json
{
  "code": 200,
  "data": {
    "workspaceId": "ws-xxx",
    "agentToken": "a1b2c3...",
    "joinURL": "lattice://join?token=a1b2c3...&server=https://console.alattice.io&name=laptop-b",
    "qrURL": "..."
  }
}
```

### 3.3 场景三：SSO + 扫码入网

最简入网路径：用户扫 QR 码 → 浏览器 SSO 认证 → 自动生成 token → CLI 完成入网。

**流程**：

```
1. 管理员生成 workspace 入网 QR 码
   → QR 码指向：https://console.alattice.io/join?workspace=<ws_id>

2. 新用户扫描 QR 码（手机浏览器）
   → 打开 join 页面
   → 若未登录 → SSO 登录（Dex OIDC）
   → 已登录 → 直接进入 agent 配置页

3. 页面展示：
   ┌─────────────────────────────────────┐
   │  加入 Lattice Network               │
   │                                     │
   │  Workspace: my-workspace            │
   │  身份:     user@example.com (SSO)   │
   │  Agent 名: [laptop-b      ]         │
   │                                     │
   │  请在终端执行：                      │
   │  $ lattice join "lattice://join?    │
   │    token=xyz&server=..."            │
   │                                     │
   │  [ 复制命令 ]  [ 显示 QR ]          │
   └─────────────────────────────────────┘

4. 用户复制命令 → 终端执行 → 入网完成
```

**关键设计**：
- Workspace 级别的"加入 QR 码"（非个人 token，而是触发 SSO 流程的入口）
- SSO 认证后动态生成个人专属 agent token
- Token 绑定 SSO identity — 每个用户拿到自己的 token
- 页面展示 `lattice://join` URL 和一个终端可直接运行的命令

**Workspace 加入 QR 码**：

```
https://console.alattice.io/join?workspace=<ws_id>
```

与 agent token QR 码（`lattice://join?token=...`）不同：
- **Workspace QR 码**：浏览器 HTTPS URL，触发 SSO 流程，动态生成 token
- **Token QR 码**：`lattice://` URL scheme，CLI 直接消费

### 3.4 SSO 相关 Token 审计

Token 审计日志关联 SSO identity：

| 事件 | action | 审计字段 |
|------|--------|----------|
| SSO 用户创建 token | `token.create` | `{created_by, created_by_email, workspace_id, ttl, usage_limit}` |
| SSO 邀请接受 + 自动 provision token | `token.provision` | `{invitation_id, sso_subject, sso_email, workspace_id, agent_name}` |
| Token 被 agent 使用 | `token.use` | `{token_id, peer_name, peer_ip, agent_sandbox_mode}` |
| Token 被 SSO 用户撤销 | `token.revoke` | `{token_id, revoked_by, revoked_by_email}` |

### 3.5 前端邀请流增强

当前邀请页：`fronted/src/pages/invite/[token].vue`

**增强**：
- 邀请链接打开 → 检查 SSO 登录状态
  - 未登录 → 跳转 SSO 登录（Dex），回调回邀请页
  - 已登录 → 展示 workspace 信息和 agent 配置
- 接受邀请后 → 调用 `POST /api/v1/invitations/accept-and-provision`
- 展示生成的 `lattice://join` URL + QR 码 + 复制按钮
- 可选：显示 CLI 安装指引（`curl -sSL https://get.alattice.io | bash`）

### 3.6 场景四：`lattice login` 双重 Token

让 `lattice login` 一次认证同时返回两个 token：

- **管理 session JWT**：30 天有效期，用于管理操作（`token`、`policy`、`peer` 等命令）
- **Agent join token**：设备入网凭证，绑定当前用户 SSO identity，默认 7 天有效期

**用户视角**：

```bash
$ lattice init
  → Server URL: https://console.alattice.io

$ lattice login
  → Username: alice@example.com
  → Password: ********
  → Logged in as alice. Session saved (30 days).
  → Agent token auto-provisioned (expires in 7 days).

$ lattice up
  → Using saved agent token...
  → Registering peer "alice-laptop"...
  → WireGuard tunnel established ✓
  → Peer online at 10.100.0.5
```

**后端流程**：

```
POST /api/v1/users/login  { username, password }

1. 验证 credentials（本地或 Dex SSO）
2. 签发管理 session JWT（30 天）
3. 查找或创建该用户的 agent token：
   a. 查 DB: 是否有该 user 名下的未过期、未用完 agent token？
      → 有 → 复用，返回已有的
      → 无 → 生成新 join token（UsageLimit=1, TTL=7d, CreatedBy=<user_id>）
4. 返回: { token: <JWT>, agentToken: <join_token>, agentTokenExpiry: <time> }
```

**`lattice login` CLI 改动**：

```go
// 当前：只保存 auth-token
cfgManager.Viper().Set("auth-token", result.Data.Token)

// 增强后：同时保存 agent token
cfgManager.Viper().Set("auth-token", result.Data.Token)
if result.Data.AgentToken != "" {
    cfgManager.Viper().Set("join-token", result.Data.AgentToken)
    cfgManager.Viper().Set("join-token-expiry", result.Data.AgentTokenExpiry)
    fmt.Printf("Agent token auto-provisioned (expires in %s).\n",
        time.Until(result.Data.AgentTokenExpiry).Truncate(time.Minute))
}
```

**`lattice up` 改动**：

```go
// 当前：必须 --token 或 lattice.yaml 中 auth-token
// 增强后：优先读 join-token，没有则读 auth-token 兼容旧行为
func resolveAgentToken(cfg *config.Config) string {
    if cfg.JoinToken != "" && !isExpired(cfg.JoinTokenExpiry) {
        return cfg.JoinToken
    }
    // fallback: 旧版 auth-token 不自动入网
    return ""
}
```

`lattice up` 无参数时，自动使用 saved agent token。若 token 已过期，提示用户 `lattice login` 重新获取。

**配置模型 `lattice.yaml` 新增字段**：

```yaml
# 管理 session（已有）
auth_token: eyJhbG...       # 30 天管理 JWT

# Agent 入网（新增）
join_token: wf_live_zxY...  # 7 天 agent join token
join_token_expiry: 2026-05-20T14:00:00Z
```

**Token 复用逻辑**：

| 情况 | 行为 |
|------|------|
| 首次登录 | 自动生成新 join token（UsageLimit=1, TTL=7d） |
| 再次登录，token 未使用未过期 | 复用已有 token（不创建新的） |
| 再次登录，token 已过期 | 生成新 token |
| 再次登录，token 已用 | 若用户已是 active peer → 不再自动生成；若 peer 不存在 → 生成新 token |

---

## 四、用户自定义 Tool 注册

### 4.1 概述

Token 的 `AllowedTools` 不应仅限于 Lattice 平台内置的 MCP tool。用户可注册自己的外部 MCP server，在创建 token 时从**平台内置 + 用户注册**的 tool 池中勾选。

注册模式：**MCP Server 代理**——用户为自己的外部 MCP server 在 Lattice 注册一个名称，Agent 调用该 tool 时 Lattice 代理转发到该 MCP server，同时做 RBAC 权限管控。

### 4.2 数据模型

```go
// UserMCPTool 表示用户注册的外部 MCP server tool
type UserMCPTool struct {
    Model
    Name         string `gorm:"uniqueIndex:idx_user_mcp_tool;size:128"` // tool 在 Lattice 中的名称
    Description  string `gorm:"size:512"`
    OwnerID      string `gorm:"index;size:64"`
    WorkspaceID  string `gorm:"index;size:64"`
    Visibility   string `gorm:"size:16;default:'workspace'"` // private / workspace / public
    MCPServerURL string `gorm:"size:512"`                    // 外部 MCP server 地址
}
```

外部 MCP server 自己提供 tool 列表和参数 schema（遵循 MCP 协议），Lattice 不重复存储。

### 4.3 API

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/tools/user/register` | 注册外部 MCP server |
| GET | `/api/v1/tools/all?workspace=` | 列出所有可用 tool（平台内置 + 用户注册） |
| DELETE | `/api/v1/tools/user/:name` | 删除注册的 MCP server |

**注册请求**：

```json
{
  "name": "query-db",
  "description": "Production database query tools",
  "workspaceId": "ws-xxx",
  "visibility": "workspace",
  "mcpServerURL": "http://my-mcp-server:8080"
}
```

**列出所有 tool（前端 token 页展示）**：

```
GET /api/v1/tools/all?workspace=ws-xxx

{
  "tools": [
    { "name": "list_peers",         "source": "builtin" },
    { "name": "check_connectivity", "source": "builtin" },
    { "name": "query-db",           "source": "user", "mcpServerURL": "http://..." },
    { "name": "slack-bot",          "source": "user", "mcpServerURL": "http://..." }
  ]
}
```

`source: "builtin"` 是 Lattice 平台内置 tool。`source: "user"` 是用户注册的外部 MCP server。

### 4.4 执行流程

```
Agent → POST /api/v1/agents/tools/call { tool: "query-db", input: {...} }
  → AgentAuth middleware 验证 Agent JWT + AllowedTools
  → tool 是 "user" → 查 UserMCPTool → 获取 mcpServerURL
  → Lattice HTTP POST 转发到 http://my-mcp-server:8080/tools/call
  → MCP server 执行 → 结果返回 → Lattice 透传给 Agent
```

### 4.5 前端 Token 创建页改造

```typescript
// 当前：硬编码
const toolOptions = [
  { value: 'list_peers', label: 'list_peers' },
  ...
]

// 改进：动态获取
const { data } = await listAllTools(workspaceId)
const toolOptions = data.tools.map(t => ({
  value: t.name,
  label: t.name,
  description: t.description,
  source: t.source,
}))
```

UI 分组展示：**平台内置**（`source: builtin`，不可管理）+ **外部 MCP Server**（`source: user`，可注册/删除）。

### 4.6 外部 MCP Server 管理页面 `/manage/mcp-servers`

独立页面用于注册和管理用户自己的外部 MCP server。

**页面结构**：

```
┌──────────────────────────────────────────────────┐
│  外部 MCP Server 管理                             │
│                                                  │
│  ┌─ 注册新 MCP Server ──────────────────────────┐│
│  │  名称: [query-db            ]                ││
│  │  描述: [Production DB tools ]                ││
│  │  URL:  [http://my-server:8080]               ││
│  │  可见性: ○ workspace  ○ private  ○ public    ││
│  │  [ 注册 ]                                    ││
│  └──────────────────────────────────────────────┘│
│                                                  │
│  ┌─ 已注册的 MCP Server ────────────────────────┐│
│  │  名称       │ URL                   │ 操作   ││
│  │  query-db   │ http://my-server:8080 │ 删除   ││
│  │  slack-bot  │ http://slack:9090     │ 删除   ││
│  └──────────────────────────────────────────────┘│
└──────────────────────────────────────────────────┘
```

**路由**：`/manage/mcp-servers`

**侧边栏**：在 AI Assistant 分组下新增「MCP Server」入口。

---

## 五、API 变更（全部）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/token/list` | Token 列表（聚合 join + enrollment tokens） |
| GET | `/api/v1/token/list?workspace=` | 按 workspace 过滤 |

无需新增 `resolve` API（方案 A 下 token 直接编入 URL，无需服务端解析）。

### 4.2 修改 API

| 方法 | 路径 | 变更 |
|------|------|------|
| POST | `/api/v1/users/login` | 登录响应增加 `agentToken` + `agentTokenExpiry` 字段 |
| POST | `/api/v1/token/generate` | `Expiry` 改为 `expirySecs` int64 |
| DELETE | `/api/v1/token/:token` | 改为软删除（标记 revoked 而非物理删除） |
| POST | `/api/v1/agent-isolation/enrollment-tokens` | 响应返回完整 token 信息（含 QR 码参数） |

### 3.3 Token 列表聚合

`GET /api/v1/token/list` 返回统一结构：

```json
{
  "code": 200,
  "data": {
    "tokens": [
      {
        "type": "join",
        "token": "a1b2c3...",
        "namespace": "lattice-system",
        "usageLimit": 5,
        "usedCount": 3,
        "expiresAt": "2026-05-20T14:00:00Z",
        "status": "active",
        "qrURL": "lattice://join?token=a1b2c3...&server=https://console.alattice.io"
      },
      {
        "type": "enrollment",
        "token": "8f3a...",
        "namespace": "lattice-system",
        "usageLimit": 1,
        "usedCount": 1,
        "expiresAt": "2026-05-13T15:00:00Z",
        "status": "used",
        "allowedTools": ["list_peers", "check_connectivity"]
      }
    ]
  }
}
```

---

## 六、CLI 变更

### 4.1 新增 `lattice join` 命令

```
lattice join <url>
```

- 解析 `lattice://join?token=...&server=...&name=...`
- 写入 `~/.lattice/lattice.yaml`
- 自动执行 `lattice up`

### 4.2 修改 `lattice token create` 命令

```bash
# 当前
lattice token create dev-team --expiry 168h -n lattice-system

# 增强后
lattice token create dev-team --ttl 7d -n lattice-system --qr
```

- `--ttl`: 支持 `1h`, `6h`, `1d`, `7d`, `30d`, `permanent`
- `--qr`: 输出 token 的同时打印 QR 码（终端 ASCII QR 或直接输出 lattice:// URL）

---

## 七、前端变更

### 5.1 Token 列表页 (`/manage/tokens`)

**新增列**：
- "已用/限额"列
- "类型"列（Join / Enrollment）
- "QR"操作按钮

**增强筛选**：
- 状态 filter：全部 / 有效 / 已过期 / 已撤销 / 已用完
- 类型 filter：全部 / Join / Enrollment

### 5.2 创建 Token 对话框

**TTL 选择**：
- 预设按钮：1h, 6h, 24h, 7d, 30d, 永久
- 改为发送 `expirySecs` 而非字符串

**创建成功后**：
- 原"复制 token"按钮
- 新增"查看 QR 码"按钮
- 新增"复制 lattice:// URL"按钮

### 5.3 QR 码展示组件

新增 `fronted/src/components/TokenQRCode.vue`：

```vue
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import QRCode from 'qrcode'

const props = defineProps<{ joinURL: string }>()
const canvasRef = ref<HTMLCanvasElement>()

onMounted(async () => {
  if (canvasRef.value) {
    await QRCode.toCanvas(canvasRef.value, props.joinURL, {
      width: 300,
      margin: 2,
      color: { dark: '#1e1b4b', light: '#ffffff' },
    })
  }
})
</script>

<template>
  <canvas ref="canvasRef" />
</template>
```

---

## 八、安全性

| 关注点 | 措施 |
|--------|------|
| Token 在 QR 码中明文传输 | QR 码本身不存储到服务端，仅在前端生成。`lattice://` URL 是本地 scheme，不会被浏览器缓存 |
| Token 在 URL 中可被截获 | `lattice://join` 是本地 CLI 命令，不经过网络传输。用户在终端执行，token 写入本地文件 |
| 扫码后 token 残留 | CLI `lattice join` 执行后，从 shell history 清除（`set +o history`） |
| CRD 明文 token | 本次不改（用户确认先不改存储模型）。后续可考虑 `sealedSecret` 或 bcrypt hash |
| QR 码劫持 | 前端页面需 HTTPS，QR 码弹窗仅在已认证 session 中展示 |
| Brute force 枚举 | Join token 16 字符 alphanumeric（~3×10^28 搜索空间），Enrollment token 64 字符 hex（~10^77 搜索空间）。即使 1M QPS 尝试仍需数十年。足够抵御。 |

---

## 九、文件变更汇总

### 新增

| 文件 | 说明 |
|------|------|
| `cmd/lattice/cmd/join.go` | `lattice join` CLI 命令 |
| `fronted/src/components/TokenQRCode.vue` | QR 码展示组件 |
| `fronted/src/pages/join/index.vue` | 扫码/邀请后的 join 页面（含 SSO 回调） |

### 修改

| 文件 | 变更 |
|------|------|
| `api/v1alpha1/lattice_token.go` | `LatticeEnrollmentTokenStatus` 增加 `RevokedAt`/`RevokedBy`；`LatticeEnrollmentTokenSpec` 增加 `CreatedBy`/`CreatedByEmail` |
| `internal/agent/controller/token_controller.go` | 增加过期后 7 天自动清理逻辑、revoke 检查 |
| `internal/server/service/token.go` | `Expiry` → `expirySecs`，软删除替代物理删除 |
| `internal/server/service/agent_registration.go` | 注册时检查 token `RevokedAt` |
| `internal/server/service/user.go` | 登录方法增加 agent token 自动 provisioning：查找或创建 join token |
| `internal/server/service/invitation.go` | 新增 `AcceptAndProvision`：接受邀请 + 自动生成 agent token |
| `internal/server/models/agent_enrollment.go` | 增加 `RevokedAt`/`RevokedBy` |
| `internal/server/models/workspace.go` | `WorkspaceInvitation` 增加 `ProvisionedToken` 字段 |
| `internal/db/gormstore/agent_enrollment.go` | 增加 `MarkRevoked`、定时清理 goroutine |
| `internal/db/gormstore/migrate.go` | 迁移新字段 |
| `internal/server/service/audit.go` | Token + invitation 审计事件写入 |
| `internal/server/dex/login.go` | SSO 回调支持 `redirect_to` 参数（join 页面） |
| `cmd/lattice/cmd/token/token.go` | `--ttl` 替换 `--expiry` |
| `cmd/lattice/cmd/login.go` | 登录成功后保存 `join-token` + `join-token-expiry` 到 lattice.yaml |
| `cmd/lattice/cmd/up.go` | `lattice up` 无参数时自动使用 saved agent token |
| `internal/agent/config/config.go` | `Config` 结构体增加 `JoinToken` / `JoinTokenExpiry` 字段 |
| `internal/agent/config/validate.go` | 校验 agent token 过期状态 |
| `fronted/src/pages/manage/tokens/index.vue` | QR 按钮、新列、新筛选 |
| `fronted/src/pages/invite/[token].vue` | 增强接受流程：展示 agent 配置 + `lattice://join` URL + QR 码 |
| `fronted/src/pages/manage/tokens/index.vue` | Tool 选择器改为动态获取（平台内置 + 用户自定义分组） |
| `fronted/package.json` | 增加 `qrcode` 依赖 |

### User MCP Tool 新增

| 文件 | 说明 |
|------|------|
| `internal/server/models/user_mcp_tool.go` | `UserMCPTool` 数据模型 |
| `internal/server/service/user_mcp_tool.go` | 外部 MCP server 注册、列表、删除服务 |
| `internal/server/controller/user_mcp_tool.go` | MCP tool 管理 API controller |
| `internal/server/server/user_mcp_tool_router.go` | 路由注册 |
| `internal/db/gormstore/user_mcp_tool.go` | CRUD repository |
| `internal/server/service/agent_tool_proxy.go` | Agent tool 调用代理转发（查 DB → POST 到外部 MCP server） |
| `internal/server/service/agent_registration.go` | `allowedTools` 合并平台内置 + 用户注册 tool |
| `fronted/src/components/TokenToolPicker.vue` | Token 创建页 tool 选择组件（分组：内置/外部 MCP） |
| `fronted/src/pages/manage/mcp-servers/index.vue` | 外部 MCP Server 管理页面（注册/列表/删除） |
| `fronted/src/api/mcp-server.ts` | MCP Server 管理 API client |
| `fronted/src/stores/useMcpServerStore.ts` | MCP Server 状态管理 |

---

## 十、实现顺序

1. **CLI `lattice join` 命令**：URL 解析 + lattice.yaml 写入 + 自动 up
2. **用户自定义 Tool**：UserTool 模型 + 注册 API + `GET /api/v1/tools/all` 聚合列表 → 前端 tool 选择器改造
3. **SSO 邀请入网**：`AcceptAndProvision` API → 邀请页增强 → join 页面
4. **SSO 扫码入网**：workspace join QR → SSO 回调 → 动态 token 生成 → join 页面
5. **Token 生命周期**：TTL 标准化 → 撤销机制 → 使用跟踪 → 过期清理 → 审计日志
6. **前端 QR 码 + token 页增强**：TokenQRCode 组件 → token 页面 QR 列、新列、筛选

---

*最后更新: 2026-05-13*
