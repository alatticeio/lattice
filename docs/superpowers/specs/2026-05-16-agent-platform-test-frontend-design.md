# Agent Platform — 测试策略与前端界面设计

**日期**：2026-05-16
**分支**：`feat/agent-platform-integrated`
**范围**：四层测试金字塔 + Agent 详情 Drawer 前端设计

---

## 背景

`feat/agent-platform-integrated` 实现了以下子系统：

- **gVisor 沙箱**：Community 版支持 gVisor 隔离模式
- **MCP ToolSpan 追踪**：每次工具调用写入 `la_tool_spans` 表，记录 traceId、status（ok/error/blocked）、duration
- **Sub-agent 委托**：父 Agent 通过 JWT 生成子 Agent 的短期 enrollment token，工具集严格为父集的子集
- **NATS 流量审计（PRO）**：gVisor AuditWriter → NATS `lattice.audit.flow` → `la_flow_events` 表

现有前端 `/sandbox` 页面已有 agents 列表、enrollment tokens 管理、traffic audit（旧格式），但缺少：
1. ToolSpan 追踪日志查看
2. Sub-agent 委托操作入口
3. FlowEvent 网络流量审计（PRO）

---

## 一、测试策略

采用**四层金字塔**，由底向上覆盖。

### Layer 1 — Store 层（已完成，无需新增）

`internal/db/gormstore/tool_span_test.go` 和 `flow_event_test.go` 已覆盖 GORM Write / List。

---

### Layer 2 — Service 层

**文件**：`internal/server/service/agent_registration_test.go`

依赖：
- k8s client：`sigs.k8s.io/controller-runtime/pkg/client/fake.NewClientBuilder()`
- Store：`github.com/glebarez/sqlite` 内存数据库（同 `tool_span_test.go` 模式）

| 测试用例 | 验证点 |
|---|---|
| `TestCreateEnrollmentToken_OK` | 返回 token，ExpiresAt 正确，token 存入 store |
| `TestCreateEnrollmentToken_DefaultTTL` | TTL=0 时默认 1 小时 |
| `TestRegisterAgent_OK` | 消耗 token，创建 LatticePeer + AgentIdentity CRD，返回有效 JWT |
| `TestRegisterAgent_ExpiredToken` | 返回错误 "enrollment token expired" |
| `TestRegisterAgent_UsedToken` | 返回错误 "enrollment token already used" |
| `TestRegisterAgent_SetsParentRef` | token.ParentAgentID 非空时，AgentIdentity.Spec.ParentRef 被赋值 |
| `TestDelegateToken_SubsetTools` | 请求工具 ⊆ 父工具，返回短期 token（TTL 15min） |
| `TestDelegateToken_ToolNotPermitted` | 请求工具超出父权限，返回 error |
| `TestDelegateToken_SpawnableRole_OK` | roleName 在父 SpawnableRoles 内，工具从 role template AgentIdentity 取 |
| `TestDelegateToken_RoleNotInSpawnableRoles` | roleName 不在 SpawnableRoles，返回 error |
| `TestValidateAgentJWT_WrongAlg` | 非 HMAC 签名方法被拒绝 |

---

### Layer 3 — Handler 层

**文件**：`internal/server/server/agent_isolation_handler_test.go`

使用 `net/http/httptest` + `gin.New()` + mock `AgentRegistrationService` 接口，不依赖真实数据库或 k8s。

| 测试用例 | 验证点 |
|---|---|
| `TestHandleCreateEnrollmentToken_OK` | 200，响应包含 token 和 expiresAt |
| `TestHandleCreateEnrollmentToken_MissingNamespace` | 400 Bad Request |
| `TestHandleAgentRegister_OK` | 200，响应包含 jwt 和 agentIdentityName |
| `TestHandleAgentRegister_InvalidToken` | 400，错误信息透传 |
| `TestHandleAgentRegister_ServiceNil` | 402 Payment Required |
| `TestHandleIsolationAgentRevoke_NoNamespace` | 400，namespace 参数缺失 |
| `TestHandleListTraces_QueryParams` | agentId / from / to / limit 参数正确透传 service |
| `TestHandleDelegate_NoBearerToken` | 401 Unauthorized |
| `TestHandleDelegate_InvalidParentJWT` | 400，错误信息包含 "invalid parent JWT" |

---

### Layer 4 — E2E with envtest

**文件**：`internal/server/server/agent_isolation_e2e_test.go`

使用 `envtest` 加载真实 CRD（`config/crd/bases/`），启动完整 Server 实例，通过 HTTP 调用验证端到端链路。

**链路验证顺序**：

