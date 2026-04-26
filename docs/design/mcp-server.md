# Wireflow MCP Server — 设计文档

## 1. 背景

### 1.1 现有方案的局限

`docs/design/ai-assistant.md` 设计的是一个**内嵌 AI 助手**：LLM Provider、工具调用、K8s 访问全部耦合在 Wireflow 后端，只能通过 Wireflow 自己的 Dashboard 或 `wf-ai` CLI 使用。

这意味着：使用 Claude Code、Claude Desktop、Cursor 等外部 AI 工具的用户无法直接让这些工具操作 Wireflow，只能人工切换界面。

### 1.2 MCP 是什么

**Model Context Protocol（MCP）** 是 Anthropic 发布的开放协议，定义了 AI 模型与外部系统之间的标准化通信方式。任何实现了 MCP Server 的服务，都可以被任意 MCP 客户端调用：

```
Claude Desktop / Claude Code / Cursor / 任何 MCP 客户端
          ↓  MCP 协议 (JSON-RPC 2.0 over stdio / HTTP)
    Wireflow MCP Server
          ↓  REST / K8s API
    Wireflow 管理 API + 网络资源
```

### 1.3 与内嵌 AI 助手的关系

两者**互补，不冲突**，面向不同用户：

| | 内嵌 AI 助手 | MCP Server |
|---|---|---|
| 目标用户 | 普通用户，在 Dashboard 操作 | 运维/开发，用自己的 AI 工具 |
| LLM | Wireflow 统一配置 | 用户自己的 Claude / GPT |
| 工具调用逻辑 | 耦合在 AIService | 独立暴露，标准化 |
| 工具扩展性 | 需修改 Wireflow 后端 | 任何 MCP 客户端都可发现并调用 |
| 写操作确认 | 前端 UI 确认流 | 由 LLM 在对话中发起确认 |
| 适用场景 | Web UI、wf-ai CLI | Claude Code、Claude Desktop、Cursor、自定义脚本 |

### 1.4 目标

1. 让运维工程师可以在 **Claude Code / Claude Desktop** 里直接管理 Wireflow 网络
2. 工具逻辑可复用于内嵌 AI 助手，消除重复实现
3. 支持 **Pro** 版特有的写入操作（策略创建、Peer 修改），与现有 RBAC 集成

---

## 2. 架构

### 2.1 整体架构

```
┌──────────────────────────────────────────────────────────────────┐
│                      MCP 客户端层                                  │
│                                                                    │
│   Claude Desktop       Claude Code       Cursor / 自定义脚本       │
│        │                    │                    │                 │
│        └────────────────────┴────────────────────┘                │
│                             │ MCP 协议 (JSON-RPC 2.0)             │
└─────────────────────────────┼────────────────────────────────────┘
                              │
              ┌───────────────┴───────────────┐
              │      传输层（二选一）           │
              │                               │
              │  stdio transport              │  HTTP transport
              │  (本地进程通信，                │  (远程 / 团队共享)
              │   Claude Desktop/Code 推荐)   │  POST /mcp  (Streamable HTTP)
              └───────────────┬───────────────┘
                              │
┌─────────────────────────────▼────────────────────────────────────┐
│                    Wireflow MCP Server                             │
│                                                                    │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐    │
│  │  Tools       │  │  Resources   │  │  Prompts             │    │
│  │  (操作)      │  │  (只读数据)   │  │  (常用提示词模板)     │    │
│  └──────┬───────┘  └──────┬───────┘  └──────────────────────┘    │
│         │                 │                                        │
│  ┌──────▼─────────────────▼───────────────────────────────────┐   │
│  │               Wireflow API Client                           │   │
│  │  Bearer Token 认证，调用 Management Server REST API         │   │
│  └────────────────────────────┬───────────────────────────────┘   │
└───────────────────────────────┼────────────────────────────────────┘
                                │ HTTPS + Bearer Token
                                ▼
                   Wireflow Management Server
                   (已有 /api/v1/* REST API)
```

### 2.2 MCP Server 部署模式

**模式 A：本地 stdio（主要模式）**

```bash
# Claude Desktop / Claude Code 以子进程方式启动 MCP Server
wireflow-mcp --server-url https://wf.example.com --token <JWT>

# 或从 ~/.wireflow/config.yaml 自动读取认证信息
wireflow-mcp
```

**模式 B：远程 HTTP（团队共享）**

