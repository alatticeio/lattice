# Lattice Agent Platform 整合设计

> 日期: 2026-05-16
> 性质: 实现规范（结合现有代码的差量设计）
> 关联: `2026-05-16-lattice-future-vision-and-roadmap.md`
>       `2026-05-16-agent-sandbox-security-review-and-observability.md`

---

## 零、现状盘点（先搞清楚已有什么）

读代码后发现，现有实现比设计文档描述的更完善。下表是真实状态：

| 能力 | 现状 | 结论 |
|------|------|------|
| AllowedTools 强制检查 | **已实现**，`ai.go:726` CheckToolAccess | 不需要重新设计 |
| 工具调用日志 | **已实现**，logToolAudit（无 traceID） | 需要加 traceID |
| gVisor AuditWriter 接入 | **已实现**，写到本地 `/tmp/lattice-audit.jsonl` | 需要改为上报控制面 |
| gVisor 社区版 | **直接返回错误**，`start_sandbox_community.go` | 需要移入 Community |
| AgentIdentity parentRef | **不存在** | 需要新增 CRD 字段 |
| Delegate API（sub-agent） | **不存在** | 需要新增 |
| tool_spans / flow_events 表 | **不存在** | 需要新增 |
| Lattice Trace traceID | **不存在** | 需要新增 |

**结论：现有设计方向正确，不需要推翻，只需要差量补齐。**

---

## 一、最应该先做哪个

按**投入产出比**排序：

### 第一优先：gVisor 移入 Community（1-2 天）

**为什么第一**：Community 版 sandbox 直接 `return nil, errors.New("gVisor agent sandbox is a Lattice Pro feature")`，用户根本感受不到沙箱价值，后续所有推广都是空谈。改动最小，影响最大。

**改动范围**：
- `cmd/lattice-agent-sandbox/cmd/start_sandbox_community.go` — 实现真正的沙箱（去掉 `//go:build !pro`，提取 gVisor 基础功能到 Community）
- `cmd/lattice-agent-sandbox/cmd/start_sandbox_pro.go` — 保留 PRO 专属增强（加密审计、NATS trace 上报）
- 分界线：Community = gVisor 网络隔离 + 本地文件审计；PRO = 中心化审计 + traceID + 加密参数

### 第二优先：MCP Trace Middleware（3-5 天）

**为什么第二**：工具调用日志已有，缺的是 `traceID`、持久化到 DB、查询 API。这是 Lattice Trace 的基础，不做后面所有可观测性都是空的。

**改动范围**：
- 新增 `tool_spans` DB 表（GORM model）
- 在 `ai.go` ExecuteTool 前后生成 traceID、记录 span
- 新增查询 API `GET /api/v1/audit/traces`

### 第三优先：Sub-agent Delegate API（5-7 天）

**为什么第三**：Multi-agent 是最核心的未来场景，但依赖前两个（需要 traceID 才能追溯调用链）。

**改动范围**：
- `AgentIdentity` CRD 新增 `parentRef` + `spawnableRoles`
- `AgentRegistrationService` 新增 `Delegate()` 方法
- 新增 API `POST /api/v1/agent-isolation/delegate`

### 第四优先：gVisor AuditWriter → 控制面上报（5-7 天）

**为什么第四**：AuditWriter 已接入，只是写到本地文件。改为上报到控制面才能中心化查询并与 tool_spans 关联。

**改动范围**：
- `start_sandbox_pro.go` fileAuditWriter → HTTP/NATS 上报审计事件
- `flow_events` DB 表
- 控制面新增接收审计事件的 endpoint

---

## 二、差量设计

### 2.1 gVisor Community 化

**原则**：去掉 build tag 二元对立，改为三段式：

```
Community:  gVisor 网络隔离 + 本地文件审计（已有逻辑，移走 build tag）
PRO 扩展:   中心化审计上报 + 加密参数 + NATS traceID 订阅
```

**`start_sandbox_community.go` 新实现**（去掉 `//go:build !pro`，改为公共基础实现）：

```go
// 不再有 build tag，成为所有版本共用的基础沙箱实现。
// PRO 版通过 start_sandbox_pro.go（build tag: pro）在此基础上注入增强能力。

func createSandbox(sandboxName, localIP, agentJWT string,
    wgEnabled bool, privateKey wgtypes.Key, peers []wgtypes.PeerConfig,
    opts ...SandboxOption,  // PRO 版通过 opts 注入增强 AuditWriter/PolicyChecker
) (*sandboxCloser, error) {

    // 默认：本地文件审计
    auditWriter := newFileAuditWriter("/tmp/lattice-audit-" + sandboxName + ".jsonl")

    // PRO 版覆盖 auditWriter（通过 opts 注入）
    for _, opt := range opts {
        opt.apply(&auditWriter)
    }

    cfg := gvisor.Config{
        ID:          sandboxName,
        LocalIP:     localIP,
        AuditWriter: auditWriter,
    }
    // ... 其余 WireGuard 配置逻辑不变
}
```

