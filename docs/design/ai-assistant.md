# Wireflow AI Assistant — 设计文档

## 1. 背景与目标

Wireflow 是基于 WireGuard 的 Kubernetes 网络管理平台。网络配置的专业门槛（WireGuard 概念、CRD 结构、策略语义）是用户上手最大的障碍。通过引入 AI 能力，可以：

1. **降低使用门槛**：用自然语言描述网络意图，AI 生成配置
2. **加速故障排查**：自动分析网络状态，定位连通性问题
3. **提升安全合规**：持续扫描策略风险，生成可读报告
4. **作为 Pro 差异化功能**：AI 能力作为商业版独特价值点

---

## 2. 功能范围

| 功能 | 描述 | 定位 |
|------|------|------|
| **自然语言配置** | 用中文/英文描述网络意图，AI 生成并应用 CRD | Pro |
| **智能诊断** | 分析 Peer/Policy/网络状态，定位连通性问题 | Pro |
| **安全审计** | 定期扫描策略风险，生成安全评分和报告 | Pro |
| **运维 Copilot** | 对话式查询、操作网络资源，覆盖前三者 | Pro |

---

## 3. 整体架构

```
┌──────────────────────────────────────────────────────────┐
│                      Frontend (Dashboard)                 │
│                                                          │
│   ┌─────────────────┐    ┌──────────────────────────┐   │
│   │   Chat Panel    │    │  Security Audit Widget   │   │
│   │  (对话 Copilot) │    │  (安全评分 + 建议)        │   │
│   └────────┬────────┘    └──────────┬───────────────┘   │
└────────────┼──────────────────────┼──────────────────────┘
             │ SSE/HTTP             │ HTTP
             ▼                      ▼
┌──────────────────────────────────────────────────────────┐
│               Management Server (Gin)                     │
│                                                          │
│   POST /api/v1/ai/chat  (SSE 流式)                       │
│   GET  /api/v1/ai/audit (安全扫描报告)                    │
│   POST /api/v1/ai/apply (执行 AI 生成的变更)              │
│                                                          │
│   ┌──────────────────────────────────────────────────┐  │
│   │                  AIService                        │  │
│   │                                                  │  │
│   │  ContextBuilder ──→ ToolRegistry ──→ LLMClient   │  │
│   │       │                  │               │        │  │
│   │  读取工作区状态        执行工具调用    Provider抽象│  │
│   └──────────────────────────────────────────────────┘  │
│                                                          │
│   ┌──────────────────────────────────────────────────┐  │
│   │              已有 Controllers/Services            │  │
│   │  PeerController  PolicyController  AuditService  │  │
│   └──────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────┘
             │
             ▼ HTTPS
┌─────────────────────────────────────────────────────┐
│                   LLM Provider                       │
│                                                      │
│  AnthropicClient        OpenAICompatClient           │
│  (claude-sonnet-4-6)    (deepseek-chat / gpt-4o /   │
│   Anthropic API 格式)    任意 OpenAI 兼容接口)        │
└─────────────────────────────────────────────────────┘
```

---

## 4. 核心组件设计

### 4.1 AIService

所有 AI 功能的统一入口，位于 `management/service/ai.go`。

```go
type AIService interface {
    // Chat 处理一轮对话，通过 callback 流式输出 token
    Chat(ctx context.Context, req *ChatRequest, out StreamWriter) error

    // Audit 扫描当前工作区的安全风险，返回结构化报告
    Audit(ctx context.Context, workspaceID string) (*AuditReport, error)
}

type ChatRequest struct {
    Message     string        `json:"message"`
    WorkspaceID string        `json:"workspaceId"`
    History     []ChatMessage `json:"history"`   // 前端维护对话历史
}

type ChatMessage struct {
    Role    string `json:"role"`    // "user" | "assistant"
    Content string `json:"content"`
}

type StreamWriter interface {
    WriteToken(token string) error
    WriteToolUse(name string, input json.RawMessage) error
    WriteResult(result *ApplyPreview) error
    Close() error
}
```

### 4.2 LLMClient（多 Provider 抽象）