```bash
# 以 HTTP 服务方式运行，供团队多人共用
wireflow-mcp server --listen :3000

# 客户端配置 URL 即可接入，每个用户携带自己的 Bearer Token
```

### 2.3 与 Management Server 的关系

MCP Server 是一个**独立进程**，通过 Management Server 的公开 REST API 访问资源，**不直接连 K8s**。这样：

- MCP Server 可以在任意机器运行（开发者笔记本、CI、远程服务器）
- 鉴权复用现有 Management Server 的 JWT 体系
- 不需要 K8s 访问权限

---

## 3. 协议能力

MCP 协议定义了三类能力：

| 能力 | 说明 | Wireflow 实现 |
|------|------|---------------|
| **Tools** | 可调用的操作，LLM 决定何时调用 | 网络查询、策略管理、连通性检查等 |
| **Resources** | 可订阅的只读数据，LLM 在上下文中引用 | 网络列表、Peer 列表、拓扑图 |
| **Prompts** | 预定义提示词模板，用户/LLM 直接使用 | 常见运维场景的标准化提问 |

---

## 4. Tools 定义

工具按操作类型分为只读和写入两类。

### 4.1 只读工具

#### `list_networks`

列出工作区内所有 WireflowNetwork 及其状态。

```json
{
  "name": "list_networks",
  "description": "列出 Wireflow 工作区内所有网络，包括 CIDR、状态和节点数量",
  "inputSchema": {
    "type": "object",
    "properties": {
      "workspace_id": {
        "type": "string",
        "description": "工作区 ID（namespace），不填则使用当前配置的默认工作区"
      }
    }
  }
}
```

返回示例：
```json
[
  {
    "name": "prod-network",
    "cidr": "10.100.1.0/24",
    "phase": "Ready",
    "peer_count": 12,
    "active_peers": 11
  }
]
```

---

#### `list_peers`

列出指定网络内所有 WireflowPeer 及其在线状态。

```json
{
  "name": "list_peers",
  "description": "列出指定网络内所有 Peer，包括 IP 地址、在线状态、标签和最后在线时间",
  "inputSchema": {
    "type": "object",
    "properties": {
      "workspace_id": { "type": "string" },
      "network":      { "type": "string", "description": "网络名称，不填则列出工作区全部 Peer" },
      "online_only":  { "type": "boolean", "description": "是否只返回在线 Peer" }
    }
  }
}
```

---

#### `list_policies`

列出指定工作区内的所有 WireflowPolicy。

```json
{
  "name": "list_policies",
  "description": "列出访问控制策略，说明哪些 Peer 之间可以通信",
  "inputSchema": {
    "type": "object",
    "properties": {
      "workspace_id": { "type": "string" },
      "network":      { "type": "string" }
    }
  }
}
```

---

#### `check_connectivity`

模拟检查两个 Peer 之间是否有策略允许通信，并说明原因。

```json
{
  "name": "check_connectivity",
  "description": "检查两个 Peer 之间是否可以通信，返回策略匹配结果和阻断原因",
  "inputSchema": {
    "type": "object",
    "required": ["workspace_id", "from_peer", "to_peer"],
    "properties": {
      "workspace_id": { "type": "string" },
      "from_peer":    { "type": "string", "description": "源 Peer 名称或 IP" },
      "to_peer":      { "type": "string", "description": "目标 Peer 名称或 IP" }
    }
  }
}
```

返回示例：
```json
{
  "allowed": false,
  "reason": "no matching policy",
  "matching_policy": null,
  "suggestion": "创建一条 WireflowPolicy，selector 匹配 from_peer，egress 允许 to_peer"
}
```

---

#### `get_peer`

获取单个 Peer 的详细信息。

```json
{
  "name": "get_peer",
  "description": "获取指定 Peer 的详细信息，包括公钥、IP、标签、在线状态",
  "inputSchema": {
    "type": "object",
    "required": ["workspace_id", "peer_name"],
    "properties": {
      "workspace_id": { "type": "string" },
      "peer_name":    { "type": "string" }
    }
  }
}
```

---

#### `get_topology`

返回工作区网络拓扑（节点 + 连接关系）。

```json
{
  "name": "get_topology",
  "description": "返回工作区网络拓扑图，包括节点、连接和策略覆盖情况",
  "inputSchema": {
    "type": "object",
    "properties": {
      "workspace_id": { "type": "string" },
      "network":      { "type": "string" }
    }
  }
}
```