**`start_sandbox_pro.go`**（`//go:build pro`）：只负责注入 PRO 增强：

```go
//go:build pro

func init() {
    // PRO 版在 cobra 的 PersistentPreRunE 中注入增强 opts
    // 如: 加密 AuditWriter、NATS traceID 订阅
}
```

---

### 2.2 MCP Trace Middleware（最重要的新增）

#### 2.2.1 DB 表（新增）

```go
// internal/server/models/tool_span.go

type ToolSpan struct {
    ID          uint      `gorm:"primaryKey;autoIncrement"`
    TraceID     string    `gorm:"uniqueIndex;size:36"`   // UUID
    AgentID     string    `gorm:"index;size:128"`
    ParentID    string    `gorm:"index;size:128"`        // sub-agent 场景
    Namespace   string    `gorm:"size:128"`
    Tool        string    `gorm:"size:128"`
    ArgsHash    string    `gorm:"size:64"`               // sha256（Community）
    ArgsEncData []byte    // 加密密文（PRO，nil = 未加密）
    ArgsEncKey  []byte    // RSA 加密的 data key（PRO）
    Status      string    `gorm:"size:32"`               // ok | error | blocked
    ErrorMsg    string
    DurationMs  int64
    StartedAt   time.Time `gorm:"index"`
}
```

#### 2.2.2 在 ExecuteTool 注入 traceID

`ai.go` ExecuteTool 当前结构：

```go
// 现有代码（已有）
if s.agentIsolation != nil {
    if err := s.agentIsolation.CheckToolAccess(ctx, namespace, name); err != nil {
        s.logToolAudit(ctx, namespace, name, AuditActionAgentToolBlocked)
        return "", err
    }
}
s.logToolAudit(ctx, namespace, name, AuditActionAgentToolCall)
```

**改动**：在 CheckToolAccess 前后包一层 Trace：

```go
// 新增：生成 traceID，注入 ctx
traceID := uuid.New().String()
ctx = context.WithValue(ctx, traceIDKey{}, traceID)
start := time.Now()

status := "ok"
if s.agentIsolation != nil {
    if err := s.agentIsolation.CheckToolAccess(ctx, namespace, name); err != nil {
        status = "blocked"
        s.writeToolSpan(ctx, namespace, name, traceID, start, status, err.Error())
        s.logToolAudit(ctx, namespace, name, AuditActionAgentToolBlocked)
        return "", err
    }
}
s.logToolAudit(ctx, namespace, name, AuditActionAgentToolCall)

result, err := s.dispatchTool(ctx, namespace, name, input) // 提取 switch 到独立函数

if err != nil {
    status = "error"
}
s.writeToolSpan(ctx, namespace, name, traceID, start, status, errorMsg(err))
return result, err
```

```go
// 新增方法
func (s *aiService) writeToolSpan(ctx context.Context, namespace, tool, traceID string,
    start time.Time, status, errMsg string) {

    claims := agentClaimsFromContext(ctx)
    agentID, parentID := "", ""
    if claims != nil {
        agentID = claims.AgentID
        parentID = claims.ParentAgentID // 新增字段，见 2.3
    }

    span := &models.ToolSpan{
        TraceID:    traceID,
        AgentID:    agentID,
        ParentID:   parentID,
        Namespace:  namespace,
        Tool:       tool,
        ArgsHash:   "", // TODO: 从 input 计算
        Status:     status,
        ErrorMsg:   errMsg,
        DurationMs: time.Since(start).Milliseconds(),
        StartedAt:  start,
    }
    if s.toolSpanStore != nil {
        _ = s.toolSpanStore.Write(span)
    }
}
```

#### 2.2.3 查询 API（新增）

```
GET /api/v1/audit/traces?agentId=xxx&from=RFC3339&to=RFC3339&limit=50
GET /api/v1/audit/traces/:traceId
GET /api/v1/audit/agents/:agentId/calltree    # 含 sub-agent 调用树
```

---

### 2.3 Sub-agent Delegate API

#### 2.3.1 AgentIdentity CRD 新增字段

```go
// api/v1alpha1/agent_identity_types.go

type AgentIdentitySpec struct {
    // ... 现有字段不变 ...

    // ParentRef 指向父 AgentIdentity 的名称（sub-agent 场景）。
    // 空字符串表示顶级 Agent。
    // +optional
    ParentRef string `json:"parentRef,omitempty"`

    // SpawnableRoles 是该 Agent 被允许创建的子 Agent 角色名列表。
    // 子 Agent 继承角色模板的权限，不受父 Agent 自身权限限制。
    // 空列表表示不允许创建任何子 Agent。
    // +optional
    SpawnableRoles []string `json:"spawnableRoles,omitempty"`
}
```

