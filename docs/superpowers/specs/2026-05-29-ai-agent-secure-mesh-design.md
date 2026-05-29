# Lattice AI Agent Secure Mesh — 设计文档

**日期**：2026-05-29  
**状态**：草稿  
**受众**：内部技术决策、产品路线图、开发者参考

---

## 一、重新定位

### 1.1 从网络基础设施到 AI Agent Secure Mesh

Lattice 原有定位：Kubernetes-native WireGuard overlay 网络基础设施。

**新定位**：AI Agent Secure Mesh——AI agent 系统的统一安全层。

不区分对面是 agent、MCP server 还是外部 API，所有 agent 通信都经过同一套 **identity + policy + audit** 机制。

### 1.2 与现有工具的边界

| 工具 | 定位 | 与 Lattice 的关系 |
|---|---|---|
| Istio / Tetrate | 微服务通信安全（代码确定，行为可预期） | 互补，Lattice overlay ↔ Istio mesh 是集成点 |
| Kong / APISIX | API Gateway（ingress 层） | 不同层，不冲突 |
| LangSmith / Helicone | LLM observability | 只做观测，无网络隔离和策略 |
| Lattice | **AI agent 通信安全**（自主决策，行为需监督） | 本文档描述的定位 |

Lattice 的独特价值：代码行为不可预期的 AI agent，需要的不只是"能不能访问"（Istio 能做），更需要"调了什么工具、传了什么参数、行为是否异常"。

---

## 二、问题定义

当前 AI agent 系统存在四个安全盲区：

**1. 没有 agent 级身份**
微服务有 mTLS + Kubernetes service account，AI agent 只有进程级权限，换台机器就是"新人"，无法追踪跨环境的 agent 行为。

**2. 策略粒度太粗**
"agent 能访问内网"不够。需要的是"agent-worker-1 只能调 file-server 的 read_file，不能调 exec_command"，当前没有任何工具做到工具调用级别的访问控制。

**3. 没有语义审计**
日志记录"TCP connect to 10.0.7.2:3000"，但需要的是"agent 调了 read_file('/etc/passwd')，返回 4KB，耗时 23ms"。网络事件和工具调用之间的关联是 AI agent 安全分析的核心，当前完全缺失。

**4. 失控 agent 没有即时处置手段**
Kill pod 慢且不一定有效。需要密码学级别的即时吊销：吊销 AgentIdentity，该 agent 的 WireGuard key 立即失效，所有 peer 拒绝与其通信。

---

## 三、架构全景

```
┌─────────────────────────────────────────────────────────────┐
│                   AI Agent Secure Mesh                       │
├──────────────┬───────────────────┬──────────────────────────┤
│   身份层      │    策略层          │     审计层               │
│              │                   │                          │
│ AgentIdentity│ LatticePolicy     │ 语义审计                  │
│ enrollment   │ (peer 级，已有)    │ MCP 调用日志              │
│ token        │ AgentPolicy       │ 工具调用 + 网络事件关联    │
│ 即时吊销      │ (工具级，新增)     │                          │
├──────────────┴───────────────────┴──────────────────────────┤
│                      传输层                                   │
│          WireGuard overlay（P2P 加密，跨云/跨集群）            │
├─────────────────────────────────────────────────────────────┤
│                      接入层                                   │
│  lattice-run（单容器）  │  K8s agent pod  │  MCPServer peer  │
└─────────────────────────────────────────────────────────────┘
```

### 三种交互模式，统一处理

```
Agent ↔ Agent：      WireGuard P2P + LatticePolicy（已有）
Agent → MCP（内部）：WireGuard P2P + AgentPolicy 工具级策略 + MCP 审计（新增）
Agent → MCP（外部）：HTTPS 直连 + HTTP proxy 拦截 + AgentPolicy 工具级策略 + MCP 审计（新增）
Agent → 外网/内网：  egress 策略 + 出口控制（Phase 3 扩展）
```

**关键认知：大多数 MCP server 不在 overlay 里。**