---

#### `run_audit`

对工作区执行安全审计，返回结构化风险报告。

```json
{
  "name": "run_audit",
  "description": "对指定工作区执行安全审计，检查策略风险和不活跃节点，返回评分和建议",
  "inputSchema": {
    "type": "object",
    "required": ["workspace_id"],
    "properties": {
      "workspace_id": { "type": "string" }
    }
  }
}
```

---

### 4.2 写入工具（Pro）

写入工具通过 Wireflow Management Server 的现有鉴权体系控制权限，非 workspace admin 调用将收到 403。LLM 在对话中负责展示变更预览，用户明确回复"确认"后才真正执行。

#### `create_or_update_policy`

创建或更新 WireflowPolicy。

```json
{
  "name": "create_or_update_policy",
  "description": "创建或更新访问控制策略。调用前必须在对话中展示 YAML 预览并等待用户确认",
  "inputSchema": {
    "type": "object",
    "required": ["workspace_id", "policy"],
    "properties": {
      "workspace_id": { "type": "string" },
      "policy": {
        "type": "object",
        "description": "策略规格",
        "properties": {
          "name":          { "type": "string" },
          "network":       { "type": "string" },
          "peer_selector": { "type": "object", "description": "matchLabels 或 matchExpressions" },
          "egress": {
            "type": "array",
            "items": {
              "type": "object",
              "properties": {
                "to": { "type": "array" }
              }
            }
          },
          "ingress": {
            "type": "array",
            "items": {
              "type": "object",
              "properties": {
                "from": { "type": "array" }
              }
            }
          }
        }
      }
    }
  }
}
```

---

#### `delete_policy`

删除策略。

```json
{
  "name": "delete_policy",
  "description": "删除指定策略。调用前必须在对话中确认要删除的策略名和影响范围",
  "inputSchema": {
    "type": "object",
    "required": ["workspace_id", "policy_name"],
    "properties": {
      "workspace_id": { "type": "string" },
      "policy_name":  { "type": "string" }
    }
  }
}
```

---

#### `update_peer_labels`

修改 Peer 标签（用于策略选择器匹配）。

```json
{
  "name": "update_peer_labels",
  "description": "修改 Peer 的标签。标签变更会影响所有通过标签选择器引用此 Peer 的策略",
  "inputSchema": {
    "type": "object",
    "required": ["workspace_id", "peer_name", "labels"],
    "properties": {
      "workspace_id": { "type": "string" },
      "peer_name":    { "type": "string" },
      "labels": {
        "type": "object",
        "description": "要设置的标签键值对，null 值表示删除该标签"
      }
    }
  }
}
```

---

## 5. Resources 定义

Resources 是 LLM 可以在上下文窗口中直接引用的只读数据，通过 URI 标识。

### URI 规范

```
wireflow://{workspace_id}/networks
wireflow://{workspace_id}/networks/{network_name}
wireflow://{workspace_id}/peers
wireflow://{workspace_id}/peers/{peer_name}
wireflow://{workspace_id}/policies
wireflow://{workspace_id}/topology
wireflow://{workspace_id}/audit/latest
```

### Resource 列表

| URI 模式 | 名称 | MIME Type | 说明 |
|----------|------|-----------|------|
| `wireflow://{ws}/networks` | 网络列表 | `application/json` | 所有网络摘要 |
| `wireflow://{ws}/networks/{name}` | 网络详情 | `application/json` | 单个网络完整状态 |
| `wireflow://{ws}/peers` | Peer 列表 | `application/json` | 所有 Peer 及状态 |
| `wireflow://{ws}/peers/{name}` | Peer 详情 | `application/json` | 单个 Peer 完整信息 |
| `wireflow://{ws}/policies` | 策略列表 | `application/json` | 所有策略 |
| `wireflow://{ws}/topology` | 拓扑图 | `application/json` | 节点+边的图结构 |
| `wireflow://{ws}/audit/latest` | 最新审计报告 | `application/json` | 最近一次安全扫描结果 |

---

## 6. Prompts 定义

MCP Prompts 是预定义的提示词模板，LLM 客户端可以直接调用这些模板，减少用户输入。

### `network_status`

```json
{
  "name": "network_status",
  "description": "生成当前工作区网络状态的全面概览报告",
  "arguments": [
    {
      "name": "workspace_id",
      "description": "工作区 ID",
      "required": true
    }
  ]
}
```

