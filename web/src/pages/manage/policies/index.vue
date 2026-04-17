<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import {
  useVueTable, getCoreRowModel, FlexRender, type ColumnDef,
} from '@tanstack/vue-table'
import {
  Shield, Plus, RefreshCw, MoreHorizontal, Search,
  Pencil, Trash2, ArrowDown, ArrowUp, ChevronLeft, ChevronRight,
  Info, CheckCircle2, XCircle, X,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter,
} from '@/components/ui/dialog'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem,
  DropdownMenuSeparator, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { toast } from 'vue-sonner'
import { usePolicyPageStore } from '@/stores/usePolicyPageStore'
import AppAlertDialog from '@/components/AlertDialog.vue'

definePage({
  meta: { title: '策略管理', description: '管理网络访问控制策略。' },
})

const store = usePolicyPageStore()
onMounted(() => store.actions.refresh())

// ── Types ─────────────────────────────────────────────────────────
type Policy = (typeof store.rows)[number]

// ── Style maps ────────────────────────────────────────────────────
const actionBadge: Record<string, string> = {
  Allow: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 ring-1 ring-emerald-500/20',
  Deny:  'bg-rose-500/10 text-rose-600 dark:text-rose-400 ring-1 ring-rose-500/20',
}
const typeBadge: Record<string, string> = {
  Ingress: 'bg-blue-500/10 text-blue-600 dark:text-blue-400 ring-1 ring-blue-500/20',
  Egress:  'bg-violet-500/10 text-violet-600 dark:text-violet-400 ring-1 ring-violet-500/20',
}

// ── Delete confirm ────────────────────────────────────────────────
const deleteTarget     = ref<Policy | null>(null)
const deleteDialogOpen = ref(false)

function promptDelete(policy: Policy) {
  deleteTarget.value = policy
  deleteDialogOpen.value = true
}
async function confirmDelete() {
  if (deleteTarget.value) await store.actions.handleDelete(deleteTarget.value, toast)
  deleteTarget.value = null
}

// ── Detail dialog ─────────────────────────────────────────────────
const detailOpen   = ref(false)
const detailPolicy = ref<Policy | null>(null)

function openDetail(policy: Policy) {
  detailPolicy.value = policy
  detailOpen.value = true
}

// ── Search & action filter (client-side over loaded rows) ─────────
const searchValue    = ref('')
const actionFilter   = ref<'all' | 'Allow' | 'Deny'>('all')

const filtered = computed(() => {
  const q = searchValue.value.toLowerCase().trim()
  return store.rows.filter(p => {
    const matchSearch = !q || p.name?.toLowerCase().includes(q) || p.description?.toLowerCase().includes(q)
    const matchAction = actionFilter.value === 'all' || p.action === actionFilter.value
    return matchSearch && matchAction
  })
})

// ── Stats ─────────────────────────────────────────────────────────
const stats = computed(() => {
  const rows  = store.rows as any[]
  const allow = rows.filter(p => (p.action ?? 'Allow') === 'Allow').length
  const deny  = rows.filter(p => p.action === 'Deny').length
  const total = rows.length
  const ingressRules = rows.reduce((s, p) => s + (p.ingress?.length ?? 0), 0)
  const egressRules  = rows.reduce((s, p) => s + (p.egress?.length ?? 0), 0)
  const totalRules   = ingressRules + egressRules
  return {
    total, allow, deny, totalRules, ingressRules, egressRules,
    allowRate: total ? Math.round((allow / total) * 100) : 0,
    denyRate:  total ? Math.round((deny  / total) * 100) : 0,
    avgRules:  total ? (totalRules / total).toFixed(1) : '0',
  }
})

// ── Pagination (server-side) ───────────────────────────────────────
const PAGE_SIZE    = store.params.pageSize
const currentPage  = computed(() => store.params.page)
const totalPages   = computed(() => Math.max(1, Math.ceil(store.total / PAGE_SIZE)))
const visiblePages = computed(() => {
  const cur   = currentPage.value
  const total = totalPages.value
  const start = Math.max(1, Math.min(cur - 1, total - 2))
  const end   = Math.min(total, start + 2)
  return Array.from({ length: end - start + 1 }, (_, i) => start + i)
})

