# Agent Sandbox 安全评审与工具调用可观测性设计

> 日期: 2026-05-16
> 状态: 设计阶段
> 关联文档: `2026-05-11-agent-sandbox-and-ecosystem-design.md`, `2026-05-09-ai-agent-isolation-design.md`

---

## 一、当前设计安全评审

### 1.1 已有能力（做得好的部分）

当前 Agent Sandbox 在**网络隔离层**是扎实的：

| 能力 | 实现方式 |
|------|----------|
| Agent 加密身份 | WireGuard public key，密钥即身份，不可伪造 |
| 时间窗口限制 | TTL 自动销毁，AgentIdentity ExpiresAt + Manager reconciler |
| 默认拒绝策略 | iptables/eBPF default-deny，AllowedTools 白名单 |
| 用户态网络栈 | gVisor，Agent 进程无法直接调用内核网络接口 |
| 工具权限声明 | AgentIdentitySpec.AllowedTools + AllowedNamespaces |

**能阻止的威胁：**
1. 提示词注入后的横向移动（Lateral Movement）— WireGuard 在网络层硬拦截
2. 未授权 Agent 接入内部网络
3. Agent 长期存活（TTL 机制）
4. 工具权限滥用（AllowedTools 白名单）

---

### 1.2 当前设计的安全空白

#### 空白 1：主机进程隔离未实现

gVisor 只隔离了**网络层**。Agent 进程在 Pod 内仍可访问：
- 宿主文件系统（挂载点）
- 环境变量（包含 `$AWS_SECRET_KEY`、`$OPENAI_API_KEY` 等凭据）
- `/proc` 伪文件系统

设计文档 Phase 1 里的 cgroup/PID 绑定方案目前没有对应代码实现。

**风险等级：高**。进程内凭据泄露是提示词注入攻击后最常见的第二步。

#### 空白 2：Sub-agent 动态创建场景设计缺失

Claude Code、LangGraph、AutoGen 等现代 Agent 框架都会动态 spawn sub-agent。当前设计存在三个问题：

**问题 A — 权限平传**：父 Agent 把自己的 JWT 直接传给子 Agent 进程环境变量 → 子 Agent 拥有与父相同的权限 → 隔离失效。

**问题 B — 注册断层**：Enrollment Token 是单次使用的。父 Agent 运行时无法为子 Agent 实时生成新 Token，除非提前预备，但预备数量无法预知。

**问题 C — 审计不可追溯**：`AgentIdentity` 没有 `parentRef` 字段。审计日志看不出哪个操作是哪个子 Agent 发起的，调用链（call tree）不可见。

**风险等级：高**。Multi-agent 编排是当前 AI Agent 最主流的范式，这个空白让整个沙箱模型在真实场景中形同虚设。

#### 空白 3：工具调用审计形同虚设

`AuditLevel=write` 字段存在于 CRD，但：
- `shim.AuditWriter` 接口只有声明，没有实现
- MCP 工具调用没有流经任何审计路径
- 发生提示词注入后，事后溯源依赖的是应用层日志，不是结构化审计

**风险等级：中**。合规性要求（SOC2、HIPAA）明确要求审计记录不可篡改。

#### 空白 4：运行时动态策略调整缺失

Agent 运行中发现异常行为，唯一手段是删除 AgentIdentity（踢掉 Agent）。无法：
- 临时降级权限（禁用特定工具）
- 临时收紧 egress（仅允许访问指定 IP）
- 发出告警但不中断（audit-only 模式）

**风险等级：中**。实时响应能力是 Zero-Trust 模型的核心要求之一。

---

### 1.3 Sub-agent 权限模型详细分析

#### 父 Agent 启动子 Agent 的机制

子 Agent 是由父进程 fork/exec 启动的。父 Agent 无法"创造"新的控制面资源，除非它主动回调 Lattice 控制面。**SDK 自动委托**是最小适配路径：

