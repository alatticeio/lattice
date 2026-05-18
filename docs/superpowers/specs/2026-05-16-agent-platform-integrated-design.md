# Lattice Agent Platform 整合实现参考

> 日期: 2026-05-16（更新: 2026-05-18）
> 性质: 实现参考文档（基于已合入 `dev` 分支的代码）
> 关联: `2026-05-16-lattice-future-vision-and-roadmap.md`
>       `2026-05-16-agent-sandbox-security-review-and-observability.md`
>       `2026-05-18-sandbox-agent-architecture.md`（沙箱架构详细文档）

---

## 零、实现状态总览

本 PR（`dev` 分支）基于 `2026-05-16` 的差量设计，完成了 Agent Platform 的四项核心功能。下表是**实际完成状态**：

| 能力 | 设计状态 | 实现状态 | 关键文件 |
|------|---------|---------|---------|
| gVisor 移入 Community（P1） | 社区版返回 "Pro feature" 错误 | **已实现**，`//go:build !pro` 完整 gVisor 沙箱 | `cmd/lattice/cmd/sandbox/sandbox_community.go` |
| MCP Trace 可观测性（P2） | 无 traceID，无持久化 | **已实现**，`la_tool_spans` 表 + `ExecuteTool` 包裹 | `internal/server/models/tool_span.go`、`service/ai.go` |
| Sub-agent Delegate API（P3） | 无 `parentRef`、无 Delegate 端点 | **已实现**，CRD 字段 + `DelegateToken()` + HTTP 端点 | `api/v1alpha1/agent_identity_types.go`、`service/agent_registration.go` |
| NATS 流量审计（P4，PRO） | AuditWriter 写本地文件 | **服务端已实现**（`AuditConsumer` + `la_flow_events`），沙箱侧仍写本地文件 | `internal/server/controller/audit_consumer.go`（`//go:build pro`） |

> **P4 进度说明**：服务端订阅 `lattice.audit.flow` 并持久化到 `la_flow_events` 表已就绪，
> 但沙箱侧的 `natsAuditWriter`（负责发布到 NATS）尚未完成，PRO sandbox 目前也写本地文件。
> 数据模型和服务端管道已打通，等待沙箱侧接入。

---

## 一、gVisor 社区版（P1）

### 文件结构

```
cmd/lattice/cmd/sandbox/
├── sandbox.go            # 公共 flags（--name, --server-url, --token）
├── sandbox_shared.go     # 共享：sandboxCredentials、fileAuditWriter（无 build tag）
├── sandbox_community.go  # //go:build !pro — 完整社区版沙箱
└── sandbox_pro.go        # //go:build pro  — PRO 专属增强
```

### 社区版 vs PRO 能力分界

| 能力 | Community | PRO |
|------|-----------|-----|
| gVisor 用户态网络栈 | ✅ | ✅ |
| NATS 注册 + ICE/LRP 连接 | ✅ | ✅ |
| 凭证持久化（重启免重注册） | ✅ | ✅ |
| 本地文件审计（`/tmp/lattice-audit-<name>.jsonl`） | ✅ | ✅（自定义路径） |
| 出站策略过滤（`EgressFilter`，`--egress-allow`） | ❌ | ✅ |
| 入站端口转发（`--forward`） | ❌ | ✅ |
| HTTP 正向代理（`--proxy-addr`） | ❌ | ✅ |

### 社区版沙箱启动流程

```
1. 加载 /etc/lattice/sandbox-credentials.json（容器重启恢复路径）
   ├── 成功 → ResumeSandboxViaNATS(jwt, privKey)
   │   ├── OK  → 跳过注册，直接获取 VPN IP
   │   └── 失败 → 降级走新注册
   └── 不存在 → 走新注册

2. 新注册：
   a. wgtypes.GeneratePrivateKey()
   b. RegisterSandboxViaNATS(serverURL, token, name, privKey)
      → 返回 infra.Peer{Address, Token, LrpUrl, ...}
   c. saveSandboxCredentials(privKey, peer.Token) → 0600 权限

3. 若 peer.LrpUrl != "" → 开启 LRP relay

4. fileAuditWriter → /tmp/lattice-audit-<name>.jsonl

5. gvisor.New(Config{ID, LocalIP, AuditWriter, PolicyChecker: nil})
   // Community 不传 PolicyChecker，放行全部出站

6. gvisor.NewTUNAdapter(sb.Channel(), InjectIntoChannel)

7. agent.NewNode(ctx, NodeConfig{CustomTUN, CurrentPeer, ...})
   // 与普通节点共享 NATS signaling + ICE + LRP 基础设施

8. node.Start(ctx) → go StartHeartbeat(30s) → go 周期 RefreshConfig(15s)

9. 阻塞等待 SIGINT/SIGTERM → node.Stop()
```