核心接口，屏蔽不同 LLM 服务商的 API 差异。

```go
// LLMClient 是统一的 LLM 调用接口，所有 Provider 实现此接口
type LLMClient interface {
    // Stream 发起带工具调用的流式对话
    Stream(ctx context.Context, req *LLMRequest, handler StreamHandler) error
}

type LLMRequest struct {
    System    string       // system prompt
    Messages  []LLMMessage // 对话历史（含 tool result）
    Tools     []LLMTool    // 工具定义（JSON Schema）
    MaxTokens int
}

type LLMMessage struct {
    Role       string          // "user" | "assistant" | "tool"
    Content    string
    ToolCallID string          // tool result 时使用
}

type LLMTool struct {
    Name        string
    Description string
    InputSchema json.RawMessage
}

// StreamHandler 处理流式事件
type StreamHandler interface {
    OnToken(token string)
    OnToolCall(id, name string, input json.RawMessage)
    OnDone()
    OnError(err error)
}
```

#### Provider 实现

**AnthropicClient**（Anthropic 原生 API）：

```go
type AnthropicClient struct {
    apiKey  string
    model   string   // "claude-sonnet-4-6"
    baseURL string   // 默认 https://api.anthropic.com
    httpCli *http.Client
}
```

- 使用 Anthropic Messages API（`/v1/messages`）
- tool_use block 格式：`{"type":"tool_use","id":"...","name":"...","input":{...}}`
- 流式事件：`content_block_delta` → `input_json_delta`

**OpenAICompatClient**（OpenAI 兼容 API，DeepSeek / OpenAI 等）：

```go
type OpenAICompatClient struct {
    apiKey  string
    model   string   // "deepseek-chat" | "deepseek-reasoner" | "gpt-4o"
    baseURL string   // DeepSeek: https://api.deepseek.com/v1
    httpCli *http.Client
}
```

- 使用 OpenAI Chat Completions API（`/chat/completions`）
- function calling 格式：`tool_calls[].function.{name, arguments}`
- 流式事件：`delta.tool_calls[].function.arguments`

两种格式差异由各自 Client 内部处理，对上层 `AIService` 完全透明。

#### Provider 工厂

```go
func NewLLMClient(cfg config.AIConfig) (LLMClient, error) {
    switch cfg.Provider {
    case "anthropic", "":
        return NewAnthropicClient(cfg.APIKey, cfg.Model, cfg.BaseURL), nil
    case "deepseek":
        baseURL := cfg.BaseURL
        if baseURL == "" {
            baseURL = "https://api.deepseek.com/v1"
        }
        return NewOpenAICompatClient(baseURL, cfg.APIKey, cfg.Model), nil
    case "openai":
        return NewOpenAICompatClient("https://api.openai.com/v1", cfg.APIKey, cfg.Model), nil
    default:
        // 支持任意 OpenAI 兼容服务（私有部署等），通过 base-url 指定
        if cfg.BaseURL != "" {
            return NewOpenAICompatClient(cfg.BaseURL, cfg.APIKey, cfg.Model), nil
        }
        return nil, fmt.Errorf("unsupported provider: %s", cfg.Provider)
    }
}
```

#### Provider 能力对比

| 方面 | Anthropic Claude | DeepSeek | OpenAI |
|------|-----------------|----------|--------|
| Function Calling | 支持（tool_use） | 支持（OpenAI 格式） | 支持 |
| 流式输出 | SSE | SSE | SSE |
| 中文理解 | 好 | 优秀 | 好 |
| 价格 | 较高 | 极低（约 1/10） | 中 |
| 国内访问 | 需代理 | 直连 | 需代理 |
| 私有部署 | 不支持 | 支持（开源权重） | 不支持 |
| 推荐场景 | 默认/海外 | 国内/成本敏感 | 已有账号 |

### 4.3 ContextBuilder

在每次对话前构建注入 LLM 的系统上下文。