```
父 Agent 启动子 Agent 时：
  环境变量: LATTICE_PARENT_JWT=<父的JWT>
            LATTICE_SERVER_URL=<控制面地址>
            LATTICE_SPAWNABLE_ROLES=executor,searcher  # 可选

子 Agent 的 Lattice SDK 启动时：
  1. 检测到 LATTICE_PARENT_JWT 存在
  2. 自动调用 POST /api/v1/agent-isolation/delegate
     { parentJWT, requestedTools: ["exec", "read"] }
  3. 控制面校验: requestedTools ⊆ parent.AllowedTools
  4. 签发子 JWT（权限 ≤ 父级），agentID 携带 parentRef
  5. 子 Agent 用子 JWT 走正常注册流程，获得独立 WireGuard IP
```

**Agent 适配工作量**：使用 Lattice SDK 的 Agent 零改动，SDK 透明处理。不使用 SDK 的 Agent 需要手动调用 delegate API（约 10 行代码）。

#### 子 Agent 需要比父 Agent 更大权限的场景

这是真实存在的：

```
Claude Code (orchestrator)
  权限: read-only，只做规划、读文件
  ↓ spawn
  Code Executor sub-agent
    需要: exec、write，权限大于 orchestrator
```

此时**不能从父级权限派生**，强行限制 ≤ 父级会让整个编排模式失效。

**解决方案：管理员预授权角色模板（Spawnable Roles）**

```yaml
# Workspace 管理员在控制面预定义
spawnableRoles:
  - name: executor
    allowedTools: ["exec", "write", "read"]
    maxTTL: 1h
  - name: searcher
    allowedTools: ["web_search", "read"]
    maxTTL: 30m
```

父 Agent 的 JWT claim 里包含 `spawnableRoles: ["executor", "searcher"]`。当父 Agent 请求 spawn 一个 executor 时：

1. 控制面检查：父 JWT 的 `spawnableRoles` 是否包含 `executor` → 包含
2. 签发 executor 级别的子 JWT（权限来自角色模板，不从父级派生）
3. 子 AgentIdentity 的 `parentRef` 指向父 AgentIdentity

**类比**：这与 Linux `sudo` 白名单机制相同——父进程不需要自身拥有某个权限，只需被预授权可以创建那个角色的子进程。

**关键原则**：
- 子 Agent 权限 ≤ 父级 → 走 delegate 派生，无需额外授权
- 子 Agent 权限 > 父级 → 必须来自管理员预定义的 spawnableRoles，父 Agent 只是触发者
- 任何情况下，子 AgentIdentity 都携带 `parentRef`，审计日志可还原完整调用树

---

## 二、工具调用可观测性设计（Lattice Trace）

### 2.1 目标

> 每一次 MCP 工具调用，都能知道：**谁调用的、调用了什么、触发了哪些网络流量、结果是什么**。参数明文存储（PRO），加密静态存放，管理员持有解密密钥。

### 2.2 整体架构

gVisor 使用**用户态网络栈**（wireguard-go → UDP socket），流量不经过 Linux 内核 TC 挂载点，因此不能用 eBPF TC hook 采集流量。正确的拦截点是 gVisor netstack 内部已预留的 `shim.AuditWriter` 钩子。

```
┌─────────────────────────────────────────────────────────────┐
│  Layer 3: 查询 & 可视化                                      │
│  Dashboard 调用时间轴 + REST Query API + Sub-agent 调用树     │
└────────────────────────┬────────────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────────────┐
│  Layer 2: 关联引擎                                           │
│  tool_span ←→ gVisor 流量事件（时间窗口匹配 或 NATS traceID）│
│  Sub-agent 调用树重建（parentRef 递归查询）                   │
└──────────────┬──────────────────────────┬───────────────────┘
               │                          │
┌──────────────▼──────────┐  ┌────────────▼─────────────────┐
│  Layer 1a:              │  │  Layer 1b:                    │
│  MCP Trace Middleware   │  │  gVisor AuditWriter           │
│  （工具调用拦截 + 加密） │  │  （netstack 内部流量钩子）   │
└─────────────────────────┘  └──────────────────────────────┘
```

**traceID 传递方案：**

| 方案 | 实现 | 精度 | 复杂度 |
|------|------|------|--------|
| 时间窗口匹配（社区版） | AuditWriter 无条件记录所有流量+时间戳，事后按工具调用时间窗口关联 | 中（并发调用时可能混淆） | 低 |
| NATS 推送 traceID（PRO） | MCP server 在工具调用前后发 NATS 消息，sandbox 进程订阅并更新内存 active trace map | 高（精确到单次调用） | 中 |