现实中 MCP server 的分布：
- **平台级外部 MCP（80%+）**：GitHub、Stripe、Notion 等平台的 MCP，通过 HTTPS 访问，完全在 Lattice overlay 之外
- **公司内网 MCP（~10%）**：内部 API、数据库 MCP，可选择加入 overlay，也可不加
- **本地 MCP（~5%）**：filesystem、本地工具，同机运行

因此 Lattice 的 MCP 安全层必须**对两种模式都有效**，而不能要求 MCP server 必须是 LatticePeer。

---

## 四、路线图

### Phase 1（本文档详细设计）：架构清理 + MCP 安全传输

- 去掉 gVisor netstack（sandbox 改 kernel TUN + SOCKS5）
- 废弃 sandbox sidecar
- 新增 MCPServer CRD
- 新增 AgentPolicy CRD（工具级策略）
- MCP 调用审计（结构化日志）
- 修复 e2e 测试

### Phase 2（中期）：语义审计增强 + 多 agent 拓扑

- 审计查询 API
- MCP 调用链追踪（工具调用 + 网络事件时序关联）
- 异常检测（调用频率、访问模式变化告警）
- UI：agent 通信拓扑实时图

### Phase 3（后期）：出口管控 + 高安全模式

- DNS 级出口过滤（agent 只能访问允许的域名，不只是 IP）
- Firecracker 模式（code execution 工具的 MicroVM 强隔离）
- seccomp profile 内置（堵死 raw socket 逃逸，用户零感知）

---

## 五、Phase 1 详细设计

### Block 1：架构清理

#### 1.1 问题：gVisor 的误用

当前 `sandbox run` 和 `sandbox sidecar` 使用 gVisor netstack 作为 wireguard-go 的 CustomTUN，动机是"不想创建 kernel wg0"。

这个选择带来了三个问题：

1. **两个网络世界并存**：kernel netstack 和 gVisor userspace netstack 各自独立，所有问题都要考虑两套处理路径。
2. **桥接组件爆炸**：tproxy（出向桥接）+ ForwardListener（入向桥接）+ iptables REDIRECT 全部是为了弥合两个世界之间的鸿沟。
3. **误解 gVisor 的价值**：gVisor 的核心价值是"syscall 拦截提供进程隔离"，当前用法只是"用它的 channel 作为 TUN 接口"，既没得到进程隔离，又引入了大量复杂度。

参考 Tailscale：其主要产品（tailscaled、iOS/Android App）均使用 OS 级 TUN 接口 + wireguard-go，而非 gVisor。gVisor netstack 仅用于 tsnet（应用嵌入式库），tsnet 本身也不是 Tailscale 核心产品的组成部分。

#### 1.2 sandbox run 改造

**数据路径对比**：

```
改前：
  AI agent（UID 999）
    → kernel socket
    → iptables OUTPUT REDIRECT
    → tproxy（SO_ORIGINAL_DST）
    → gVisor netstack（CustomTUN）
    → wireguard-go → WireGuard UDP

改后：
  lattice-run 启动序列：
    1. registerOrResume → 获取 overlay IP + tier
    2. infra.CreateTUN() → 创建 kernel TUN（与普通 agent 一致）
    3. 启动 wireguard-go（使用 kernel TUN）
    4. 启动 shim.Socks5Server → 监听 127.0.0.1:{随机端口}
    5. 设置 ALL_PROXY=socks5h://127.0.0.1:{port}
    6. fork AI agent（无需 UID 999，无需 iptables）

  AI agent TCP 出向：
    requests/httpx/openai-sdk → 读 ALL_PROXY → SOCKS5 → WireGuard → peer
```

**保留的组件**：
- `registerOrResume`、tier 判断逻辑
- `shim.EgressFilter`、`shim.AuditWriter`（policy + audit 仍通过 shim）
- `shim.Socks5Server`（出向代理）

