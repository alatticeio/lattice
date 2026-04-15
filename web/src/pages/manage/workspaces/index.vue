<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import {
  useVueTable,
  getCoreRowModel,
  FlexRender,
  type ColumnDef,
} from '@tanstack/vue-table'
import {
  Search, Plus, RefreshCw, MoreHorizontal,
  Layers, Key, ArrowRight, Pencil, Trash2,
  ChevronLeft, ChevronRight, Server,
  Wifi, WifiOff, Network, ArrowUpRight, ArrowDownRight,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Table, TableBody, TableCell, TableHead,
  TableHeader, TableRow,
} from '@/components/ui/table'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter,
} from '@/components/ui/dialog'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem,
  DropdownMenuSeparator, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { useWorkspaceStore, getWsInitials } from '@/stores/workspace'
import {getWsColor} from "@/utils/color";
import type { Workspace } from '@/stores/workspace'
import AppAlertDialog from '@/components/AlertDialog.vue'

definePage({
  meta: { title: '空间管理', description: '管理网络隔离工作空间。' },
})

const store = useWorkspaceStore()
onMounted(() => store.fetchList())

// ── Delete confirm ───────────────────────────────────────────────
const deleteTarget    = ref<Workspace | null>(null)
const deleteDialogOpen = ref(false)

function promptDelete(ws: Workspace) {
  deleteTarget.value = ws
  deleteDialogOpen.value = true
}
async function confirmDelete() {
  if (deleteTarget.value) await store.deleteWorkspace(deleteTarget.value.id)
  deleteTarget.value = null
  await store.fetchList()
}

// ── Edit / Create dialog ─────────────────────────────────────────
const dialogOpen       = ref(false)
const editingWorkspace = ref<Workspace | null>(null)
const form             = ref({ displayName: '', slug: '', maxNodeCount: 20 })

function openCreate() {
  editingWorkspace.value = null
  form.value = { displayName: '', slug: '', maxNodeCount: 20 }
  dialogOpen.value = true
}
function openEdit(ws: Workspace) {
  editingWorkspace.value = ws
  form.value = { displayName: ws.displayName, slug: ws.slug, maxNodeCount: ws.maxNodeCount }
  dialogOpen.value = true
}
async function saveWorkspace() {
  const ok = await store.saveWorkspace(form.value, editingWorkspace.value?.id)
  if (ok) dialogOpen.value = false
}
function slugify(v: string) {
  form.value.slug = v.toLowerCase().replace(/\s+/g, '-').replace(/[^a-z0-9-]/g, '')
}

// ── Helpers ──────────────────────────────────────────────────────
const usagePct = (ws: Workspace) => {
  const used = ws.quotaUsage ?? 0
  const max = ws.nodeCount ?? ws.maxNodeCount ?? 0
  return max > 0 ? Math.round((used / max) * 100) : 0
}

// 格式化创建时间
function formatCreatedAt(isoStr: string | undefined | null): string {
  if (!isoStr) return '—'
  try {
    const date = new Date(isoStr)
    if (isNaN(date.getTime())) return '—'
    // 格式: YYYY-MM-DD HH:mm
    const year = date.getFullYear()
    const month = String(date.getMonth() + 1).padStart(2, '0')
    const day = String(date.getDate()).padStart(2, '0')
    const hours = String(date.getHours()).padStart(2, '0')
    const minutes = String(date.getMinutes()).padStart(2, '0')
    return `${year}-${month}-${day} ${hours}:${minutes}`
  } catch {
    return '—'
  }
}

// 网络状态映射
const networkStatusLabel: Record<string, string> = {
  Ready: '就绪',
  Pending: '等待中',
  Error: '错误',
  Failed: '失败',
}

// ── Stats ────────────────────────────────────────────────────────
const stats = computed(() => {
  const rows     = store.rows
  const active   = rows.filter(w => w.status === 'active').length
  const inactive = rows.filter(w => w.status === 'inactive').length
  const total    = rows.length
  // 使用 quotaUsage（实际使用量）而不是 nodeCount（配额限制）
  const totalNodes = rows.reduce((s, w) => s + (w.quotaUsage ?? 0), 0)
  const topWs = [...rows].sort((a, b) => (b.quotaUsage ?? 0) - (a.quotaUsage ?? 0))[0]
  const networksReady = rows.filter(w => w.networkStatus === 'Ready').length
  return {
    total, active, inactive, totalNodes, networksReady,
    activeRate:  total ? Math.round((active / total) * 100) : 0,
    avgNodes:    total ? (totalNodes / total).toFixed(1) : '0',
    topWsName:   topWs?.displayName ?? '—',
    topWsNodes:  topWs?.quotaUsage ?? 0,
    initials:    rows.slice(0, 3).map(w => ({ label: getWsInitials(w.displayName), color: getWsColor(w.displayName) })),
    networkReadyRate: total ? Math.round((networksReady / total) * 100) : 0,
  }
})