```
1. POST /enrollment-tokens          → 创建 token
2. POST /register                   → 注册 Agent，返回 JWT
3. ValidateAgentJWT                 → 验证 JWT claims（AgentID、Namespace、AllowedTools）
4. ExecuteTool(list_peers)          → 工具调用成功，写入 ToolSpan
5. GET /audit/traces?agentId=...    → 查询到刚写入的 span，status=ok
6. POST /delegate (父 JWT)          → 生成子 Agent enrollment token（工具子集）
7. POST /register (子 Agent)        → 子 Agent 注册，返回子 JWT
8. ExecuteTool(list_peers) 子 Agent → 写入 ToolSpan，ParentID = 父 AgentID
9. GET /audit/traces?agentId=sub-*  → 子 Agent traces 中 parentId 正确关联
```

子 Agent 测试工具集为父集的子集（如父有 `["list_peers","create_peer"]`，子仅 `["list_peers"]`），并验证子 Agent 调用 `create_peer` 时返回 blocked。

---

## 二、前端界面设计

### 2.1 文件结构

```
fronted/src/
├── api/
│   └── sandbox.ts                    ← 扩展类型 + 新增 API 函数
├── composables/
│   └── useAgentDetailDrawer.ts       ← 抽屉状态管理 + 数据 fetch
└── pages/sandbox/
    ├── index.vue                     ← 行点击 → 触发 drawer.open(agent)
    ├── AgentDetailDrawer.vue         ← 主抽屉组件
    └── components/
        ├── TracesSplitPanel.vue      ← Traces tab：左列表 + 右详情面板
        ├── NetworkFlowTable.vue      ← Network tab：FlowEvent 表格（PRO）
        └── SubAgentsPanel.vue        ← Sub-agents tab：卡片列表 + 委托 Dialog
```

---

### 2.2 API 层扩展（sandbox.ts）

新增类型：

```typescript
export interface ToolSpan {
  traceId: string
  agentId: string
  parentId?: string
  namespace: string
  tool: string
  status: 'ok' | 'error' | 'blocked'
  errorMsg?: string
  durationMs: number
  startedAt: string
}

export interface FlowEvent {
  traceId: string
  agentId: string
  direction: 'egress' | 'ingress'
  dstIp: string
  dstPort: number
  bytes: number
  ts: string
}
```

新增 API 函数：

```typescript
export const listTraces = (
  agentId: string,
  params?: { from?: string; to?: string; limit?: number }
): Promise<ToolSpan[]> =>
  request.get('/agent-isolation/audit/traces', { agentId, ...params })

export const getTrace = (traceId: string): Promise<ToolSpan> =>
  request.get(`/agent-isolation/audit/traces/${traceId}`)

export const listFlowEvents = (traceId: string): Promise<FlowEvent[]> =>
  request.get('/agent-isolation/flow-events', { traceId })

// 委托子 Agent：扩展现有 createToken，传入 parentAgentId
export const createDelegateToken = (input: {
  allowedTools: string[]
  ttlSeconds: number
  namespace: string
  parentAgentId: string
}): Promise<EnrollmentToken> =>
  request.post('/agent-isolation/enrollment-tokens', input)
```

---

### 2.3 Composable：useAgentDetailDrawer

```typescript
// 职责：管理抽屉开关、当前 agent、tab 切换、三个数据源的 fetch 状态
interface AgentDetailDrawerState {
  open: boolean
  agent: SandboxAgent | null
  activeTab: 'traces' | 'network' | 'subagents'
  traces: ToolSpan[]
  tracesLoading: boolean
  selectedTrace: ToolSpan | null
  flowEvents: FlowEvent[]
  flowLoading: boolean
  subAgents: SandboxAgent[]
  subAgentsLoading: boolean
}
```

- `open(agent)` 打开抽屉，自动 fetch traces（默认 tab）
- tab 切换时按需 fetch 对应数据，避免预加载未查看的 tab
- `close()` 清空所有状态

---

### 2.3b Stats 卡片数据来源

Stats 卡片的三个指标由前端从 `listTraces` 返回的数据中计算：
- **总调用**：`traces.length`
- **成功率**：`(traces.filter(t => t.status === 'ok').length / traces.length * 100).toFixed(0) + '%'`
- **blocked**：`traces.filter(t => t.status === 'blocked').length`

默认拉取最近 100 条用于统计，与列表展示的 50 条分开请求（limit 参数不同）。

---

### 2.4 AgentDetailDrawer 布局

```
┌─────────────────────────────────────────┐
│ Header                                  │
│  claude-agent-01  [online] [gVisor]  10.0.0.5 │
├─────────────────────────────────────────┤
│ Stats 卡片                              │
│  [总调用: 24]  [成功率: 91%]  [blocked: 2] │
├─────────────────────────────────────────┤
│ Tab 栏: [Traces] [Network PRO] [Sub-agents] │
├─────────────────────────────────────────┤
│ Tab 内容（滚动区）                       │
│                                         │
│  Traces tab:                            │
│  ┌────────────────┬──────────────────┐  │
│  │ 左：trace 列表 │ 右：选中 trace 详情│  │
│  │ ● list_peers   │ tool: list_peers │  │
│  │ ● create_policy│ status: blocked  │  │
│  │ ● create_peer  │ reason: not in   │  │
│  │                │ AllowedTools     │  │
│  └────────────────┴──────────────────┘  │
└─────────────────────────────────────────┘
```