**删除的组件**：
- `gvisor.NewTUNAdapter`、`gvisor.InjectIntoChannel`
- `installRunIPTables`、UID 999 的 `forkAndWait` 逻辑
- `tproxy.Proxy`
- `gvisor.NewSandboxProvisionerFactory`

**局限性说明**（写入文档，不回避）：
- SOCKS5 只对遵从 `ALL_PROXY` 环境变量的库有效（Python requests/httpx/aiohttp ✓，Go 原生 net/http ✗）
- 对不遵从 ALL_PROXY 的 AI agent，可回退到 iptables 模式（保留 `--use-iptables` flag）
- UDP 流量不被拦截（与改前行为一致）

#### 1.3 sandbox sidecar 处理

- `sidecar.go` 顶部加 `// Deprecated: use lattice-run instead. Will be removed in v0.6.`
- 停止新增功能，bug fix 按情况处理
- e2e 测试 `agent_sandbox_test.go` 改为测试 `sandbox run` 路径

#### 1.4 e2e 测试修复

当前已知问题（随 Block 1 修复）：

| 问题 | 原因 | 修复 |
|---|---|---|
| `wget: can't connect to remote host (10.0.7.3): Connection refused` | sidecar 缺少 ForwardListener | 改用 sandbox run 模式，companion→sandbox 改为 sandbox→companion |
| `liveness: handshake query failed ... device : file does not exist` | `node.go` 用 `config.Conf.InterfaceName`（空）而非 `node.Name` | 已修复（`node.Name` 替换） |

e2e 测试重构方向：BeforeAll 改为让 sandbox agent 主动 wget companion（出向测试），不再依赖 companion→sandbox 的入向连接。

---

### Block 2：MCPServer CRD

MCPServer 支持两种模式：

| 模式 | 场景 | peerName | endpoint |
|---|---|---|---|
| **内部模式** | MCP server 加入了 Lattice overlay | 必填（LatticePeer 名称） | 本地地址（如 `http://localhost:3000/mcp`） |
| **外部模式** | 平台级 MCP，不在 overlay 内 | 空 | 完整外部 URL（如 `https://mcp.github.com`） |

#### 2.1 CRD 定义

```go
// api/v1alpha1/mcp_server_types.go

type MCPServer struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec   MCPServerSpec   `json:"spec,omitempty"`
    Status MCPServerStatus `json:"status,omitempty"`
}

type MCPServerSpec struct {
    // PeerName 是对应的 LatticePeer 名称（可选）。
    // 有值 → 内部模式：MCP server 通过 WireGuard overlay 访问。
    // 空   → 外部模式：MCP server 通过 endpoint URL 直接访问（HTTPS）。
    PeerName string `json:"peerName,omitempty"`

    // Endpoint 是 AI agent 访问该 MCP server 时使用的地址。
    // 内部模式：MCP server 本地监听的地址（如 "http://localhost:3000/mcp"）。
    // 外部模式：完整的外部 URL（如 "https://mcp.github.com"）。
    Endpoint string `json:"endpoint"`

    // Tools 声明该 server 暴露的工具列表，供 AgentPolicy 引用和 UI 展示。
    Tools []MCPTool `json:"tools,omitempty"`
}

type MCPTool struct {
    Name        string    `json:"name"`
    Description string    `json:"description,omitempty"`
    RiskLevel   RiskLevel `json:"riskLevel,omitempty"` // low/medium/high/critical
}

// RiskLevel 用于 UI 展示和策略建议，不直接影响策略执行
type RiskLevel string

const (
    RiskLevelLow      RiskLevel = "low"
    RiskLevelMedium   RiskLevel = "medium"
    RiskLevelHigh     RiskLevel = "high"
    RiskLevelCritical RiskLevel = "critical"
)

type MCPServerStatus struct {
    Phase        MCPServerPhase `json:"phase,omitempty"`
    Mode         string         `json:"mode,omitempty"`         // "internal" | "external"
    PeerAddress  string         `json:"peerAddress,omitempty"`  // 仅内部模式有值
    LastSyncedAt *metav1.Time   `json:"lastSyncedAt,omitempty"`
}

type MCPServerPhase string

const (
    MCPServerPhasePending  MCPServerPhase = "Pending"
    MCPServerPhaseReady    MCPServerPhase = "Ready"
    MCPServerPhaseDegraded MCPServerPhase = "Degraded"
)
```