### 凭证文件格式

```json
{
  "privateKey": "base64-wg-private-key",
  "jwt": "eyJ..."
}
```

路径：`$LATTICE_CONFIG_DIR/sandbox-credentials.json`（默认 `/etc/lattice/sandbox-credentials.json`），权限 `0600`。

---

## 二、MCP Trace 可观测性（P2）

### DB 模型

```go
// internal/server/models/tool_span.go
// 表名: la_tool_spans

type ToolSpan struct {
    Model                              // ID, CreatedAt, UpdatedAt, DeletedAt
    TraceID    string    `gorm:"uniqueIndex;size:36" json:"traceId"`
    AgentID    string    `gorm:"index;size:128"      json:"agentId"`
    ParentID   string    `gorm:"index;size:128"      json:"parentId,omitempty"`   // sub-agent 场景
    Namespace  string    `gorm:"size:128"            json:"namespace"`
    Tool       string    `gorm:"size:128"            json:"tool"`
    Status     string    `gorm:"size:32"             json:"status"` // ok | error | blocked
    ErrorMsg   string    `gorm:"type:text"           json:"errorMsg,omitempty"`
    DurationMs int64     `json:"durationMs"`
    StartedAt  time.Time `gorm:"index"               json:"startedAt"`
}
```

> 注：设计阶段提到的 `ArgsHash`/`ArgsEncData`/`ArgsEncKey` 在实现中未采用，以保持简洁性。
> 如需参数记录，后续可以单独添加。

### ExecuteTool 中的 traceID 注入

`internal/server/service/ai.go`（`ExecuteTool` 方法，约 759 行起）：

```go
traceID := uuid.New().String()
start := time.Now()
status := "ok"

// RBAC 检查
if s.agentIsolation != nil {
    if err := s.agentIsolation.CheckToolAccess(ctx, namespace, name); err != nil {
        s.logToolAudit(ctx, namespace, name, AuditActionAgentToolBlocked)
        s.writeToolSpan(ctx, namespace, name, traceID, start, "blocked", err.Error())
        return "", err
    }
}
s.logToolAudit(ctx, namespace, name, AuditActionAgentToolCall)

result, err := s.dispatchTool(...)    // 执行工具
if err != nil {
    status = "error"
}
s.writeToolSpan(ctx, namespace, name, traceID, start, status, errorMsg(err))
```

`writeToolSpan` 从 context 读取 `AgentClaims`，提取 `AgentID` 和 `ParentAgentID`：

```go
func (s *aiService) writeToolSpan(ctx context.Context, namespace, tool, traceID string,
    start time.Time, status, errMsg string) {
    claims := agentClaimsFromContext(ctx)
    agentID, parentID := "", ""
    if claims != nil {
        agentID = claims.AgentID
        parentID = claims.ParentAgentID
    }
    span := &models.ToolSpan{
        TraceID: traceID, AgentID: agentID, ParentID: parentID,
        Namespace: namespace, Tool: tool, Status: status,
        ErrorMsg: errMsg, DurationMs: time.Since(start).Milliseconds(),
        StartedAt: start,
    }
    _ = s.toolSpanRepo.Write(ctx, span)
}
```

`SetToolSpanRepo(svc, repo)` 是注入点，服务启动时由 `server.go` 调用。

### 查询 API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/agent-isolation/audit/traces` | 分页列表，支持 `agentId`、`from`、`to`（RFC3339）、`limit`（默认 50） |
| GET | `/api/v1/agent-isolation/audit/traces/:id` | 按 traceId 查单条 |

> 注：设计阶段规划的 `/calltree` 端点（调用树查询）**未在此次实现**，父子关系已记录在
> `ParentID` 字段，调用树查询可在后续通过在存储层递归 `ParentID` 实现。

---

## 三、Sub-agent Delegate API（P3）

### CRD 新增字段（`api/v1alpha1/agent_identity_types.go`）

