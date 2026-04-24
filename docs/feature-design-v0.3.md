# Wireflow Feature Design v0.3

> 涵盖本阶段新增的四个子系统：用户成员管理、审计日志、平台管理员权限旁路、Dashboard 监控 API。

---

## 目录

1. [用户身份与成员管理](#1-用户身份与成员管理)
2. [审计日志模块](#2-审计日志模块)
3. [平台管理员权限旁路](#3-平台管理员权限旁路)
4. [Dashboard 监控 API](#4-dashboard-监控-api)

---

## 1. 用户身份与成员管理

### 1.1 背景

原有系统将用户与工作空间直接绑定，无法支持外部 IdP 接入（Dex / LDAP / GitHub）、邀请流程和细粒度角色管理。本次重构将身份层与权限层分离。

### 1.2 数据模型

```
┌──────────┐       ┌────────────────┐       ┌───────────────────┐
│   User   │──1:N──│  UserIdentity  │       │  WorkspaceMember  │
│          │       │  provider      │       │  workspace_id     │
│ system_  │       │  external_id   │       │  user_id          │
│ role     │       │  email         │       │  role             │
└──────────┘       └────────────────┘       │  status           │
     │                                      │  invited_by       │
     └──────────────────────────────────────│  joined_at        │
                                            └───────────────────┘
                                                     │
                                            ┌────────────────────┐
                                            │ WorkspaceInvitation│
                                            │  email             │
                                            │  role              │
                                            │  token (unique)    │
                                            │  status            │
                                            │  expires_at        │
                                            └────────────────────┘
```

#### User

| 字段 | 类型 | 说明 |
|------|------|------|
| `system_role` | `platform_admin` / `platform_user` | 平台层角色，写入 JWT |
| `email` | string | 全局唯一 |
| `avatar` | string | 头像 URL |

#### UserIdentity（外部身份关联）

一个 User 可拥有多条 Identity，支持同账号绑定多个 IdP。

| 字段 | 说明 |
|------|------|
| `provider` | `local` / `dex` / `ldap` / `github` |
| `external_id` | IdP 侧的 subject |
| `metadata` | 原始 claims（JSON）|

#### WorkspaceMember（RoleBinding）

等价于 Kubernetes RoleBinding，所有权限校验在数据库层完成，不依赖 K8s RBAC。

| 角色 | 权重 | 说明 |
|------|------|------|
| `admin` | 40 | 可管理成员、配置策略 |
| `editor` | 30 | 可创建/修改节点与策略 |
| `member` | 20 | 可接入节点 |
| `viewer` | 10 | 只读 |

#### WorkspaceInvitation（邀请令牌）

```
邀请方 POST /invitations → 生成 token（7天有效）→ 邮件发送链接
被邀请人 GET  /invitations/accept?token=xxx → 创建 User + WorkspaceMember
```

状态机：`pending → accepted / expired / revoked`

### 1.3 API

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/workspaces/:id/members` | 列出成员（含 provider badge） |
| `PUT` | `/api/v1/workspaces/:id/members/:userID` | 修改角色 |
| `DELETE` | `/api/v1/workspaces/:id/members/:userID` | 移除成员 |
| `GET` | `/api/v1/workspaces/:id/invitations` | 列出邀请 |
| `POST` | `/api/v1/workspaces/:id/invitations` | 发起邀请（email + role） |
| `DELETE` | `/api/v1/workspaces/:id/invitations/:invID` | 撤销邀请 |

### 1.4 MemberVo 响应结构

```json
{
  "userId": "abc",
  "name": "Alice",
  "email": "alice@example.com",
  "avatar": "https://...",
  "role": "admin",
  "provider": "dex",
  "status": "active",
  "joinedAt": "2025-01-01T00:00:00Z"
}
```

`provider` 字段来源于 `User.Identities[0].Provider`，通过 `Preload("User.Identities")` 在查询时一次性加载。

### 1.5 前端

- **成员 Tab**：头像 + 姓名/邮箱 / 角色 badge（4 级） / provider badge（local/dex/ldap）/ 加入时间 / 编辑角色下拉 / 移除按钮
- **邀请 Tab**：邮箱 / 角色 / 状态 badge（pending/accepted/expired/revoked）/ 过期时间 / 撤销按钮
- **邀请对话框**：仅填写 email + 角色，无密码字段
- 两个 Tab 均使用 `DataTablePagination`（客户端分页，每页 10 条）

---

## 2. 审计日志模块

### 2.1 设计原则

- **Append-only**：无 `UpdatedAt`，无软删除
- **异步非阻塞**：HTTP handler 不等待写库
- **HTTP 层自动捕获**：业务代码零侵入
- **工作空间隔离**：普通成员只查自己空间，`platform_admin` 可查全局

### 2.2 数据模型

```go
type AuditLog struct {
    ID           string    // UUID
    CreatedAt    time.Time // 索引
    UserID       string    // 操作者
    UserName     string    // 非规范化，展示用
    UserIP       string    // 客户端 IP（支持 IPv4/IPv6）
    WorkspaceID  string    // 空 = 平台级操作
    Action       string    // CREATE / UPDATE / DELETE / LOGIN / INVITE / REVOKE / ACCEPT
    Resource     string    // member / workspace / policy / token / relay / peer
    ResourceID   string
    ResourceName string
    Scope        string    // 影响范围描述，e.g. "成员: alice@example.com → 角色: editor"
    Status       string    // success | failed
    StatusCode   int       // HTTP 状态码
    Detail       string    // JSON 快照（可选）
}
```

### 2.3 系统架构

```
HTTP Request
    │
    ▼
AuditMiddleware (gin)
    │  ① 包装 ResponseWriter 捕获状态码
    │  ② c.Next()（执行实际 Handler）
    │  ③ 从路径推导 Action / Resource / Scope
    │  ④ svc.Log(entry)  ← 非阻塞 channel send
    │
    ▼
AuditService
    │  buffered channel (cap=512)
    │
    ▼
background goroutine
    │  批量写入（最多 100 条 / 最多 1s 一次）
    │  ctx 取消时 drain channel 再退出
    ▼
t_audit_log (DB)
```

### 2.4 Action / Resource 推导规则

**Action**（从 HTTP Method + Path 推导）：

| 条件 | Action |
|------|--------|
| Path 含 `/login` | `LOGIN` |
| Path 含 `/logout` | `LOGOUT` |
| Path 含 `/accept` | `ACCEPT` |
| `POST /invitations` | `INVITE` |
| `DELETE /invitations` | `REVOKE` |
| `POST` | `CREATE` |
| `PUT / PATCH` | `UPDATE` |
| `DELETE` | `DELETE` |

**Resource**（从 URL 倒序找最后一个有意义的名词，自动单数化）：

```
/api/v1/workspaces/:id/members/:userID  →  "member"
/api/v1/workspaces/:id/invitations/:id  →  "invitation"
/api/v1/workspaces/:id                  →  "workspace"
```

**Scope**（影响范围）：
- 默认：从 Gin Path Params 自动拼接（`workspace:<id>` `userID:<id>`）
- Handler 可调用 `middleware.SetAuditScope(c, "成员: alice@example.com → 角色: editor")` 覆盖

### 2.5 API

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/v1/workspaces/:id/audit-logs` | 工作空间审计日志（分页+过滤） |
| `GET` | `/api/v1/audit-logs` | 全平台审计日志（platform_admin） |

**查询参数**：`action` / `resource` / `status` / `keyword` / `from` / `to` / `page` / `pageSize`

### 2.6 前端（Settings → Audit Logs）

- **顶部统计卡片**：操作总数 / 失败操作 / 活跃用户 / 删除操作
- **时间范围 Tab**：1d / 7d / 30d
- **过滤工具栏**：操作类型 / 资源类型 / 结果 / 关键字搜索 / 刷新
- **表格列**：时间 / 操作者（姓名 + IP）/ 操作 badge / 资源（类型 + 名称）/ 影响范围 / 结果图标 / 详情展开
- **可展开行**：显示格式化 JSON 详情
- Action 颜色规范：`CREATE`=emerald / `UPDATE`=blue / `DELETE`=red / `LOGIN`=violet / `INVITE`=amber / `REVOKE`=orange
- `DataTablePagination`，默认每页 20 条

---

## 3. 平台管理员权限旁路

### 3.1 问题

`WorkspaceAuthMiddleware` 仅查询 `t_workspaces_member` 表，`platform_admin` 用户即使没有显式 RoleBinding 也应能访问所有工作空间。

### 3.2 方案选型

| 方案 | 说明 | 结论 |
|------|------|------|
| 自动写入 member 行 | 创建 workspace 时批量 INSERT | ❌ 数据噪音，需同步维护 |
| **中间件旁路（Superuser Bypass）** | 校验前检查 `system_role`，admin 直接放行 | ✅ 推荐 |
| Store 层虚拟成员 | 查询时 WHERE 加 OR 子查询 | ❌ 逻辑混乱 |

### 3.3 实现

JWT Payload 中已携带 `system_role`，由 `AuthMiddleware` 写入 Gin Context（`c.Set("system_role", ...)`）。

```go
// WorkspaceAuthMiddleware 修改点
if systemRole, _ := c.Get("system_role"); systemRole == "platform_admin" {
    c.Next()  // 旁路所有 workspace 成员校验
    return
}
// 否则走正常 WorkspaceMember 查询...
```

### 3.4 权限矩阵

| 用户类型 | 需要 WorkspaceMember 记录 | 访问结果 |
|----------|--------------------------|----------|
| `platform_admin` | 不需要 | ✅ 所有工作空间（旁路） |
| workspace `admin` | 需要 | ✅ 该工作空间 |
| workspace `editor/member/viewer` | 需要 | ✅ 按角色 |
| 无记录的普通用户 | — | ❌ 403 |

> `platform_admin` 不显示在成员列表中，其访问是"隐式"的，不污染 RoleBinding 数据。

---

## 4. Dashboard 监控 API

### 4.1 背景与问题

原有监控代码存在三个 Bug 导致查询永远返回空数据：

| # | 问题 | 位置 |
|---|------|------|
| 1 | PromQL 过滤 `workspace_id`，但 scraper 实际 emit `network_id` | `service/monitor.go` |
| 2 | 查询 `wireflow_workspace_tunnels` 但该指标从未被推送 | `service/monitor.go` |
| 3 | 查询 `wireflow_node_uptime_seconds` 但 scraper 未 emit | `scraper_system.go` |

### 4.2 指标清单

#### 现有指标（scraper emit）

| 指标名 | 标签 | Scraper |
|--------|------|---------|
| `wireflow_node_cpu_usage_percent` | peer_id, **network_id** | system |
| `wireflow_node_memory_bytes` | peer_id, **network_id** | system |
| `wireflow_node_goroutines` | peer_id, **network_id** | system |
| `wireflow_peer_status` | peer_id, **network_id**, remote_peer_id, endpoint | wireguard |
| `wireflow_peer_last_handshake_seconds` | peer_id, **network_id**, remote_peer_id | wireguard |
| `wireflow_peer_traffic_bytes_total` | peer_id, **network_id**, remote_peer_id, direction | wireguard |
| `wireflow_node_traffic_bytes_total` | peer_id, **network_id**, direction | wireguard |
| `wireflow_peering_traffic_bytes_total` | peer_id, local_network_id, remote_network_id, direction | wireguard |
| `wireflow_peer_latency_ms` | peer_id, **network_id**, remote_peer_id, remote_peer_ip | icmp |
| `wireflow_peer_packet_loss_percent` | peer_id, **network_id**, remote_peer_id | icmp |

#### 新增指标

| 指标名 | 标签 | 说明 |
|--------|------|------|
| `wireflow_node_uptime_seconds` | peer_id, **network_id** | 进程启动至今秒数，用于统计在线节点数 |

> **标签约定**：`network_id` = `workspace.Namespace`（如 `wf-abc123`）。管理层按 workspace UUID 查询时，需先 JOIN DB 获取 Namespace，再用于 PromQL 过滤。

### 4.3 PromQL 查询参考

#### 工作空间 Dashboard（`$ns` = network_id 值）

| 面板 | PromQL | 模式 |
|------|--------|------|
| 在线节点数 | `count(last_over_time(wireflow_node_uptime_seconds{network_id="$ns"}[5m]))` | instant |
| 实时吞吐 TX (Mbps) | `sum(irate(wireflow_node_traffic_bytes_total{network_id="$ns",direction="tx"}[2m])) * 8 / 1e6` | instant |
| 平均延迟 (ms) | `avg(wireflow_peer_latency_ms{network_id="$ns"})` | instant |
| 丢包率 (%) | `avg(wireflow_peer_packet_loss_percent{network_id="$ns"})` | instant |
| 吞吐趋势 TX | `sum(irate(wireflow_node_traffic_bytes_total{network_id="$ns",direction="tx"}[5m])) * 8 / 1e6` | range, 1h, step=2m |
| 吞吐趋势 RX | `sum(irate(wireflow_node_traffic_bytes_total{network_id="$ns",direction="rx"}[5m])) * 8 / 1e6` | range, 1h, step=2m |
| CPU per node | `last_over_time(wireflow_node_cpu_usage_percent{network_id="$ns"}[5m])` | instant |
| Memory per node | `last_over_time(wireflow_node_memory_bytes{network_id="$ns"}[5m])` | instant |
| Top 10 节点 | `topk(10, sum by (peer_id)(increase(wireflow_node_traffic_bytes_total{network_id="$ns"}[24h])))` | instant |
| 活动隧道数 | `ceil(sum(wireflow_peer_status{network_id="$ns"} == 1) / 2)` | instant |

#### 全域 Dashboard

| 面板 | PromQL |
|------|--------|
| 活跃工作空间 | `count(count by (workspace_id)(wireflow_peer_status{workspace_id!=""}))` |
| 全网在线节点 | `count(count by (node_id)(wireflow_peer_status == 1))` |
| 全域吞吐 (Gbps) | `sum(irate(wireflow_peer_traffic_bytes_total{direction="tx"}[2m])) * 8 / 1e9` |
| 各空间在线节点 | `count by (network_id)(last_over_time(wireflow_node_uptime_seconds[5m]))` |
| 各空间 24h 流量 | `sum by (network_id)(increase(wireflow_node_traffic_bytes_total{direction="tx"}[24h]))` |

### 4.4 API

#### 全局视图（platform_admin）

```
GET /api/v1/dashboard/overview
Authorization: Bearer <token>
```

返回 `DashboardResponse`（全域聚合）。

#### 工作空间视图（workspace member）

```
GET /api/v1/workspaces/:id/dashboard
Authorization: Bearer <token>
```

返回 `WorkspaceDashboardResponse`：

```json
{
  "stat_cards": [
    { "label": "在线节点", "value": "12",    "unit": "台",  "trend": "stable", "trend_pct": "",    "color": "text-emerald-500" },
    { "label": "实时吞吐", "value": "234.5", "unit": "Mbps","trend": "up",     "trend_pct": "+5%", "color": "text-blue-500"   },
    { "label": "平均延迟", "value": "18.3",  "unit": "ms",  "trend": "stable", "trend_pct": "",    "color": "text-amber-500"  },
    { "label": "丢包率",   "value": "0.12",  "unit": "%",   "trend": "stable", "trend_pct": "",    "color": "text-emerald-500"}
  ],
  "throughput_trend": {
    "timestamps": ["10:00", "10:02", "..."],
    "tx_data":    [120.1, 131.4, "..."],
    "rx_data":    [88.2,  91.0,  "..."]
  },
  "node_cpu": [
    { "peer_id": "abc", "name": "node-01", "cpu": 72.4, "memory_mb": 512.0 }
  ],
  "top_nodes": [
    { "id": "abc", "name": "node-01", "endpoint": "1.2.3.4:51820",
      "total_tx": 10485760, "total_rx": 5242880, "online": true, "cpu": 72.4 }
  ]
}
```

### 4.5 服务层架构

```
MonitorController.GetWorkspaceDashboard(wsID)
    │
    ├─ store.Workspaces().GetByID(wsID)  →  ws.Namespace
    │
    └─ MonitorService.GetWorkspaceDashboard(namespace)
           │  7 个并发 goroutine（errgroup）
           ├─ [0] 在线节点数      → instant query
           ├─ [1] 实时吞吐 TX     → instant query + getTrend()
           ├─ [2] 平均延迟        → instant query
           ├─ [3] 平均丢包率      → instant query
           ├─ [4] 吞吐趋势 1h     → range query
           ├─ [5] 节点 CPU/Memory → 2x instant query
           └─ [6] Top 10 节点     → instant query + status overlay
```

**wsID → namespace 转换**由 `MonitorController` 负责（持有 `store.Store`），`MonitorService` 只接受最终的 `namespace` 字符串，保持服务层的单一职责。

### 4.6 前端

```
useDashboardStore
  ├─ fetch()          ← 全域数据（30s 轮询）
  ├─ fetchWorkspace() ← 工作空间数据（active_ws_id 存在时调用）
  │
  ├─ displayStatCards   → isWorkspaceMode ? wsStatCards : statCards
  ├─ displayTxData      → isWorkspaceMode ? ws.throughput_trend.tx_data : globalTrend.tx_data
  ├─ displayRxData      → isWorkspaceMode ? ws.throughput_trend.rx_data : globalTrend.rx_data
  ├─ nodeLoadBar        → isWorkspaceMode ? wsData.node_cpu : topNodes by CPU
  └─ topTrafficNodes    → isWorkspaceMode ? wsData.top_nodes : topNodes by TX
```

Dashboard 页面根据 `active_ws_id` 是否存在自动切换模式，顶部显示「全域视图」或「工作空间视图」标签。

---

## 附录：数据库表清单

| 表名 | 说明 |
|------|------|
| `t_user` | 用户基础信息，含 `system_role` |
| `t_user_identity` | 外部 IdP 身份关联（1:N） |
| `t_user_profile` | 用户详细资料（1:1） |
| `t_workspace` | 工作空间，`namespace` 对应 K8s Namespace |
| `t_workspaces_member` | WorkspaceMember RoleBinding |
| `t_workspace_invitation` | 邀请令牌，7 天有效 |
| `t_audit_log` | 审计日志，Append-only |

## 附录：文件索引

| 文件 | 职责 |
|------|------|
| `management/models/user.go` | User / UserIdentity / UserProfile |
| `management/models/workspace.go` | Workspace / WorkspaceMember / WorkspaceInvitation |
| `management/models/audit.go` | AuditLog（含 Scope 字段）|
| `management/models/dashboard.go` | DashboardResponse / WorkspaceDashboardResponse |
| `management/service/audit.go` | AuditService（channel + batch write）|
| `management/service/monitor.go` | MonitorService（PromQL 查询）|
| `management/controller/monitor.go` | wsID → namespace 转换 |
| `management/server/middleware/audit.go` | AuditMiddleware（HTTP 自动捕获）|
| `management/server/member.go` | 成员管理 REST 路由 |
| `management/server/invitation.go` | 邀请管理 REST 路由 |
| `management/server/audit.go` | 审计日志 REST 路由 |
| `management/server/dashboard.go` | Dashboard REST 路由（pro build）|
| `internal/telemetry/scraper_system.go` | 新增 wireflow_node_uptime_seconds |
| `fronted/src/pages/manage/members/index.vue` | 成员管理页面 |
| `fronted/src/pages/settings/audit/index.vue` | 审计日志页面 |
| `fronted/src/pages/dashboard/index.vue` | Dashboard 页面（全域/工作空间自动切换）|
| `fronted/src/stores/useDashboard.ts` | Dashboard Pinia Store |
| `docs/dashboard-api-design.md` | Dashboard PromQL 详细设计 |