YAML 示例：

```yaml
# 内部模式：数据库 MCP，已加入 Lattice overlay
apiVersion: lattice.io/v1alpha1
kind: MCPServer
metadata:
  name: db-tools
  namespace: my-workspace
spec:
  peerName: db-mcp-server          # 对应已注册的 LatticePeer
  endpoint: "http://localhost:3000/mcp"
  tools:
    - name: query_db
      riskLevel: high
    - name: list_tables
      riskLevel: low
---
# 外部模式：平台级 MCP，不在 overlay 内
apiVersion: lattice.io/v1alpha1
kind: MCPServer
metadata:
  name: github-mcp
  namespace: my-workspace
spec:
  # 没有 peerName
  endpoint: "https://api.githubcopilot.com/mcp"
  tools:
    - name: create_issue
      riskLevel: high
    - name: list_repos
      riskLevel: low
    - name: create_pull_request
      riskLevel: high
```

#### 2.2 Controller 职责

```
reconcile MCPServer：

  if spec.peerName != "":  // 内部模式
    1. 查找 spec.peerName 对应的 LatticePeer
    2. 若不存在或 phase != Ready → status.phase = Pending/Degraded
    3. 若 Ready → status.peerAddress = peer.Status.AllocatedAddress
    4. status.mode = "internal", status.phase = Ready

  else:  // 外部模式
    1. 验证 spec.endpoint 是合法 URL（scheme 必须是 http 或 https）
    2. status.mode = "external", status.phase = Ready（不依赖任何 peer）
    3. status.peerAddress = ""（外部模式无 overlay IP）

  更新 status.lastSyncedAt = now
  controller 不创建 LatticePeer，LatticePeer 由 MCP server 自行注册。
```

#### 2.3 API 端点

```
GET    /api/v1/mcp-servers              列出 workspace 下所有 MCPServer
POST   /api/v1/mcp-servers              创建 MCPServer（内部或外部模式均支持）
GET    /api/v1/mcp-servers/:name        获取单个
PUT    /api/v1/mcp-servers/:name        更新（tools 声明等）
DELETE /api/v1/mcp-servers/:name        删除
GET    /api/v1/mcp-servers/:name/tools  列出该 server 的工具列表（供 AgentPolicy 引用）
```

---

### Block 3：AgentPolicy CRD + MCP Proxy

#### 3.1 CRD 定义

```go
// api/v1alpha1/agent_policy_types.go

type AgentPolicy struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec   AgentPolicySpec   `json:"spec,omitempty"`
}

type AgentPolicySpec struct {
    // AgentSelector 选择受此策略约束的 agent（匹配 AgentIdentity labels）
    AgentSelector metav1.LabelSelector `json:"agentSelector"`
    // AllowedTools 白名单：未列出的工具调用一律拒绝（当 DefaultDeny=true 时）
    AllowedTools []AgentToolPermission `json:"allowedTools,omitempty"`
    // DefaultDeny 为 true 时，未匹配 AllowedTools 的工具调用被拒绝
    DefaultDeny bool `json:"defaultDeny,omitempty"`
}

type AgentToolPermission struct {
    // MCPServer 引用同 namespace 下的 MCPServer 名称
    MCPServer string   `json:"mcpServer"`
    // Tools 允许调用的工具名列表；["*"] 表示允许该 server 的所有工具
    Tools     []string `json:"tools"`
}
```

YAML 示例：

```yaml
apiVersion: lattice.io/v1alpha1
kind: AgentPolicy
metadata:
  name: worker-policy
  namespace: my-workspace
spec:
  agentSelector:
    matchLabels:
      role: worker
  defaultDeny: true
  allowedTools:
    - mcpServer: file-tools
      tools: ["read_file"]
    - mcpServer: http-tools
      tools: ["http_get", "http_post"]
```

