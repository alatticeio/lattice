# Lattice x AI Agent 战略设计

**日期**: 2026-05-06
**状态**: 已确认，待实现
**资源**: 1-2 人 x 1 个月

---

## 背景

Lattice 是 K8s-native WireGuard overlay 网络，已具备：
- `internal/server/llm/`：Anthropic + OpenAI-compat LLM 客户端
- `AIService`：Chat（SSE 流式）+ Audit（安全扫描），4 个只读工具
- `WorkflowService`：审批工作流（Submit -> Approve -> Execute）
- `LatticePeer / LatticePolicy / LatticeNetwork` CRD：网络状态完全声明化

目标：借助 AI Agent 浪潮，同时实现 GitHub Stars 增长、付费用户转化、技术话题度、赛道占位四个目标。

---

## 整体架构：五层 AI 能力模型

```
+-----------------------------------------------------------------+
|  Layer 5: Compliance-as-Conversation                            |
|  SOC2/PCI-DSS/HIPAA 合规报告自动生成                            |
+-----------------------------------------------------------------+
|  Layer 4: Time-Travel Network Debugging                         |
|  网络状态历史快照 + AI 根因分析                                  |
+-----------------------------------------------------------------+
|  Layer 3: Network Intent Engine                                 |
|  自然语言意图 -> CRD 变更计划（声明式 AI Ops）                   |
+-----------------------------------------------------------------+
|  Layer 2: Zero-Trust for AI Agent Fleets                        |
|  AI Agent 自动入网、TTL 身份、网络隔离预设                       |
+-----------------------------------------------------------------+
|  Layer 1: Foundation — MCP Server + AI ChatOps 写工具           |
|  所有上层能力的入口和执行引擎                                     |
+-----------------------------------------------------------------+
```

**设计原则**：
- L1 先行：写工具 + WorkflowService 审批是所有变更的唯一执行路径
- MCP = 统一入口：Claude Desktop / Cursor / 自定义 Agent 统一接入
- CRD as Source of Truth：所有状态读写走 K8s API
- Human-in-the-loop：写操作默认走 WorkflowService，`auto_approve` 按工具粒度配置

**仓库规划**：

| 内容 | 位置 |
|------|------|
| `cmd/lattice-mcp` + `internal/mcp/` | 主仓库 |
| AI 写工具 / IntentService / DebugService / ComplianceService | 主仓库 |
| NetworkSnapshot 快照存储 | 主仓库 |
| Python Agent SDK | `lattice-sdk-python`（新仓库）|
| Node Agent SDK | `lattice-sdk-node`（新仓库）|

---

## Layer 1：MCP Server + AI ChatOps 写工具

### 新增目录结构

```
cmd/lattice-mcp/
  main.go
internal/mcp/
  server.go          # MCP 协议（stdio + SSE 双模式）
  tools.go           # 工具注册，复用 AIService.executeTool
  transport/
    stdio.go
    sse.go
```

### MCP 工具列表

| 工具名 | 类型 | 说明 |
|--------|------|------|
| `list_peers` | 读 | 已有 |
| `list_policies` | 读 | 已有 |
| `list_networks` | 读 | 已有 |
| `check_connectivity` | 读 | 已有 |
| `create_peer` | 写 | 创建 LatticePeer + Enrollment Token |
| `delete_peer` | 写 | 删除 Peer，下线前发 NATS 通知 |
| `create_policy` | 写 | 创建 LatticePolicy |
| `update_policy` | 写 | 修改已有策略 |
| `delete_policy` | 写 | 删除策略 |
| `audit_workspace` | 读 | 触发 AIService.Audit |

### 写工具安全模型

```
MCP 调用写工具
    |
    +- auto_approve=true  -> 直接执行
    +- auto_approve=false -> WorkflowService.Submit()
           -> 返回 workflow_id，等待人工审批
           -> 审批后 WorkflowService.Approve() -> K8s Apply
```

`lattice.yaml` 配置：

```yaml
ai:
  enabled: true
  api-key: "sk-..."
  provider: anthropic   # anthropic | openai-compat
  workflow:
    auto_approve:
      create_peer: false
      delete_peer: false
      create_policy: false
```

### Claude Desktop 集成

```json
{
  "mcpServers": {
    "lattice": {
      "command": "lattice-mcp",
      "args": ["--config", "/etc/lattice/lattice.yaml"]
    }
  }
}
```

---

## Layer 2：Zero-Trust for AI Agent Fleets

### 核心问题

AI Agent 时代的安全威胁：Agent 被 Prompt Injection 攻击后可能横向移动。Lattice 的 WireGuard + Policy 在网络层强制隔离，即使 Agent 被攻击也无法突破网络边界。

### Agent Enrollment API

```
POST /api/v1/agent-enroll
```

请求：
```json
{
  "agentName": "code-executor-001",
  "agentType": "code-executor",
  "workspaceId": "ws-prod-agents",
  "ttl": "1h",
  "policyPreset": "sandboxed"
}
```