```go
type ContextBuilder struct {
    peerSvc    service.PeerService
    policySvc  service.PolicyService
    networkSvc service.NetworkService
}

// Build 生成 system prompt，包含：
// 1. 角色定义和能力说明
// 2. Wireflow 核心概念解释（WireflowPeer / Network / Policy）
// 3. 当前工作区快照（网络数、Peer 数、策略数、活跃告警）
// 4. 操作规范（写操作必须先展示预览）
func (b *ContextBuilder) Build(ctx context.Context, workspaceID string) (string, error)
```

系统提示词模板（关键部分）：

```
你是 Wireflow 的网络管理助手，帮助用户管理基于 WireGuard 的私有网络。

## 当前工作区状态
- 工作区 ID: {workspaceID}
- 网络数量: {networkCount}
- 活跃 Peer: {activePeerCount} / {totalPeerCount}
- 策略条数: {policyCount}

## Wireflow 核心概念
- **WireflowNetwork**: 一个隔离的 WireGuard 网络，每个网络有独立 CIDR（如 10.100.1.0/24）
- **WireflowPeer**: 网络中的节点，代表一台设备或服务
- **WireflowPolicy**: 访问控制策略，控制哪些 Peer 之间可以通信（默认拒绝）

## 操作规范
- 查询操作：直接返回结果
- 创建/修改/删除操作：必须先展示变更预览，用户确认后才能执行
- 不确定的操作：先询问用户意图，再给出方案
```

### 4.4 ToolRegistry

工具分为只读和写入两类，按安全级别隔离。

```go
type Tool struct {
    Name        string
    Description string
    InputSchema json.RawMessage  // JSON Schema
    Handler     ToolHandler
    ReadOnly    bool
}

type ToolHandler func(ctx context.Context, input json.RawMessage) (string, error)
```

#### 只读工具集

| 工具名 | 说明 | 数据来源 |
|--------|------|----------|
| `list_networks` | 列出所有网络及其 CIDR、状态 | K8s CRD |
| `list_peers` | 列出指定网络的所有 Peer 及状态 | K8s CRD + Presence |
| `list_policies` | 列出访问控制策略 | K8s CRD |
| `get_peer_detail` | 获取单个 Peer 的详细信息（IP、标签、最后在线时间） | K8s + DB |
| `check_connectivity` | 模拟检查两个 Peer 之间是否有策略允许通信 | 策略引擎计算 |
| `get_audit_logs` | 查询审计日志（支持时间范围、操作类型过滤） | DB |
| `explain_policy` | 用自然语言解释一条策略的实际效果 | 策略解析 |
| `get_network_topology` | 返回网络拓扑图数据（节点+边） | K8s CRD |

#### 写入工具集（需用户二次确认）

| 工具名 | 说明 | 影响范围 |
|--------|------|----------|
| `create_network` | 创建新的 WireflowNetwork | 创建 CRD |
| `create_policy` | 创建或更新 WireflowPolicy | 创建/更新 CRD |
| `delete_policy` | 删除策略 | 删除 CRD |
| `update_peer_labels` | 修改 Peer 标签（用于策略匹配） | 更新 CRD |

写入工具在实际执行前返回 `ApplyPreview`，由前端展示 diff 并等待确认：

```go
type ApplyPreview struct {
    Action       string `json:"action"`       // "create" | "update" | "delete"
    Resource     string `json:"resource"`     // 资源类型
    Name         string `json:"name"`
    Namespace    string `json:"namespace"`
    YAMLDiff     string `json:"yamlDiff"`     // unified diff 格式
    ConfirmToken string `json:"confirmToken"` // 一次性 token，POST /ai/apply 时用
}
```

### 4.5 Tool Use 循环（Agentic Loop）

Tool use 循环逻辑完全在 `AIService` 中实现，与具体 Provider 无关：

```
用户消息
   │
   ▼
LLMClient.Stream(system, messages, tools)
   │
   ├─→ OnToken: 流式输出文本给前端
   │
   └─→ OnToolCall: 工具调用事件
          │
          ▼
       ToolRegistry.Execute(name, input)
          │
          ├─→ 只读工具：直接执行，结果追加到 messages
          │
          └─→ 写入工具：生成 ApplyPreview，结果追加到 messages
                │
                ▼
          再次调用 LLMClient.Stream（带 tool result）
          LLM 生成最终文本回答 + 把 ApplyPreview 推给前端
```