#### 3.2 MCP Proxy（策略执行点）

MCP proxy 内嵌在 `lattice-run` 进程中，作为 **HTTP 层代理**（不是 SOCKS5）。SOCKS5 是盲 TCP 隧道，无法检查请求体；MCP 使用 HTTP POST + JSON-RPC，必须在 HTTP 层拦截。

`lattice-run` 同时启动两个代理：
- **SOCKS5**（`ALL_PROXY`）：处理所有非 MCP 的通用 TCP 出向流量
- **HTTP proxy**（`HTTP_PROXY` / `HTTPS_PROXY`）：处理所有 HTTP 出向流量，对 MCP 请求做 JSON-RPC 检查

**MCP proxy 同时支持内部和外部 MCP server：**

```
内部 MCP（MCPServer.peerName 有值）：
  AI agent
    → HTTP POST http://10.0.7.5:3000/mcp  (读 HTTP_PROXY)
    → MCP Proxy
        ├── 识别目标 IP 10.0.7.5 = MCPServer "db-tools" 的 overlay peer
        ├── 解析 JSON-RPC → tool name = "query_db"
        ├── 查 AgentPolicy → ALLOW/DENY
        ├── ALLOW → 通过 WireGuard overlay 转发（kernel 路由到 wf0）
        └── DENY  → 返回 MCP 错误 + 写审计

外部 MCP（MCPServer.peerName 为空）：
  AI agent
    → HTTP POST https://mcp.github.com  (读 HTTP_PROXY / HTTPS_PROXY)
    → MCP Proxy
        ├── 识别目标 host "mcp.github.com" = MCPServer "github-mcp" 的 endpoint
        ├── 解析 JSON-RPC → tool name = "create_issue"
        ├── 查 AgentPolicy → ALLOW/DENY
        ├── ALLOW → 直接 HTTPS 转发到 mcp.github.com（正常互联网）
        └── DENY  → 返回 MCP 错误 + 写审计

非 MCP 流量（不匹配任何 MCPServer endpoint）：
  AI agent → HTTP_PROXY → 透传（不解析 JSON-RPC，只做 IP/CIDR 级过滤）
```

**MCPServer 匹配逻辑**：
- 内部模式：对比目标 IP 与已知 MCPServer 的 `status.peerAddress`（overlay IP）
- 外部模式：对比请求的 URL host 与 MCPServer 的 `spec.endpoint` host
- 匹配到 MCPServer → 进入 MCP JSON-RPC 检查流程
- 未匹配 → 透传，不做 JSON-RPC 解析

**策略缓存**：
- 启动时从 API server 拉取全量 AgentPolicy + MCPServer 列表
- 15s 定时刷新
- NATS 推送策略变更事件，实时失效

**分层策略**：

```
内部 MCP：
  LatticePolicy（网络层）：agent IP 能不能到 MCP server overlay IP
  AgentPolicy（语义层） ：agent 能不能调该工具
  两层都过才 ALLOW

外部 MCP：
  AgentPolicy（语义层）：agent 能不能调该工具（唯一检查层）
  网络层无法拦截（外部 HTTPS 绕过 WireGuard）
  → 因此 HTTP proxy 是外部 MCP 的唯一策略执行点，必须经过 HTTP_PROXY 才有效
```

**局限性（外部 MCP）**：
- 依赖 AI agent 遵从 `HTTP_PROXY` 环境变量（Python requests/httpx ✓，Go net/http ✓，原生 curl ✓）
- 不遵从 HTTP_PROXY 的 agent 可绕过外部 MCP 策略（此时回退到 IP/CIDR 级 EgressFilter）
- 外部 MCP server 本身无法验证 → 只能控制 agent 侧行为，无法端到端身份验证

#### 3.3 API 端点

