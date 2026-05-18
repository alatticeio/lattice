import { ref, computed, inject, type InjectionKey, type Ref, type ComputedRef } from 'vue'
import {
  listTraces, getTrace, listFlowEvents, createDelegateToken,
  type ToolSpan, type FlowEvent, type SandboxAgent,
} from '@/api/sandbox'

// ── 模块级单例状态（index.vue 和 AgentDetailDrawer.vue 共享） ──

const open = ref(false)
const agent = ref<SandboxAgent | null>(null)
const activeTab = ref<'traces' | 'network' | 'subagents'>('traces')

const traces = ref<ToolSpan[]>([])
const tracesLoading = ref(false)
const selectedTraceId = ref<string | null>(null)
const traceDetail = ref<ToolSpan | null>(null)
const traceDetailLoading = ref(false)
const tracesError = ref<string | null>(null)
const statsTraces = ref<ToolSpan[]>([])

const flowEvents = ref<FlowEvent[]>([])
const flowLoading = ref(false)
const flowError = ref<string | null>(null)

const subAgents = ref<SandboxAgent[]>([])
const subAgentsLoading = ref(false)
const subAgentsError = ref<string | null>(null)

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

// ── 方法 ──

async function openDrawer(a: SandboxAgent) {
  agent.value = a
  open.value = true
  activeTab.value = 'traces'
  selectedTraceId.value = null
  traceDetail.value = null
  flowEvents.value = []
  subAgents.value = []

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

async function selectTrace(traceId: string) {
  selectedTraceId.value = traceId
  traceDetailLoading.value = true
  try {
    traceDetail.value = await getTrace(traceId)
  } catch {
    traceDetail.value = traces.value.find(t => t.traceId === traceId) ?? null
  } finally {
    traceDetailLoading.value = false
  }
}

function switchTab(tab: 'traces' | 'network' | 'subagents') {
  activeTab.value = tab
  if (tab === 'network') {
    loadFlowEvents()
  } else if (tab === 'subagents') {
    loadSubAgents()
  }
}

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

async function loadSubAgents() {
  if (!agent.value) return
  subAgentsLoading.value = true
  subAgentsError.value = null
  try {
    const { listSandboxes } = await import('@/api/sandbox')
    const all = await listSandboxes()
    subAgents.value = all.filter(
      s => s.name !== agent.value!.name && s.allowedTools.some(
        t => agent.value!.allowedTools.includes(t)
      )
    )
  } catch (e: any) {
    subAgentsError.value = e?.message ?? 'Failed to load sub-agents'
  } finally {
    subAgentsLoading.value = false
  }
}

function openDelegateDialog() {
  delegateResult.value = null
  delegateDialogOpen.value = true
}

async function submitDelegate(input: {
  agentName: string
  allowedTools: string[]
  ttlSeconds: number
}) {
  if (!agent.value) return
  delegateSubmitting.value = true
  try {
    const result = await createDelegateToken({
      agentName: input.agentName,
      allowedTools: input.allowedTools,
      ttlSeconds: input.ttlSeconds,
      namespace: agent.value.namespace,
      parentAgentId: agent.value.name,
    })
    delegateResult.value = {
      token: result.token,
      expiresAt: result.expiresAt,
    }
    await loadSubAgents()
  } finally {
    delegateSubmitting.value = false
  }
}

export interface UseAgentDetailDrawer {
  open: Ref<boolean>
  agent: Ref<SandboxAgent | null>
  activeTab: Ref<'traces' | 'network' | 'subagents'>
  traces: Ref<ToolSpan[]>
  tracesLoading: Ref<boolean>
  tracesError: Ref<string | null>
  selectedTraceId: Ref<string | null>
  selectedTrace: ComputedRef<ToolSpan | null>
  traceDetailLoading: Ref<boolean>
  flowEvents: Ref<FlowEvent[]>
  flowLoading: Ref<boolean>
  flowError: Ref<string | null>
  subAgents: Ref<SandboxAgent[]>
  subAgentsLoading: Ref<boolean>
  subAgentsError: Ref<string | null>
  delegateDialogOpen: Ref<boolean>
  delegateSubmitting: Ref<boolean>
  delegateResult: Ref<{ token: string; expiresAt: string } | null>
  stats: ComputedRef<{ total: number; successRate: string; blocked: number }>
  openDrawer: (a: SandboxAgent) => Promise<void>
  closeDrawer: () => void
  selectTrace: (traceId: string) => Promise<void>
  switchTab: (tab: 'traces' | 'network' | 'subagents') => void
  loadFlowEvents: () => Promise<void>
  loadSubAgents: () => Promise<void>
  openDelegateDialog: () => void
  submitDelegate: (input: { agentName: string; allowedTools: string[]; ttlSeconds: number }) => Promise<void>
}

/** 获取模块级单例 drawer 实例 */
export function useAgentDetailDrawer(): UseAgentDetailDrawer {
  return {
    open, agent, activeTab,
    traces, tracesLoading, tracesError,
    selectedTraceId, selectedTrace, traceDetailLoading,
    flowEvents, flowLoading, flowError,
    subAgents, subAgentsLoading, subAgentsError,
    delegateDialogOpen, delegateSubmitting, delegateResult,
    stats,
    openDrawer, closeDrawer, selectTrace, switchTab,
    loadFlowEvents, loadSubAgents, openDelegateDialog, submitDelegate,
  }
}

export const drawerKey: InjectionKey<UseAgentDetailDrawer> = Symbol('agentDetailDrawer')

/** 子组件中注入父抽屉实例 (必须在 AgentDetailDrawer 子组件树内调用) */
export function useDrawer(): UseAgentDetailDrawer {
  const d = inject(drawerKey)
  if (!d) throw new Error('useDrawer() must be called inside AgentDetailDrawer component tree')
  return d
}
