<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import {
  useVueTable, getCoreRowModel, FlexRender, type ColumnDef,
} from '@tanstack/vue-table'
import {
  ShieldCheck, Search, RefreshCw, MoreHorizontal, Plus, Trash2, Pencil,
  Shield, CheckCircle2, XCircle, Tag, X,
  ChevronLeft, ChevronRight, UserSearch, Layers,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Separator } from '@/components/ui/separator'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter,
} from '@/components/ui/dialog'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem,
  DropdownMenuSeparator, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Card, CardContent } from '@/components/ui/card'
import AppAlertDialog from '@/components/AlertDialog.vue'
import { useAgentPolicyStore } from '@/stores/useAgentPolicyStore'
import type { AgentPolicy } from '@/api/agent-policy'

definePage({
  meta: { titleKey: 'manage.agentPolicies.title', descKey: 'manage.agentPolicies.desc' },
})

const { t } = useI18n()
const store = useAgentPolicyStore()

// ── Style maps ────────────────────────────────────────────────────
const modeLabel: Record<string, string> = {
  deny: 'bg-rose-500/10 text-rose-600 dark:text-rose-400 ring-1 ring-rose-500/20',
  allow: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 ring-1 ring-emerald-500/20',
}

// ── Stats / filter ────────────────────────────────────────────────
type FilterMode = 'all' | 'deny' | 'allow'
const filterMode = ref<FilterMode>('all')
const searchValue = ref('')

const stats = computed(() => {
  const all = store.rows
  return {
    total: all.length,
    defaultDeny: all.filter(p => p.spec.defaultDeny !== false).length,
    defaultAllow: all.filter(p => p.spec.defaultDeny === false).length,
    toolRules: all.reduce((sum, p) => sum + (p.spec.allowedTools?.length ?? 0), 0),
  }
})

function setFilter(val: FilterMode) {
  filterMode.value = val
  searchValue.value = ''
}

// ── Tab state ─────────────────────────────────────────────────────
const activeTab = ref<'policies' | 'agentView'>('policies')

// ── Agent View state ──────────────────────────────────────────────
const agentLabels = ref<Record<string, string>>({})
const newLabelKey = ref('')
const newLabelValue = ref('')

function addLabel() {
  const k = newLabelKey.value.trim()
  const v = newLabelValue.value.trim()
  if (!k) return
  agentLabels.value = { ...agentLabels.value, [k]: v || '*' }
  newLabelKey.value = ''
  newLabelValue.value = ''
}

function removeLabel(key: string) {
  const { [key]: _, ...rest } = agentLabels.value
  agentLabels.value = rest
}

function matchesSelector(policy: AgentPolicy): boolean {
  const matchLabels = policy.spec.agentSelector?.matchLabels ?? {}
  const entries = Object.entries(matchLabels)
  if (entries.length === 0) return true // empty selector matches all
  return entries.every(([k, v]) => agentLabels.value[k] === v)
}

const matchingPolicies = computed(() => {
  if (Object.keys(agentLabels.value).length === 0) return []
  return store.rows.filter(p => matchesSelector(p))
})

interface EffectiveTool {
  name: string
  allowed: boolean
  via: string // policy name or 'wildcard'
}

interface EffectiveServer {
  mcpServer: string
  tools: EffectiveTool[]
  allAllowed: boolean
}

const effectiveAccess = computed<EffectiveServer[]>(() => {
  const labels = agentLabels.value
  if (Object.keys(labels).length === 0) return []

  const matched = store.rows.filter(p => matchesSelector(p))

  // If no policies match, all tools allowed
  if (matched.length === 0) return []

  // If any matching policy is defaultDeny=false, all tools allowed
  if (matched.some(p => p.spec.defaultDeny === false)) {
    // Collect all known MCP servers from all policies
    const servers = new Set<string>()
    store.rows.forEach(p => p.spec.allowedTools?.forEach(at => servers.add(at.mcpServer)))
    return Array.from(servers).map(s => ({
      mcpServer: s,
      tools: [],
      allAllowed: true,
    }))
  }

  // All matching policies are defaultDeny=true — merge allowed tools
  const serverMap = new Map<string, Map<string, string>>() // server → tool → policy name
  for (const policy of matched) {
    for (const at of policy.spec.allowedTools ?? []) {
      if (!serverMap.has(at.mcpServer)) serverMap.set(at.mcpServer, new Map())
      const toolMap = serverMap.get(at.mcpServer)!
      for (const tool of at.tools) {
        if (!toolMap.has(tool)) toolMap.set(tool, policy.name)
      }
    }
  }

  return Array.from(serverMap.entries()).map(([mcpServer, toolMap]) => ({
    mcpServer,
    tools: Array.from(toolMap.entries()).map(([name, via]) => ({
      name,
      allowed: true,
      via,
    })),
    allAllowed: false,
  }))
})