Drawer 宽度：`w-[640px]`，从右侧滑入，使用现有 `Sheet` 组件（shadcn/ui）。

---

### 2.5 Traces Tab（TracesSplitPanel）

- **左侧列表**：每行显示状态色点（绿/红/橙）+ 工具名 + 时间戳 + 耗时
- **右侧详情**：选中 trace 后显示：
  - traceId（可复制）
  - tool / status / durationMs
  - errorMsg（blocked/error 时显示，红色文字）
  - startedAt / namespace
  - parentId（有值时显示，可点击切换到父 Agent）
- 列表顶部有时间范围筛选器（from/to），默认显示最近 50 条

---

### 2.6 Network Tab（NetworkFlowTable，PRO）

- PRO 未启用时：显示空状态 + "升级到 PRO 版本以查看网络流量审计" 提示
- PRO 启用时：表格列为 ts / direction / dstIp:dstPort / bytes
- 需先在 Traces 左侧选中一条 trace，Network tab 才显示该 trace 对应的 FlowEvents（通过 traceId 关联）
- 未选中 trace 时显示提示："请先在 Traces 中选择一条记录"

---

### 2.7 Sub-agents Tab（SubAgentsPanel）

**列表视图**：
- 每个子 Agent 以卡片展示：name + status badge + allowedTools badges
- 顶部显示子 Agent 数量 + 「+ 委托」按钮

**委托 Dialog**：
- 触发：点击「+ 委托」按钮
- 字段：
  - Agent 名称（文本输入，required）
  - 允许的工具（多选，选项来自父 Agent 的 `allowedTools`，全选/取消全选快捷操作）
  - 有效期（TTL 下拉：15分钟 / 1小时）
- 提交：调用扩展后的 `POST /enrollment-tokens`，传入可选字段 `parentAgentId: agent.name`。
  - **注意**：`POST /delegate` 要求 parent agent JWT（由 agent 自身调用），不适合管理员 UI。需在 `handleCreateEnrollmentToken` handler 中新增可选字段 `parentAgentId string`，存入 `AgentEnrollmentToken.ParentAgentID`，令注册后的子 Agent 自动带上 `ParentRef`。
  - 工具校验：前端仅展示父 Agent 的 `allowedTools` 供选择，后端暂不做子集强校验（管理员已知权限边界）。
- 成功后：显示生成的 enrollmentToken（一次性展示，可复制），和 enrollment-tokens 页的 token 展示 Dialog 保持风格一致

---

### 2.8 index.vue 修改

在现有 agents 表格的行上添加点击事件：

```vue
<TableRow
  v-for="agent in store.sandboxes"
  :key="agent.name"
  class="cursor-pointer hover:bg-muted/50"
  @click="drawer.open(agent)"
>
```

页面末尾挂载：

```vue
<AgentDetailDrawer />
```

---

### 2.9 Composable 完整实现