```go
type AgentIdentitySpec struct {
    PeerRef           string          // 关联的 LatticePeer 名称
    AllowedTools      []string        // 工具白名单
    AllowedNamespaces []string        // 允许操作的 namespace 列表
    Sandbox           SandboxMode     // none | pod | gvisor | microvm
    AuditLevel        AuditLevel      // none | write | full
    EnforcementMode   EnforcementMode // disabled | audit | enforce

    // 新增：sub-agent 场景
    ParentRef      string   `json:"parentRef,omitempty"`
    SpawnableRoles []string `json:"spawnableRoles,omitempty"`
}
```

### JWT Claims（`internal/server/models/agent_claims.go`）

```go
type AgentClaims struct {
    jwt.RegisteredClaims
    AgentID       string   `json:"agent_id"`
    Namespace     string   `json:"namespace"`
    AllowedTools  []string `json:"allowed_tools"`
    ParentAgentID string   `json:"parent_agent_id,omitempty"` // sub-agent 场景
}
```

### DelegateToken 逻辑（`internal/server/service/agent_registration.go`）

```
DelegateToken(req DelegateRequest):
  1. ValidateAgentJWT(req.ParentJWT) → parentClaims

  2. 权限计算（两路径）：
     a. req.RoleName 非空（SpawnableRoles 路径）
        → Get K8s AgentIdentity(parentClaims.AgentID)
        → 校验 parentIdentity.Spec.SpawnableRoles 包含 req.RoleName
        → Get AgentIdentity(req.RoleName) 作为角色模板
        → allowedTools = roleTemplate.Spec.AllowedTools
        → 子权限可以超过父级（管理员预授权）

     b. req.RoleName 为空（派生路径）
        → 校验 req.RequestedTools ⊆ parentClaims.AllowedTools
        → allowedTools = req.RequestedTools
        → 子权限严格不超过父级

  3. 创建一次性 EnrollmentToken（TTL=15min）
     → AgentEnrollmentToken.ParentAgentID = parentClaims.AgentID

  4. 子 Agent 用此 Token 调用 POST /api/v1/agent-isolation/register
     → RegisterAgent 读出 ParentAgentID → 写入 AgentIdentity.Spec.ParentRef
     → 签发的子 Agent JWT 携带 parent_agent_id claim
```

### HTTP 端点

```
POST /api/v1/agent-isolation/delegate
Authorization: Bearer <parent-agent-jwt>

{
  "agentName":      "sub-executor-001",
  "requestedTools": ["exec", "read"],
  "roleName":       ""               // 填角色名则走 SpawnableRoles 路径
}

响应:
{
  "code": 200,
  "data": {
    "enrollmentToken": "abc123...",
    "expiresAt": "2026-05-18T14:15:00Z"
  }
}
```

### 完整 API 路由表（`internal/server/server/agent_isolation_router.go`）

| 方法 | 路径 | 认证 | 说明 |
|------|------|------|------|
| POST | `/api/v1/agent-isolation/register` | 无（enrollment token 在 body）| Agent 用一次性 token 注册，换取 JWT |
| POST | `/api/v1/agent-isolation/enrollment-tokens` | 用户 JWT | 创建一次性 enrollment token |
| DELETE | `/api/v1/agent-isolation/agents/:name` | 用户 JWT | 吊销 Agent（Patch AgentIdentity status → Revoked） |
| GET | `/api/v1/agent-isolation/audit/traces` | 用户 JWT | 列出工具调用 span |
| GET | `/api/v1/agent-isolation/audit/traces/:id` | 用户 JWT | 查单条 span |
| POST | `/api/v1/agent-isolation/delegate` | Agent JWT | 派生子 Agent enrollment token |

---

## 四、NATS 流量审计（P4，PRO）

### 服务端（已就绪）

**`la_flow_events` 表**（`internal/server/models/flow_event.go`）：

```go
type FlowEvent struct {
    Model
    TraceID   string    `gorm:"index;size:36"  json:"traceId"`   // 关联 la_tool_spans
    AgentID   string    `gorm:"index;size:128" json:"agentId"`
    Direction string    `gorm:"size:16"        json:"direction"` // egress | ingress
    DstIP     string    `gorm:"size:64"        json:"dstIp"`
    DstPort   int       `json:"dstPort"`
    Bytes     int64     `json:"bytes"`
    Ts        time.Time `gorm:"index"          json:"ts"`
}
```

