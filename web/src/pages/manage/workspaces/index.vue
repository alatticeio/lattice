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
  ChevronLeft, ChevronRight, Server, CheckCircle2,
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
const usagePct = (ws: Workspace) =>
  ws.maxNodeCount ? Math.round((ws.nodeCount / ws.maxNodeCount) * 100) : 0

// ── Stats ────────────────────────────────────────────────────────
const stats = computed(() => {
  const rows     = store.rows
  const active   = rows.filter(w => w.status === 'active').length
  const inactive = rows.filter(w => w.status === 'inactive').length
  const total    = rows.length
  const totalNodes = rows.reduce((s, w) => s + w.nodeCount, 0)
  const topWs = [...rows].sort((a, b) => b.nodeCount - a.nodeCount)[0]
  return {
    total, active, inactive, totalNodes,
    activeRate:  total ? Math.round((active / total) * 100) : 0,
    avgNodes:    total ? (totalNodes / total).toFixed(1) : '0',
    topWsName:   topWs?.displayName ?? '—',
    topWsNodes:  topWs?.nodeCount ?? 0,
    initials:    rows.slice(0, 3).map(w => ({ label: getWsInitials(w.displayName), color: getWsColor(w.displayName) })),
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
          class: `size-9 rounded-lg flex items-center justify-center shrink-0 text-xs font-bold ${getWsColor(ws.displayName)}`,
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
    header: '节点数',
    cell: ({ row }) => h('span', { class: 'font-semibold tabular-nums text-sm' }, String(row.original.nodeCount)),
  },
  {
    accessorKey: 'maxNodeCount',
    header: '限额',
    cell: ({ row }) => h('span', { class: 'tabular-nums text-sm text-muted-foreground' }, String(row.original.maxNodeCount)),
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
    accessorKey: 'createdAt',
    header: '创建时间',
    cell: ({ row }) => h('span', { class: 'text-xs text-muted-foreground tabular-nums' }, row.original.createdAt),
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
    <div class="grid grid-cols-2 sm:grid-cols-4 gap-3">

      <!-- 全部空间 -->
      <button
        class="relative bg-card border border-border rounded-xl p-4 text-left hover:border-primary/30 hover:shadow-sm transition-all group overflow-hidden"
        :class="statusFilter === 'all' ? 'border-primary/40 ring-1 ring-primary/10' : ''"
        @click="setStatusFilter('all')"
      >
        <div class="absolute -right-3 -top-3 size-16 rounded-full bg-primary/5 group-hover:bg-primary/8 transition-colors" />
        <div class="relative">
          <div class="flex items-center justify-between mb-3">
            <span class="text-xs font-semibold text-muted-foreground uppercase tracking-wider">全部空间</span>
            <div class="size-7 rounded-lg bg-primary/10 flex items-center justify-center">
              <Layers class="size-3.5 text-primary" />
            </div>
          </div>
          <p class="text-3xl font-black tracking-tighter tabular-nums">{{ stats.total }}</p>

          <!-- Workspace initials stack -->
          <div class="flex items-center gap-2 mt-3">
            <div class="flex -space-x-2">
              <div
                v-for="(ws, i) in stats.initials" :key="i"
                class="size-6 rounded-lg ring-2 ring-card flex items-center justify-center text-[9px] font-black shrink-0"
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
        </div>
      </button>

      <!-- 运行中 -->
      <button
        class="relative bg-card border border-border rounded-xl p-4 text-left hover:border-emerald-500/30 hover:shadow-sm transition-all group overflow-hidden"
        :class="statusFilter === 'active' ? 'border-emerald-500/40 ring-1 ring-emerald-500/10' : ''"
        @click="setStatusFilter('active')"
      >
        <div class="absolute -right-3 -top-3 size-16 rounded-full bg-emerald-500/5 group-hover:bg-emerald-500/10 transition-colors" />
        <div class="relative">
          <div class="flex items-center justify-between mb-3">
            <span class="text-xs font-semibold text-muted-foreground uppercase tracking-wider">运行中</span>
            <div class="size-7 rounded-lg bg-emerald-500/10 flex items-center justify-center">
              <span class="relative flex size-2">
                <span class="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-60" />
                <span class="relative inline-flex size-2 rounded-full bg-emerald-500" />
              </span>
            </div>
          </div>
          <p class="text-3xl font-black tracking-tighter tabular-nums text-emerald-500">{{ stats.active }}</p>
          <p class="text-[11px] text-muted-foreground/60 mt-1">
            健康率 <span class="font-bold text-emerald-500">{{ stats.activeRate }}%</span>
          </p>
          <div class="mt-3 space-y-1.5">
            <div class="flex h-1.5 rounded-full overflow-hidden bg-muted/50">
              <div class="bg-emerald-500 rounded-full transition-all duration-700" :style="{ width: `${stats.activeRate}%` }" />
            </div>
            <p class="text-[10px] text-muted-foreground/50">节点总量 <span class="font-semibold text-foreground">{{ stats.totalNodes }}</span> · 均 <span class="font-semibold text-foreground">{{ stats.avgNodes }}</span> 个/空间</p>
          </div>
        </div>
      </button>

      <!-- 已停用 -->
      <button
        class="relative bg-card border border-border rounded-xl p-4 text-left hover:border-zinc-400/30 hover:shadow-sm transition-all group overflow-hidden"
        :class="statusFilter === 'inactive' ? 'border-zinc-400/40 ring-1 ring-zinc-400/10' : ''"
        @click="setStatusFilter('inactive')"
      >
        <div class="absolute -right-3 -top-3 size-16 rounded-full bg-zinc-400/5 group-hover:bg-zinc-400/10 transition-colors" />
        <div class="relative">
          <div class="flex items-center justify-between mb-3">
            <span class="text-xs font-semibold text-muted-foreground uppercase tracking-wider">已停用</span>
            <div class="size-7 rounded-lg bg-muted flex items-center justify-center">
              <span class="size-2 rounded-full bg-zinc-400" />
            </div>
          </div>
          <p class="text-3xl font-black tracking-tighter tabular-nums text-muted-foreground">{{ stats.inactive }}</p>
          <div class="mt-3">
            <div v-if="stats.inactive === 0"
              class="inline-flex items-center gap-1.5 text-[10px] font-semibold px-2 py-1 rounded-lg bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 ring-1 ring-emerald-500/20">
              <CheckCircle2 class="size-3" /> 全部空间运行中
            </div>
            <p v-else class="text-[10px] text-muted-foreground/50 mt-1">
              占全部空间 <span class="font-semibold text-foreground">{{ stats.total ? Math.round((stats.inactive / stats.total) * 100) : 0 }}%</span>
            </p>
          </div>
        </div>
      </button>

      <!-- 在线节点 -->
      <div class="relative bg-card border border-border rounded-xl p-4 text-left overflow-hidden">
        <div class="absolute -right-3 -top-3 size-16 rounded-full bg-primary/5" />
        <div class="relative">
          <div class="flex items-center justify-between mb-3">
            <span class="text-xs font-semibold text-muted-foreground uppercase tracking-wider">在线节点</span>
            <div class="size-7 rounded-lg bg-primary/10 flex items-center justify-center">
              <Server class="size-3.5 text-primary" />
            </div>
          </div>
          <p class="text-3xl font-black tracking-tighter tabular-nums text-primary">{{ stats.totalNodes }}</p>
          <p class="text-[11px] text-muted-foreground/60 mt-1">
            均 <span class="font-bold text-foreground">{{ stats.avgNodes }}</span> 节点/空间
          </p>
          <div class="mt-3 pt-3 border-t border-border/60">
            <p class="text-[10px] text-muted-foreground/50">最多节点</p>
            <p class="text-[11px] font-semibold truncate mt-0.5">{{ stats.topWsName }}
              <span class="text-muted-foreground/50 font-normal">· {{ stats.topWsNodes }} 个</span>
            </p>
          </div>
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
            <TableHead v-for="header in headerGroup.headers" :key="header.id">
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
              :data-state="row.getIsSelected() ? 'selected' : undefined"
            >
              <TableCell v-for="cell in row.getVisibleCells()" :key="cell.id">
                <FlexRender :render="cell.column.columnDef.cell" :props="cell.getContext()" />
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