### 2.3 Layer 1a：MCP Trace Middleware

在 MCP Server 每个工具 handler 前后加拦截器：

```go
// internal/server/mcp/trace_middleware.go

func TraceMiddleware(auditStore AuditStore, encryptor Encryptor, nats NATSClient) ToolMiddleware {
    return func(next ToolHandler) ToolHandler {
        return func(ctx context.Context, req ToolRequest) ToolResponse {
            traceID := uuid.New().String()
            start := time.Now()

            // PRO: 通知 sandbox 进程当前 traceID，用于精确流量关联
            // 社区版跳过此步，依赖事后时间窗口匹配
            nats.Publish("lattice.trace.active", TraceEvent{
                AgentID: req.AgentID,
                TraceID: traceID,
                Active:  true,
            })
            defer nats.Publish("lattice.trace.active", TraceEvent{
                AgentID: req.AgentID,
                TraceID: traceID,
                Active:  false,
            })

            resp := next(ctx, req)

            span := &ToolSpan{
                TraceID:       traceID,
                AgentID:       req.AgentID,       // 从 JWT claim 取
                ParentID:      req.ParentAgentID, // sub-agent 场景，可为空
                Tool:          req.Tool,
                ArgsEncrypted: encryptor.Encrypt(req.Args), // PRO: 信封加密
                ArgsHash:      sha256(req.Args),             // 社区版
                Status:        resp.Status,
                ErrorMsg:      resp.Error,
                DurationMs:    time.Since(start).Milliseconds(),
                StartedAt:     start,
            }
            auditStore.WriteSpan(span)
            return resp
        }
    }
}
```

#### 参数加密方案（PRO）

采用 **AES-256-GCM + 信封加密**：

```
参数明文
  → AES-256-GCM 加密（data key，每次工具调用随机生成）
  → data key 用 workspace 级 RSA public key 加密
  → 密文 + 加密的 data key 存入 audit store

解密路径（仅管理员）:
  → 管理员持有 RSA private key（离线保存或 KMS）
  → 解密 data key → 解密参数明文
  → 审计界面展示，操作留有日志
```

这样即使 audit store 被拖库，参数明文不会泄露。管理员解密操作本身也会被审计。

### 2.4 Layer 1b：gVisor AuditWriter（netstack 内部流量钩子）

gVisor sandbox 使用用户态网络栈，流量不经过内核 TC，**不能用 eBPF**。正确做法是实现 `shim.AuditWriter` 接口，该接口在 gVisor netstack 内部处理每个 IP 包时被调用。

```go
// internal/agent/gvisor/audit_writer.go

// LatticeAuditWriter 实现 shim.AuditWriter，采集 gVisor netstack 内部流量事件。
type LatticeAuditWriter struct {
    store      FlowEventStore
    activeTrace sync.Map // agentID → traceID（PRO: 由 NATS 消息更新）
}

// WriteFlow 由 gVisor netstack 每处理一个 IP 包时调用。
// direction: "ingress" | "egress"
func (w *LatticeAuditWriter) WriteFlow(agentID, direction string, pkt FlowPacket) {
    traceID := ""
    if v, ok := w.activeTrace.Load(agentID); ok {
        traceID = v.(string) // PRO: 精确 traceID
    }

    w.store.WriteFlow(&FlowEvent{
        TraceID:  traceID, // 社区版为空，事后按时间窗口关联
        AgentID:  agentID,
        Direction: direction,
        DstIP:    pkt.DstIP,
        DstPort:  pkt.DstPort,
        Bytes:    pkt.Len,
        Ts:       time.Now(),
    })
}

// OnTraceEvent 由 NATS 订阅者调用，更新 active trace map（PRO）。
func (w *LatticeAuditWriter) OnTraceEvent(evt TraceEvent) {
    if evt.Active {
        w.activeTrace.Store(evt.AgentID, evt.TraceID)
    } else {
        w.activeTrace.Delete(evt.AgentID)
    }
}
```

**注入到 Sandbox 初始化：**