响应：
```json
{
  "peerId": "peer-xxx",
  "overlayIP": "10.96.2.5/32",
  "enrollmentToken": "lt-xxx",
  "wireguardConfig": "...",
  "expiresAt": "2026-05-06T11:00:00Z"
}
```

### Policy Preset

| Preset | 规则 |
|--------|------|
| `sandboxed` | 只允许出流量到指定工具服务，拒绝所有入流量 |
| `coordinator` | 可接受同 workspace 内其他 agent 的入流量 |
| `isolated` | 完全隔离，只允许指定 IP/端口白名单 |

### TTL 自动销毁

- LatticePeer 写入 `ExpiresAt` annotation
- Manager reconciler 每分钟扫描过期 Peer，自动删除
- Agent 主动退出时调 `DELETE /api/v1/peers/:id`（SDK 封装）

### Python SDK

```python
from lattice_sdk import LatticeAgent

async with LatticeAgent(
    server="https://lattice.company.com",
    token="lt-workspace-token",
    agent_name="code-executor",
    policy_preset="sandboxed",
) as agent:
    result = await my_agent_task()
```

### 主流框架集成

| 框架 | 集成方式 |
|------|---------|
| LangGraph | `StateGraph` lifespan context manager |
| AutoGen | `ConversableAgent` init/del hook |
| Claude agent-sdk | agent 启动脚本 wrapper |
| Kubernetes Job | Init container enroll + sidecar 心跳 |

### 发布策略

- README 新增 "AI Agent Networking" 章节，排第一屏
- 博客：《Prompt Injection 之后，你的 Agent 能横向移动吗？》
- 在 LangChain / AutoGen GitHub Discussions 发集成教程

---

## Layer 3：Network Intent Engine（Pro）

### 两阶段 LLM 调用

**第一阶段**：结构化意图提取（强 JSON schema 约束）
- 输入：用户自然语言 + 当前网络状态
- 输出：`{ changes: CRDChange[], risk: string, reasoning: string }`

**第二阶段**：生成人类可读 diff 说明
- 输出：Markdown，用于 UI 展示和审批邮件

两阶段分离原因：第一阶段需要精确，第二阶段需要自然，同一 prompt 难以两全。

### IntentService 接口

```go
type IntentService interface {
    // Plan 接收自然语言意图，返回变更计划（不执行）
    Plan(ctx context.Context, req IntentRequest) (*IntentPlan, error)
    // Apply 执行已确认的计划，通过 WorkflowService
    Apply(ctx context.Context, planID string, approvedBy string) error
}

type IntentRequest struct {
    WorkspaceID string
    Intent      string
    DryRun      bool
}

type IntentPlan struct {
    ID        string
    Summary   string
    Changes   []CRDChange   // before/after YAML diff
    RiskLevel string        // low | medium | high
    ExpiresAt time.Time     // 10 分钟未执行自动失效
}

type CRDChange struct {
    Action   string // create | update | delete
    Resource string
    Before   string // YAML（空表示新建）
    After    string // YAML（空表示删除）
}
```

### API 端点

```
POST /api/v1/ai/intent/plan   -> IntentPlan（含 planId）
POST /api/v1/ai/intent/apply  -> workflowId（进入审批流）
```

### MCP 工具

```
plan_network_change(intent: string)   -> plan preview
apply_network_change(plan_id: string) -> workflow id
```

### Pro 特性门控

```go
//go:build pro
func NewIntentService(...) IntentService { return &intentService{...} }

//go:build !pro
func NewIntentService(...) IntentService { return &intentServiceStub{} }
func (s *intentServiceStub) Plan(...) (*IntentPlan, error) {
    return nil, ErrPaymentRequired("intent engine is a Pro feature")
}
```

---

## Layer 4：Time-Travel Network Debugging（Pro）

### 快照模型

```go
type NetworkSnapshot struct {
    ID          string
    WorkspaceID string
    Namespace   string
    CapturedAt  time.Time
    TriggerType string    // policy_change | peer_online | peer_offline | manual | scheduled
    TriggerBy   string

    Peers    string // JSON
    Policies string // JSON
    Networks string // JSON
    Presence string // JSON map[appId]status
}
```

### 触发时机

| 触发事件 | 快照类型 |
|---------|---------|
| LatticePolicy 创建/更新/删除 | `policy_change` |
| LatticePeer 上线/下线 | `peer_online` / `peer_offline` |
| WorkflowService 执行完成 | `workflow_executed` |
| 用户手动调 API | `manual` |
| 每天凌晨 2 点 | `scheduled` |

### 快照写入

`internal/server/controller/snapshot_controller.go`：监听 CRD 变更事件，防抖后触发快照。
- 单条快照 < 10KB（语义化 JSON，去除 managedFields）
- 保留 90 天（Pro: 1 年）

### API 端点

```
GET  /api/v1/workspaces/:id/snapshots
GET  /api/v1/workspaces/:id/snapshots/:snapshotId
GET  /api/v1/workspaces/:id/snapshots/diff?from=:id1&to=:id2
POST /api/v1/ai/debug                                          # SSE 根因分析
```