```typescript
// fronted/src/composables/useAgentDetailDrawer.ts
import { ref, computed } from 'vue'
import { listTraces, getTrace, listFlowEvents, createDelegateToken } from '@/api/sandbox'
import type { ToolSpan, FlowEvent, SandboxAgent } from '@/api/sandbox'

export function useAgentDetailDrawer() {
  // ── 核心状态 ──
  const open = ref(false)
  const agent = ref<SandboxAgent | null>(null)
  const activeTab = ref<'traces' | 'network' | 'subagents'>('traces')

  // ── Traces 状态 ──
  const traces = ref<ToolSpan[]>([])
  const tracesLoading = ref(false)
  const selectedTraceId = ref<string | null>(null)
  const traceDetail = ref<ToolSpan | null>(null)
  const traceDetailLoading = ref(false)
  const tracesError = ref<string | null>(null)
  // 统计用（100条）
  const statsTraces = ref<ToolSpan[]>([])

  // ── Network 状态 ──
  const flowEvents = ref<FlowEvent[]>([])
  const flowLoading = ref(false)
  const flowError = ref<string | null>(null)

  // ── Sub-agents 状态 ──
  const subAgents = ref<SandboxAgent[]>([])
  const subAgentsLoading = ref(false)
  const subAgentsError = ref<string | null>(null)

  // ── 委托 Dialog 状态 ──
  const delegateDialogOpen = ref(false)
  const delegateSubmitting = ref(false)
  const delegateResult = ref<{ token: string; expiresAt: string } | null>(null)

  // ── 计算属性 ──
  const selectedTrace = computed(() => traceDetail.value)

  const stats = computed(() => {
    const list = statsTraces.value
    const total = list.length
    if (total === 0) return { total: 0, successRate: '-', blocked: 0 }
    const ok = list.filter(t => t.status === 'ok').length
    const blocked = list.filter(t => t.status === 'blocked').length
    return {
      total,
      successRate: Math.round((ok / total) * 100) + '%',
      blocked,
    }
  })

  const parentAgentId = computed(() => agent.value?.name ?? null)

  // ── 方法 ──

  /** 打开抽屉，开始加载 traces 列表和统计 */
  async function openDrawer(a: SandboxAgent) {
    agent.value = a
    open.value = true
    activeTab.value = 'traces'
    selectedTraceId.value = null
    traceDetail.value = null
    flowEvents.value = []
    subAgents.value = []

    // 并行加载：列表（50条）+ 统计（100条）
    tracesLoading.value = true
    tracesError.value = null
    try {
      const [list, statsList] = await Promise.all([
        listTraces(a.name, { limit: 50 }),
        listTraces(a.name, { limit: 100 }),
      ])
      traces.value = list
      statsTraces.value = statsList
    } catch (e: any) {
      tracesError.value = e?.message ?? 'Failed to load traces'
    } finally {
      tracesLoading.value = false
    }
  }

  /** 关闭抽屉，清空所有状态 */
  function closeDrawer() {
    open.value = false
    agent.value = null
    activeTab.value = 'traces'
    traces.value = []
    statsTraces.value = []
    selectedTraceId.value = null
    traceDetail.value = null
    flowEvents.value = []
    subAgents.value = []
    tracesError.value = null
    flowError.value = null
    subAgentsError.value = null
    delegateDialogOpen.value = false
    delegateResult.value = null
  }

  /** 选中 trace 行，加载详情 */
  async function selectTrace(traceId: string) {
    selectedTraceId.value = traceId
    traceDetailLoading.value = true
    try {
      traceDetail.value = await getTrace(traceId)
    } catch {
      // 从本地列表 fallback
      traceDetail.value = traces.value.find(t => t.traceId === traceId) ?? null
    } finally {
      traceDetailLoading.value = false
    }
  }

  /** 切换 tab */
  function switchTab(tab: 'traces' | 'network' | 'subagents') {
    activeTab.value = tab
    if (tab === 'network') {
      loadFlowEvents()
    } else if (tab === 'subagents') {
      loadSubAgents()
    }
  }

  /** 加载 Network FlowEvents（需先选中 trace） */
  async function loadFlowEvents() {
    if (!selectedTraceId.value) return
    flowLoading.value = true
    flowError.value = null
    try {
      flowEvents.value = await listFlowEvents(selectedTraceId.value)
    } catch (e: any) {
      flowError.value = e?.message ?? 'Failed to load flow events'
    } finally {
      flowLoading.value = false
    }
  }

  /** 加载子 Agent 列表 */
  async function loadSubAgents() {
    if (!agent.value) return
    subAgentsLoading.value = true
    subAgentsError.value = null
    try {
      // 复用现有 agents API，按 parentRef 过滤
      const { request } = await import('@/api/request')
      subAgents.value = await request.get('/agent-isolation/agents', {
        parentRef: agent.value.name,
      })
    } catch (e: any) {
      subAgentsError.value = e?.message ?? 'Failed to load sub-agents'
    } finally {
      subAgentsLoading.value = false
    }
  }

  /** 打开委托 Dialog */
  function openDelegateDialog() {
    delegateResult.value = null
    delegateDialogOpen.value = true
  }

  /** 提交委托请求 */
  async function submitDelegate(input: {
    agentName: string
    allowedTools: string[]
    ttlSeconds: number
  }) {
    if (!agent.value) return
    delegateSubmitting.value = true
    try {
      const result = await createDelegateToken({
        allowedTools: input.allowedTools,
        ttlSeconds: input.ttlSeconds,
        namespace: agent.value.namespace,
        parentAgentId: agent.value.name,
      })
      delegateResult.value = {
        token: result.token,
        expiresAt: result.expiresAt,
      }
      // 刷新子 Agent 列表
      await loadSubAgents()
    } finally {
      delegateSubmitting.value = false
    }
  }

  return {
    // 状态
    open,
    agent,
    activeTab,
    traces,
    tracesLoading,
    tracesError,
    selectedTraceId,
    selectedTrace,
    traceDetailLoading,
    flowEvents,
    flowLoading,
    flowError,
    subAgents,
    subAgentsLoading,
    subAgentsError,
    delegateDialogOpen,
    delegateSubmitting,
    delegateResult,
    parentAgentId,
    // 计算
    stats,
    // 方法
    openDrawer,
    closeDrawer,
    selectTrace,
    switchTab,
    loadFlowEvents,
    loadSubAgents,
    openDelegateDialog,
    submitDelegate,
  }
}

export type UseAgentDetailDrawer = ReturnType<typeof useAgentDetailDrawer>
```

---

### 2.10 组件规格总览

#### 2.10.1 AgentDetailDrawer.vue

| 项目 | 说明 |
|------|------|
| **用途** | 主抽屉容器：Header + StatsCards + Tabs + 内容区 |
| **Props** | 无（通过 `useAgentDetailDrawer()` composable 自驱动） |
| **Provide** | 无需（composable 在每个子组件中 `inject` 或直接 import singleton） |

**模板结构**：