单轮对话最多允许 **5 次工具调用**，防止无限循环。

---

## 5. API 设计

### 5.1 对话接口（SSE 流式）

```
POST /api/v1/ai/chat
Authorization: Bearer {token}
Content-Type: application/json

{
  "message": "为什么 api-server 连不上 redis？",
  "workspaceId": "ws-abc123",
  "history": [
    {"role": "user", "content": "..."},
    {"role": "assistant", "content": "..."}
  ]
}
```

响应（SSE 格式）：

```
data: {"type":"token","content":"api-server "}
data: {"type":"token","content":"和 redis "}
data: {"type":"tool_use","tool":"check_connectivity","input":{"from":"api-server","to":"redis"}}
data: {"type":"token","content":"两个 Peer 均在线，但没有策略允许通信。"}
data: {"type":"preview","data":{"action":"create","resource":"WireflowPolicy","yamlDiff":"...","confirmToken":"tok_xxx"}}
data: {"type":"done"}
```

### 5.2 执行变更接口

```
POST /api/v1/ai/apply
Authorization: Bearer {token}
Content-Type: application/json

{
  "confirmToken": "tok_xxx"
}
```

`confirmToken` 由服务端生成，有效期 5 分钟，确保用户只能执行 AI 刚刚生成的变更，防止重放。

### 5.3 安全审计接口

```
GET /api/v1/ai/audit?workspaceId=ws-abc123
Authorization: Bearer {token}
```

响应：

```json
{
  "score": 72,
  "generatedAt": "2026-04-25T10:00:00Z",
  "findings": [
    {
      "severity": "high",
      "rule": "allow-all-detected",
      "resource": "policy/allow-all",
      "description": "策略 allow-all 允许工作区内所有 Peer 互相通信，违反最小权限原则",
      "suggestion": "改为只允许必要的 Peer 对之间通信"
    },
    {
      "severity": "medium",
      "rule": "unused-peer",
      "resource": "peer/old-dev-laptop",
      "description": "Peer old-dev-laptop 已 30 天未上线",
      "suggestion": "确认是否可以删除"
    }
  ]
}
```

---

## 6. 安全审计规则引擎

安全审计独立于对话，定时（每日）或按需触发，不依赖 LLM 做规则判断（降低成本），只用 LLM 生成**可读的建议文本**。

```go
type AuditRule interface {
    Name() string
    Severity() string  // "high" | "medium" | "low"
    Check(ctx context.Context, workspace *WorkspaceSnapshot) []Finding
}

// 内置规则
var defaultRules = []AuditRule{
    &AllowAllPolicyRule{},      // 检测 allow-all 策略
    &CrossEnvAccessRule{},      // 检测跨环境（prod/staging/dev）直连
    &UnusedPeerRule{days: 30},  // 检测长期离线的 Peer
    &NoEncryptionPolicyRule{},  // 检测无策略保护的网络（全开放）
    &ShadowPeerStaleRule{},     // 检测过期的 shadow peer（peering 已删但 shadow 还在）
}
```

规则计算完成后，将 findings 发给 LLM 生成中文建议，填充 `description` 和 `suggestion` 字段。

**安全评分算法**：

```
基础分: 100
每条 high finding:   -15
每条 medium finding: -8
每条 low finding:    -3
最低分: 0
```

---

## 7. 前端设计

### 7.1 Chat Panel（侧边栏）

在现有 Dashboard 中加入一个可折叠的右侧抽屉，不改变现有页面布局：

```
┌─────────────────────────────────────────────┐
│  Dashboard                    [AI 助手] [×] │
│                                             │
│  [主内容区]          │  ┌─────────────────┐ │
│                      │  │  Wireflow AI    │ │
│                      │  ├─────────────────┤ │
│                      │  │                 │ │
│                      │  │  [对话历史]     │ │
│                      │  │                 │ │
│                      │  │  [工具调用指示] │ │
│                      │  │                 │ │
│                      │  │  [变更预览卡片] │ │
│                      │  │  YAML diff      │ │
│                      │  │  [确认] [取消]  │ │
│                      │  ├─────────────────┤ │
│                      │  │  输入框...  [发] │ │
│                      │  └─────────────────┘ │
└─────────────────────────────────────────────┘
```