```go
// cmd/lattice-agent-sandbox/cmd/start_sandbox_pro.go

auditWriter := &gvisor.LatticeAuditWriter{store: flowStore}

// PRO: 订阅 NATS trace 事件，精确关联 traceID
natsClient.Subscribe("lattice.trace.active", func(evt TraceEvent) {
    auditWriter.OnTraceEvent(evt)
})

sb, err := gvisor.New(gvisor.Config{
    ID:          sandboxName,
    LocalIP:     sandboxLocalIP,
    AuditWriter: auditWriter, // 注入钩子
    // ...
})
```

**流量事件流：**

```
Agent 进程发起 HTTP 请求
  → gVisor netstack 处理 TCP 包
  → shim.AuditWriter.WriteFlow() 被调用
  → 记录 {traceID, dstIP, dstPort, bytes, ts}
  → 写入 FlowEventStore（本地 SQLite 或上报到 latticed）
```

### 2.5 Layer 2：关联存储

```sql
-- 工具调用表
CREATE TABLE tool_spans (
    trace_id       TEXT        PRIMARY KEY,
    agent_id       TEXT        NOT NULL,
    parent_id      TEXT,                    -- sub-agent: 父 agent_id
    tool           TEXT        NOT NULL,
    args_hash      TEXT        NOT NULL,    -- sha256，社区版用
    args_encrypted BLOB,                   -- PRO: 信封加密的密文
    args_key       BLOB,                   -- PRO: RSA 加密的 data key
    status         TEXT,                   -- ok | error | timeout
    error_msg      TEXT,
    duration_ms    INTEGER,
    started_at     DATETIME    NOT NULL
);

-- 网络流量表
CREATE TABLE flow_events (
    id         INTEGER  PRIMARY KEY AUTOINCREMENT,
    trace_id   TEXT     NOT NULL REFERENCES tool_spans(trace_id),
    src_ip     TEXT     NOT NULL,
    dst_ip     TEXT     NOT NULL,
    dst_port   INTEGER,
    bytes      INTEGER,
    ts         DATETIME NOT NULL
);

-- 常用查询索引
CREATE INDEX idx_spans_agent    ON tool_spans(agent_id, started_at);
CREATE INDEX idx_spans_parent   ON tool_spans(parent_id);
CREATE INDEX idx_flows_trace    ON flow_events(trace_id);
CREATE INDEX idx_flows_dst      ON flow_events(dst_ip, dst_port);
```

### 2.6 Layer 3：查询 API

```
# 按 Agent 查询工具调用历史
GET /api/v1/audit/traces?agentId=xxx&from=2026-05-16T00:00:00Z&to=...&limit=100

# 单次工具调用详情（含网络流量）
GET /api/v1/audit/traces/:trace_id

# Sub-agent 完整调用树（递归展开）
GET /api/v1/audit/agents/:agentId/calltree?rootTraceId=xxx

# PRO: 管理员解密参数明文（此操作本身被审计）
POST /api/v1/audit/traces/:trace_id/decrypt
  Authorization: Bearer <admin-jwt>
  { "privateKeyPem": "..." }  # 或配置 KMS 路径
```

### 2.7 Dashboard 时间轴视图

```
Agent: code-executor-001  [2026-05-16 16:33:40 - 16:33:43]
  ├─ 16:33:40.120  exec("ls /etc")              2ms   ✅  出流量: 0 bytes
  ├─ 16:33:41.050  web_search("CVE-2024-1234")  45ms  ✅  出流量: → 10.0.7.5:443  1.2KB  ⚠️ 非预期目标
  ├─ 16:33:42.800  read_file("/etc/passwd")      1ms   ✅  出流量: 0 bytes
  └─ 16:33:43.100  [spawn sub-agent]
       └─ Agent: code-executor-001-exec-7f3a  [sub-agent]
            └─ 16:33:43.500  exec("curl attacker.com")  ❌ BLOCKED by policy
```

Sub-agent 调用以树形嵌套展示，可折叠。异常流量（访问非白名单 IP）自动标红。

### 2.8 社区版 vs PRO 功能边界