// ── Column definitions ───────────────────────────────────────────
const columns: ColumnDef<Workspace>[] = [
  {
    accessorKey: 'displayName',
    header: '工作空间',
    cell: ({ row }) => {
      const ws = row.original
      return h('div', { class: 'flex items-center gap-3' }, [
        h('div', {
          class: `size-9 rounded-lg flex items-center justify-center shrink-0 text-xs font-bold bg-primary/10 text-primary ring-1 ring-primary/20 ${getWsColor(ws.displayName)}`,
        }, getWsInitials(ws.displayName)),
        h('div', { class: 'min-w-0' }, [
          h('p', { class: 'font-semibold text-sm leading-none' }, ws.displayName),
          h('p', { class: 'font-mono text-[11px] text-muted-foreground mt-1' }, ws.slug),
        ]),
      ])
    },
  },
  {
    accessorKey: 'status',
    header: '状态',
    cell: ({ row }) => {
      const active = row.original.status === 'active'
      return h('div', { class: 'flex items-center gap-1.5' }, [
        h('span', { class: 'relative flex size-1.5 shrink-0' }, [
          active && h('span', { class: 'absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-500 opacity-60' }),
          h('span', { class: `relative inline-flex size-1.5 rounded-full ${active ? 'bg-emerald-500' : 'bg-zinc-400'}` }),
        ]),
        h('span', {
          class: `text-xs font-medium px-2 py-0.5 rounded-full ${active
            ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 ring-1 ring-emerald-500/20'
            : 'bg-muted text-muted-foreground ring-1 ring-border'}`,
        }, active ? '运行中' : '已停用'),
      ])
    },
  },
  {
    accessorKey: 'nodeCount',
    header: '节点使用',
    cell: ({ row }) => {
      const ws = row.original
      // 后端 nodeCount 实际是 Hard（最大值），quotaUsage 是 Used（当前使用量）
      const used = ws.quotaUsage ?? 0
      const max = ws.nodeCount ?? ws.maxNodeCount ?? 0
      return h('div', { class: 'flex flex-col gap-0.5' }, [
        h('div', { class: 'flex items-baseline gap-1' }, [
          h('span', { class: 'font-semibold tabular-nums text-sm' }, String(used)),
          max > 0 && h('span', { class: 'text-[11px] text-muted-foreground/60' }, `/ ${max}`),
        ]),
        max > 0 && h('span', { class: 'text-[10px] text-muted-foreground/50' }, `${Math.round((used / max) * 100)}% 已用`),
      ])
    },
  },
  {
    id: 'usage',
    header: '空间利用率',
    cell: ({ row }) => {
      const pct = usagePct(row.original)
      const barColor = pct > 80 ? 'bg-rose-500' : pct > 60 ? 'bg-amber-500' : 'bg-emerald-500'
      const textColor = pct > 80 ? 'text-rose-500' : pct > 60 ? 'text-amber-500' : 'text-emerald-600 dark:text-emerald-400'
      return h('div', { class: 'flex items-center gap-2.5 w-36' }, [
        h('div', { class: 'flex-1 h-1.5 bg-muted rounded-full overflow-hidden' },
          h('div', { class: `h-full rounded-full transition-all ${barColor}`, style: { width: `${pct}%` } })
        ),
        h('span', { class: `text-xs font-semibold tabular-nums w-8 text-right shrink-0 ${textColor}` }, `${pct}%`),
      ])
    },
  },
  {
    accessorKey: 'tokenCount',
    header: 'Token',
    cell: ({ row }) =>
      h('div', { class: 'flex items-center gap-1.5 text-xs text-muted-foreground' }, [
        h(Key, { class: 'size-3 shrink-0' }),
        h('span', { class: 'tabular-nums font-medium' }, String(row.original.tokenCount)),
      ]),
  },
  {
    id: 'network',
    header: '网络信息',
    cell: ({ row }) => {
      const ws = row.original
      const status = ws.networkStatus

      // 如果完全没有网络信息
      if (!ws.networkName && !ws.networkCIDR && !status) {
        return h('div', { class: 'flex items-center gap-1.5' }, [
          h(WifiOff, { class: 'size-3 shrink-0 text-muted-foreground/40' }),
          h('span', { class: 'text-xs text-muted-foreground/40' }, '未配置'),
        ])
      }

      // 状态颜色和图标
      const statusColor = status === 'Ready' ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 ring-1 ring-emerald-500/20'
        : status === 'Pending' ? 'bg-amber-500/10 text-amber-600 dark:text-amber-400 ring-1 ring-amber-500/20'
        : status === 'Error' || status === 'Failed' ? 'bg-rose-500/10 text-rose-600 dark:text-rose-400 ring-1 ring-rose-500/20'
        : 'bg-zinc-500/10 text-zinc-600 dark:text-zinc-400 ring-1 ring-zinc-500/20'

      const statusText = status ? (networkStatusLabel[status] || status) : '初始化中'
      const statusIcon = status === 'Ready' ? Wifi : status === 'Pending' ? Network : WifiOff

      return h('div', { class: 'flex flex-col gap-1.5' }, [
        // 网络名称
        ws.networkName ? h('div', { class: 'flex items-center gap-1.5' }, [
          h(statusIcon, { class: 'size-3 shrink-0 text-muted-foreground' }),
          h('span', { class: 'text-xs font-medium leading-none' }, ws.networkName),
        ]) : h('div', { class: 'flex items-center gap-1.5' }, [
          h(Network, { class: 'size-3 shrink-0 text-muted-foreground/40' }),
          h('span', { class: 'text-xs text-muted-foreground/40 leading-none' }, '网络配置中'),
        ]),
        // CIDR 网段
        ws.networkCIDR ? h('span', { class: 'font-mono text-[11px] text-muted-foreground/70 leading-none' }, ws.networkCIDR) : null,
        // 状态徽章
        h('div', { class: 'flex items-center gap-1 mt-0.5' }, [
          h('span', { class: `text-[10px] font-medium px-1.5 py-0.5 rounded-full leading-none ${statusColor}` }, statusText),
        ]),
      ])
    },
  },
  {
    accessorKey: 'createdAt',
    header: '创建时间',
    cell: ({ row }) => {
      const timeStr = formatCreatedAt(row.original.createdAt)
      if (timeStr === '—') return h('span', { class: 'text-xs text-muted-foreground/40' }, '—')
      return h('span', {
        class: 'text-xs text-muted-foreground tabular-nums',
        title: row.original.createdAt
      }, timeStr)
    },
  },
  {
    id: 'actions',
    header: '',
    cell: ({ row }) => {
      const ws = row.original
      return h(DropdownMenu, {}, {
        default: () => [
          h(DropdownMenuTrigger, { asChild: true }, () =>
            h(Button, { variant: 'ghost', size: 'sm', class: 'size-8 p-0' }, () => h(MoreHorizontal, { class: 'size-4' }))
          ),
          h(DropdownMenuContent, { align: 'end', class: 'w-36' }, () => [
            h(DropdownMenuItem, { onClick: () => store.switchWorkspace(ws) }, () => [h(ArrowRight, { class: 'mr-2 size-3.5' }), '进入空间']),
            h(DropdownMenuItem, { onClick: () => openEdit(ws) }, () => [h(Pencil, { class: 'mr-2 size-3.5' }), '编辑']),
            h(DropdownMenuSeparator),
            h(DropdownMenuItem, {
              class: 'text-destructive focus:text-destructive',
              onClick: () => promptDelete(ws),
            }, () => [h(Trash2, { class: 'mr-2 size-3.5' }), '删除']),
          ]),
        ],
      })
    },
  },
]