```vue
<template>
  <Sheet v-model:open="open" side="right" class="w-[640px] sm:max-w-[640px]">
    <SheetHeader>
      <SheetTitle>
        <div class="flex items-center gap-2">
          <span class="font-mono text-sm">{{ agent?.name }}</span>
          <StatusBadge :status="agent?.status" />
          <SandboxBadge :type="agent?.sandboxType" />  <!-- gVisor / Process -->
          <span class="text-xs text-muted-foreground">{{ agent?.ipAddress }}</span>
        </div>
      </SheetTitle>
    </SheetHeader>

    <!-- Stats 卡片 -->
    <StatsCards :stats="stats" :loading="tracesLoading" class="px-6 py-3" />

    <!-- Tab 栏 -->
    <Tabs v-model="activeTab" class="px-6">
      <TabsList>
        <TabsTrigger value="traces">Traces</TabsTrigger>
        <TabsTrigger value="network">Network
          <ProBadge v-if="!isPro" />
        </TabsTrigger>
        <TabsTrigger value="subagents">Sub-agents</TabsTrigger>
      </TabsList>
    </Tabs>

    <!-- Tab 内容 -->
    <div class="flex-1 overflow-y-auto px-6 py-4">
      <TracesSplitPanel   v-if="activeTab === 'traces'" />
      <NetworkFlowTable   v-if="activeTab === 'network'" />
      <SubAgentsPanel     v-if="activeTab === 'subagents'" />
    </div>
  </Sheet>
</template>
```

---

#### 2.10.2 TracesSplitPanel.vue

| 项目 | 说明 |
|------|------|
| **用途** | 左列表 + 右详情，分屏展示 ToolSpan |
| **Props** | 无（通过 composable 获取数据） |
| **Emits** | 无 |

**左侧列表**（`w-1/2`）：

```
┌─────────────────────────────┐
│ 时间筛选: [from ▼] [to ▼]   │
│ ────────────────────────────│
│ ● list_peers    2m ago  12ms│  ← 点击选中，高亮底色
│ ● create_policy 5m ago  45ms│
│ ✗ delete_peer   8m ago   3ms│  ← 红色状态点 = error
│ ⊘ create_peer  12m ago   1ms│  ← 橙色状态点 = blocked
│ ...                         │
└─────────────────────────────┘
```

每行模板：
```vue
<div
  v-for="trace in traces"
  :key="trace.traceId"
  class="flex items-center gap-2 py-2 px-3 cursor-pointer rounded-md hover:bg-muted/50"
  :class="{ 'bg-muted': selectedTraceId === trace.traceId }"
  @click="selectTrace(trace.traceId)"
>
  <!-- 状态色点 -->
  <span class="w-2 h-2 rounded-full flex-shrink-0" :class="statusColor(trace.status)" />
  <!-- 工具名 -->
  <span class="flex-1 truncate text-sm font-medium">{{ trace.tool }}</span>
  <!-- 时间戳 -->
  <span class="text-xs text-muted-foreground">{{ timeAgo(trace.startedAt) }}</span>
  <!-- 耗时 -->
  <span class="text-xs text-muted-foreground tabular-nums w-10 text-right">{{ trace.durationMs }}ms</span>
</div>
```

`statusColor(status)` 映射：
- `'ok'` → `bg-green-500`
- `'error'` → `bg-red-500`
- `'blocked'` → `bg-orange-500`

**右侧详情**（`w-1/2`，左侧有 `border-l` 分隔）：

```vue
<template v-if="selectedTrace">
  <div class="space-y-3">
    <!-- traceId 可复制 -->
    <div>
      <Label>Trace ID</Label>
      <div class="flex items-center gap-1">
        <code class="text-xs bg-muted px-2 py-1 rounded">{{ selectedTrace.traceId }}</code>
        <CopyButton :text="selectedTrace.traceId" />
      </div>
    </div>

    <div class="grid grid-cols-2 gap-2">
      <div><Label>Tool</Label> <p class="text-sm">{{ selectedTrace.tool }}</p></div>
      <div><Label>Status</Label> <StatusBadge :status="selectedTrace.status" /></div>
      <div><Label>Duration</Label> <p class="text-sm tabular-nums">{{ selectedTrace.durationMs }}ms</p></div>
      <div><Label>Namespace</Label> <p class="text-sm">{{ selectedTrace.namespace }}</p></div>
    </div>

    <div><Label>Started At</Label> <p class="text-sm">{{ formatTime(selectedTrace.startedAt) }}</p></div>

    <!-- 错误信息 -->
    <div v-if="selectedTrace.errorMsg" class="bg-red-50 border border-red-200 rounded-md p-3">
      <Label class="text-red-700">Error</Label>
      <p class="text-sm text-red-600 font-mono">{{ selectedTrace.errorMsg }}</p>
    </div>

    <!-- 父 Agent 关联 -->
    <div v-if="selectedTrace.parentId" class="flex items-center gap-2 text-sm text-muted-foreground">
      <span>Parent:</span>
      <button class="text-primary hover:underline" @click="navigateToAgent(selectedTrace.parentId!)">
        {{ selectedTrace.parentId }}
      </button>
    </div>
  </div>
</template>

<template v-else>
  <div class="flex items-center justify-center h-full text-sm text-muted-foreground">
    Select a trace from the list
  </div>
</template>
```