function goToPage(p: number) {
  if (p < 1 || p > totalPages.value) return
  store.params.page = p
  store.actions.refresh()
}

// ── Search debounce ───────────────────────────────────────────────
let searchTimer: ReturnType<typeof setTimeout>
function onSearchInput() {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => { actionFilter.value = 'all' }, 400)
}

function setActionFilter(val: typeof actionFilter.value) {
  actionFilter.value = val
  searchValue.value = ''
}

// ── Quick templates ───────────────────────────────────────────────
const templates = [
  { key: 'isolate',  label: '全隔离',    desc: 'Deny All In/Out' },
  { key: 'db',       label: '数据库保护', desc: 'Postgres Ingress' },
  { key: 'internet', label: '放通出口',   desc: 'Allow HTTPS Out' },
]

// ── Column definitions ────────────────────────────────────────────
const columns: ColumnDef<Policy>[] = [
  {
    accessorKey: 'name',
    header: '策略名称',
    cell: ({ row }) => {
      const p = row.original
      const isDeny = p.action === 'Deny'
      return h('div', { class: 'flex items-center gap-3' }, [
        h('div', {
          class: `size-9 rounded-lg flex items-center justify-center shrink-0 ${isDeny ? 'bg-rose-500/10' : 'bg-emerald-500/10'}`,
        }, h(Shield, { class: `size-4 ${isDeny ? 'text-rose-500' : 'text-emerald-500'}` })),
        h('div', { class: 'min-w-0' }, [
          h('p', { class: 'font-semibold text-sm leading-none' }, p.name),
          h('p', { class: 'text-[11px] text-muted-foreground mt-1 truncate max-w-48' }, p.description || '无描述'),
        ]),
      ])
    },
  },
  {
    accessorKey: 'action',
    header: '动作',
    cell: ({ row }) => {
      const action = row.original.action ?? 'Allow'
      return h('span', {
        class: `text-xs font-semibold px-2 py-0.5 rounded-full ${actionBadge[action] ?? actionBadge.Allow}`,
      }, action)
    },
  },
  {
    accessorKey: 'policyTypes',
    header: '方向',
    cell: ({ row }) => {
      const types: string[] = row.original.policyTypes ?? []
      if (!types.length) return h('span', { class: 'text-xs text-muted-foreground/40' }, '—')
      return h('div', { class: 'flex gap-1 flex-wrap' },
        types.map(t => h('span', {
          class: `flex items-center gap-0.5 text-[11px] font-bold px-2 py-0.5 rounded-md ${typeBadge[t] ?? 'bg-muted text-muted-foreground'}`,
        }, [
          h(t === 'Ingress' ? ArrowDown : ArrowUp, { class: 'size-3' }),
          t,
        ]))
      )
    },
  },
  {
    id: 'selector',
    header: '目标选择器',
    cell: ({ row }) => {
      const labels = row.original.peerSelector?.matchLabels ?? {}
      const entries = Object.entries(labels)
      if (!entries.length) return h('span', { class: 'text-[11px] text-muted-foreground/40 italic' }, '未设置')
      return h('div', { class: 'flex flex-wrap gap-1' },
        entries.map(([k, v]) =>
          h('span', {
            class: 'font-mono text-[10px] px-1.5 py-0.5 rounded bg-muted/60 text-muted-foreground ring-1 ring-border',
          }, `${k}=${v}`)
        )
      )
    },
  },
  {
    id: 'rules',
    header: '规则数',
    cell: ({ row }) => {
      const p = row.original
      const ingress = p.ingress?.length ?? 0
      const egress  = p.egress?.length ?? 0
      return h('div', { class: 'flex items-center gap-2 text-xs text-muted-foreground' }, [
        ingress > 0 && h('span', { class: 'flex items-center gap-0.5' }, [h(ArrowDown, { class: 'size-3 text-blue-500' }), ingress]),
        egress > 0  && h('span', { class: 'flex items-center gap-0.5' }, [h(ArrowUp, { class: 'size-3 text-violet-500' }), egress]),
        !ingress && !egress && h('span', { class: 'text-muted-foreground/40' }, '—'),
      ])
    },
  },
  {
    id: 'actions',
    header: '',
    cell: ({ row }) => {
      const policy = row.original
      return h('div', { onClick: (e: Event) => e.stopPropagation() }, [h(DropdownMenu, {}, {
        default: () => [
          h(DropdownMenuTrigger, { asChild: true }, () =>
            h(Button, { variant: 'ghost', size: 'sm', class: 'size-8 p-0' }, () =>
              h(MoreHorizontal, { class: 'size-4' })
            )
          ),
          h(DropdownMenuContent, { align: 'end', class: 'w-36' }, () => [
            h(DropdownMenuItem, { onClick: () => store.actions.openDrawer('edit', policy) }, () => [
              h(Pencil, { class: 'mr-2 size-3.5' }), '编辑',
            ]),
            h(DropdownMenuSeparator),
            h(DropdownMenuItem, {
              class: 'text-destructive focus:text-destructive',
              onClick: () => promptDelete(policy),
            }, () => [h(Trash2, { class: 'mr-2 size-3.5' }), '删除']),
          ]),
        ],
      })])
    },
  },
]