#### 2.3.2 AgentClaims 新增 ParentAgentID

```go
// internal/server/models/agent_claims.go

type AgentClaims struct {
    jwt.RegisteredClaims
    AgentID       string   `json:"agent_id"`
    Namespace     string   `json:"namespace"`
    AllowedTools  []string `json:"allowed_tools"`
    ParentAgentID string   `json:"parent_agent_id,omitempty"` // 新增
}
```

#### 2.3.3 Delegate API

```go
// internal/server/service/agent_registration.go

type DelegateRequest struct {
    // ParentJWT 是父 Agent 的 JWT（由父进程传入）。
    ParentJWT      string   `json:"parentJWT"`
    // AgentName 是子 Agent 的期望名称。
    AgentName      string   `json:"agentName"`
    // RequestedTools 是子 Agent 请求的工具列表。
    // 若父 Agent 走 delegate 派生，服务端会校验 RequestedTools ⊆ parent.AllowedTools。
    RequestedTools []string `json:"requestedTools"`
    // RoleName 非空时走 SpawnableRoles 路径（子权限可大于父）。
    // 服务端校验 parent.SpawnableRoles 包含此角色名。
    RoleName string `json:"roleName,omitempty"`
}

type DelegateResponse struct {
    // EnrollmentToken 是一次性的子 Agent 注册 Token。
    // 子 Agent 用此 Token 调用 RegisterAgent 注册自己，获得独立 WireGuard 身份。
    EnrollmentToken string    `json:"enrollmentToken"`
    ExpiresAt       time.Time `json:"expiresAt"`
}

// AgentRegistrationService 新增接口方法：
type AgentRegistrationService interface {
    // ... 现有方法 ...
    DelegateToken(ctx context.Context, req DelegateRequest) (*DelegateResponse, error)
}
```

**实现逻辑**：

```
DelegateToken(req):
  1. 解析 parentJWT → parentClaims（ValidateAgentJWT）
  2. 分支判断：
     a. req.RoleName 非空
        → 查 K8s AgentIdentity(parentClaims.AgentID).Spec.SpawnableRoles
        → 校验包含 req.RoleName
        → 从角色模板取 allowedTools（不受父级限制）
     b. req.RoleName 为空
        → 校验 req.RequestedTools ⊆ parentClaims.AllowedTools
        → allowedTools = req.RequestedTools
  3. 创建一次性 EnrollmentToken（TTL=15min）
     → 额外携带 parentAgentID = parentClaims.AgentID
  4. 子 Agent 用此 Token 调用 RegisterAgent
     → RegisterAgent 读出 parentAgentID，写入 AgentIdentity.Spec.ParentRef
     → 签发的子 JWT 携带 ParentAgentID claim
```

**HTTP endpoint**：

```
POST /api/v1/agent-isolation/delegate
Authorization: Bearer <parent-agent-jwt>
{
  "agentName": "sub-executor-001",
  "requestedTools": ["exec", "read"],
  "roleName": ""    // 或填角色名走 SpawnableRoles 路径
}
```

#### 2.3.4 SDK 集成（子 Agent 侧零感知）

```python
# lattice-sdk-python

class LatticeAgent:
    def spawn(self, name: str, tools: list[str] | None = None, role: str | None = None):
        """父 Agent 调用，创建子 Agent。子进程自动继承环境变量。"""
        # 调用 DelegateToken API 获取一次性 token
        token = self._delegate(name, tools, role)
        # 在子进程环境中注入 token
        env = {
            **os.environ,
            "LATTICE_ENROLLMENT_TOKEN": token,
            "LATTICE_PARENT_AGENT_ID": self.agent_id,
            "LATTICE_SERVER_URL": self.server_url,
        }
        return subprocess.Popen(["python", "sub_agent.py"], env=env)

# 子 Agent 侧：自动检测环境变量，完成注册
class LatticeAgent:
    def __init__(self):
        if token := os.getenv("LATTICE_ENROLLMENT_TOKEN"):
            self._register_with_token(token)  # 走正常注册流程，自动携带 parentRef
```

---

### 2.4 gVisor AuditWriter → 控制面上报（PRO）

**流量事件表**：

```go
// internal/server/models/flow_event.go

type FlowEvent struct {
    ID        uint      `gorm:"primaryKey;autoIncrement"`
    TraceID   string    `gorm:"index;size:36"`   // 关联 tool_spans.trace_id
    AgentID   string    `gorm:"index;size:128"`
    Direction string    `gorm:"size:16"`          // egress | ingress
    DstIP     string    `gorm:"size:64"`
    DstPort   int
    Bytes     int64
    Ts        time.Time `gorm:"index"`
}
```