---

#### 2.10.3 NetworkFlowTable.vue

| 项目 | 说明 |
|------|------|
| **用途** | 展示选中 trace 的 FlowEvent 表格（PRO） |
| **Props** | 无 |
| **Emits** | 无 |

**状态分支**：

| 条件 | 展示 |
|------|------|
| `!isPro` | 空状态 1 — "升级到 PRO 版本以查看网络流量审计"，配升级链接 |
| `isPro && !selectedTraceId` | 空状态 2 — 提示文案 "请先在 Traces 中选择一条记录" |
| `isPro && flowLoading` | Skeleton（3 行 shimmer） |
| `isPro && flowError` | 错误提示 + Retry 按钮 |
| `isPro && flowEvents.length === 0` | 空状态 3 — "该次调用无网络流量记录" |
| `isPro && flowEvents.length > 0` | 表格展示 |

**表格列**：

| 列 | 宽度 | 渲染方式 |
|----|------|---------|
| ts | 25% | `formatTime(ts)` |
| direction | 15% | Badge: egress (orange) / ingress (blue) |
| dstIp:dstPort | 35% | `<code>dstIp:dstPort</code>` |
| bytes | 25% | 右对齐，`formatBytes(bytes)`（1KB/1MB/1GB 缩写） |

```vue
<template>
  <div>
    <!-- PRO 未启用 -->
    <div v-if="!isPro" class="flex flex-col items-center justify-center py-12 gap-3">
      <ShieldIcon class="w-10 h-10 text-muted-foreground" />
      <p class="text-sm text-muted-foreground">升级到 PRO 版本以查看网络流量审计</p>
      <Button variant="outline" size="sm" as="a" href="/billing">Upgrade</Button>
    </div>

    <!-- 未选中 trace -->
    <div v-else-if="!selectedTraceId" class="flex items-center justify-center py-12">
      <p class="text-sm text-muted-foreground">请先在 Traces 中选择一条记录</p>
    </div>

    <!-- 加载中 -->
    <div v-else-if="flowLoading" class="space-y-2">
      <Skeleton v-for="i in 3" :key="i" class="h-8 w-full" />
    </div>

    <!-- 错误 -->
    <div v-else-if="flowError" class="flex flex-col items-center py-8 gap-2">
      <p class="text-sm text-red-500">{{ flowError }}</p>
      <Button variant="ghost" size="sm" @click="loadFlowEvents()">Retry</Button>
    </div>

    <!-- 空数据 -->
    <div v-else-if="flowEvents.length === 0" class="flex items-center justify-center py-12">
      <p class="text-sm text-muted-foreground">该次调用无网络流量记录</p>
    </div>

    <!-- 表格 -->
    <Table v-else>
      <TableHeader>
        <TableRow>
          <TableHead>Time</TableHead>
          <TableHead>Direction</TableHead>
          <TableHead>Destination</TableHead>
          <TableHead class="text-right">Bytes</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow v-for="event in flowEvents" :key="`${event.ts}-${event.dstPort}`">
          <TableCell class="text-xs">{{ formatTime(event.ts) }}</TableCell>
          <TableCell>
            <Badge :variant="event.direction === 'egress' ? 'outline' : 'secondary'">
              {{ event.direction }}
            </Badge>
          </TableCell>
          <TableCell class="text-xs font-mono">{{ event.dstIp }}:{{ event.dstPort }}</TableCell>
          <TableCell class="text-xs text-right tabular-nums">{{ formatBytes(event.bytes) }}</TableCell>
        </TableRow>
      </TableBody>
    </Table>
  </div>
</template>
```

---

#### 2.10.4 SubAgentsPanel.vue

| 项目 | 说明 |
|------|------|
| **用途** | 子 Agent 列表 + 委托 Dialog |
| **Props** | 无 |
| **Emits** | 无 |

**列表视图**：