**NATS 订阅**（`internal/server/controller/audit_consumer.go`，`//go:build pro`）：

```go
// 订阅 lattice.audit.flow，收到消息后写入 la_flow_events
type AuditConsumer struct {
    nc         *nats.Conn
    flowEvents store.FlowEventRepository
    sub        *nats.Subscription
}

// FlowAuditMsg 是沙箱侧发布的消息格式
type FlowAuditMsg struct {
    AgentID   string `json:"agentId"`
    TraceID   string `json:"traceId"`    // 关联对应的工具调用 span
    Direction string `json:"direction"`
    DstIP     string `json:"dstIp"`
    DstPort   int    `json:"dstPort"`
    Bytes     int64  `json:"bytes"`
    Ts        string `json:"ts"`         // RFC3339Nano
}
```

服务端通过 `initFlowAuditConsumer(natsURL, store)`（`audit_consumer_pro.go`，`//go:build pro`）在启动时初始化订阅。

### 沙箱侧（待实现）

目前 PRO sandbox 也使用 `fileAuditWriter` 写本地文件。后续需要实现 `natsAuditWriter`：

```go
// 待实现（cmd/lattice/cmd/sandbox/sandbox_pro.go）
type natsAuditWriter struct {
    nc      *nats.Conn
    agentID string
}

func (w *natsAuditWriter) Write(event shimfwd.AuditEvent) error {
    payload, _ := json.Marshal(FlowAuditMsg{
        AgentID:   w.agentID,
        TraceID:   "", // 需要从全局 traceID 上下文读取
        Direction: "egress",
        DstIP:     event.DstIP,
        DstPort:   event.DstPort,
        Bytes:     event.Bytes,
        Ts:        time.Now().UTC().Format(time.RFC3339Nano),
    })
    return w.nc.Publish("lattice.audit.flow", payload)
}
```

> 主要挑战：sandbox 进程与控制面的 `writeToolSpan` 在不同进程，traceID 需要通过某种方式（如通过 JWT context 注入或环境变量）传递到 AuditWriter。

---

## 五、AgentIdentity CRD 完整字段参考

```yaml
apiVersion: lattice.io/v1alpha1
kind: AgentIdentity
metadata:
  name: my-agent
  namespace: default
spec:
  peerRef: my-agent                          # 关联的 LatticePeer 名称
  allowedTools:
    - list_peers
    - check_connectivity
    - create_policy
  allowedNamespaces:
    - default
  sandbox: gvisor                            # none | pod | gvisor | microvm
  auditLevel: write                          # none | write | full
  enforcementMode: enforce                   # disabled | audit | enforce
  parentRef: ""                              # 父 AgentIdentity（sub-agent 场景）
  spawnableRoles: []                         # 允许派生的角色模板名称列表
status:
  phase: Active                              # Active | Revoked | Expired
```

---

## 六、前端 AgentDetailDrawer

`fronted/src/composables/useAgentDetailDrawer.ts` 提供模块级单例状态管理：

| Tab | 数据来源 | 功能 |
|-----|---------|------|
| Traces | `GET /api/v1/agent-isolation/audit/traces` | 工具调用记录列表，点击查看单条详情 |
| Network | `GET /api/v1/agent-isolation/audit/traces/:id` 关联 flow events | gVisor 出站流量（PRO） |
| Sub-agents | 本地过滤 `listSandboxes()` | 展示子 Agent 列表，触发 Delegate 对话框 |

Delegate 对话框调用 `POST /api/v1/agent-isolation/delegate`（需要父 Agent JWT），返回一次性 enrollment token 供子 Agent 使用。

---

## 七、未实现项（遗留 Roadmap）

| 功能 | 原因 | 建议时机 |
|------|------|---------|
| `/calltree` 查询端点 | 未在此次 Sprint 实现；`ParentID` 字段已就绪 | 下次迭代，存储层递归查询 |
| sandbox 侧 `natsAuditWriter` | 需要解决跨进程 traceID 传递问题 | P4 完整版 |
| PID ↔ TUN 绑定（eBPF `cgroup/connect4`） | 需要 root + 内核 5.10+ | 长期 Roadmap |
| Sidecar 意图拦截（seccomp notify） | 架构复杂，需单独设计 | 长期 Roadmap |