**PRO AuditWriter 改为 NATS 上报**：

```go
// start_sandbox_pro.go 中替换 fileAuditWriter

type natsAuditWriter struct {
    nc      *nats.Conn
    agentID string
}

func (w *natsAuditWriter) WriteAudit(agentID string, event shim.AuditEvent) error {
    payload, _ := json.Marshal(FlowAuditMsg{
        TraceID: activeTraceID(agentID), // 从内存 map 读取当前 traceID
        AgentID: agentID,
        Event:   event,
    })
    return w.nc.Publish("lattice.audit.flow", payload)
}
```

**控制面订阅**（`internal/server/controller/audit_consumer.go`）：

```go
// 订阅 NATS，写入 flow_events 表
nc.Subscribe("lattice.audit.flow", func(msg *nats.Msg) {
    var m FlowAuditMsg
    json.Unmarshal(msg.Data, &m)
    flowEventStore.Write(&models.FlowEvent{
        TraceID: m.TraceID,
        AgentID: m.AgentID,
        DstIP:   m.Event.DstIP,
        DstPort: m.Event.DstPort,
        Bytes:   m.Event.Bytes,
        Ts:      time.Now(),
    })
})
```

---

## 三、需要改动的现有设计总结

| 文件/模块 | 变动类型 | 说明 |
|-----------|---------|------|
| `start_sandbox_community.go` | 重写 | 去掉 "Pro feature" 错误，实现真正的社区版沙箱 |
| `start_sandbox_pro.go` | 瘦身 | 只保留 PRO 增强（加密审计、NATS 上报） |
| `agent_identity_types.go` | 新增字段 | `parentRef`、`spawnableRoles` |
| `agent_claims.go` | 新增字段 | `ParentAgentID` |
| `agent_registration.go` | 新增方法 | `DelegateToken()` |
| `ai.go` (ExecuteTool) | 微改 | 包一层 traceID 生成 + writeToolSpan |
| `agent_isolation_router.go` | 新增路由 | `POST /delegate`、`GET /audit/traces` |
| DB migrations | 新增 | `tool_spans`、`flow_events` 表 |
| `agent_enrollment.go` (model) | 新增字段 | `ParentAgentID` 透传 |

**不需要改的**：
- `AgentIsolationService.CheckToolAccess` — 已经正确实现
- `AgentRegistrationService.RegisterAgent` / `CreateEnrollmentToken` — 只需要加 Delegate 方法
- `gvisor/sandbox.go` — Config.AuditWriter 接口已经预留好，不需要改
- LatticePolicy CRD — 网络层策略已完整

---

## 四、实现顺序与工作量估算

```
Sprint 1（本周）: gVisor 移入 Community
  改动: start_sandbox_community.go 重写
  工作量: 1-2 天
  验收: make test-e2e 无需 IS_PRO=true 沙箱测试也通过

Sprint 2（下周）: MCP Trace Middleware
  改动: ToolSpan model + DB migration + ExecuteTool 包装 + 查询 API
  工作量: 3-5 天
  验收: 每次工具调用可在 /api/v1/audit/traces 查到记录

Sprint 3（2 周后）: Sub-agent Delegate API
  改动: CRD 字段 + AgentClaims + DelegateToken + HTTP endpoint
  工作量: 5-7 天
  验收: 子 Agent 用 delegate token 注册，AgentIdentity.parentRef 正确填写，
        calltree API 能还原父子关系

Sprint 4（3 周后）: AuditWriter → 控制面上报（PRO）
  改动: natsAuditWriter + NATS 订阅 + flow_events 表
  工作量: 5-7 天
  验收: 工具调用的网络流量能在 /api/v1/audit/traces/:id 里看到关联的流事件
```

**总计**：约 4 个 Sprint，3-4 周完成 Q2 2026 所有 P0/P1 项。

---

## 五、验收标准（Q2 2026 完成后）

1. **Community 用户** 可以用 `lattice-agent-sandbox start` 启动真正的 gVisor 沙箱，网络流量受 WireGuard 隔离，操作被记录到本地审计文件
2. **每次 MCP 工具调用** 在控制面可以查到：谁调用的、什么工具、结果、耗时
3. **Claude Code 类场景**：父 Agent 调用 `delegate` API 获得子 Token，子 Agent 以独立 WireGuard 身份注册，权限不超过父级（或来自管理员授权的 role template）
4. **PRO 版**：工具调用记录包含 traceID，gVisor 流量事件关联到对应工具调用