模板内容：
```
请对工作区 {workspace_id} 的网络状态进行全面检查：
1. 列出所有网络和 Peer，标注离线节点
2. 检查是否有明显的策略配置问题
3. 提供简洁的健康状态摘要
```

---

### `diagnose_connectivity`

```json
{
  "name": "diagnose_connectivity",
  "description": "诊断两个 Peer 之间的连通性问题",
  "arguments": [
    { "name": "workspace_id", "required": true },
    { "name": "from_peer",    "required": true },
    { "name": "to_peer",      "required": true }
  ]
}
```

---

### `security_review`

```json
{
  "name": "security_review",
  "description": "对工作区执行安全审计并提供修复建议",
  "arguments": [
    { "name": "workspace_id", "required": true }
  ]
}
```

---

### `onboard_peer`

```json
{
  "name": "onboard_peer",
  "description": "引导用户完成新 Peer 的接入和策略配置",
  "arguments": [
    { "name": "workspace_id", "required": true },
    { "name": "peer_name",    "required": true },
    { "name": "purpose",      "required": false, "description": "Peer 的用途，如 web-server / database / gateway" }
  ]
}
```

---

## 7. 传输层

### 7.1 stdio 传输（本地，推荐）

MCP Server 以子进程方式运行，通过 stdin/stdout 与客户端通信。Claude Desktop、Claude Code 默认使用此方式。

**Claude Desktop 配置** (`~/Library/Application Support/Claude/claude_desktop_config.json`)：

```json
{
  "mcpServers": {
    "wireflow": {
      "command": "wireflow-mcp",
      "env": {
        "WIREFLOW_SERVER_URL": "https://wireflow.example.com",
        "WIREFLOW_TOKEN": "eyJ..."
      }
    }
  }
}
```

或使用配置文件自动读取（推荐）：

```json
{
  "mcpServers": {
    "wireflow": {
      "command": "wireflow-mcp"
    }
  }
}
```

**Claude Code 配置**（项目级，`.claude/mcp.json`）：

```json
{
  "mcpServers": {
    "wireflow": {
      "command": "wireflow-mcp",
      "args": ["--workspace", "wf-prod-xxx"]
    }
  }
}
```

### 7.2 HTTP 传输（远程，团队共享）

基于 MCP 规范的 **Streamable HTTP** 传输（2025-03-26 规范），单个 HTTP 端点处理所有请求。

```
POST /mcp
Content-Type: application/json
Authorization: Bearer <wireflow-jwt>
```

客户端配置：

```json
{
  "mcpServers": {
    "wireflow": {
      "url": "https://mcp.wireflow.example.com/mcp",
      "headers": {
        "Authorization": "Bearer eyJ..."
      }
    }
  }
}
```

HTTP 模式下 MCP Server 可以集成进 Wireflow Management Server，在 `/mcp` 路径注册：

```go
// management/server/api.go
s.mcpRouter()   // POST /mcp
```

---

## 8. 认证

### 本地 stdio 模式

优先级顺序：

1. 命令行参数：`--token <JWT>`
2. 环境变量：`WIREFLOW_TOKEN`
3. 配置文件：`~/.wireflow/config.yaml`（与 `wireflow` CLI 共享）

```yaml
# ~/.wireflow/config.yaml
server-url: https://wireflow.example.com
token: eyJhbGci...
workspace: wf-default-xxx
```

### 远程 HTTP 模式

MCP Server 接收 HTTP 请求头中的 `Authorization: Bearer <JWT>`，透传给 Management Server 做认证。MCP Server 本身是无状态的。

---

## 9. 实现方案

### 9.1 技术选型

**TypeScript + `@modelcontextprotocol/sdk`**，与 `wf-ai` CLI 共用同一个 npm 包（`cli/`），打包为单一可执行文件分发：

```
cli/
├── src/
│   ├── mcp/
│   │   ├── server.ts          # MCP Server 入口，注册 tools/resources/prompts
│   │   ├── tools/
│   │   │   ├── readonly.ts    # 只读工具实现
│   │   │   └── write.ts       # 写入工具实现（Pro）
│   │   ├── resources.ts       # Resources 注册
│   │   └── prompts.ts         # Prompts 注册
│   ├── api/
│   │   └── wireflow.ts        # Wireflow Management API 客户端（REST）
│   ├── config.ts              # 读取 ~/.wireflow/config.yaml
│   └── index.ts               # 入口：路由到 mcp / repl / chat / diagnose / audit
├── package.json
└── tsconfig.json
```