```
GET    /api/v1/agent-policies           列出 workspace 下所有 AgentPolicy
POST   /api/v1/agent-policies           创建
GET    /api/v1/agent-policies/:name     获取单个
PUT    /api/v1/agent-policies/:name     更新
DELETE /api/v1/agent-policies/:name     删除
```

---

### Block 4：MCP 审计

#### 4.1 审计事件结构

```json
{
  "timestamp": "2026-05-29T10:00:00.123Z",
  "agentName": "worker-1",
  "agentIdentity": "did:lattice:abc123",
  "namespace": "my-workspace",
  "mcpServer": "file-tools",
  "tool": "read_file",
  "params": {
    "path": "/data/report.pdf"
  },
  "paramsSummary": "path=/data/report.pdf",
  "resultSize": 4096,
  "latencyMs": 23,
  "verdict": "allow",
  "denyReason": "",
  "network": {
    "srcIP": "10.0.7.3",
    "dstIP": "10.0.7.5",
    "dstPort": 3000
  },
  "sessionId": "sess-xyz789"
}
```

#### 4.2 参数处理规则

| 参数类型 | 处理方式 |
|---|---|
| 数值、布尔 | 直接记录 |
| 字符串 ≤ 200 字节 | 直接记录 |
| 字符串 > 200 字节 | 前 100 字节 + `...[truncated, total=Nk]` |
| key 含 password/token/secret/key/auth | 替换为 `[REDACTED]` |

#### 4.3 写入策略

```
本地文件（所有 tier）：
  路径：/tmp/lattice-audit.jsonl
  格式：JSONL，每行一个事件
  flush：每 100 条或每 5s，进程退出时强制 flush

API 上报（Pro tier）：
  端点：POST /api/v1/audit/events（批量）
  触发：积累 100 条或每 5s
  失败处理：本地保留，下次重试，不丢事件
```

#### 4.4 e2e 测试覆盖

- 正常工具调用 → `/tmp/lattice-audit.jsonl` 包含 `verdict=allow` 事件
- AgentPolicy 拒绝 → 包含 `verdict=deny` + `denyReason` 事件
- 参数截断验证：超长参数正确截断
- 敏感 key 脱敏验证：`password` 字段替换为 `[REDACTED]`

---

## 六、关键设计决策记录

| 决策 | 选择 | 原因 |
|---|---|---|
| gVisor 去留 | 从主数据路径移除 | gVisor 在 Tailscale 等主流产品中不作为核心组件；其价值是 syscall 隔离而非网络策略；在 Lattice 中是权限规避工具的误用 |
| 出向代理机制 | SOCKS5（ALL_PROXY）替换 tproxy+iptables | 无需 iptables 权限；shim 已有现成实现；对 Python/Node AI agent 透明 |
| 策略执行点 | agent 侧 MCP proxy | 策略跟着 agent 走；不需要在每个 MCP server 部署 proxy |
| sandbox sidecar | 废弃而非删除 | 保留现有用户的兼容性窗口 |
| AgentPolicy 与 LatticePolicy 分层 | 独立 CRD，分层执行 | 关注点分离；网络层策略和语义层策略互不干扰 |
| gVisor 的正确使用场景 | code execution 工具的进程隔离（Phase 3） | gVisor/MicroVM 用于真正需要进程隔离的场景，不混入网络策略层 |
| MCPServer.peerName | 可选字段（非必填） | 现实中 80%+ 的 MCP server 是平台级外部服务（GitHub、Stripe 等），不在 Lattice overlay 内，要求 peerName 会排除绝大多数实际场景 |
| 外部 MCP 的策略执行 | HTTP proxy 层（HTTP_PROXY env var） | 外部 MCP 不经过 WireGuard，LatticePolicy 无效；唯一可靠的拦截点是 HTTP 层代理，与 SOCKS5 并列启动 |
| 外部 MCP 的身份验证 | 不支持（仅控制 agent 侧行为） | 外部 MCP server 无法验证身份，端到端身份验证需要 mTLS 或 OAuth，超出 Phase 1 范围 |