const filtered = computed(() => {
  const q = searchValue.value.toLowerCase().trim()
  return store.rows.filter(p => {
    const matchSearch = !q
      || p.name.toLowerCase().includes(q)
      || (p.spec.allowedTools ?? []).some(at => at.mcpServer.toLowerCase().includes(q))
    const isDeny = p.spec.defaultDeny !== false
    const matchMode = filterMode.value === 'all'
      || (filterMode.value === 'deny' && isDeny)
      || (filterMode.value === 'allow' && !isDeny)
    return matchSearch && matchMode
  })
})

// ── Pagination ─────────────────────────────────────────────────────
const PAGE_SIZE = 10
const currentPage = ref(1)
const totalPages = computed(() => Math.max(1, Math.ceil(filtered.value.length / PAGE_SIZE)))
const paged = computed(() => {
  const start = (currentPage.value - 1) * PAGE_SIZE
  return filtered.value.slice(start, start + PAGE_SIZE)
})
const visiblePages = computed(() => {
  const cur = currentPage.value
  const total = totalPages.value
  const start = Math.max(1, Math.min(cur - 1, total - 2))
  const end = Math.min(total, start + 2)
  return Array.from({ length: end - start + 1 }, (_, i) => start + i)
})
function goToPage(p: number) {
  if (p < 1 || p > totalPages.value) return
  currentPage.value = p
}

// ── Selector string helper ────────────────────────────────────────
function selectorStr(p: AgentPolicy): string {
  const labels = p.spec.agentSelector?.matchLabels ?? {}
  return Object.entries(labels).map(([k, v]) => `${k}=${v}`).join(', ')
}

// ── Column definitions ─────────────────────────────────────────────
const columns = computed<ColumnDef<AgentPolicy>[]>(() => [
  {
    id: 'status',
    header: '',
    size: 40,
    cell: ({ row }) => {
      const isDeny = row.original.spec.defaultDeny !== false
      return isDeny
        ? h(Shield, { class: 'size-4 text-rose-500' })
        : h(CheckCircle2, { class: 'size-4 text-emerald-500' })
    },
  },
  {
    id: 'name',
    header: t('manage.agentPolicies.colName'),
    cell: ({ row }) => {
      const p = row.original
      const isDeny = p.spec.defaultDeny !== false
      return h('div', { class: 'flex flex-col gap-0.5' }, [
        h('span', { class: 'font-semibold text-sm leading-none' }, p.name),
        h('span', { class: 'text-[11px] text-muted-foreground' },
          isDeny ? t('manage.agentPolicies.defaultDeny') : t('manage.agentPolicies.defaultAllow')),
      ])
    },
  },
  {
    id: 'selector',
    header: t('manage.agentPolicies.colSelector'),
    cell: ({ row }) => {
      const labels = row.original.spec.agentSelector?.matchLabels ?? {}
      const entries = Object.entries(labels)
      if (!entries.length) return h('span', { class: 'text-[11px] text-muted-foreground/40 italic' }, '* (all agents)')
      return h('div', { class: 'flex flex-wrap gap-1' },
        entries.map(([k, v]) =>
          h('span', {
            class: 'inline-flex items-center gap-1 text-[11px] font-medium px-1.5 py-0.5 rounded bg-blue-500/10 text-blue-600 dark:text-blue-400',
          }, [
            h(Tag, { class: 'size-2.5' }),
            `${k}=${v}`,
          ])
        )
      )
    },
  },
  {
    id: 'tools',
    header: t('manage.agentPolicies.colAllowedTools'),
    cell: ({ row }) => {
      const allowed = row.original.spec.allowedTools ?? []
      if (!allowed.length) return h('span', { class: 'text-[11px] text-muted-foreground/40' }, '—')
      const totalTools = allowed.reduce((sum, at) => sum + at.tools.length, 0)
      const servers = allowed.slice(0, 2).map(at =>
        h('span', {
          class: 'text-[11px] font-medium px-1.5 py-0.5 rounded bg-violet-500/10 text-violet-600 dark:text-violet-400',
          title: at.tools.join(', '),
        }, `${at.mcpServer} (${at.tools.length})`)
      )
      const rest = allowed.length - 2
      return h('div', { class: 'flex flex-col gap-1' }, [
        h('span', { class: 'text-xs font-medium' }, `${totalTools} ${t('manage.agentPolicies.toolLabel')}`),
        h('div', { class: 'flex flex-wrap gap-1' }, [
          ...servers,
          rest > 0 && h('span', { class: 'text-[11px] text-muted-foreground px-1 py-0.5' }, `+${rest}`),
        ]),
      ])
    },
  },
  {
    id: 'mode',
    header: t('manage.agentPolicies.colMode'),
    cell: ({ row }) => {
      const isDeny = row.original.spec.defaultDeny !== false
      return h('span', {
        class: `text-xs font-medium px-2 py-0.5 rounded-full ${isDeny ? modeLabel.deny : modeLabel.allow}`,
      }, isDeny ? 'defaultDeny' : 'defaultAllow')
    },
  },
  {
    id: 'actions',
    header: '',
    cell: ({ row }) => {
      const policy = row.original
      return h(DropdownMenu, {}, {
        default: () => [
          h(DropdownMenuTrigger, { asChild: true }, () =>
            h(Button, { variant: 'ghost', size: 'sm', class: 'size-8 p-0' }, () =>
              h(MoreHorizontal, { class: 'size-4' })
            )
          ),
          h(DropdownMenuContent, { align: 'end', class: 'w-40' }, () => [
            h(DropdownMenuItem, { onClick: () => openDetail('view', policy) }, () => [
              h(ChevronRight, { class: 'mr-2 size-3.5' }), t('manage.agentPolicies.actions.view'),
            ]),
            h(DropdownMenuItem, { onClick: () => openDetail('edit', policy) }, () => [
              h(Pencil, { class: 'mr-2 size-3.5' }), t('manage.agentPolicies.actions.edit'),
            ]),
            h(DropdownMenuSeparator),
            h(DropdownMenuItem, {
              class: 'text-destructive focus:text-destructive',
              onClick: () => promptDelete(policy),
            }, () => [h(Trash2, { class: 'mr-2 size-3.5' }), t('common.action.delete')]),
          ]),
        ],
      })
    },
  },
])