**关键 UI 元素：**

- **工具调用指示器**：AI 调用工具时显示 `正在查询 Peer 状态...`，增加透明度
- **变更预览卡片**：内嵌 YAML diff（高亮增删），确认/取消按钮
- **流式渲染**：token 逐字显示，参考 Claude.ai 体验
- **快捷提示词**：空对话时展示常用问题入口

  ```
  常用操作：
  > 分析当前网络安全状态
  > 帮我创建一个开发网络
  > 为什么 [Peer A] 连不上 [Peer B]？
  ```

### 7.2 安全审计 Widget

在 Dashboard 概览页展示安全评分卡片：

```
┌──────────────────────────┐
│  安全评分                │
│                          │
│       72 / 100           │
│     ████████░░           │
│                          │
│  2 个高危  3 个中危       │
│                          │
│  [查看详情] [立即修复]    │
└──────────────────────────┘
```

点击"立即修复"会打开 Chat Panel，并自动填入 `帮我修复安全审计中的高危问题`。

---

## 8. 配置扩展

在 `internal/config/config.go` 的 `Config` 结构体中新增：

```go
AI AIConfig `mapstructure:"ai"`
```

```go
type AIConfig struct {
    // Enabled 是否启用 AI 功能，APIKey 为空时自动关闭
    Enabled bool `mapstructure:"enabled"`

    // Provider 指定 LLM 服务商，支持：
    //   "anthropic"（默认）、"deepseek"、"openai"、自定义（需配合 base-url）
    // 对应环境变量: WIREFLOW_AI_PROVIDER
    Provider string `mapstructure:"provider"`

    // APIKey 服务商 API Key
    // 对应环境变量: WIREFLOW_AI_API_KEY
    APIKey string `mapstructure:"api-key"`

    // Model 指定模型名称，不填时使用各 Provider 默认值：
    //   anthropic → claude-sonnet-4-6
    //   deepseek  → deepseek-chat
    //   openai    → gpt-4o
    Model string `mapstructure:"model"`

    // BaseURL 自定义 API 端点，用于：
    //   - DeepSeek（https://api.deepseek.com/v1）
    //   - 私有部署的 OpenAI 兼容服务
    //   - API 中转代理
    // 对应环境变量: WIREFLOW_AI_BASE_URL
    BaseURL string `mapstructure:"base-url"`

    // MaxToolCalls 单轮对话最大工具调用次数，默认 5
    MaxToolCalls int `mapstructure:"max-tool-calls"`

    // AuditSchedule 安全审计定时任务 cron 表达式，默认 "0 2 * * *"（每日凌晨 2 点）
    // 值为空时禁用定时审计
    AuditSchedule string `mapstructure:"audit-schedule"`
}
```

各 Provider 配置示例：

```yaml
# Anthropic（默认）
ai:
  enabled: true
  provider: anthropic
  api-key: sk-ant-xxx
  model: claude-sonnet-4-6

# DeepSeek（国内推荐）
ai:
  enabled: true
  provider: deepseek
  api-key: sk-xxx
  model: deepseek-chat   # 或 deepseek-reasoner（推理增强）

# 私有部署 / OpenAI 兼容服务
ai:
  enabled: true
  provider: custom
  api-key: xxx
  model: your-model-name
  base-url: http://your-llm-server/v1
```

AI 功能为**弱依赖**：`APIKey` 为空时优雅降级，所有 `/api/v1/ai/*` 接口返回 `503 AI not configured`，前端隐藏 AI 入口。

---

## 9. CLI 工具设计

### 9.1 定位与技术选型

`wf-ai` 是独立的 TypeScript + **Ink** CLI 工具，与 Dashboard AI 共享同一套后端服务。