// ── TanStack Table（仅渲染，分页/过滤交由后端）────────────────────
const table = useVueTable({
  get data() { return store.rows },
  columns,
  getCoreRowModel: getCoreRowModel(),
  manualPagination: true,
  manualFiltering: true,
  get rowCount() { return store.total },
})

// ── 后端分页 ─────────────────────────────────────────────────────
const currentPage  = computed(() => store.page)
const totalPages   = computed(() => Math.max(1, Math.ceil(store.total / store.pageSize)))
const visiblePages = computed(() => {
  const cur   = currentPage.value
  const total = totalPages.value
  const start = Math.max(1, Math.min(cur - 1, total - 2))
  const end   = Math.min(total, start + 2)
  return Array.from({ length: end - start + 1 }, (_, i) => start + i)
})

function goToPage(p: number) {
  if (p < 1 || p > totalPages.value) return
  store.fetchList({ page: p, search: searchValue.value, status: statusFilter.value === 'all' ? undefined : statusFilter.value })
}

// ── 搜索 & 状态过滤（触发后端重新拉取）───────────────────────────
const searchValue  = ref('')
const statusFilter = ref<'all' | 'active' | 'inactive'>('all')

let searchTimer: ReturnType<typeof setTimeout>
function onSearchInput() {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => goToPage(1), 400)
}