### Debug 专属工具集

| 工具 | 说明 |
|------|------|
| `list_snapshots` | 列出时间范围内的快照 |
| `get_snapshot` | 获取某时间点的完整状态 |
| `diff_snapshots` | 对比两个快照的变更 |
| `check_connectivity_at` | 在指定快照状态下模拟连通性检查 |

### AIService 扩展

```go
type AIService interface {
    Chat(ctx context.Context, req *ChatRequest, out StreamWriter) error
    Audit(ctx context.Context, workspaceID string) (*AuditReport, error)
    Debug(ctx context.Context, req *DebugRequest, out StreamWriter) error  // 新增
}

type DebugRequest struct {
    WorkspaceID string
    Question    string
    TimeRange   TimeRange
}
```

---

## Layer 5：Compliance-as-Conversation（Pro 高客单价）

### 合规框架映射

支持：SOC2 Type II、PCI-DSS、HIPAA。

| 数据来源 | 用途 |
|---------|------|
| LatticePolicy CRD | 验证隔离规则存在且正确 |
| NetworkSnapshot 历史 | 验证审计期内无未授权变更 |
| WorkflowService 审批记录 | 验证每次变更都有 reviewedBy |

PCI-DSS 映射示例：

| PCI-DSS 条款 | 检查逻辑 |
|-------------|---------|
| 1.3 禁止直接公共访问持卡人数据 | 验证 cardholder-data peer 无 ALLOW ingress from * |
| 1.2 限制入站和出站流量 | 验证存在 deny-all 基础策略 |
| 10.2 记录所有访问事件 | 验证每次策略变更都有 reviewedBy 字段 |
| 6.4 变更管理流程 | 验证无 auto_approve 的变更记录 |

### ComplianceService 接口

```go
type ComplianceService interface {
    Assess(ctx context.Context, req AssessRequest) (*ComplianceReport, error)
    GenerateEvidence(ctx context.Context, req EvidenceRequest) ([]byte, error) // ZIP
}

type AssessRequest struct {
    WorkspaceID string
    Framework   string    // soc2 | pci-dss | hipaa
    PeriodStart time.Time
    PeriodEnd   time.Time
}

type ComplianceReport struct {
    Framework   string
    GeneratedAt time.Time
    Score       int
    Status      string          // compliant | non-compliant | partial
    Controls    []ControlResult
    Summary     string          // AI 生成执行摘要（给 CISO）
    EvidenceID  string
}

type ControlResult struct {
    ID          string  // e.g. "PCI-DSS-1.3"
    Title       string
    Status      string  // pass | fail | warning
    Evidence    string  // AI 生成证据描述
    Remediation string
}
```

### 证据包结构

```
lattice-compliance-evidence-2026-05-06.zip
+-- executive-summary.md
+-- controls/
|   +-- PCI-DSS-1.2-result.md
|   +-- PCI-DSS-1.3-result.md
|   +-- ...
+-- raw-data/
|   +-- policies-current.yaml
|   +-- snapshots-timeline.json
|   +-- workflow-audit-log.csv
+-- attestation.json    # SHA256 防篡改签名
```

### 目录结构

```
internal/server/service/compliance/
  framework.go   # 框架定义接口
  soc2.go
  pci.go
  hipaa.go
  report.go      # 报告 + 证据包生成器
```

### 定价定位

| 功能 | Community | Pro |
|------|-----------|-----|
| 基础安全审计（现有 Audit） | 是 | 是 |
| 合规框架评估（Assess） | 402 | 是 |
| 证据包生成（ZIP） | 402 | 是 |
| 自定义合规框架 | 否 | 是 |
| 审计报告历史存档 | 否 | 是（3 年）|

---

## 交付排期（1-2 人 x 1 个月）

| 周次 | 人力 | 任务 | 目标 |
|------|------|------|------|
| Week 1-2 | Person A | L1：MCP Server + 写工具 + Workflow 集成 | Stars + 话题度 |
| Week 2-3 | Person A | L3：IntentService（Plan + Apply） | 付费转化 |
| Week 3-4 | Person A | L4：快照基础设施 + Debug API | 付费转化 |
| Week 1-2 | Person B | L2：Agent Enrollment API + Python SDK | 赛道占位 |
| Week 3-4 | Person B | L5：ComplianceService（PCI-DSS MVP） + 博客 | 企业客户 |

L5 完整三框架实现可延至下个月，Week 3-4 先交付 PCI-DSS 单框架 MVP。

---

## 完整能力地图

| Layer | 功能 | 主要受众 | 商业模式 |
|-------|------|---------|---------|
| L1 | MCP Server + 写工具 | 开发者 | 开源引流 -> Stars |
| L2 | AI Agent Zero-Trust | AI 工程师 | 开源 + 赛道占位 |
| L3 | Network Intent Engine | 运维/DevOps | Pro |
| L4 | Time-Travel Debugging | SRE | Pro |
| L5 | Compliance-as-Conversation | CISO / 企业 | Pro 高客单价 |
