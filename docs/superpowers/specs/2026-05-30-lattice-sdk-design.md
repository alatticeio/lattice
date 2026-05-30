# Lattice SDK 设计文档

**日期**：2026-05-30
**状态**：Draft
**关联文档**：
- [AI Agent Zero Trust 定位](./2026-05-30-ai-agent-zero-trust-positioning.md)
- [AI Agent Secure Mesh](./2026-05-29-ai-agent-secure-mesh-design.md)

---

## 一、目标

让任意 AI agent（LangChain、LangGraph、CrewAI、AutoGen、自研）无需 `lattice sandbox run`，通过三行代码接入 Lattice 的 **身份 + 策略 + 审计** 体系。

```python
from lattice import SecureMCPClient

client = SecureMCPClient(identity="worker-1", token="lt-xxx",
                         server="https://lattice.mycompany.com")
result = client.call("github-mcp", "create_issue", title="bug")
```

---

## 二、与 `sandbox run` 的关系

SDK 和 `sandbox run` 并存，面向不同场景：

| | `lattice sandbox run` | Lattice SDK |
|---|---|---|
| 接入方式 | 包裹进程（`lattice sandbox run -- python agent.py`）| 库（`import lattice`）|
| WireGuard | kernel wg0（NET_ADMIN）| wireguard-go + gVisor netstack（userspace，无特权）|
| 流量覆盖 | agent 所有 TCP 流量（透明强制）| 仅 SDK 发出的 MCP 调用 |
| 进程隔离 | netns L1 隔离 | 无 |
| 适用场景 | 高安全，需要透明强制执行 | 开发者友好，快速接入，主流 agent 框架 |

**选择原则**：安全团队管控的生产环境用 `sandbox run`；开发者自己集成用 SDK。

---

## 三、核心架构

```
SecureMCPClient
  │
  ├── AgentRegistrar          — 向 Lattice 服务器注册身份，获取 WireGuard 配置
  ├── PolicyCache             — 本地缓存 AgentPolicy，15s 刷新，NATS 实时失效
  ├── AuditSender             — 异步批量上报审计事件
  │
  ├── ExternalRouter          — 外部 MCP（无 peerName）→ 直接 HTTPS
  └── OverlayRouter           — 内部 MCP（有 peerName）→ userspace WireGuard
        └── wireguard-go + gVisor netstack（无 kernel 模块，无特权）
```

### 调用流程

```
client.call("github-mcp", "create_issue", title="bug")
  │
  ├── 1. PolicyCache.Check("worker-1", "github-mcp", "create_issue")
  │         → DENY：立即返回错误 + 写审计
  │         → ALLOW：继续
  │
  ├── 2. 查 MCPServerCache：github-mcp 是 external（无 peerName）
  │
  ├── 3. ExternalRouter：HTTPS POST https://api.githubcopilot.com/mcp
  │         → JSON-RPC: method=tools/call, tool=create_issue
  │
  └── 4. AuditSender.Send(event)  ← 异步，不阻塞返回
```

内部 MCP server 流程（第 2-3 步不同）：

```
  ├── 2. 查 MCPServerCache：db-tools 是 internal（peerName=db-mcp-peer）
  │
  └── 3. OverlayRouter.Dial("10.0.7.5:3000")
            → wireguard-go + gVisor netstack 建立 overlay 连接
            → HTTP POST http://10.0.7.5:3000/mcp
```

---

## 四、OverlayRouter：userspace WireGuard

### 为什么用 gVisor netstack

| | kernel wg0 | wireguard-go + gVisor netstack |
|---|---|---|
| 特权要求 | NET_ADMIN（root）| 无 |
| 平台支持 | Linux only | Linux / macOS / Windows |
| 影响范围 | 系统全局路由 | 仅库内部 |
| 适用场景 | 系统级（sandbox run）| 应用级（SDK）|

这与 Tailscale tsnet 的选择完全一致：
- `tailscaled`（系统级）用 kernel TUN
- `tsnet`（库级）用 wireguard-go + gVisor netstack

### 初始化流程

```
1. AgentRegistrar.Register()
     → POST /api/v1/sdk/register {identity, publicKey}
     → 返回：{overlayIP, peers[], natsURL}

2. OverlayRouter.Start()
     → gVisor netstack 创建虚拟网卡
     → wireguard-go 绑定到虚拟网卡
     → 加载 peers（AllowedIPs + Endpoint + PublicKey）

3. OverlayRouter.Dial("10.0.7.5:3000")
     → 通过 netstack 发包 → wireguard-go 加密 → UDP → overlay peer
```

### 与 sandbox run 的共存

- `sandbox run` 创建 kernel wg0，系统路由优先级更高
- SDK OverlayRouter 使用 userspace netstack，完全独立，不冲突
- 同一台机器上两者可以同时运行

---

## 五、PolicyCache

与 MCP proxy 中现有的 PolicyCache 设计一致，复用实现：

```
启动时：GET /api/v1/agent-policies → 拉取全量 AgentPolicy + MCPServer 列表
定时：每 15s 刷新
实时：NATS 订阅策略变更事件 → 立即失效
```

**本地检查，零网络延迟**：每次 MCP 调用的策略检查在内存里完成，不发网络请求。

### 检查逻辑