// ── TanStack Table ─────────────────────────────────────────────────
const table = useVueTable({
  get data() { return paged.value },
  get columns() { return columns.value },
  getCoreRowModel: getCoreRowModel(),
  manualPagination: true,
  manualFiltering: true,
})

// ── Detail / Edit dialog ──────────────────────────────────────────
const detailOpen = ref(false)
const detailType = ref<'view' | 'edit'>('view')
const detailPolicy = ref<AgentPolicy | null>(null)
const editDefaultDeny = ref(true)
const editSelectorStr = ref('')
const editAllowedTools = ref<Array<{ mcpServer: string; tools: string[] }>>([])

function openDetail(type: 'view' | 'edit', policy: AgentPolicy) {
  detailType.value = type
  detailPolicy.value = policy
  editDefaultDeny.value = policy.spec.defaultDeny !== false
  editSelectorStr.value = selectorStr(policy)
  editAllowedTools.value = JSON.parse(JSON.stringify(policy.spec.allowedTools ?? []))
  detailOpen.value = true
}

function addServerRule() {
  editAllowedTools.value.push({ mcpServer: '', tools: [] })
}
function removeServerRule(i: number) {
  editAllowedTools.value.splice(i, 1)
}

async function handleDetailSave() {
  if (!detailPolicy.value) return
  store.formName = detailPolicy.value.name
  store.formSpec.defaultDeny = editDefaultDeny.value
  const labels: Record<string, string> = {}
  editSelectorStr.value.split(',').forEach(pair => {
    const [k, v] = pair.split('=').map(s => s.trim())
    if (k && v) labels[k] = v
  })
  store.formSpec.agentSelector.matchLabels = labels
  store.formSpec.allowedTools = editAllowedTools.value.filter(at => at.mcpServer.trim())
  store.drawerType = 'edit'
  store.selectedPolicy = detailPolicy.value
  const ok = await store.handleCreateOrUpdate()
  if (ok) {
    toast.success(t('manage.agentPolicies.editSuccess'))
    detailOpen.value = false
  } else {
    toast.error(t('manage.agentPolicies.saveFailed'))
  }
}

// ── Create dialog ─────────────────────────────────────────────────
const formSelectorStr = computed({
  get: () => store.getSelectorLabel(),
  set: (v: string) => store.setSelectorLabel(v),
})

function openCreate() {
  store.openDrawer('create')
}

function addFormServerRule() {
  if (!store.formSpec.allowedTools) store.formSpec.allowedTools = []
  store.formSpec.allowedTools.push({ mcpServer: '', tools: [] })
}
function removeFormServerRule(i: number) {
  store.formSpec.allowedTools?.splice(i, 1)
}

async function handleCreate() {
  if (!store.formName.trim()) return
  const ok = await store.handleCreateOrUpdate()
  if (ok) {
    toast.success(t('manage.agentPolicies.createSuccess'))
  } else {
    toast.error(t('manage.agentPolicies.saveFailed'))
  }
}

// ── Delete confirm ────────────────────────────────────────────────
const deleteTarget = ref<AgentPolicy | null>(null)
const deleteDialogOpen = ref(false)

function promptDelete(policy: AgentPolicy) {
  deleteTarget.value = policy
  deleteDialogOpen.value = true
}
async function confirmDelete() {
  if (!deleteTarget.value) return
  store.selectedPolicy = deleteTarget.value
  const ok = await store.handleDelete()
  if (ok) {
    toast.success(t('manage.agentPolicies.deleteSuccess'))
    deleteDialogOpen.value = false
    deleteTarget.value = null
  } else {
    toast.error(t('manage.agentPolicies.deleteError'))
  }
}

// ── Lifecycle ─────────────────────────────────────────────────────
onMounted(() => store.refresh())
</script>