**选择 TypeScript + Ink 的原因：**
- Ink 基于 React 组件模型，终端 UI 质量显著优于 Go TUI 方案
- 丰富的现成组件生态（Spinner、TextInput、Select、Markdown 渲染等）
- Claude Code、Vercel CLI、Prisma CLI 等均采用此方案，视觉标杆已验证

CLI 是后端 AI API 的**薄客户端**，不内嵌 LLM 逻辑，所有 AI 推理和工具调用均在服务端完成。

### 9.2 核心依赖

```json
{
  "dependencies": {
    "ink": "^5",
    "react": "^18",
    "ink-markdown": "^1",
    "ink-select-input": "^5",
    "ink-text-input": "^6",
    "eventsource": "^2",
    "js-yaml": "^4",
    "diff": "^5",
    "chalk": "^5",
    "conf": "^12"
  },
  "devDependencies": {
    "typescript": "^5",
    "@types/react": "^18",
    "@types/js-yaml": "^4",
    "bun": "^1"
  }
}
```

**分发策略**：使用 `bun build --compile` 编译为单一可执行文件，无 Node.js 运行时依赖，与现有 Go 二进制分发方式一致。

### 9.3 命令结构

```
wf-ai                          # 进入交互式 REPL（推荐日常使用）
wf-ai "<问题>"                  # 单次问答
wf-ai diagnose                 # 专项：连通性检查（带退出码，CI 友好）
wf-ai audit                    # 专项：安全扫描报告
```

> 作为独立二进制 `wf-ai`，而非挂在 `wireflow` 子命令下，便于单独分发和版本管理。

### 9.4 交互式 REPL（Ink 组件）

```
╭─────────────────────────────────────────────────────────╮
│  Wireflow AI                                            │
│  workspace: dev-team · model: deepseek-chat             │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  你                                                     │
│  为什么 api-server 连不上 redis？                        │
│                                                         │
│  AI  ⠸ 正在检查连通性...                                │
│                                                         │
│  AI                                                     │
│  api-server 和 redis 均在线，但没有策略允许通信。         │
│  建议创建以下访问策略：                                  │
│                                                         │
│  ╭─ 变更预览 ──────────────────────────────────────╮    │
│  │ + apiVersion: wireflow.run/v1alpha1              │    │
│  │ + kind: WireflowPolicy                          │    │
│  │ + metadata:                                     │    │
│  │ +   name: api-to-redis                          │    │
│  │ + spec:                                         │    │
│  │ +   selector:                                   │    │
│  │ +     matchLabels: {app: api-server}            │    │
│  │ +   ingress:                                    │    │
│  │ +     - from: [{matchLabels: {app: redis}}]     │    │
│  ╰─────────────────────────────────────────────────╯    │
│                                                         │
│  ❯ 确认执行    取消                                      │
│                                                         │
├─────────────────────────────────────────────────────────┤
│  › _                                            ⌃C 退出 │
╰─────────────────────────────────────────────────────────╯
```

**组件拆分：**

```
<App>
  ├── <Header>          工作区信息、模型名
  ├── <MessageList>     对话历史滚动视图（Ink Scrollable）
  │   ├── <UserMessage>
  │   └── <AIMessage>
  │       ├── <Spinner>          工具调用进行中
  │       ├── <MarkdownBlock>    流式渲染 AI 回答
  │       └── <ChangePreview>    变更预览 + 确认/取消
  │           └── <DiffView>     YAML diff（绿色增/红色删）
  └── <InputBar>        文本输入框 + 快捷键提示
```

### 9.5 单次问答模式

```bash
# 直接输出，适合脚本捕获
$ wf-ai "列出 prod 工作区所有策略" -w wf-prod-123
  allow-frontend-to-api   ALLOW  frontend → api-server
  allow-api-to-db         ALLOW  api-server → postgres
  deny-external           DENY   * → internal-*

# 非交互模式（--yes 跳过确认，CI/CD 用）
$ wf-ai "为 api-server 和 redis 创建互通策略" -w wf-abc --yes
  ✓ 策略 api-to-redis 已创建

# JSON 输出（管道处理）
$ wf-ai audit -w wf-prod --format json | jq '.findings[] | select(.severity=="high")'
```