`mcp/server.ts` 核心结构：

```typescript
import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js'
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js'
import { StreamableHTTPServerTransport } from '@modelcontextprotocol/sdk/server/streamableHttp.js'
import { z } from 'zod'
import { WireflowClient } from '../api/wireflow.js'

export function createServer(client: WireflowClient): McpServer {
  const server = new McpServer({
    name: 'wireflow',
    version: '1.0.0',
  })

  // ── Tools ──────────────────────────────────────────────────────
  server.tool(
    'list_networks',
    '列出 Wireflow 工作区内所有网络',
    { workspace_id: z.string().optional() },
    async ({ workspace_id }) => {
      const networks = await client.listNetworks(workspace_id)
      return { content: [{ type: 'text', text: JSON.stringify(networks, null, 2) }] }
    },
  )

  server.tool(
    'check_connectivity',
    '检查两个 Peer 之间是否可以通信',
    {
      workspace_id: z.string(),
      from_peer: z.string(),
      to_peer: z.string(),
    },
    async ({ workspace_id, from_peer, to_peer }) => {
      const result = await client.checkConnectivity(workspace_id, from_peer, to_peer)
      return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] }
    },
  )

  // ... 其他工具

  // ── Resources ──────────────────────────────────────────────────
  server.resource(
    'wireflow-networks',
    new ResourceTemplate('wireflow://{workspace_id}/networks', { list: undefined }),
    async (uri) => {
      const ws = uri.pathname.split('/')[1]
      const networks = await client.listNetworks(ws)
      return { contents: [{ uri: uri.toString(), text: JSON.stringify(networks, null, 2) }] }
    },
  )

  // ── Prompts ────────────────────────────────────────────────────
  server.prompt(
    'network_status',
    '生成工作区网络状态概览',
    { workspace_id: z.string() },
    ({ workspace_id }) => ({
      messages: [{
        role: 'user',
        content: {
          type: 'text',
          text: `请对工作区 ${workspace_id} 的网络状态进行全面检查，包括离线节点和策略风险。`,
        },
      }],
    }),
  )

  return server
}
```

启动入口（支持两种传输）：

```typescript
// src/index.ts
const args = process.argv.slice(2)

if (args[0] === 'mcp') {
  // stdio 模式（默认，Claude Desktop / Code 使用）
  const transport = new StdioServerTransport()
  const server = createServer(client)
  await server.connect(transport)
} else if (args[0] === 'mcp-server') {
  // HTTP 模式（团队共享）
  startHttpServer(client, port)
}
```

### 9.2 核心依赖

```json
{
  "dependencies": {
    "@modelcontextprotocol/sdk": "^1.0",
    "zod": "^3"
  }
}
```

`@modelcontextprotocol/sdk` 内置 stdio 和 HTTP 传输层，无需额外依赖。

### 9.3 工具实现与内嵌 AI 助手的复用

MCP Server 的工具实现（`cli/src/mcp/tools/readonly.ts`）调用 Management Server 的 REST API，与内嵌 AI 助手共用**相同的接口**（`/api/v1/ai/chat` 背后的工具逻辑已在 `management/service/ai.go` 实现）。

工具逻辑在**两处维护**：

```
管理服务端 management/service/ai.go      ← 内嵌 AI 助手用
CLI 客户端  cli/src/api/wireflow.ts       ← MCP Server / wf-ai CLI 用
```

两者通过相同的 REST API 访问相同的数据，避免了直接共享 Go 代码的复杂性。

---

## 10. 使用示例

### Claude Code 里管理 Wireflow

```
# 在项目根目录配置 MCP
$ cat .claude/mcp.json
{
  "mcpServers": {
    "wireflow": { "command": "wireflow-mcp" }
  }
}

# 然后在 Claude Code 中
> 帮我检查 wf-prod 工作区里所有离线的 peer

[Claude 调用 list_peers(workspace_id="wf-prod", online_only=false)]
→ 发现 3 个节点离线：old-dev-laptop（30天）、ci-runner-01（2天）、staging-db（1小时）

> ci-runner-01 为什么离线？上次在线时传输了多少数据？

[Claude 调用 get_peer(workspace_id="wf-prod", peer_name="ci-runner-01")]
→ 上次在线：2天前，最后在线时长：15分钟，累计传输：2.3GB

> 帮 api-server 创建一条可以访问 redis 的策略

[Claude 调用 check_connectivity → 无策略 → 展示预览 YAML]
确认创建以下策略吗？
  name: api-to-redis
  selector: app=api-server
  egress: to app=redis

用户：确认
[Claude 调用 create_or_update_policy]
✓ 策略已创建
```

