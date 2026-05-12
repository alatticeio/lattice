<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useWorkspaceStore } from '@/stores/workspace'
import {
  Clock, Send, Loader2, AlertCircle, Eye, Layers, Server, ShieldCheck, Network,
  Bot, Copy, Check,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import hljs from 'highlight.js/lib/core'
import bash from 'highlight.js/lib/languages/bash'
import json from 'highlight.js/lib/languages/json'
import yaml from 'highlight.js/lib/languages/yaml'
import { listSnapshots, diffSnapshots, type NetworkSnapshot, type DiffResult } from '@/api/snapshot'
import { streamDebug } from '@/api/debug'

hljs.registerLanguage('bash', bash)
hljs.registerLanguage('shell', bash)
hljs.registerLanguage('json', json)
hljs.registerLanguage('yaml', yaml)

marked.use({
  renderer: {
    code({ text, lang }: { text: string; lang?: string }) {
      const language = lang && hljs.getLanguage(lang) ? lang : 'plaintext'
      let highlighted: string
      try {
        highlighted = hljs.highlight(text, { language }).value
      } catch {
        highlighted = text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
      }
      const encoded = encodeURIComponent(text)
      return `<div class="code-block my-3 rounded-lg overflow-hidden border border-white/10">
        <div class="code-block-header flex items-center justify-between px-4 py-1.5 bg-zinc-800 border-b border-white/10">
          <span class="text-[11px] text-zinc-400 font-mono">${language}</span>
          <button class="code-copy-btn text-[11px] text-zinc-400 hover:text-white border border-zinc-600 rounded px-2 py-0.5 transition-colors" data-code="${encoded}">复制</button>
        </div>
        <pre class="overflow-x-auto bg-zinc-950 m-0"><code class="hljs language-${language} text-xs font-mono !p-4 block leading-relaxed">${highlighted}</code></pre>
      </div>`
    },
  },
})

definePage({
  meta: { titleKey: 'common.ai.debug.title', descKey: 'common.ai.debug.desc' },
})

const { t } = useI18n()
const workspaceStore = useWorkspaceStore()

// Snapshots
const snapshots = ref<NetworkSnapshot[]>([])
const snapsLoading = ref(false)
const selectedId = ref<string | null>(null)
const compareIds = ref<string[]>([])

// AI debug
const question = ref('')
const askedQuestion = ref('')
const composing = ref(false)
const debugLoading = ref(false)
const debugResult = ref('')
const debugError = ref('')
const copied = ref(false)

interface ThinkingStep {
  tool: string
  label: string
  done: boolean
}
const thinkingSteps = ref<ThinkingStep[]>([])

const toolLabels: Record<string, string> = {
  list_snapshots: '查询快照列表',
  get_snapshot: '读取快照详情',
  diff_snapshots: '对比快照差异',
  check_connectivity_at: '检查连通性',
  list_peers: '查询 Peers',
  list_policies: '查询策略',
  list_networks: '查询网络',
  check_connectivity: '检查连通性',
}

function renderMarkdown(text: string): string {
  return DOMPurify.sanitize(marked.parse(text, { async: false }), { ALLOW_DATA_ATTR: true })
}

async function copyResult() {
  await navigator.clipboard.writeText(debugResult.value)
  copied.value = true
  setTimeout(() => { copied.value = false }, 2000)
}

// Snapshot diff
const diffLoading = ref(false)
const diffResult = ref<DiffResult | null>(null)

async function handleDiff() {
  const wsId = workspaceStore.currentWorkspace?.id
  if (!wsId || compareIds.value.length < 2) return
  diffLoading.value = true
  try {
    const res = await diffSnapshots(wsId, compareIds.value[0], compareIds.value[1])
    diffResult.value = res
  } catch (e: any) {
    debugError.value = e?.message || t('common.ai.debug.error')
  } finally {
    diffLoading.value = false
  }
}

const triggerBadgeColor = (type: string) => {
  const map: Record<string, string> = {
    policy_change: 'bg-blue-500/10 text-blue-600 dark:text-blue-400',
    peer_online: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
    peer_offline: 'bg-amber-500/10 text-amber-600 dark:text-amber-400',
  }
  return map[type] || 'bg-muted text-muted-foreground'
}

async function fetchSnapshots() {
  const wsId = workspaceStore.currentWorkspace?.id
  if (!wsId) return
  snapsLoading.value = true
  try {
    const res = await listSnapshots(wsId)
    snapshots.value = res
    if (snapshots.value.length > 0 && !selectedId.value) {
      selectedId.value = snapshots.value[0].id
    }
  } catch {
    // ignore
  } finally {
    snapsLoading.value = false
  }
}

function toggleCompare(id: string) {
  const idx = compareIds.value.indexOf(id)
  if (idx >= 0) {
    compareIds.value.splice(idx, 1)
  } else if (compareIds.value.length < 2) {
    compareIds.value.push(id)
  }
}

async function handleDebug() {
  const wsId = workspaceStore.currentWorkspace?.id
  if (!wsId || !question.value.trim()) return
  debugLoading.value = true
  debugError.value = ''
  const msg = question.value.trim()
  askedQuestion.value = msg
  question.value = ''
  let firstEvent = true
  try {
    diffResult.value = null
    await streamDebug(wsId, msg, undefined, undefined, (event) => {
      // Clear previous result only when the first real event arrives
      if (firstEvent && (event.type === 'token' || event.type === 'tool_use' || event.type === 'error')) {
        firstEvent = false
        debugResult.value = ''
        thinkingSteps.value = []
      }
      if (event.type === 'token' && event.content) {
        thinkingSteps.value.forEach(s => { s.done = true })
        debugResult.value += event.content
      } else if (event.type === 'tool_use' && event.tool) {
        if (thinkingSteps.value.length > 0) {
          thinkingSteps.value[thinkingSteps.value.length - 1].done = true
        }
        thinkingSteps.value.push({
          tool: event.tool,
          label: toolLabels[event.tool] ?? event.tool,
          done: false,
        })
      } else if (event.type === 'error' && event.error) {
        debugError.value = event.error
      } else if (event.type === 'done') {
        thinkingSteps.value.forEach(s => { s.done = true })
        debugLoading.value = false
      }
    })
  } catch (e: any) {
    debugError.value = e?.message || t('common.ai.debug.error')
  } finally {
    debugLoading.value = false
  }
}

const selectedSnapshot = computed(() =>
  snapshots.value.find(s => s.id === selectedId.value) ?? null,
)

const previousSnapshot = computed(() => {
  const idx = snapshots.value.findIndex(s => s.id === selectedId.value)
  return idx >= 0 && idx < snapshots.value.length - 1 ? snapshots.value[idx + 1] : null
})

function parseField(snap: typeof snapshots.value[0] | null, field: 'peers' | 'policies' | 'networks') {
  try { return JSON.parse(snap?.[field] || '[]') } catch { return [] }
}

interface PolicyDiffItem {
  name: string
  status: 'added' | 'removed' | 'changed'
  before?: any
  after?: any
  changedFields: string[]
}

interface PeerDiffItem {
  name: string
  status: 'added' | 'removed'
  ip?: string
}

function diffPolicies(before: any[], after: any[]): PolicyDiffItem[] {
  const beforeMap = Object.fromEntries(before.map(x => [x.name, x]))
  const afterMap  = Object.fromEntries(after.map(x => [x.name, x]))
  const result: PolicyDiffItem[] = []

  for (const k of Object.keys(afterMap)) {
    if (!beforeMap[k]) {
      result.push({ name: k, status: 'added', after: afterMap[k], changedFields: [] })
    } else {
      const b = beforeMap[k], a = afterMap[k]
      const changedFields: string[] = []
      if (b.action !== a.action) changedFields.push('action')
      if (b.network !== a.network) changedFields.push('network')
      if (b.selector !== a.selector) changedFields.push('selector')
      if (changedFields.length > 0)
        result.push({ name: k, status: 'changed', before: b, after: a, changedFields })
    }
  }
  for (const k of Object.keys(beforeMap)) {
    if (!afterMap[k]) result.push({ name: k, status: 'removed', before: beforeMap[k], changedFields: [] })
  }
  return result
}

function formatSelector(selectorStr?: string): string {
  if (!selectorStr || selectorStr === 'null' || selectorStr === '{}') return '所有 Peer'
  try {
    const s = JSON.parse(selectorStr)
    if (s.matchLabels) {
      return Object.entries(s.matchLabels).map(([k, v]) => `${k}=${v}`).join(', ')
    }
    if (s.matchExpressions?.length) {
      return s.matchExpressions.map((e: any) => `${e.key} ${e.operator} [${(e.values||[]).join(',')}]`).join('; ')
    }
    return selectorStr
  } catch { return selectorStr }
}

const snapshotDiff = computed(() => {
  const cur  = selectedSnapshot.value
  const prev = previousSnapshot.value
  if (!cur) return null
  const curPolicies  = parseField(cur, 'policies')
  const prevPolicies = parseField(prev, 'policies')
  const curPeers     = parseField(cur, 'peers')
  const prevPeers    = parseField(prev, 'peers')

  const policies = diffPolicies(prevPolicies, curPolicies)
  const beforePeerNames = new Set(prevPeers.map((p: any) => p.name))
  const afterPeerNames  = new Set(curPeers.map((p: any) => p.name))
  const peers: PeerDiffItem[] = [
    ...curPeers.filter((p: any) => !beforePeerNames.has(p.name)).map((p: any) => ({ name: p.name, status: 'added' as const, ip: p.ip })),
    ...prevPeers.filter((p: any) => !afterPeerNames.has(p.name)).map((p: any) => ({ name: p.name, status: 'removed' as const, ip: p.ip })),
  ]
  return { policies, peers, hasPrev: !!prev }
})

function analyzeSnapshot() {
  if (!selectedSnapshot.value) return
  const snap = selectedSnapshot.value
  const diff = snapshotDiff.value
  const changes: string[] = []
  if (diff?.hasPrev) {
    diff.policies.forEach(p => {
      if (p.status === 'added') changes.push(`新增策略 ${p.name}`)
      else if (p.status === 'removed') changes.push(`删除策略 ${p.name}`)
      else if (p.status === 'changed') changes.push(`修改策略 ${p.name}（${p.changedFields.join('、')}）`)
    })
    diff.peers.forEach(p => {
      if (p.status === 'added') changes.push(`新增 Peer ${p.name}`)
      else if (p.status === 'removed') changes.push(`Peer ${p.name} 离线`)
    })
  }
  const changeDesc = changes.length > 0
    ? `检测到以下变更：${changes.join('；')}。`
    : '与上一快照相比网络配置无明显变化。'
  question.value = [
    `请分析快照 ${snap.id}（时间：${new Date(snap.capturedAt).toLocaleString()}，触发类型：${snap.triggerType}）。`,
    changeDesc,
    previousSnapshot.value
      ? `上一快照 ID 为 ${previousSnapshot.value.id}，请使用 diff_snapshots 对比这两个快照的详细差异。`
      : '这是最早的快照，请使用 get_snapshot 展示当前完整网络状态。',
    '请给出：① 变更内容摘要；② 可能的网络影响；③ 安全或连通性风险评估。',
  ].join(' ')
  handleDebug()
}

onMounted(fetchSnapshots)
</script>

<template>
  <div class="flex h-[calc(100vh-var(--header-height,56px)-var(--page-header-height,0px))] overflow-hidden">
    <!-- Left panel: snapshot timeline -->
    <div class="flex w-80 shrink-0 flex-col border-r border-border bg-card">
      <div class="border-b border-border p-4">
        <h3 class="text-sm font-semibold">{{ t('common.ai.debug.timeline') }}</h3>
      </div>
      <div v-if="snapsLoading" class="flex flex-col gap-2 p-4">
        <div v-for="i in 4" :key="i" class="h-16 animate-pulse rounded-lg bg-muted" />
      </div>
      <div v-else-if="snapshots.length === 0" class="flex flex-col items-center gap-2 p-8 text-sm text-muted-foreground">
        <Clock class="size-8 opacity-50" />
        <p>{{ t('common.ai.debug.noSnapshots') }}</p>
      </div>
      <div v-else class="flex-1 space-y-1 overflow-y-auto p-2">
        <div
          v-for="snap in snapshots"
          :key="snap.id"
          class="cursor-pointer rounded-lg p-3 text-sm transition-colors hover:bg-muted/50"
          :class="selectedId === snap.id ? 'bg-muted ring-1 ring-border' : ''"
          @click="selectedId = snap.id"
        >
          <div class="mb-1 flex items-center justify-between">
            <span class="font-medium">{{ new Date(snap.capturedAt).toLocaleTimeString() }}</span>
            <input
              type="checkbox"
              :checked="compareIds.includes(snap.id)"
              class="size-3.5"
              @click.stop="toggleCompare(snap.id)"
            />
          </div>
          <div class="flex items-center gap-2">
            <span class="rounded-full px-2 py-0.5 text-[10px] font-medium" :class="triggerBadgeColor(snap.triggerType)">
              {{ snap.triggerType }}
            </span>
            <span class="text-[11px] text-muted-foreground">{{ snap.triggerBy }}</span>
          </div>
        </div>
      </div>
      <div class="border-t border-border p-2">
        <Button
          variant="outline"
          size="sm"
          class="w-full text-xs"
          :disabled="compareIds.length < 2 || diffLoading"
          @click="handleDiff"
        >
          <Layers class="mr-1 size-3.5" />
          {{ t('common.ai.debug.compareSelected') }}
        </Button>
      </div>
    </div>

    <!-- Right panel: AI debug -->
    <div class="flex flex-1 flex-col">
      <div class="flex-1 overflow-y-auto p-4">
        <Alert v-if="debugError" variant="destructive" class="mb-4">
          <AlertCircle class="size-4" />
          <AlertTitle>{{ t('common.ai.debug.error') }}</AlertTitle>
          <AlertDescription>{{ debugError }}</AlertDescription>
        </Alert>

        <!-- No snapshot selected -->
        <div v-if="!selectedSnapshot && !debugResult && !debugLoading && !debugError" class="flex h-full flex-col items-center justify-center gap-2 text-sm text-muted-foreground">
          <Eye class="size-10 opacity-40" />
          <p>从左侧选择一个快照查看变更详情</p>
        </div>

        <!-- Snapshot diff view -->
        <div v-if="selectedSnapshot && !debugResult && !debugLoading && !debugError">
          <!-- Header -->
          <div class="mb-4 flex items-center gap-3">
            <div class="flex-1">
              <p class="font-semibold text-sm">{{ new Date(selectedSnapshot.capturedAt).toLocaleString() }}</p>
              <div class="flex items-center gap-2 mt-1">
                <span class="rounded-full px-2 py-0.5 text-[10px] font-medium" :class="triggerBadgeColor(selectedSnapshot.triggerType)">
                  {{ selectedSnapshot.triggerType }}
                </span>
                <span class="text-xs text-muted-foreground">by {{ selectedSnapshot.triggerBy }}</span>
              </div>
            </div>
            <Button size="sm" variant="outline" class="gap-1.5 text-xs shrink-0" :disabled="debugLoading" @click="analyzeSnapshot">
              <Loader2 v-if="debugLoading" class="size-3 animate-spin" />
              <Send v-else class="size-3" /> 让 AI 分析
            </Button>
          </div>

          <!-- No previous snapshot: show current state -->
          <template v-if="!snapshotDiff?.hasPrev">
            <div class="mb-2 flex items-center gap-2 text-xs text-muted-foreground">
              <Network class="size-3.5" />
              <span>初始快照 · 当前网络状态</span>
            </div>
            <div class="rounded-lg border border-border bg-card overflow-hidden mb-3">
              <p class="border-b border-border px-3 py-2 text-xs font-semibold text-muted-foreground uppercase tracking-wide">Policies ({{ parseField(selectedSnapshot, 'policies').length }})</p>
              <div class="divide-y divide-border">
                <div v-for="pol in parseField(selectedSnapshot, 'policies')" :key="pol.name" class="flex items-center justify-between px-3 py-2 text-sm">
                  <span class="font-medium">{{ pol.name }}</span>
                  <div class="flex items-center gap-2">
                    <span class="rounded px-1.5 py-0.5 text-[10px] font-semibold"
                      :class="pol.action === 'allow' ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400' : 'bg-red-500/10 text-red-600 dark:text-red-400'">
                      {{ pol.action }}
                    </span>
                    <span class="text-xs text-muted-foreground">{{ pol.network }}</span>
                  </div>
                </div>
                <div v-if="parseField(selectedSnapshot, 'policies').length === 0" class="px-3 py-3 text-xs text-muted-foreground">暂无策略</div>
              </div>
            </div>
            <div class="rounded-lg border border-border bg-card overflow-hidden">
              <p class="border-b border-border px-3 py-2 text-xs font-semibold text-muted-foreground uppercase tracking-wide">Peers ({{ parseField(selectedSnapshot, 'peers').length }})</p>
              <div class="divide-y divide-border">
                <div v-for="peer in parseField(selectedSnapshot, 'peers')" :key="peer.name" class="flex items-center justify-between px-3 py-2 text-sm">
                  <span class="font-medium">{{ peer.name }}</span>
                  <span class="font-mono text-xs text-muted-foreground">{{ peer.ip || '—' }}</span>
                </div>
                <div v-if="parseField(selectedSnapshot, 'peers').length === 0" class="px-3 py-3 text-xs text-muted-foreground">暂无 Peer</div>
              </div>
            </div>
          </template>

          <!-- Has previous snapshot: show diff -->
          <template v-else>
            <div class="mb-2 flex items-center gap-2 text-xs text-muted-foreground">
              <Layers class="size-3.5" />
              <span>与上一快照对比 · {{ new Date(previousSnapshot!.capturedAt).toLocaleString() }}</span>
            </div>

            <!-- No changes -->
            <div v-if="snapshotDiff!.policies.length === 0 && snapshotDiff!.peers.length === 0"
              class="flex flex-col items-center gap-2 py-10 text-sm text-muted-foreground">
              <ShieldCheck class="size-8 text-emerald-500 opacity-60" />
              <p>与上一快照相比，网络配置无变化</p>
            </div>

            <!-- Policy changes -->
            <div v-if="snapshotDiff!.policies.length > 0" class="mb-3 space-y-2">
              <p class="text-xs font-semibold text-muted-foreground uppercase tracking-wide flex items-center gap-1.5">
                <ShieldCheck class="size-3.5" /> 策略变更 ({{ snapshotDiff!.policies.length }})
              </p>

              <div v-for="item in snapshotDiff!.policies" :key="item.name"
                class="rounded-lg border bg-card overflow-hidden"
                :class="{
                  'border-emerald-500/30': item.status === 'added',
                  'border-red-500/30': item.status === 'removed',
                  'border-amber-500/30': item.status === 'changed',
                }">
                <!-- Policy header -->
                <div class="flex items-center justify-between px-3 py-2 border-b border-border">
                  <span class="font-semibold text-sm">{{ item.name }}</span>
                  <span class="rounded px-2 py-0.5 text-[10px] font-bold"
                    :class="{
                      'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400': item.status === 'added',
                      'bg-red-500/10 text-red-600 dark:text-red-400': item.status === 'removed',
                      'bg-amber-500/10 text-amber-600 dark:text-amber-400': item.status === 'changed',
                    }">
                    {{ item.status === 'added' ? '+ 新增' : item.status === 'removed' ? '- 删除' : '~ 修改' }}
                  </span>
                </div>

                <!-- Added: show full content -->
                <div v-if="item.status === 'added'" class="px-3 py-2 space-y-1 text-xs">
                  <div class="flex gap-2">
                    <span class="text-muted-foreground w-16 shrink-0">动作</span>
                    <span class="font-mono font-semibold"
                      :class="item.after.action === 'allow' ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-600 dark:text-red-400'">
                      {{ item.after.action }}
                    </span>
                  </div>
                  <div class="flex gap-2">
                    <span class="text-muted-foreground w-16 shrink-0">网络</span>
                    <span class="font-mono">{{ item.after.network || '—' }}</span>
                  </div>
                  <div class="flex gap-2">
                    <span class="text-muted-foreground w-16 shrink-0">选择器</span>
                    <span class="font-mono">{{ formatSelector(item.after.selector) }}</span>
                  </div>
                </div>

                <!-- Removed: show full content with strikethrough -->
                <div v-else-if="item.status === 'removed'" class="px-3 py-2 space-y-1 text-xs opacity-60 line-through">
                  <div class="flex gap-2">
                    <span class="text-muted-foreground w-16 shrink-0">动作</span>
                    <span class="font-mono">{{ item.before.action }}</span>
                  </div>
                  <div class="flex gap-2">
                    <span class="text-muted-foreground w-16 shrink-0">网络</span>
                    <span class="font-mono">{{ item.before.network || '—' }}</span>
                  </div>
                  <div class="flex gap-2">
                    <span class="text-muted-foreground w-16 shrink-0">选择器</span>
                    <span class="font-mono">{{ formatSelector(item.before.selector) }}</span>
                  </div>
                </div>

                <!-- Changed: before → after for each changed field -->
                <div v-else class="divide-y divide-border">
                  <template v-for="field in ['action', 'network', 'selector']" :key="field">
                    <div v-if="item.changedFields.includes(field)"
                      class="px-3 py-2 text-xs">
                      <span class="text-muted-foreground capitalize">{{ field === 'action' ? '动作' : field === 'network' ? '网络' : '选择器' }}</span>
                      <div class="mt-1 flex items-center gap-2 flex-wrap">
                        <span class="rounded bg-red-500/10 px-1.5 py-0.5 font-mono text-red-600 dark:text-red-400 line-through">
                          {{ field === 'selector' ? formatSelector(item.before[field]) : item.before[field] }}
                        </span>
                        <span class="text-muted-foreground">→</span>
                        <span class="rounded bg-emerald-500/10 px-1.5 py-0.5 font-mono text-emerald-600 dark:text-emerald-400">
                          {{ field === 'selector' ? formatSelector(item.after[field]) : item.after[field] }}
                        </span>
                      </div>
                    </div>
                  </template>
                </div>
              </div>
            </div>

            <!-- Peer changes -->
            <div v-if="snapshotDiff!.peers.length > 0" class="rounded-lg border border-border bg-card overflow-hidden">
              <p class="border-b border-border px-3 py-2 text-xs font-semibold text-muted-foreground uppercase tracking-wide flex items-center gap-1.5">
                <Server class="size-3.5" /> Peer 变更 ({{ snapshotDiff!.peers.length }})
              </p>
              <div class="divide-y divide-border">
                <div v-for="item in snapshotDiff!.peers" :key="item.name" class="flex items-center justify-between px-3 py-2.5 text-sm">
                  <div>
                    <span class="font-medium">{{ item.name }}</span>
                    <span v-if="item.ip" class="ml-2 font-mono text-xs text-muted-foreground">{{ item.ip }}</span>
                  </div>
                  <span class="rounded px-1.5 py-0.5 text-[10px] font-bold"
                    :class="item.status === 'added'
                      ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                      : 'bg-red-500/10 text-red-600 dark:text-red-400'">
                    {{ item.status === 'added' ? '+ 加入' : '- 离开' }}
                  </span>
                </div>
              </div>
            </div>
          </template>
        </div>

        <div v-if="diffResult" class="rounded-lg border border-border bg-card p-4 mb-4">
          <div class="flex items-center justify-between mb-2">
            <h4 class="text-sm font-semibold">Snapshot Diff</h4>
            <Button variant="ghost" size="icon" class="size-6" @click="diffResult = null">&times;</Button>
          </div>
          <p class="text-xs text-muted-foreground">{{ diffResult.diffNotes }}</p>
        </div>

        <!-- User question bubble -->
        <div v-if="askedQuestion && (debugLoading || debugResult)" class="mt-4 flex justify-end">
          <div class="max-w-[80%] rounded-2xl rounded-br-sm bg-primary px-4 py-2.5 text-sm text-primary-foreground">
            {{ askedQuestion }}
          </div>
        </div>

        <!-- Thinking steps -->
        <div v-if="thinkingSteps.length > 0 || (debugLoading && !debugResult)" class="mt-3 rounded-xl border border-border bg-card/50 px-4 py-3">
          <p class="mb-2 flex items-center gap-1.5 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
            <Loader2 v-if="debugLoading" class="size-3 animate-spin" />
            <Check v-else class="size-3 text-emerald-500" />
            {{ debugLoading ? '分析中...' : '分析完成' }}
          </p>
          <div class="space-y-1.5">
            <!-- Placeholder dots when no steps yet -->
            <div v-if="thinkingSteps.length === 0 && debugLoading" class="flex items-center gap-2 text-xs text-muted-foreground">
              <span class="flex gap-1">
                <span class="inline-block size-1.5 animate-bounce rounded-full bg-muted-foreground/50" style="animation-delay:0ms" />
                <span class="inline-block size-1.5 animate-bounce rounded-full bg-muted-foreground/50" style="animation-delay:150ms" />
                <span class="inline-block size-1.5 animate-bounce rounded-full bg-muted-foreground/50" style="animation-delay:300ms" />
              </span>
              <span>等待 AI 响应...</span>
            </div>
            <div v-for="(step, i) in thinkingSteps" :key="i" class="flex items-center gap-2 text-xs">
              <span class="shrink-0">
                <Check v-if="step.done" class="size-3.5 text-emerald-500" />
                <Loader2 v-else class="size-3.5 animate-spin text-primary" />
              </span>
              <span :class="step.done ? 'text-foreground' : 'text-primary font-medium'">
                {{ step.label }}
              </span>
              <span class="font-mono text-[10px] text-muted-foreground">{{ step.tool }}</span>
            </div>
          </div>
        </div>

        <!-- AI analysis: streaming + final result -->
        <div v-if="debugResult" class="mt-3 rounded-xl border border-border bg-card overflow-hidden">
          <!-- Card header -->
          <div class="flex items-center justify-between border-b border-border px-4 py-2.5">
            <div class="flex items-center gap-2">
              <div class="flex size-7 shrink-0 items-center justify-center rounded-full bg-primary/10">
                <Bot class="size-4 text-primary" />
              </div>
              <span class="text-sm font-medium">Lattice AI</span>
              <span v-if="debugLoading" class="inline-flex items-center gap-1 rounded-full bg-primary/10 px-2 py-0.5 text-[10px] font-medium text-primary">
                <span class="size-1.5 animate-pulse rounded-full bg-primary" />
                分析中
              </span>
            </div>
            <button
              v-if="!debugLoading"
              class="flex items-center gap-1 rounded-md px-2 py-1 text-xs text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
              @click="copyResult"
            >
              <Check v-if="copied" class="size-3.5 text-emerald-500" />
              <Copy v-else class="size-3.5" />
              {{ copied ? '已复制' : '复制' }}
            </button>
          </div>
          <!-- Rendered markdown body -->
          <div
            class="prose prose-sm max-w-none dark:prose-invert px-4 py-3
              prose-p:my-1.5 prose-li:my-0.5 prose-headings:mt-3 prose-headings:mb-1.5
              prose-code:text-xs prose-pre:p-0 prose-pre:bg-transparent prose-pre:my-0"
            v-html="renderMarkdown(debugResult)"
          />
          <div v-if="debugLoading" class="px-4 pb-3">
            <span class="inline-block h-4 w-0.5 animate-pulse rounded-full bg-foreground/60 align-middle" />
          </div>
        </div>
      </div>

      <div class="border-t border-border p-4">
        <div class="flex gap-2">
          <Input
            v-model="question"
            :placeholder="t('common.ai.debug.inputPlaceholder')"
            :disabled="debugLoading"
            @keyup.enter.exact="!composing && handleDebug()"
            @compositionstart="composing = true"
            @compositionend="composing = false"
          />
          <Button :disabled="debugLoading || !question.trim()" @click="handleDebug">
            <Loader2 v-if="debugLoading" class="size-4 animate-spin" />
            <Send v-else class="size-4" />
          </Button>
        </div>
      </div>
    </div>
  </div>
</template>