非 TTY 环境（管道/重定向）自动降级为纯文本输出，不渲染 Ink 组件。

### 9.6 专项子命令

```bash
# 连通性检查（退出码：0=畅通 1=阻断 2=错误）
$ wf-ai diagnose --from api-server --to redis -w wf-abc
  ✓ 路径畅通：api-server → redis（匹配策略: api-to-redis）

$ wf-ai diagnose --from frontend --to database -w wf-prod
  ✗ 路径阻断：frontend → database
  原因：无匹配策略

# 安全审计
$ wf-ai audit -w wf-prod
  安全评分：72 / 100  ████████░░

  ● 高危  allow-all-detected
    策略 allow-all 允许工作区内所有 Peer 互相通信
    建议：改为只允许必要的 Peer 对之间通信

  ○ 中危  unused-peer
    Peer old-dev-laptop 已 30 天未上线
    建议：确认是否可以删除
```

### 9.7 认证

CLI 读取 `~/.wireflow/wireflow.yaml`（与 Go CLI 共享配置文件），无需单独登录：

```yaml
# ~/.wireflow/wireflow.yaml（已有，Go CLI 写入）
server-url: https://wireflow.example.com
auth: <JWT token>
```

首次使用若未登录，引导执行 `wireflow` 命令登录后自动共享 token。

### 9.8 项目结构

```
cli/                           # TypeScript 独立包
├── src/
│   ├── index.tsx              # 入口：解析参数，路由到对应命令
│   ├── commands/
│   │   ├── repl.tsx           # 交互式 REPL（Ink App 主组件）
│   │   ├── chat.tsx           # 单次问答（非 TTY 安全）
│   │   ├── diagnose.tsx       # 连通性检查
│   │   └── audit.tsx          # 安全审计
│   ├── components/
│   │   ├── Header.tsx
│   │   ├── MessageList.tsx
│   │   ├── AIMessage.tsx
│   │   ├── ChangePreview.tsx  # YAML diff 渲染 + 确认交互
│   │   ├── DiffView.tsx
│   │   └── Spinner.tsx
│   ├── api/
│   │   └── client.ts          # Management Server HTTP 客户端（SSE + REST）
│   └── config.ts              # 读取 ~/.wireflow/wireflow.yaml
├── package.json
├── tsconfig.json
└── README.md
```

---

## 10. 目录结构（服务端）

```
management/
├── service/
│   ├── ai.go              # AIService 接口 + 实现（含 ContextBuilder、ToolRegistry）
│   └── audit_rules.go     # 安全审计规则引擎
├── llm/
│   ├── client.go          # LLMClient 接口 + LLMRequest/StreamHandler 类型定义
│   ├── anthropic.go       # AnthropicClient 实现
│   ├── openai_compat.go   # OpenAICompatClient 实现（DeepSeek / OpenAI 等）
│   └── factory.go         # NewLLMClient 工厂函数
├── server/
│   └── ai.go              # HTTP handler: /api/v1/ai/*
└── controller/
    └── ai.go              # AIController 接口（薄层，调用 AIService）
```

`llm/` 包设计为**无业务依赖**的纯 LLM 通信层，可独立测试和复用。

---

## 10. 数据流：自然语言配置全链路

以"帮我让 api-server 能访问 redis"为例：