<template>
  <div class="flex flex-col gap-5 p-6 animate-in fade-in duration-300">

    <!-- ── Stat cards ─────────────────────────────────────────────── -->
    <div class="grid grid-cols-2 sm:grid-cols-4 gap-4">
      <button
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left hover:shadow-md transition-all"
        :class="filterMode === 'all' ? 'ring-2 ring-blue-500/20 border-blue-500/30' : ''"
        @click="setFilter('all')"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">{{ t('manage.agentPolicies.statTotal') }}</span>
            <span class="text-2xl font-bold tracking-tight">{{ stats.total }}</span>
          </div>
          <div class="bg-blue-500/10 rounded-lg p-2">
            <ShieldCheck class="text-blue-500 size-4" />
          </div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-xs text-muted-foreground">
          <Tag class="size-3.5 shrink-0 text-blue-500" />
          <span>{{ stats.toolRules }} {{ t('manage.agentPolicies.statToolRules') }}</span>
        </div>
      </button>

      <button
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left hover:shadow-md transition-all"
        :class="filterMode === 'deny' ? 'ring-2 ring-rose-500/20 border-rose-500/30' : ''"
        @click="setFilter('deny')"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">{{ t('manage.agentPolicies.statDeny') }}</span>
            <span class="text-2xl font-bold tracking-tight text-rose-600 dark:text-rose-400">{{ stats.defaultDeny }}</span>
          </div>
          <div class="bg-rose-500/10 rounded-lg p-2">
            <XCircle class="text-rose-500 size-4" />
          </div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-xs text-muted-foreground">
          <Shield class="size-3.5 shrink-0 text-rose-500" />
          <span>{{ t('manage.agentPolicies.statDenyHint') }}</span>
        </div>
      </button>

      <button
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left hover:shadow-md transition-all"
        :class="filterMode === 'allow' ? 'ring-2 ring-emerald-500/20 border-emerald-500/30' : ''"
        @click="setFilter('allow')"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">{{ t('manage.agentPolicies.statAllow') }}</span>
            <span class="text-2xl font-bold tracking-tight text-emerald-600 dark:text-emerald-400">{{ stats.defaultAllow }}</span>
          </div>
          <div class="bg-emerald-500/10 rounded-lg p-2">
            <CheckCircle2 class="text-emerald-500 size-4" />
          </div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-xs text-muted-foreground">
          <CheckCircle2 class="size-3.5 shrink-0 text-emerald-500" />
          <span>{{ t('manage.agentPolicies.statAllowHint') }}</span>
        </div>
      </button>

      <button
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left hover:shadow-md transition-all"
        @click="setFilter('all')"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">{{ t('manage.agentPolicies.statToolRules') }}</span>
            <span class="text-2xl font-bold tracking-tight">{{ stats.toolRules }}</span>
          </div>
          <div class="bg-violet-500/10 rounded-lg p-2">
            <Tag class="text-violet-500 size-4" />
          </div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-xs text-muted-foreground">
          <ShieldCheck class="size-3.5 shrink-0 text-violet-500" />
          <span>{{ t('manage.agentPolicies.statAcross', { count: stats.total }) }}</span>
        </div>
      </button>
    </div>

    <!-- ── Tab bar ─────────────────────────────────────────────────── -->
    <div class="flex bg-muted/50 rounded-lg p-1 border border-border gap-1 w-fit">
      <button
        class="px-4 py-1.5 rounded-md text-xs font-semibold transition-all flex items-center gap-1.5"
        :class="activeTab === 'policies'
          ? 'bg-background text-foreground shadow-sm ring-1 ring-border'
          : 'text-muted-foreground hover:text-foreground'"
        @click="activeTab = 'policies'"
      >
        <ShieldCheck class="size-3.5" />
        {{ t('manage.agentPolicies.tabPolicies') }}
      </button>
      <button
        class="px-4 py-1.5 rounded-md text-xs font-semibold transition-all flex items-center gap-1.5"
        :class="activeTab === 'agentView'
          ? 'bg-background text-foreground shadow-sm ring-1 ring-border'
          : 'text-muted-foreground hover:text-foreground'"
        @click="activeTab = 'agentView'"
      >
        <UserSearch class="size-3.5" />
        {{ t('manage.agentPolicies.tabAgentView') }}
      </button>
    </div>

    <!-- ── Policies Tab ───────────────────────────────────────────── -->
    <template v-if="activeTab === 'policies'">

    <!-- ── Toolbar ────────────────────────────────────────────────── -->
    <div class="flex items-center gap-2">
      <div class="relative w-72">
        <Search class="absolute left-2.5 top-1/2 -translate-y-1/2 size-4 text-muted-foreground" />
        <Input v-model="searchValue" :placeholder="t('manage.agentPolicies.searchPlaceholder')" class="pl-8 h-9" />
      </div>
      <div class="ml-auto flex items-center gap-2">
        <Button variant="outline" size="sm" class="gap-1.5" :disabled="store.loading" @click="store.refresh()">
          <RefreshCw class="size-3.5" :class="store.loading ? 'animate-spin' : ''" />
          {{ t('common.action.refresh') }}
        </Button>
        <Button size="sm" class="gap-1.5" @click="openCreate">
          <Plus class="size-3.5" />
          {{ t('manage.agentPolicies.create') }}
        </Button>
      </div>
    </div>

    <!-- ── Data Table ─────────────────────────────────────────────── -->
    <div class="rounded-md border">
      <Table>
        <TableHeader>
          <TableRow v-for="hg in table.getHeaderGroups()" :key="hg.id">
            <TableHead v-for="header in hg.headers" :key="header.id">
              <FlexRender v-if="!header.isPlaceholder" :render="header.column.columnDef.header" :props="header.getContext()" />
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <template v-if="table.getRowModel().rows.length">
            <TableRow v-for="row in table.getRowModel().rows" :key="row.id" class="cursor-pointer" @click="openDetail('view', row.original)">
              <TableCell v-for="cell in row.getVisibleCells()" :key="cell.id"
                @click.stop="cell.column.id === 'actions' ? undefined : openDetail('view', row.original)">
                <FlexRender :render="cell.column.columnDef.cell" :props="cell.getContext()" />
              </TableCell>
            </TableRow>
          </template>
          <TableRow v-else>
            <TableCell :colspan="columns.length" class="h-48 text-center">
              <template v-if="store.loading">
                <span class="text-sm text-muted-foreground">{{ t('common.status.loading') }}</span>
              </template>
              <template v-else>
                <div class="flex flex-col items-center justify-center gap-3 py-6">
                  <ShieldCheck class="size-8 text-muted-foreground/40" />
                  <p class="text-sm text-muted-foreground">{{ t('manage.agentPolicies.empty') }}</p>
                  <Button size="sm" class="gap-1.5" @click="openCreate">
                    <Plus class="size-3.5" />
                    {{ t('manage.agentPolicies.createFirst') }}
                  </Button>
                </div>
              </template>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>

    <!-- ── Pagination ─────────────────────────────────────────────── -->
    <div v-if="filtered.length > 0" class="flex items-center justify-between text-sm text-muted-foreground">
      <span>{{ t('common.pagination.total', { total: filtered.length, page: currentPage, totalPages }) }}</span>
      <div class="flex items-center gap-1">
        <Button variant="outline" size="sm" class="size-8 p-0" :disabled="currentPage <= 1" @click="goToPage(currentPage - 1)">
          <ChevronLeft class="size-4" />
        </Button>
        <Button v-for="p in visiblePages" :key="p" variant="outline" size="sm" class="size-8 p-0 text-xs"
          :class="p === currentPage ? 'bg-primary text-primary-foreground border-primary hover:bg-primary/90 hover:text-primary-foreground' : ''"
          @click="goToPage(p)">{{ p }}</Button>
        <Button variant="outline" size="sm" class="size-8 p-0" :disabled="currentPage >= totalPages" @click="goToPage(currentPage + 1)">
          <ChevronRight class="size-4" />
        </Button>
      </div>
    </div>

    </template><!-- end Policies tab -->

    <!-- ── Agent View Tab ─────────────────────────────────────────── -->
    <template v-if="activeTab === 'agentView'">

      <!-- Label input -->
      <div class="rounded-lg border bg-card p-5 space-y-4">
        <div class="flex items-center gap-2">
          <UserSearch class="size-4 text-muted-foreground" />
          <h3 class="text-sm font-semibold">{{ t('manage.agentPolicies.agentLabels') }}</h3>
        </div>

        <!-- Existing labels -->
        <div class="flex flex-wrap gap-2" v-if="Object.keys(agentLabels).length">
          <span v-for="(v, k) in agentLabels" :key="k"
            class="inline-flex items-center gap-1.5 rounded-md bg-blue-500/10 px-2.5 py-1 text-xs font-medium text-blue-700 dark:text-blue-300">
            <Tag class="size-3" />
            <span class="font-mono">{{ k }}={{ v }}</span>
            <button class="opacity-50 hover:opacity-100 hover:text-destructive transition-colors" @click="removeLabel(k as string)">
              <X class="size-3" />
            </button>
          </span>
        </div>

        <!-- Add label form -->
        <div class="flex items-center gap-2">
          <Input v-model="newLabelKey" :placeholder="t('manage.agentPolicies.labelKey')" class="w-32 h-8 text-xs" />
          <span class="text-muted-foreground text-xs">=</span>
          <Input v-model="newLabelValue" :placeholder="t('manage.agentPolicies.labelValue')" class="w-32 h-8 text-xs" />
          <Button variant="outline" size="sm" class="gap-1 h-8" :disabled="!newLabelKey.trim()" @click="addLabel">
            <Plus class="size-3" /> {{ t('manage.agentPolicies.addLabel') }}
          </Button>
        </div>

        <p class="text-[11px] text-muted-foreground flex items-center gap-1">
          <Shield class="size-3" />
          {{ t('manage.agentPolicies.allPoliciesApply') }}
        </p>
      </div>

      <!-- Empty state -->
      <div v-if="Object.keys(agentLabels).length === 0"
        class="flex flex-col items-center gap-3 py-16 text-sm text-muted-foreground">
        <UserSearch class="size-10 opacity-40" />
        <p>{{ t('manage.agentPolicies.addLabelsHint') }}</p>
      </div>

      <!-- Results -->
      <template v-else>

        <!-- Matching Policies -->
        <div class="rounded-lg border bg-card overflow-hidden">
          <div class="px-5 py-3 border-b bg-muted/30 flex items-center gap-2">
            <ShieldCheck class="size-4 text-muted-foreground" />
            <h3 class="text-sm font-semibold">{{ t('manage.agentPolicies.matchingPolicies') }}</h3>
            <span class="text-xs text-muted-foreground">({{ matchingPolicies.length }})</span>
          </div>
          <div v-if="matchingPolicies.length === 0" class="px-5 py-8 text-center text-sm text-muted-foreground">
            {{ t('manage.agentPolicies.noPolicyMatch') }}
          </div>
          <div v-else class="divide-y">
            <div v-for="policy in matchingPolicies" :key="policy.name"
              class="px-5 py-3 flex items-center gap-3 hover:bg-muted/30 cursor-pointer"
              @click="openDetail('view', policy)">
              <span class="shrink-0" :class="policy.spec.defaultDeny !== false ? 'text-rose-500' : 'text-emerald-500'">
                <Shield v-if="policy.spec.defaultDeny !== false" class="size-4" />
                <CheckCircle2 v-else class="size-4" />
              </span>
              <div class="flex-1 min-w-0">
                <span class="text-sm font-semibold">{{ policy.name }}</span>
                <span class="ml-2 text-[11px] px-1.5 py-0.5 rounded-full"
                  :class="policy.spec.defaultDeny !== false ? modeLabel.deny : modeLabel.allow">
                  {{ policy.spec.defaultDeny !== false ? 'defaultDeny' : 'defaultAllow' }}
                </span>
              </div>
              <span class="text-xs text-muted-foreground font-mono">{{ selectorStr(policy) || '*' }}</span>
            </div>
          </div>
        </div>

        <!-- Effective Access -->
        <div v-if="effectiveAccess.length > 0" class="rounded-lg border bg-card overflow-hidden">
          <div class="px-5 py-3 border-b bg-muted/30 flex items-center gap-2">
            <Layers class="size-4 text-muted-foreground" />
            <h3 class="text-sm font-semibold">{{ t('manage.agentPolicies.effectiveAccess') }}</h3>
          </div>
          <div class="divide-y">
            <div v-for="server in effectiveAccess" :key="server.mcpServer" class="px-5 py-4 space-y-2">
              <div class="flex items-center gap-2">
                <Shield class="size-3.5 text-muted-foreground" />
                <span class="text-sm font-semibold">{{ server.mcpServer }}</span>
                <span v-if="server.allAllowed" class="text-[11px] px-1.5 py-0.5 rounded-full bg-emerald-500/10 text-emerald-600 dark:text-emerald-400">
                  {{ t('manage.agentPolicies.allowedByWildcard') }}
                </span>
              </div>
              <div v-if="!server.allAllowed" class="flex flex-wrap gap-1.5 ml-5.5">
                <span v-for="tool in server.tools" :key="tool.name"
                  class="inline-flex items-center gap-1 text-[11px] font-medium px-2 py-0.5 rounded"
                  :class="tool.allowed
                    ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
                    : 'bg-rose-500/10 text-rose-600 dark:text-rose-400'">
                  <CheckCircle2 v-if="tool.allowed" class="size-2.5" />
                  <XCircle v-else class="size-2.5" />
                  {{ tool.name }}
                  <span class="text-[9px] opacity-60 ml-0.5">via {{ tool.via }}</span>
                </span>
              </div>
            </div>
          </div>
        </div>

        <!-- No effective restrictions -->
        <div v-if="matchingPolicies.length > 0 && effectiveAccess.length === 0"
          class="rounded-lg border bg-card p-8 text-center">
          <CheckCircle2 class="size-8 text-emerald-500 mx-auto mb-2" />
          <p class="text-sm text-muted-foreground">
            {{ t('manage.agentPolicies.noPolicyMatch') }}
          </p>
        </div>

      </template>
    </template><!-- end Agent View tab -->

    <!-- ── Delete confirm ─────────────────────────────────────────── -->
    <AppAlertDialog
      v-model:open="deleteDialogOpen"
      :title="t('manage.agentPolicies.deleteTitle')"
      :description="t('manage.agentPolicies.confirmDelete', { name: deleteTarget?.name })"
      :confirm-text="t('common.action.delete')"
      variant="destructive"
      @confirm="confirmDelete"
      @cancel="deleteTarget = null"
    />
  </div>

  <!-- ── Create Dialog ──────────────────────────────────────────── -->
  <Dialog :open="store.isDrawerOpen && store.drawerType === 'create'" @update:open="v => { if (!v) store.isDrawerOpen = false }">
    <DialogContent class="sm:max-w-lg p-0 gap-0 overflow-hidden">
      <DialogHeader class="px-6 pt-6 pb-4 border-b">
        <DialogTitle class="flex items-center gap-2">
          <ShieldCheck class="size-4" />
          {{ t('manage.agentPolicies.createTitle') }}
        </DialogTitle>
      </DialogHeader>

      <div class="px-6 py-5 space-y-4 max-h-[60vh] overflow-y-auto">
        <div class="space-y-1.5">
          <Label>{{ t('manage.agentPolicies.formName') }}</Label>
          <Input v-model="store.formName" placeholder="my-agent-policy" class="h-9" />
        </div>
        <div class="space-y-1.5">
          <Label>{{ t('manage.agentPolicies.formSelector') }}</Label>
          <Input v-model="formSelectorStr" placeholder="role=worker, env=prod" class="h-9" />
          <p class="text-[11px] text-muted-foreground">{{ t('manage.agentPolicies.formSelectorHelp') }}</p>
        </div>
        <div class="flex items-center justify-between rounded-lg border p-3">
          <div class="space-y-0.5">
            <Label class="text-sm">{{ t('manage.agentPolicies.formDefaultDeny') }}</Label>
            <p class="text-[11px] text-muted-foreground">{{ t('manage.agentPolicies.formDefaultDenyHelp') }}</p>
          </div>
          <Switch v-model="store.formSpec.defaultDeny" />
        </div>

        <Separator />

        <div class="space-y-2">
          <Label>{{ t('manage.agentPolicies.formAllowedTools') }}</Label>
          <div v-for="(at, i) in store.formSpec.allowedTools" :key="i" class="space-y-2 rounded-lg border p-3">
            <div class="flex items-center gap-2">
              <Input v-model="at.mcpServer" :placeholder="t('manage.agentPolicies.formServerName')" class="flex-1 h-8 text-xs" />
              <Button variant="ghost" size="icon" class="size-7 text-destructive shrink-0" @click="removeFormServerRule(i)">
                <X class="size-3.5" />
              </Button>
            </div>
            <Input
              :model-value="at.tools.join(', ')"
              @update:model-value="v => at.tools = (v as string).split(',').map(s => s.trim()).filter(Boolean)"
              placeholder="tool1, tool2, tool3"
              class="h-8 text-xs"
            />
          </div>
          <Button variant="outline" size="sm" class="gap-1 text-xs" @click="addFormServerRule">
            <Plus class="size-3" /> {{ t('manage.agentPolicies.addServerRule') }}
          </Button>
        </div>
      </div>

      <DialogFooter class="px-6 py-4 border-t bg-muted/30 sm:justify-end gap-2">
        <Button variant="outline" @click="store.isDrawerOpen = false">
          {{ t('common.action.cancel') }}
        </Button>
        <Button :disabled="store.loading || !store.formName.trim()" @click="handleCreate">
          <RefreshCw v-if="store.loading" class="size-3.5 animate-spin mr-1.5" />
          {{ t('common.action.create') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>

  <!-- ── Detail / Edit Dialog ──────────────────────────────────── -->
  <Dialog :open="detailOpen" @update:open="v => { if (!v) detailOpen = false }">
    <DialogContent class="sm:max-w-lg p-0 gap-0 overflow-hidden">

      <!-- Header -->
      <DialogHeader class="px-6 pt-6 pb-5 border-b gap-0">
        <div class="flex items-start gap-3 pr-6">
          <div class="relative shrink-0 mt-0.5">
            <div
              class="size-10 rounded-lg border flex items-center justify-center"
              :class="(detailPolicy?.spec.defaultDeny !== false)
                ? 'bg-rose-500/10 border-rose-500/20 text-rose-500'
                : 'bg-emerald-500/10 border-emerald-500/20 text-emerald-500'"
            >
              <ShieldCheck class="size-4" />
            </div>
          </div>

          <div class="flex-1 min-w-0">
            <DialogTitle class="text-sm font-semibold leading-snug truncate">
              {{ detailPolicy?.name }}
            </DialogTitle>
            <p class="text-xs text-muted-foreground mt-0.5">
              {{ (detailPolicy?.spec.defaultDeny !== false) ? t('manage.agentPolicies.defaultDeny') : t('manage.agentPolicies.defaultAllow') }}
            </p>
            <div class="flex items-center gap-2 mt-2 flex-wrap">
              <span
                class="inline-flex items-center text-[11px] font-medium px-2 py-0.5 rounded-md"
                :class="(detailPolicy?.spec.defaultDeny !== false) ? modeLabel.deny : modeLabel.allow"
              >
                {{ (detailPolicy?.spec.defaultDeny !== false) ? 'defaultDeny' : 'defaultAllow' }}
              </span>
              <span v-if="detailPolicy" class="text-[11px] text-muted-foreground font-mono">
                {{ selectorStr(detailPolicy) || '* (all agents)' }}
              </span>
            </div>
          </div>
        </div>
      </DialogHeader>

      <!-- Body -->
      <div v-if="detailPolicy" class="px-6 py-5 space-y-4 max-h-[55vh] overflow-y-auto">

        <!-- Selector card -->
        <Card class="rounded-lg shadow-none py-0">
          <CardContent class="px-4 py-3">
            <p class="text-[10px] text-muted-foreground leading-none mb-1.5">{{ t('manage.agentPolicies.formSelector') }}</p>
            <div class="flex flex-wrap gap-1.5">
              <template v-if="Object.keys(detailPolicy.spec.agentSelector?.matchLabels ?? {}).length">
                <span v-for="(v, k) in detailPolicy.spec.agentSelector.matchLabels" :key="k"
                  class="inline-flex items-center gap-1 rounded-md bg-blue-500/10 px-2 py-1 text-xs font-medium text-blue-700 dark:text-blue-300">
                  <Tag class="size-3" /> {{ k }}={{ v }}
                </span>
              </template>
              <span v-else class="text-xs text-muted-foreground/40 italic">* (all agents)</span>
            </div>
          </CardContent>
        </Card>

        <!-- Edit fields -->
        <template v-if="detailType === 'edit'">
          <div class="space-y-1.5">
            <Label>{{ t('manage.agentPolicies.formSelector') }}</Label>
            <Input v-model="editSelectorStr" placeholder="role=worker, env=prod" class="h-9" />
          </div>
          <div class="flex items-center justify-between rounded-lg border p-3">
            <Label class="text-sm">{{ t('manage.agentPolicies.formDefaultDeny') }}</Label>
            <Switch v-model="editDefaultDeny" />
          </div>
        </template>

        <Separator />

        <!-- Allowed tools -->
        <div class="space-y-2">
          <p class="text-[11px] font-medium text-muted-foreground flex items-center gap-1.5">
            <Tag class="size-3" /> {{ t('manage.agentPolicies.formAllowedTools') }}
          </p>

          <!-- View mode -->
          <div v-if="detailType === 'view'">
            <div v-if="detailPolicy.spec.allowedTools?.length" class="space-y-2">
              <div v-for="(at, i) in detailPolicy.spec.allowedTools" :key="i" class="rounded-md bg-muted px-3 py-2.5 space-y-1">
                <div class="flex items-center gap-2">
                  <Shield class="size-3 text-muted-foreground" />
                  <span class="text-xs font-semibold">{{ at.mcpServer }}</span>
                </div>
                <div class="flex flex-wrap gap-1 ml-5">
                  <span v-for="tool in at.tools" :key="tool"
                    class="text-[11px] font-medium px-1.5 py-0.5 rounded bg-emerald-500/10 text-emerald-600 dark:text-emerald-400">
                    {{ tool }}
                  </span>
                </div>
              </div>
            </div>
            <p v-else class="text-xs text-muted-foreground/50 py-1">{{ t('manage.agentPolicies.noToolsHint') }}</p>
          </div>

          <!-- Edit mode -->
          <div v-else class="space-y-2">
            <div v-for="(at, i) in editAllowedTools" :key="i" class="space-y-2 rounded-lg border p-3">
              <div class="flex items-center gap-2">
                <Input v-model="at.mcpServer" :placeholder="t('manage.agentPolicies.formServerName')" class="flex-1 h-8 text-xs" />
                <Button variant="ghost" size="icon" class="size-7 text-destructive shrink-0" @click="removeServerRule(i)">
                  <X class="size-3.5" />
                </Button>
              </div>
              <Input
                :model-value="at.tools.join(', ')"
                @update:model-value="v => at.tools = (v as string).split(',').map(s => s.trim()).filter(Boolean)"
                placeholder="tool1, tool2, tool3"
                class="h-8 text-xs"
              />
            </div>
            <Button variant="outline" size="sm" class="gap-1 text-xs" @click="addServerRule">
              <Plus class="size-3" /> {{ t('manage.agentPolicies.addServerRule') }}
            </Button>
          </div>
        </div>
      </div>

      <!-- Footer -->
      <DialogFooter class="px-6 py-4 border-t bg-muted/30 sm:justify-between">
        <Button variant="ghost" size="sm" @click="detailOpen = false">
          {{ detailType === 'view' ? t('common.action.close') : t('common.action.cancel') }}
        </Button>
        <Button v-if="detailType === 'edit'" size="sm"
          :disabled="store.loading" @click="handleDetailSave">
          <RefreshCw v-if="store.loading" class="size-3.5 animate-spin mr-1.5" />
          {{ t('common.action.save') }}
        </Button>
        <Button v-else size="sm" variant="outline" @click="detailType = 'edit'">
          <Pencil class="size-3.5 mr-1.5" /> {{ t('manage.agentPolicies.actions.edit') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