| 能力 | 社区版 | PRO |
|------|:------:|:---:|
| 工具调用日志（tool、status、时长） | ✅ | ✅ |
| 参数 sha256 哈希 | ✅ | ✅ |
| gVisor 流量关联（trace_id） | ❌ | ✅ |
| 参数明文加密存储（信封加密） | ❌ | ✅ |
| 管理员解密查看参数 | ❌ | ✅ |
| Sub-agent 调用树可视化 | ❌ | ✅ |
| 异常流量告警（访问非白名单） | ❌ | ✅ |
| OpenTelemetry 导出（Jaeger/Tempo） | ❌ | ✅ |

---

## 三、Community vs PRO 分界线设计原则

### 3.1 分界线应该怎么划

成熟开源商业产品（HashiCorp、Grafana、Tailscale）的通行做法：**先在设计阶段定好分界线，Community 先完整实现，PRO 通过接口替换/扩展**。不是 PRO 做完再拆分——事后拆分几乎必然导致架构腐化（if/else 散落各处或维护两套重复代码）。

**错误的划法**：按实现复杂度（"这个难，放 PRO"）

**正确的划法**：按买单人画像

| | Community | PRO |
|-|-----------|-----|
| 目标用户 | 个人开发者、小团队、自托管 | 企业安全团队、合规官 |
| 付费主体 | 不付费 | CISO、IT 采购 |
| 核心需求 | 产品真正好用、建立信任 | 审计报告、合规导出、加密存储、精细管控 |

**关键原则**：核心安全能力给 Community（让用户建立信任），企业合规、可观测性深度、精细管控给 PRO（企业采购标准清单）。

### 3.2 Lattice Agent Sandbox 分界线建议

当前设计将 gVisor 沙箱整体放入 PRO，存在风险：Community 用户体验到的"沙箱"只是普通 K8s Pod + seccomp，隔离感很薄弱，难以建立产品信任，更不会产生升级 PRO 的动力。

建议调整分界线：

| 功能 | 建议 | 理由 |
|------|------|------|
| gVisor 网络隔离（核心沙箱） | **Community** | 这是建立"Agent 安全"信任的基础，Community 必须感受到真实保护 |
| WireGuard 身份 + TTL 自动销毁 | Community（已做） | 正确 |
| default-deny 网络策略 | Community（已做） | 正确 |
| AllowedTools 白名单 | Community（已做） | 正确 |
| 工具调用日志（hash only） | Community | 审计可见性基础 |
| **参数明文加密审计** | PRO | 合规要求，企业采购标准 |
| **gVisor 流量 ↔ 工具调用关联** | PRO | 深度可观测性，安全团队需求 |
| **Sub-agent 调用树可视化** | PRO | 企业多 Agent 编排场景 |
| **Spawnable Roles 精细管控** | PRO | 企业权限治理 |
| **异常流量实时告警** | PRO | 企业 SOC 集成需求 |
| **合规报告导出** | PRO | SOC2/HIPAA 直接需求 |
| **OpenTelemetry 导出** | PRO | 企业可观测性平台集成 |

### 3.3 实现节奏

```
设计阶段  → 定好接口 + 分界线（本文档）
    ↓
Sprint 1  → Community 完整实现（接口有真实默认行为，不是空 stub）
    ↓
Sprint 2  → PRO 实现替换/扩展（build tag 切换，不改接口）
    ↓
持续迭代  → 新功能先问"Community 能给到什么程度"，再设计 PRO 增强
```

Lattice 现有的 `//go:build pro` + 接口 stub 机制已经是正确方向，问题不在机制，在分界线画在哪里。

---

## 四、实现优先级建议

| 优先级 | 项目 | 价值 | 复杂度 |
|--------|------|------|--------|
| P0 | MCP Trace Middleware（社区版，只记哈希） | 审计基础，无此后续无从建立 | 低 |
| P0 | Sub-agent delegate API + JWT parentRef | 当前最大设计空白 | 中 |
| P1 | gVisor AuditWriter 流量采集（PRO） | PRO 核心差异点 | 中 |
| P1 | 参数信封加密 + 管理员解密 API | 合规要求（SOC2 审计条款） | 中 |
| P2 | Dashboard 调用时间轴 + Sub-agent 树 | 产品可见性 | 中 |
| P2 | 异常流量告警 | 实时响应能力 | 中 |
| P3 | Spawnable Roles 管理员配置 UI | 完善 sub-agent 权限模型 | 低 |
| P3 | OpenTelemetry 导出 | 企业集成需求 | 低 |