```
1. 前端 POST /api/v1/ai/chat
   { message: "帮我让 api-server 能访问 redis", workspaceId: "ws-abc" }

2. ContextBuilder.Build("ws-abc")
   → 读取工作区 Peer/Policy 列表，生成 system prompt

3. LLMClient.Stream(system, messages, tools)
   → LLM 返回 tool_call: list_peers({ namespace: "wf-ws-abc" })

4. ToolRegistry.Execute("list_peers", ...)
   → 调用 PeerService，返回 Peer 列表（含 api-server、redis 的标签）

5. LLMClient 继续（带 tool result）
   → LLM 返回 tool_call: check_connectivity({ from: "api-server", to: "redis" })

6. ToolRegistry.Execute("check_connectivity", ...)
   → 策略引擎计算：无策略匹配 → 返回 "blocked"

7. LLMClient 继续
   → LLM 生成策略 YAML，返回 tool_call: create_policy({ yaml: "..." })

8. ToolRegistry（写入工具）
   → 不立即执行，生成 ApplyPreview + confirmToken
   → 返回预览给 LLM

9. LLM 流式输出文本解释 + 把 ApplyPreview 通过 SSE 发给前端
   "我将创建以下策略允许 api-server 访问 redis，请确认："

10. 用户点击"确认"
    → POST /api/v1/ai/apply { confirmToken: "tok_xxx" }
    → 从缓存取出 YAML，调用 PolicyService.Apply()
    → 返回 200 OK
```

---

## 11. 实现阶段

### Phase 1：后端基础 + 只读工具（目标：能回答状态查询）

- [ ] `AIConfig` 配置项 + 弱依赖启动逻辑
- [ ] `llm/` 包：`LLMClient` 接口 + `AnthropicClient` + `OpenAICompatClient` + 工厂
- [ ] `ContextBuilder` 基础版（工作区快照 system prompt）
- [ ] 只读 ToolRegistry（list_networks / list_peers / list_policies / check_connectivity）
- [ ] `POST /api/v1/ai/chat` SSE 接口
- [ ] `GET /api/v1/ai/audit` 接口 + 审计规则引擎

### Phase 2：CLI 工具（目标：终端可用，面向运维用户）

- [ ] `cli/` TypeScript 包初始化（tsconfig、package.json、bun 编译配置）
- [ ] `api/client.ts`：Management Server HTTP 客户端（SSE 流式 + REST）
- [ ] `config.ts`：读取 `~/.wireflow/wireflow.yaml` 获取 server-url 和 token
- [ ] `commands/chat.tsx`：单次问答（非 TTY 降级为纯文本）
- [ ] `commands/repl.tsx`：交互式 REPL（Ink App + 多轮对话历史）
- [ ] `components/`：Header / MessageList / AIMessage / Spinner
- [ ] `components/ChangePreview.tsx`：YAML diff + 确认/取消交互
- [ ] `commands/diagnose.tsx`：连通性检查（带退出码）
- [ ] `commands/audit.tsx`：安全审计报告
- [ ] bun compile 打包为单一可执行文件 `wf-ai`

### Phase 3：写入工具 + 变更确认流（目标：能改配置）

- [ ] 写入 ToolRegistry（create_policy / update_peer_labels）
- [ ] `ApplyPreview` + `confirmToken` 机制
- [ ] `POST /api/v1/ai/apply` 接口
- [ ] CLI：终端内 diff 确认（`[y/N]` 交互 + `--yes` 跳过）
- [ ] 前端变更预览卡片（YAML diff + 确认/取消）

### Phase 4：Dashboard AI + 安全审计面板（目标：Web 端完整体验）

- [ ] 前端 Chat Panel（流式渲染 + 工具调用指示）
- [ ] 前端变更预览卡片
- [ ] Dashboard 安全评分 Widget
- [ ] 定时审计任务（cron）
- [ ] "立即修复"联动 Chat Panel

---

## 12. 安全与成本控制

| 关注点 | 措施 |
|--------|------|
| API Key 泄露 | 仅存服务端配置，不下发到前端；审计日志记录每次 AI 调用 |
| 误操作防护 | 写操作强制二次确认；confirmToken 有效期 5 分钟且一次性 |
| 成本控制 | 审计规则本地计算，LLM 只做文本生成；只读查询不调用 LLM；DeepSeek 可降低 90% 成本 |
| 权限隔离 | AI 的写操作走现有 RBAC，非 workspace admin 无法确认变更 |
| Prompt Injection | system prompt 中明确边界；工具返回值做长度截断（max 2000 chars） |
| Token 用量 | 对话历史最多保留 10 轮；工作区快照做字段精简，避免 context 爆炸 |