```vue
<template>
  <div class="space-y-4">
    <!-- 头部 -->
    <div class="flex items-center justify-between">
      <span class="text-sm text-muted-foreground">
        {{ subAgents.length }} sub-agent{{ subAgents.length !== 1 ? 's' : '' }}
      </span>
      <Button variant="outline" size="sm" @click="openDelegateDialog">
        <PlusIcon class="w-4 h-4 mr-1" />
        委托
      </Button>
    </div>

    <!-- 加载 / 错误 / 空 -->
    <div v-if="subAgentsLoading" class="space-y-2">
      <Skeleton v-for="i in 2" :key="i" class="h-16 w-full" />
    </div>
    <div v-else-if="subAgentsError" class="text-sm text-red-500 py-4">
      {{ subAgentsError }}
      <Button variant="link" size="sm" @click="loadSubAgents()">Retry</Button>
    </div>
    <div v-else-if="subAgents.length === 0" class="text-sm text-muted-foreground py-8 text-center">
      暂无子 Agent，点击「+ 委托」创建
    </div>

    <!-- 卡片列表 -->
    <div v-else class="space-y-2">
      <div
        v-for="sub in subAgents"
        :key="sub.name"
        class="border rounded-lg p-3 space-y-2 cursor-pointer hover:bg-muted/30"
        @click="openDrawer(sub)"
      >
        <div class="flex items-center justify-between">
          <span class="text-sm font-medium font-mono">{{ sub.name }}</span>
          <StatusBadge :status="sub.status" />
        </div>
        <div class="flex flex-wrap gap-1">
          <Badge v-for="tool in sub.allowedTools" :key="tool" variant="secondary" class="text-xs">
            {{ tool }}
          </Badge>
        </div>
      </div>
    </div>

    <!-- 委托 Dialog -->
    <DelegateDialog
      :open="delegateDialogOpen"
      :parent-tools="agent?.allowedTools ?? []"
      :submitting="delegateSubmitting"
      :result="delegateResult"
      @close="delegateDialogOpen = false"
      @submit="submitDelegate"
    />
  </div>
</template>
```

---

#### 2.10.5 DelegateDialog.vue

| 项目 | 说明 |
|------|------|
| **Props** | `open: boolean`, `parentTools: string[]`, `submitting: boolean`, `result: {token:string, expiresAt:string} | null` |
| **Emits** | `close()`, `submit(input: DelegateInput)` |

**两阶段展示**：

- **阶段 1（填写表单）**：`result === null` 时显示表单
- **阶段 2（展示结果）**：`result !== null` 时展示 token + 复制按钮 + 关闭

**表单字段**：

| 字段 | 组件 | 校验 |
|------|------|------|
| Agent 名称 | `<Input>` | required，非空，最长 63 字符 |
| 允许的工具 | 多选 `<Checkbox>` 列表 + "全选/取消全选" 按钮 | 至少勾选 1 个 |
| 有效期 | `<Select>` | 选项: `15min` (900s) / `1h` (3600s) |

**结果展示**：

```
┌───────────────────────────────────────┐
│        委托 Token 已生成              │
│                                       │
│  ┌─────────────────────────────────┐  │
│  │ lat-enr-xxxxxxxxxxxxxxxxxxxxxxx │  │  ← monospace，可选中
│  └─────────────────────────────────┘  │
│  [复制]                               │
│                                       │
│  Token 有效期至: 2026-05-16 15:30     │
│                                       │
│  ⚠ 请立即复制 Token，关闭后无法再次查看 │
│                                       │
│            [关闭]                      │
└───────────────────────────────────────┘
```

```vue
<script setup lang="ts">
import { ref, computed, watch } from 'vue'

const props = defineProps<{
  open: boolean
  parentTools: string[]
  submitting: boolean
  result: { token: string; expiresAt: string } | null
}>()

const emit = defineEmits<{
  close: []
  submit: [{ agentName: string; allowedTools: string[]; ttlSeconds: number }]
}>()

const agentName = ref('')
const selectedTools = ref<string[]>([])
const ttlSeconds = ref(900) // 默认 15min

const allSelected = computed(() => selectedTools.value.length === props.parentTools.length)
const canSubmit = computed(() =>
  agentName.value.trim().length > 0 &&
  selectedTools.value.length > 0 &&
  !props.submitting
)

function toggleAll() {
  if (allSelected.value) {
    selectedTools.value = []
  } else {
    selectedTools.value = [...props.parentTools]
  }
}

function handleSubmit() {
  emit('submit', {
    agentName: agentName.value.trim(),
    allowedTools: [...selectedTools.value],
    ttlSeconds: ttlSeconds.value,
  })
}

function handleClose() {
  agentName.value = ''
  selectedTools.value = []
  ttlSeconds.value = 900
  emit('close')
}

function copyToken() {
  if (props.result) {
    navigator.clipboard.writeText(props.result.token)
  }
}
</script>
```

---

### 2.11 状态矩阵

#### 每个数据区的四种状态

| 数据区 | Loading | Empty | Error | Normal |
|--------|---------|-------|-------|--------|
| **Stats 卡片** | 3 个 Skeleton 占位 | 显示 0/0/0（不算空状态） | Toast 提示，卡片隐藏 | 显示数值 |
| **Traces 列表** | 5 行 Skeleton 占位 | "该 Agent 尚无工具调用记录" | 错误文案 + Retry 按钮 | 列表渲染 |
| **Trace 详情** | 右侧面板 shimmer | 空白 + "Select a trace" | —（列表 fallback） | 字段展示 |
| **Network 表格** | 3 行 Skeleton | 见 2.10.3 多级空状态 | 错误 + Retry | 表格 |
| **Sub-agents 列表** | 2 张 Skeleton 卡片 | "暂无子 Agent" | 错误 + Retry | 卡片列表 |