### Claude Desktop 里定期安全审查

```
# Claude Desktop 配置后，用 Prompt 快速触发
使用 wireflow 的 security_review prompt，工作区 wf-prod

[Claude 调用 run_audit(workspace_id="wf-prod")]
→ 安全评分：68/100
→ 高危：allow-all 策略 —— 建议分解为最小权限策略
→ 中危：staging-db 已 30 天未上线 —— 建议清理
```

---

## 11. 安全设计

| 关注点 | 措施 |
|--------|------|
| Token 安全 | stdio 模式：Token 通过环境变量传入，不写入进程参数（避免 `ps aux` 泄露）；HTTP 模式：HTTPS 传输 |
| 写操作风控 | 写入工具在 SDK 描述中标注"调用前必须展示预览并等待用户确认"，LLM 遵循此约束 |
| 权限隔离 | 所有写操作由 Management Server RBAC 控制，非 workspace admin 收到 403 |
| 工具注入防护 | 工具返回值长度截断（max 8000 chars），防止恶意数据污染 LLM 上下文 |
| stdio 进程隔离 | 每个 Claude Desktop / Code 会话启动独立 MCP Server 进程，天然隔离 |
| HTTP 模式认证 | 每次请求验证 Bearer Token，MCP Server 本身无状态，不缓存 Token |

---

## 12. 与现有规划的整合

### 12.1 包结构

MCP Server 代码放入已规划的 `cli/` 包，新增 `src/mcp/` 子目录：

```
cli/src/
├── mcp/           ← 新增：MCP Server
│   ├── server.ts
│   ├── tools/
│   ├── resources.ts
│   └── prompts.ts
├── commands/      ← 已有：wf-ai CLI 命令（repl / chat / diagnose / audit）
├── components/    ← 已有：Ink 组件
├── api/
│   └── wireflow.ts  ← 已有：Management Server 客户端（MCP 和 CLI 共用）
└── config.ts      ← 已有：~/.wireflow/config.yaml 读取
```

### 12.2 二进制分发

```
wireflow-mcp                   ← 新增：MCP Server 可执行文件
wf-ai                          ← 已有规划：AI TUI CLI
wireflow（Go）                  ← 已有：主 CLI
```

三个二进制独立分发，`wireflow-mcp` 也可作为 `wf-ai mcp` 子命令调用（单包多入口）。

---

## 13. 实现阶段

### Phase 1：只读工具 + stdio 传输（2 周）

对接 Claude Desktop / Claude Code，覆盖所有只读场景：

- [ ] `cli/src/mcp/server.ts`：MCP Server 框架 + stdio 传输
- [ ] `cli/src/mcp/tools/readonly.ts`：6 个只读工具（list_networks / list_peers / list_policies / check_connectivity / get_peer / run_audit）
- [ ] `cli/src/mcp/resources.ts`：7 个 Resource URI
- [ ] `cli/src/mcp/prompts.ts`：4 个 Prompt 模板
- [ ] Claude Desktop + Claude Code 配置文档
- [ ] `bun build --compile` 打包 `wireflow-mcp` 可执行文件

### Phase 2：写入工具（1 周）

需 Management Server Phase 3（Apply 接口）完成后并行实现：

- [ ] `cli/src/mcp/tools/write.ts`：3 个写入工具（create_or_update_policy / delete_policy / update_peer_labels）
- [ ] 工具描述中补充确认流程说明

### Phase 3：HTTP 传输（1 周）

支持团队共享场景：

- [ ] `cli/src/mcp/http.ts`：Streamable HTTP 传输
- [ ] Management Server 集成：`POST /mcp` 端点注册（可选）
- [ ] 多用户 Token 透传逻辑

---

## 14. 技术参考

- MCP 规范：https://modelcontextprotocol.io/specification/2025-03-26
- TypeScript SDK：`@modelcontextprotocol/sdk`
- Streamable HTTP 传输规范：MCP 2025-03-26 Section 3.2