// ── TanStack Table ────────────────────────────────────────────────
const table = useVueTable({
  get data() { return filtered.value },
  columns,
  getCoreRowModel: getCoreRowModel(),
  manualPagination: true,
  manualFiltering: true,
})
</script>

<template>
  <div class="flex flex-col gap-5 p-6 animate-in fade-in duration-300">

    <!-- ── Stat cards ─────────────────────────────────────────────── -->
    <div class="grid grid-cols-2 sm:grid-cols-4 gap-3">

      <!-- 全部策略 -->
      <button
        class="relative bg-card border border-border rounded-xl p-4 text-left hover:border-primary/30 hover:shadow-sm transition-all group overflow-hidden"
        :class="actionFilter === 'all' ? 'border-primary/40 ring-1 ring-primary/10' : ''"
        @click="setActionFilter('all')"
      >
        <div class="absolute -right-3 -top-3 size-16 rounded-full bg-primary/5 group-hover:bg-primary/8 transition-colors" />
        <div class="relative">
          <div class="flex items-center justify-between mb-3">
            <span class="text-xs font-semibold text-muted-foreground uppercase tracking-wider">全部策略</span>
            <div class="size-7 rounded-lg bg-primary/10 flex items-center justify-center">
              <Shield class="size-3.5 text-primary" />
            </div>
          </div>
          <p class="text-3xl font-black tracking-tighter tabular-nums">{{ stats.total }}</p>
          <p class="text-[11px] text-muted-foreground/60 mt-1">共 <span class="font-bold text-foreground">{{ stats.totalRules }}</span> 条规则</p>
          <!-- Allow / Deny split bar -->
          <div class="mt-3 space-y-1.5">
            <div class="flex h-1.5 rounded-full overflow-hidden bg-muted/50 gap-px">
              <div class="bg-emerald-500 transition-all" :style="{ width: `${stats.allowRate}%` }" />
              <div class="bg-rose-500 transition-all"    :style="{ width: `${stats.denyRate}%` }" />
            </div>
            <div class="flex items-center gap-3 text-[10px] text-muted-foreground/60">
              <span class="flex items-center gap-1"><span class="size-1.5 rounded-full bg-emerald-500 inline-block" />Allow {{ stats.allow }}</span>
              <span class="flex items-center gap-1"><span class="size-1.5 rounded-full bg-rose-500 inline-block" />Deny {{ stats.deny }}</span>
            </div>
          </div>
        </div>
      </button>

      <!-- Allow -->
      <button
        class="relative bg-card border border-border rounded-xl p-4 text-left hover:border-emerald-500/30 hover:shadow-sm transition-all group overflow-hidden"
        :class="actionFilter === 'Allow' ? 'border-emerald-500/40 ring-1 ring-emerald-500/10' : ''"
        @click="setActionFilter('Allow')"
      >
        <div class="absolute -right-3 -top-3 size-16 rounded-full bg-emerald-500/5 group-hover:bg-emerald-500/10 transition-colors" />
        <div class="relative">
          <div class="flex items-center justify-between mb-3">
            <span class="text-xs font-semibold text-muted-foreground uppercase tracking-wider">Allow</span>
            <div class="size-7 rounded-lg bg-emerald-500/10 flex items-center justify-center">
              <CheckCircle2 class="size-3.5 text-emerald-500" />
            </div>
          </div>
          <p class="text-3xl font-black tracking-tighter tabular-nums text-emerald-500">{{ stats.allow }}</p>
          <p class="text-[11px] text-muted-foreground/60 mt-1">
            占比 <span class="font-bold text-emerald-500">{{ stats.allowRate }}%</span>
          </p>
          <div class="mt-3 space-y-1.5">
            <div class="flex h-1.5 rounded-full overflow-hidden bg-muted/50">
              <div class="bg-emerald-500 rounded-full transition-all duration-700" :style="{ width: `${stats.allowRate}%` }" />
            </div>
            <p class="text-[10px] text-muted-foreground/50">放通流量，允许访问</p>
          </div>
        </div>
      </button>

      <!-- Deny -->
      <button
        class="relative bg-card border border-border rounded-xl p-4 text-left hover:border-rose-500/30 hover:shadow-sm transition-all group overflow-hidden"
        :class="actionFilter === 'Deny' ? 'border-rose-500/40 ring-1 ring-rose-500/10' : ''"
        @click="setActionFilter('Deny')"
      >
        <div class="absolute -right-3 -top-3 size-16 rounded-full bg-rose-500/5 group-hover:bg-rose-500/10 transition-colors" />
        <div class="relative">
          <div class="flex items-center justify-between mb-3">
            <span class="text-xs font-semibold text-muted-foreground uppercase tracking-wider">Deny</span>
            <div class="size-7 rounded-lg bg-rose-500/10 flex items-center justify-center">
              <XCircle class="size-3.5 text-rose-500" />
            </div>
          </div>
          <p class="text-3xl font-black tracking-tighter tabular-nums text-rose-500">{{ stats.deny }}</p>
          <p class="text-[11px] text-muted-foreground/60 mt-1">
            占比 <span class="font-bold text-rose-500">{{ stats.denyRate }}%</span>
          </p>
          <div class="mt-3 space-y-1.5">
            <div class="flex h-1.5 rounded-full overflow-hidden bg-muted/50">
              <div class="bg-rose-500 rounded-full transition-all duration-700" :style="{ width: `${stats.denyRate}%` }" />
            </div>
            <p class="text-[10px] text-muted-foreground/50">拦截流量，拒绝访问</p>
          </div>
        </div>
      </button>

      <!-- 总规则数 -->
      <div class="relative bg-card border border-border rounded-xl p-4 text-left overflow-hidden">
        <div class="absolute -right-3 -top-3 size-16 rounded-full bg-primary/5" />
        <div class="relative">
          <div class="flex items-center justify-between mb-3">
            <span class="text-xs font-semibold text-muted-foreground uppercase tracking-wider">总规则数</span>
            <div class="size-7 rounded-lg bg-primary/10 flex items-center justify-center">
              <ArrowDown class="size-3.5 text-primary" />
            </div>
          </div>
          <p class="text-3xl font-black tracking-tighter tabular-nums text-primary">{{ stats.totalRules }}</p>
          <p class="text-[11px] text-muted-foreground/60 mt-1">
            均 <span class="font-bold text-foreground">{{ stats.avgRules }}</span> 条/策略
          </p>
          <div class="mt-3 pt-3 border-t border-border/60 flex items-center gap-3 text-[10px] text-muted-foreground/60">
            <span class="flex items-center gap-1"><ArrowDown class="size-3 text-blue-500" />Ingress {{ stats.ingressRules }}</span>
            <span class="flex items-center gap-1"><ArrowUp class="size-3 text-violet-500" />Egress {{ stats.egressRules }}</span>
          </div>
        </div>
      </div>

    </div>

    <!-- ── Toolbar ────────────────────────────────────────────────── -->
    <div class="flex items-center gap-2">
      <div class="relative w-72">
        <Search class="absolute left-2.5 top-1/2 -translate-y-1/2 size-4 text-muted-foreground" />
        <Input
          v-model="searchValue"
          placeholder="搜索策略名称或描述..."
          class="pl-8 h-9"
          @input="onSearchInput"
        />
      </div>
      <div class="ml-auto flex items-center gap-2">
        <Button variant="outline" size="sm" class="gap-1.5"
          :disabled="store.loading" @click="store.actions.refresh()">
          <RefreshCw class="size-3.5" :class="store.loading ? 'animate-spin' : ''" />
          刷新
        </Button>
        <Button size="sm" class="gap-1.5" @click="store.actions.openDrawer('create')">
          <Plus class="size-3.5" /> 新建策略
        </Button>
      </div>
    </div>

    <!-- ── Data Table ─────────────────────────────────────────────── -->
    <div class="rounded-md border">
      <Table>
        <TableHeader>
          <TableRow v-for="hg in table.getHeaderGroups()" :key="hg.id">
            <TableHead v-for="header in hg.headers" :key="header.id">
              <FlexRender
                v-if="!header.isPlaceholder"
                :render="header.column.columnDef.header"
                :props="header.getContext()"
              />
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <template v-if="table.getRowModel().rows.length">
            <TableRow
              v-for="row in table.getRowModel().rows"
              :key="row.id"
              class="cursor-pointer"
              @click="openDetail(row.original)"
            >
              <TableCell v-for="cell in row.getVisibleCells()" :key="cell.id">
                <FlexRender :render="cell.column.columnDef.cell" :props="cell.getContext()" />
              </TableCell>
            </TableRow>
          </template>
          <TableRow v-else>
            <TableCell :colspan="columns.length" class="h-32 text-center text-muted-foreground">
              {{ store.loading ? '加载中...' : '暂无策略' }}
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>

    <!-- ── Pagination ─────────────────────────────────────────────── -->
    <div class="flex items-center justify-between text-sm text-muted-foreground">
      <span>共 {{ store.total }} 条 · 第 {{ currentPage }} / {{ totalPages }} 页</span>
      <div class="flex items-center gap-1">
        <Button variant="outline" size="sm" class="size-8 p-0"
          :disabled="currentPage <= 1" @click="goToPage(currentPage - 1)">
          <ChevronLeft class="size-4" />
        </Button>
        <Button
          v-for="p in visiblePages" :key="p"
          variant="outline" size="sm" class="size-8 p-0 text-xs"
          :class="p === currentPage ? 'bg-primary text-primary-foreground border-primary hover:bg-primary/90 hover:text-primary-foreground' : ''"
          @click="goToPage(p)"
        >{{ p }}</Button>
        <Button variant="outline" size="sm" class="size-8 p-0"
          :disabled="currentPage >= totalPages" @click="goToPage(currentPage + 1)">
          <ChevronRight class="size-4" />
        </Button>
      </div>
    </div>

    <!-- ── Delete confirm ─────────────────────────────────────────── -->
    <AppAlertDialog
      v-model:open="deleteDialogOpen"
      title="删除策略"
      :description="`确认删除策略「${deleteTarget?.name}」？该操作不可撤销。`"
      confirm-text="删除"
      variant="destructive"
      @confirm="confirmDelete"
      @cancel="deleteTarget = null"
    />

  </div>

  <!-- ── Detail Dialog ─────────────────────────────────────────── -->
  <Dialog :open="detailOpen" @update:open="v => { if (!v) detailOpen = false }">
    <DialogContent class="sm:max-w-lg">
      <DialogHeader>
        <DialogTitle class="flex items-center gap-2.5">
          <div :class="`size-8 rounded-lg flex items-center justify-center shrink-0 ${detailPolicy?.action === 'Deny' ? 'bg-rose-500/10' : 'bg-emerald-500/10'}`">
            <Shield :class="`size-4 ${detailPolicy?.action === 'Deny' ? 'text-rose-500' : 'text-emerald-500'}`" />
          </div>
          <span>{{ detailPolicy?.name }}</span>
          <span :class="`ml-1 text-xs font-semibold px-2 py-0.5 rounded-full ${actionBadge[detailPolicy?.action ?? 'Allow'] ?? actionBadge.Allow}`">
            {{ detailPolicy?.action ?? 'Allow' }}
          </span>
        </DialogTitle>
        <DialogDescription>{{ detailPolicy?.description || '无描述' }}</DialogDescription>
      </DialogHeader>

      <div class="space-y-4 py-1 max-h-[60vh] overflow-y-auto pr-1">

        <!-- Direction -->
        <div class="space-y-1.5">
          <p class="text-xs font-semibold text-muted-foreground uppercase tracking-wider">策略方向</p>
          <div class="flex gap-1.5">
            <template v-if="detailPolicy?.policyTypes?.length">
              <span
                v-for="t in detailPolicy.policyTypes" :key="t"
                :class="`flex items-center gap-1 text-xs font-bold px-2.5 py-1 rounded-md ${typeBadge[t] ?? 'bg-muted text-muted-foreground'}`"
              >
                <ArrowDown v-if="t === 'Ingress'" class="size-3" />
                <ArrowUp v-else class="size-3" />
                {{ t }}
              </span>
            </template>
            <span v-else class="text-xs text-muted-foreground/40 italic">未设置</span>
          </div>
        </div>

        <!-- Peer selector -->
        <div class="space-y-1.5">
          <p class="text-xs font-semibold text-muted-foreground uppercase tracking-wider">目标选择器</p>
          <div class="flex flex-wrap gap-1.5">
            <template v-if="Object.keys(detailPolicy?.peerSelector?.matchLabels ?? {}).length">
              <span
                v-for="[k, v] in Object.entries(detailPolicy!.peerSelector!.matchLabels!)" :key="k"
                class="font-mono text-xs px-2 py-0.5 rounded bg-muted/60 text-muted-foreground ring-1 ring-border"
              >{{ k }}={{ v }}</span>
            </template>
            <span v-else class="text-xs text-muted-foreground/40 italic">未设置</span>
          </div>
        </div>

        <!-- Ingress rules -->
        <div v-if="detailPolicy?.ingress?.length" class="space-y-2">
          <p class="text-xs font-semibold flex items-center gap-1.5 text-blue-600 dark:text-blue-400">
            <ArrowDown class="size-3.5" /> Ingress 规则
          </p>
          <div
            v-for="(rule, i) in detailPolicy.ingress" :key="i"
            class="grid grid-cols-2 gap-2 p-3 rounded-lg border border-border bg-muted/20 text-xs"
          >
            <div class="space-y-0.5">
              <p class="text-[10px] text-muted-foreground/50 uppercase font-semibold">来源</p>
              <p class="font-mono">
                {{ (() => { const ml = rule.from?.[0]?.peerSelector?.matchLabels ?? {}; const k = Object.keys(ml)[0]; return k ? `${k}=${ml[k]}` : '—' })() }}
              </p>
            </div>
            <div class="space-y-0.5">
              <p class="text-[10px] text-muted-foreground/50 uppercase font-semibold">端口</p>
              <p class="font-mono">{{ rule.ports?.[0]?.port || '—' }} {{ rule.ports?.[0]?.protocol || '' }}</p>
            </div>
          </div>
        </div>

        <!-- Egress rules -->
        <div v-if="detailPolicy?.egress?.length" class="space-y-2">
          <p class="text-xs font-semibold flex items-center gap-1.5 text-violet-600 dark:text-violet-400">
            <ArrowUp class="size-3.5" /> Egress 规则
          </p>
          <div
            v-for="(rule, i) in detailPolicy.egress" :key="i"
            class="grid grid-cols-2 gap-2 p-3 rounded-lg border border-border bg-muted/20 text-xs"
          >
            <div class="space-y-0.5">
              <p class="text-[10px] text-muted-foreground/50 uppercase font-semibold">目标</p>
              <p class="font-mono">
                {{ (() => { const ml = rule.to?.[0]?.peerSelector?.matchLabels ?? {}; const k = Object.keys(ml)[0]; return k ? `${k}=${ml[k]}` : '—' })() }}
              </p>
            </div>
            <div class="space-y-0.5">
              <p class="text-[10px] text-muted-foreground/50 uppercase font-semibold">端口</p>
              <p class="font-mono">{{ rule.ports?.[0]?.port || '—' }} {{ rule.ports?.[0]?.protocol || '' }}</p>
            </div>
          </div>
        </div>

        <!-- No rules hint -->
        <p
          v-if="!detailPolicy?.ingress?.length && !detailPolicy?.egress?.length"
          class="text-xs text-muted-foreground/40 italic"
        >无具体规则</p>

      </div>

      <DialogFooter>
        <Button variant="outline" @click="detailOpen = false">关闭</Button>
        <Button @click="() => { detailOpen = false; store.actions.openDrawer('edit', detailPolicy) }">
          <Pencil class="size-3.5 mr-1.5" /> 编辑
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>

  <!-- ── Create / Edit Dialog ───────────────────────────────────── -->
  <Dialog :open="store.isDrawerOpen" @update:open="v => { if (!v) store.isDrawerOpen = false }">
    <DialogContent class="sm:max-w-lg">
      <DialogHeader>
        <DialogTitle>{{ store.drawerType === 'create' ? '新建策略' : '编辑策略' }}</DialogTitle>
        <DialogDescription>
          {{ store.drawerType === 'create' ? '定义一条网络访问控制规则' : '修改策略配置' }}
        </DialogDescription>
      </DialogHeader>

      <div class="space-y-4 py-1 max-h-[65vh] overflow-y-auto pr-1">

        <!-- Quick templates (create only) -->
        <div v-if="store.drawerType === 'create'" class="grid grid-cols-3 gap-2">
          <button
            v-for="tpl in templates" :key="tpl.key"
            class="p-2.5 rounded-lg border border-border bg-muted/20 hover:border-primary/40 hover:bg-primary/5 transition-all text-left"
            @click="store.actions.applyTemplate(tpl.key)"
          >
            <p class="text-xs font-bold">{{ tpl.label }}</p>
            <p class="text-[10px] text-muted-foreground/60 mt-0.5">{{ tpl.desc }}</p>
          </button>
        </div>

        <div class="grid grid-cols-2 gap-3">
          <!-- Name -->
          <div class="space-y-1.5 col-span-2">
            <label class="text-xs font-medium">策略名称</label>
            <Input v-model="store.form.name" placeholder="例如：deny-all-egress" class="font-mono text-xs" />
          </div>

          <!-- Target label -->
          <div class="space-y-1.5 col-span-2">
            <label class="text-xs font-medium">
              目标选择器
              <span class="text-muted-foreground font-normal ml-1 font-mono text-[10px]">key=value</span>
            </label>
            <Input v-model="store.form._targetLabel" placeholder="app=web" class="font-mono text-xs" />
          </div>

          <!-- Action -->
          <div class="space-y-1.5">
            <label class="text-xs font-medium">动作</label>
            <select
              v-model="store.form.action"
              class="w-full h-9 rounded-md border border-input bg-background px-3 text-sm focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50 transition-[color,box-shadow]"
            >
              <option value="Allow">Allow</option>
              <option value="Deny">Deny</option>
            </select>
          </div>

          <!-- Policy types -->
          <div class="space-y-1.5">
            <label class="text-xs font-medium">策略方向</label>
            <div class="flex gap-2 h-9 items-center">
              <label
                v-for="t in ['Ingress', 'Egress']" :key="t"
                class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border cursor-pointer transition-all select-none text-xs font-semibold"
                :class="store.form.policyTypes?.includes(t)
                  ? (t === 'Ingress' ? 'border-blue-500/50 bg-blue-500/8 text-blue-600 dark:text-blue-400' : 'border-violet-500/50 bg-violet-500/8 text-violet-600 dark:text-violet-400')
                  : 'border-border text-muted-foreground'"
              >
                <input
                  type="checkbox"
                  :checked="store.form.policyTypes?.includes(t)"
                  class="sr-only"
                  @change="store.form.policyTypes?.includes(t)
                    ? store.form.policyTypes.splice(store.form.policyTypes.indexOf(t), 1)
                    : store.form.policyTypes.push(t)"
                />
                <ArrowDown v-if="t === 'Ingress'" class="size-3.5" />
                <ArrowUp v-else class="size-3.5" />
                {{ t }}
              </label>
            </div>
          </div>

          <!-- Description -->
          <div class="space-y-1.5 col-span-2">
            <label class="text-xs font-medium">描述 <span class="text-muted-foreground font-normal">(可选)</span></label>
            <Input v-model="store.form.description" placeholder="简要说明此策略的用途..." />
          </div>
        </div>

        <!-- Ingress rules -->
        <div v-if="store.form.policyTypes?.includes('Ingress')" class="space-y-2">
          <div class="flex items-center justify-between">
            <p class="text-xs font-semibold flex items-center gap-1.5 text-blue-600 dark:text-blue-400">
              <ArrowDown class="size-3.5" /> Ingress 规则
            </p>
            <Button variant="ghost" size="sm" class="h-6 text-[11px] text-primary font-bold px-2"
              @click="store.actions.addRule('ingress')">
              + 添加
            </Button>
          </div>
          <div v-for="(rule, i) in store.form.ingress" :key="i"
            class="grid grid-cols-2 gap-2 p-3 rounded-lg border border-border bg-muted/20 relative group/rule">
            <button
              class="absolute top-2 right-2 size-5 flex items-center justify-center rounded text-muted-foreground/40 hover:text-destructive hover:bg-destructive/10 opacity-0 group-hover/rule:opacity-100 transition-all"
              type="button"
              @click="store.actions.removeRule('ingress', i)"
            >
              <X class="size-3" />
            </button>
            <div class="space-y-1">
              <p class="text-[10px] text-muted-foreground/50 uppercase font-semibold">来源选择器</p>
              <Input v-model="rule._rawLabel" placeholder="app=frontend" class="h-7 text-xs font-mono" />
            </div>
            <div class="space-y-1">
              <p class="text-[10px] text-muted-foreground/50 uppercase font-semibold">端口</p>
              <Input v-model="rule.ports[0].port" placeholder="80" class="h-7 text-xs font-mono" />
            </div>
          </div>
          <p v-if="!store.form.ingress.length" class="text-xs text-muted-foreground/40 italic">无规则 — 拒绝所有入站</p>
        </div>

        <!-- Egress rules -->
        <div v-if="store.form.policyTypes?.includes('Egress')" class="space-y-2">
          <div class="flex items-center justify-between">
            <p class="text-xs font-semibold flex items-center gap-1.5 text-violet-600 dark:text-violet-400">
              <ArrowUp class="size-3.5" /> Egress 规则
            </p>
            <Button variant="ghost" size="sm" class="h-6 text-[11px] text-primary font-bold px-2"
              @click="store.actions.addRule('egress')">
              + 添加
            </Button>
          </div>
          <div v-for="(rule, i) in store.form.egress" :key="i"
            class="grid grid-cols-2 gap-2 p-3 rounded-lg border border-border bg-muted/20 relative group/rule">
            <button
              class="absolute top-2 right-2 size-5 flex items-center justify-center rounded text-muted-foreground/40 hover:text-destructive hover:bg-destructive/10 opacity-0 group-hover/rule:opacity-100 transition-all"
              type="button"
              @click="store.actions.removeRule('egress', i)"
            >
              <X class="size-3" />
            </button>
            <div class="space-y-1">
              <p class="text-[10px] text-muted-foreground/50 uppercase font-semibold">目标选择器</p>
              <Input v-model="rule._rawLabel" placeholder="app=db" class="h-7 text-xs font-mono" />
            </div>
            <div class="space-y-1">
              <p class="text-[10px] text-muted-foreground/50 uppercase font-semibold">端口</p>
              <Input v-model="rule.ports[0].port" placeholder="5432" class="h-7 text-xs font-mono" />
            </div>
          </div>
          <p v-if="!store.form.egress.length" class="text-xs text-muted-foreground/40 italic">无规则 — 拒绝所有出站</p>
        </div>

        <!-- Hint -->
        <div class="flex gap-2 rounded-lg bg-primary/5 border border-primary/10 p-3">
          <Info class="size-4 text-primary shrink-0 mt-0.5" />
          <p class="text-xs text-muted-foreground leading-relaxed">
            策略将以 <code class="font-mono text-xs">WireflowPolicy</code> CRD 形式同步至集群，生效可能需要数秒。
          </p>
        </div>
      </div>

      <DialogFooter>
        <Button variant="outline" @click="store.isDrawerOpen = false">取消</Button>
        <Button :disabled="store.loading" @click="store.actions.handleCreateOrUpdate(toast)">
          <RefreshCw v-if="store.loading" class="size-3.5 animate-spin mr-2" />
          {{ store.drawerType === 'create' ? '发布策略' : '保存更改' }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