function setStatusFilter(val: typeof statusFilter.value) {
  statusFilter.value = val
  goToPage(1)
}

// ── Refresh ──────────────────────────────────────────────────────
const isRefreshing = ref(false)
function handleRefresh() {
  isRefreshing.value = true
  store.fetchList({ page: currentPage.value }).finally(() => (isRefreshing.value = false))
}
</script>

<template>
  <div class="flex flex-col gap-5 p-6 animate-in fade-in duration-300">

    <!-- ── Stat cards ─────────────────────────────────────────────── -->
    <div class="grid grid-cols-2 sm:grid-cols-4 gap-4">

      <!-- 全部空间 -->
      <button
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left hover:shadow-md transition-shadow"
        :class="statusFilter === 'all' ? 'ring-2 ring-primary/20 border-primary/30' : ''"
        @click="setStatusFilter('all')"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">全部空间</span>
            <span class="text-2xl font-bold tracking-tight tabular-nums">{{ stats.total }}</span>
          </div>
          <div class="bg-muted rounded-lg p-2">
            <Layers class="size-4 text-muted-foreground" />
          </div>
        </div>

        <!-- Workspace initials stack -->
        <div class="flex items-center gap-2 mt-3">
          <div class="flex -space-x-2">
            <div
              v-for="(ws, i) in stats.initials" :key="i"
              class="size-6 rounded-lg ring-2 ring-card flex items-center justify-center text-[9px] font-black shrink-0 bg-primary/10 text-primary"
              :class="ws.color"
            >{{ ws.label }}</div>
          </div>
          <span v-if="stats.total > 3" class="text-[10px] text-muted-foreground/60">+{{ stats.total - 3 }}</span>
        </div>

        <!-- Active / inactive bar -->
        <div class="mt-3 space-y-1.5">
          <div class="flex h-1.5 rounded-full overflow-hidden bg-muted/50 gap-px">
            <div class="bg-emerald-500 transition-all" :style="{ width: `${stats.activeRate}%` }" />
            <div class="bg-zinc-300 dark:bg-zinc-600 transition-all" :style="{ width: `${100 - stats.activeRate}%` }" />
          </div>
          <div class="flex items-center justify-between text-[10px] text-muted-foreground/60">
            <span class="flex items-center gap-1"><span class="size-1.5 rounded-full bg-emerald-500 inline-block" />{{ stats.active }} 运行中</span>
            <span class="flex items-center gap-1"><span class="size-1.5 rounded-full bg-zinc-400 inline-block" />{{ stats.inactive }} 已停用</span>
          </div>
        </div>
      </button>

      <!-- 运行中 -->
      <button
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left hover:shadow-md transition-shadow"
        :class="statusFilter === 'active' ? 'ring-2 ring-emerald-500/20 border-emerald-500/30' : ''"
        @click="setStatusFilter('active')"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">运行中</span>
            <span class="text-2xl font-bold tracking-tight tabular-nums">{{ stats.active }}</span>
          </div>
          <div class="bg-muted rounded-lg p-2">
            <Wifi class="size-4 text-muted-foreground" />
          </div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-sm">
          <ArrowUpRight class="text-emerald-600 size-4 shrink-0" />
          <span class="text-emerald-600 font-semibold">{{ stats.activeRate }}%</span>
          <span class="text-muted-foreground">健康率</span>
        </div>
      </button>

      <!-- 已停用 -->
      <button
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left hover:shadow-md transition-shadow"
        :class="statusFilter === 'inactive' ? 'ring-2 ring-zinc-400/20 border-zinc-400/30' : ''"
        @click="setStatusFilter('inactive')"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">已停用</span>
            <span class="text-2xl font-bold tracking-tight tabular-nums">{{ stats.inactive }}</span>
          </div>
          <div class="bg-muted rounded-lg p-2">
            <WifiOff class="size-4 text-muted-foreground" />
          </div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-sm">
          <component
            :is="stats.inactive === 0 ? ArrowUpRight : ArrowDownRight"
            :class="stats.inactive === 0 ? 'text-emerald-600' : 'text-rose-500'"
            class="size-4 shrink-0"
          />
          <span :class="stats.inactive === 0 ? 'text-emerald-600 font-semibold' : 'text-rose-500 font-semibold'">
            {{ stats.inactive === 0 ? '全部运行中' : stats.inactive + ' 个已停用' }}
          </span>
          <span class="text-muted-foreground">{{ stats.inactive === 0 ? '状态健康' : '需要检查' }}</span>
        </div>
      </button>

      <!-- 在线节点 -->
      <div class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left">
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">在线节点</span>
            <span class="text-2xl font-bold tracking-tight tabular-nums">{{ stats.totalNodes }}</span>
          </div>
          <div class="bg-muted rounded-lg p-2">
            <Server class="size-4 text-muted-foreground" />
          </div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-sm">
          <Network class="text-muted-foreground size-4 shrink-0" />
          <span class="text-muted-foreground">均 <span class="font-semibold text-foreground">{{ stats.avgNodes }}</span> 节点/空间</span>
        </div>
      </div>

    </div>

    <!-- ── Toolbar ────────────────────────────────────────────────── -->
    <div class="flex items-center gap-2">
      <div class="relative w-64">
        <Search class="absolute left-2.5 top-1/2 -translate-y-1/2 size-4 text-muted-foreground" />
        <Input v-model="searchValue" placeholder="搜索名称或 Slug..." class="pl-8 h-9" @input="onSearchInput" />
      </div>
      <div class="ml-auto flex items-center gap-2">
        <Button variant="outline" size="sm" class="gap-1.5"
          :disabled="isRefreshing" @click="handleRefresh">
          <RefreshCw class="size-3.5" :class="isRefreshing ? 'animate-spin' : ''" />
          刷新
        </Button>
        <Button size="sm" class="gap-1.5" @click="openCreate">
          <Plus class="size-3.5" /> 创建空间
        </Button>
      </div>
    </div>

    <!-- ── Data Table ─────────────────────────────────────────────── -->
    <div class="rounded-md border">
      <Table>
        <TableHeader>
          <TableRow v-for="headerGroup in table.getHeaderGroups()" :key="headerGroup.id">
            <TableHead v-for="header in headerGroup.headers" :key="header.id" class="text-left align-middle">
              <div class="flex w-full items-center justify-start text-left">
                <FlexRender
                  v-if="!header.isPlaceholder"
                  :render="header.column.columnDef.header"
                  :props="header.getContext()"
                />
              </div>
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <template v-if="table.getRowModel().rows.length">
            <TableRow
              v-for="row in table.getRowModel().rows"
              :key="row.id"
              :data-state="row.getIsSelected() ? 'selected' : undefined"
            >
              <TableCell v-for="cell in row.getVisibleCells()" :key="cell.id" class="text-left align-middle">
                <div class="flex w-full items-center justify-start text-left">
                  <FlexRender :render="cell.column.columnDef.cell" :props="cell.getContext()" />
                </div>
              </TableCell>
            </TableRow>
          </template>
          <TableRow v-else>
            <TableCell :colspan="columns.length" class="h-32 text-center text-muted-foreground">
              暂无工作空间
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
    <!-- ── Delete confirm ────────────────────────────────────────── -->
    <AppAlertDialog
      v-model:open="deleteDialogOpen"
      title="删除工作空间"
      :description="`确认删除「${deleteTarget?.displayName}」？该操作不可撤销，空间内所有节点和策略将被移除。`"
      confirm-text="删除"
      variant="destructive"
      @confirm="confirmDelete"
    />

    <!-- ── Edit / Create dialog ───────────────────────────────────── -->
    <Dialog v-model:open="dialogOpen">
      <DialogContent class="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{{ editingWorkspace ? '编辑空间' : '创建空间' }}</DialogTitle>
          <DialogDescription>
            {{ editingWorkspace ? '修改工作空间配置' : '新建一个隔离的网络工作空间' }}
          </DialogDescription>
        </DialogHeader>
        <div class="space-y-4 py-2">
          <div class="space-y-1.5">
            <label class="text-sm font-medium">显示名称</label>
            <Input
              v-model="form.displayName"
              placeholder="例如：生产环境"
              @input="!editingWorkspace && slugify(form.displayName)"
            />
          </div>
          <div class="space-y-1.5">
            <label class="text-sm font-medium">
              Slug
              <span class="text-muted-foreground font-normal text-xs ml-1">(唯一标识符)</span>
            </label>
            <Input v-model="form.slug" placeholder="例如：production" class="font-mono" />
          </div>
          <div class="space-y-1.5">
            <label class="text-sm font-medium">最大节点数（限额）</label>
            <Input v-model.number="form.maxNodeCount" type="number" min="1" max="1000" />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" @click="dialogOpen = false">取消</Button>
          <Button :disabled="store.saving || !form.displayName || !form.slug" @click="saveWorkspace">
            {{ store.saving ? '保存中...' : editingWorkspace ? '保存' : '创建' }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

  </div>
</template>