```python
def check(agent_identity, mcp_server, tool) -> (allow, deny_reason):
    policies = cache.get_policies_for_agent(agent_identity)
    if not policies:
        return False, "no policy found for agent"
    for policy in policies:
        if policy.default_deny:
            allowed_tools = policy.allowed_tools.get(mcp_server, [])
            if tool not in allowed_tools and "*" not in allowed_tools:
                return False, f"tool {tool} not in allowlist"
    return True, ""
```

---

## 六、AuditSender

```
审计事件结构（与 MCP proxy 一致）：
  agentName, agentIdentity, mcpServer, tool, params,
  verdict, denyReason, latencyMs, timestamp

写入策略：
  本地缓冲：最多 100 条 或 5s 触发一次批量上报
  上报端点：POST /api/v1/audit/events（批量）
  失败处理：本地文件保留，下次重试，不丢事件
  敏感字段：key/token/password/secret → [REDACTED]
```

---

## 七、服务端新增 API

SDK 需要以下新接口（其余复用已有 API）：

### 7.1 SDK Agent 注册

```
POST /api/v1/sdk/register
Request:
  {
    "identity": "worker-1",
    "token": "lt-xxx",               // enrollment token
    "publicKey": "wg-public-key"     // SDK 生成的 WireGuard 公钥
  }
Response:
  {
    "overlayIP": "10.0.7.8",
    "peers": [                        // 需要建立 WireGuard session 的 peers
      {"publicKey": "...", "endpoint": "...", "allowedIPs": ["10.0.7.5/32"]}
    ],
    "agentPolicies": [...],           // 初始全量策略，避免二次请求
    "mcpServers": [...]               // MCPServer 列表，含 mode 和 peerAddress
  }
```

### 7.2 审计事件批量写入

```
POST /api/v1/audit/events
Request: {"events": [...]}           // 已有设计，Phase 1 实现
```

### 7.3 策略变更推送（NATS）

```
Subject: lattice.policy.{workspaceID}.invalidate
Payload: {"type": "agent-policy"|"mcp-server", "name": "..."}
```

---

## 八、SDK 语言实现优先级

### 第一优先：Python

目标：兼容主流 agent 框架

```python
# 基础用法
client = SecureMCPClient(identity="worker-1", token="lt-xxx",
                         server="https://lattice.mycompany.com")
result = client.call("github-mcp", "create_issue", title="bug")

# LangChain 集成
from lattice.integrations.langchain import LatticeMCPToolkit
toolkit = LatticeMCPToolkit(client=client, mcp_server="file-tools")
tools = toolkit.get_tools()  # 返回 LangChain Tool 列表，自动带策略检查

# 原生 MCP session 包装
from lattice import secure_session
async with secure_session("db-tools", client=client) as session:
    result = await session.call_tool("query_db", {"sql": "SELECT ..."})
```

### 第二优先：Go

目标：与 Lattice 自身代码栈一致，服务端 agent 场景

```go
client, _ := lattice.NewSecureMCPClient(lattice.Config{
    Identity: "worker-1",
    Token:    "lt-xxx",
    Server:   "https://lattice.mycompany.com",
})
result, _ := client.Call(ctx, "github-mcp", "create_issue",
    map[string]any{"title": "bug"})
```

### 第三优先：TypeScript/Node

目标：前端 agent、Node.js 工具链

---

## 九、实现范围

### SDK 侧

| 组件 | 说明 |
|---|---|
| `AgentRegistrar` | 注册、获取 WireGuard 配置和初始策略 |
| `PolicyCache` | 本地策略缓存，15s 刷新 + NATS 失效 |
| `AuditSender` | 异步批量审计上报 |
| `ExternalRouter` | 外部 MCP HTTPS 路由 |
| `OverlayRouter` | wireguard-go + gVisor netstack userspace 隧道 |
| `SecureMCPClient` | 统一入口，自动选路 |
| LangChain 集成 | `LatticeMCPToolkit` |

### 服务端侧

| 组件 | 说明 |
|---|---|
| `POST /api/v1/sdk/register` | SDK 注册端点 |
| `POST /api/v1/audit/events` | 批量审计写入 |
| NATS 策略变更推送 | `lattice.policy.{ws}.invalidate` |

### 不在本设计范围

- SDK 的 NATS 直连（初期用 HTTP 轮询，后期可加 WebSocket 推送）
- TypeScript SDK（第三优先，后续设计）
- 异常检测（Phase 2）
- SDK 的 WireGuard 密钥轮换（初期重启重新注册）

---

## 十、关键设计决策

| 决策 | 选择 | 原因 |
|---|---|---|
| 内部 MCP 的 WireGuard 实现 | wireguard-go + gVisor netstack | 无特权要求，跨平台，与 sandbox run 的 kernel wg0 不冲突 |
| 策略检查位置 | 本地（PolicyCache）| 零延迟，MCP 调用本身已经 100ms+，不能再加网络往返 |
| 审计上报方式 | 异步批量 | 不阻塞 agent 主流程 |
| 外部 MCP 路由 | 直接 HTTPS | 80%+ 场景，最简单可靠 |
| 首要语言 | Python | 主流 agent 框架（LangChain/LangGraph/CrewAI）均为 Python |