#### 委托 Dialog 状态

| 状态 | 表现 |
|------|------|
| 填写中 | 表单可编辑，Submit 按钮受 `canSubmit` 控制 |
| 提交中 | Submit 按钮显示 spinner + "创建中..."，字段 disabled |
| 提交成功 | 切换到结果展示，Token 可复制 |
| 提交失败 | 表单保留，顶部显示错误 toast（或 inline error） |

---

### 2.12 i18n 最小 Key 清单

新增组件需要的 i18n key（zh/en）。实现时统一补到 `fronted/src/locales/`。

```json
{
  "agent.traces": "Traces / 调用追踪",
  "agent.network": "Network / 网络流量",
  "agent.subAgents": "Sub-agents / 子代理",
  "agent.totalCalls": "总调用",
  "agent.successRate": "成功率",
  "agent.blocked": "blocked / 已拦截",
  "agent.traceDetail": "Trace Detail / 调用详情",
  "agent.traceId": "Trace ID",
  "agent.noTraces": "No trace records yet / 尚无工具调用记录",
  "agent.selectTrace": "Select a trace from the list / 请从列表中选择一条记录",
  "agent.networkPro": "Upgrade to PRO to view network audit / 升级到 PRO 版本以查看网络流量审计",
  "agent.networkNoTrace": "Select a trace in Traces tab first / 请先在 Traces 中选择一条记录",
  "agent.networkEmpty": "No network traffic in this trace / 该次调用无网络流量记录",
  "agent.delegate": "Delegate / 委托",
  "agent.delegateName": "Agent Name / 代理名称",
  "agent.delegateTools": "Allowed Tools / 允许的工具",
  "agent.delegateTtl": "TTL / 有效期",
  "agent.delegateToken": "Delegation Token / 委托令牌",
  "agent.delegateCopyWarning": "Copy now, cannot be retrieved later / 请立即复制，关闭后无法再次查看",
  "agent.noSubAgents": "No sub-agents yet / 暂无子代理",
  "agent.parentAgent": "Parent / 父代理"
}
```

---

## 三、实现顺序与依赖

前端实现分三步，每一步可独立验证：

### Step 1 — API 层 + Drawer 骨架（0.5 天）

**文件**：
- `fronted/src/api/sandbox.ts` — 扩展类型 + API 函数
- `fronted/src/composables/useAgentDetailDrawer.ts` — 完整 composable
- `fronted/src/pages/sandbox/AgentDetailDrawer.vue` — 主抽屉 + Stats 卡片 + Tab 切换

**验收**：点击 agent 行 → 打开抽屉 → 看到 Header + Stats 骨架 → tab 切换正常 → 关闭抽屉

**关键前提**：后端 `GET /audit/traces` API 已实现（Sprint 2）

---

### Step 2 — Traces + Network（1 天）

**文件**：
- `fronted/src/pages/sandbox/components/TracesSplitPanel.vue`
- `fronted/src/pages/sandbox/components/NetworkFlowTable.vue`

**依赖**：
- Traces：后端 `GET /audit/traces` + `GET /audit/traces/:id` 可用
- Network：`GET /flow-events?traceId=`（目前未实现，可在 Step 2 先做 UI 占位，等 Sprint 4 后端补上再对接）

**验收**：
- Traces 列表展示正常，左右分屏交互正确
- 状态色点/时间/耗时正确渲染
- Network tab 三种空状态正确
- 时间范围筛选功能正常

---

### Step 3 — Sub-agents + 委托（1 天）

**文件**：
- `fronted/src/pages/sandbox/components/SubAgentsPanel.vue`
- `fronted/src/pages/sandbox/components/DelegateDialog.vue`

**依赖**：
- 后端 `GET /agent-isolation/agents?parentRef=` 查询 API（需确认是否已有，或通过现有 agents 列表 + 前端过滤实现）
- 后端 `POST /enrollment-tokens` 支持 `parentAgentId` 字段

**验收**：
- 子 Agent 卡片列表正常展示
- 委托 Dialog 全选/取消全选逻辑正确
- 提交成功 → Token 可复制
- 提交失败 → 错误展示

---

### Step 4 — 整合 + index.vue（0.5 天）

**文件**：
- `fronted/src/pages/sandbox/index.vue`

**改动**：行点击绑定 + 抽屉挂载

**验收**：完整交互流程——打开抽屉 → 查看 traces → 选中 trace 看详情 → 切到 Network → 切到 Sub-agents → 打开委托 Dialog → 提交 → 关闭

---

## 四、不在本次范围内

- FlowEvent 后端 REST 查询 API（`GET /flow-events?traceId=`）目前未实现，Network tab 先预留空状态，待 Sprint 4 补充
- Agent 树形多级嵌套视图（Sub-agents tab 仅显示直接子 Agent，不递归）
- i18n key 补充（实现时按 2.12 清单统一补 locales）
- `GET /audit/calltree` 调用树接口（后续需求，预留 click parent 入口）
