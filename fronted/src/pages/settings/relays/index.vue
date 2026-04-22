<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import {
  useVueTable, getCoreRowModel, FlexRender, type ColumnDef,
} from '@tanstack/vue-table'
import {
  Search, RefreshCw, MoreHorizontal, Trash2, Pencil,
  ChevronLeft, ChevronRight, Plus, Wifi, WifiOff,
  Radio, Globe, Zap, Server, ActivitySquare, CheckCircle2,
  AlertCircle,
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
import AppAlertDialog from '@/components/AlertDialog.vue'
import { toast } from 'vue-sonner'
import {
  listRelays, createRelay, updateRelay, deleteRelay, testRelay,
  type RelayServer, type CreateRelayParams,
} from '@/api/relay'

definePage({
  meta: { title: '中继服务器', description: '管理 WRRP 中继服务器，并将其推送至节点配置。' },
})

// ── state ───────────────────────────────────────────────────────────────────
const rows = ref<RelayServer[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(10)
const loading = ref(false)
const saving = ref(false)
const deleting = ref(false)
const testing = ref<string | null>(null)

const dialogOpen = ref(false)
const deleteDialogOpen = ref(false)
const editingItem = ref<RelayServer | null>(null)
const deleteTarget = ref<RelayServer | null>(null)

const searchValue = ref('')
let searchTimer: ReturnType<typeof setTimeout>
const statusFilter = ref<'all' | 'healthy' | 'degraded' | 'offline'>('all')

// ── form ─────────────────────────────────────────────────────────────────────
const form = ref<CreateRelayParams>({
  name: '',
  description: '',
  tcpUrl: '',
  quicUrl: '',
  enabled: true,
  workspaces: [],
})

function resetForm() {
  form.value = { name: '', description: '', tcpUrl: '', quicUrl: '', enabled: true, workspaces: [] }
}

function openCreate() {
  editingItem.value = null
  resetForm()
  dialogOpen.value = true
}

function openEdit(row: RelayServer) {
  editingItem.value = row
  form.value = {
    name: row.name,
    description: row.description ?? '',
    tcpUrl: row.tcpUrl,
    quicUrl: row.quicUrl ?? '',
    enabled: row.enabled,
    workspaces: row.workspaces ?? [],
  }
  dialogOpen.value = true
}

// ── API ───────────────────────────────────────────────────────────────────────
async function fetchList(params?: { page?: number }) {
  loading.value = true
  if (params?.page) page.value = params.page
  try {
    const { data, code } = await listRelays({
      page: page.value,
      pageSize: pageSize.value,
      keyword: searchValue.value || undefined,
    }) as any
    if (code === 200) {
      rows.value = Array.isArray(data) ? data : (data?.list ?? data?.items ?? [])
      total.value = Array.isArray(data) ? rows.value.length : (data?.total ?? rows.value.length)
    }
  } catch {
    toast.error('获取中继服务器列表失败')
  } finally {
    loading.value = false
  }
}

onMounted(() => fetchList())

async function handleSave() {
  if (!form.value.name.trim()) { toast.error('请填写名称'); return }
  if (!form.value.tcpUrl.trim()) { toast.error('请填写 TCP 地址'); return }
  saving.value = true
  try {
    if (editingItem.value) {
      const { code } = await updateRelay(editingItem.value.id, form.value) as any
      if (code === 200) {
        toast.success('中继服务器已更新')
        dialogOpen.value = false
        await fetchList()
      }
    } else {
      const { code } = await createRelay(form.value) as any
      if (code === 200) {
        toast.success('中继服务器已创建')
        dialogOpen.value = false
        await fetchList({ page: 1 })
      }
    }
  } catch {
    toast.error(editingItem.value ? '更新失败' : '创建失败')
  } finally {
    saving.value = false
  }
}

function promptDelete(row: RelayServer) {
  deleteTarget.value = row
  deleteDialogOpen.value = true
}

async function confirmDelete() {
  if (!deleteTarget.value) return
  deleting.value = true
  try {
    const { code } = await deleteRelay(deleteTarget.value.id) as any
    if (code === 200) {
      toast.success('中继服务器已删除')
      deleteDialogOpen.value = false
      deleteTarget.value = null
      await fetchList()
    }
  } catch {
    toast.error('删除失败')
  } finally {
    deleting.value = false
  }
}

async function handleTest(row: RelayServer) {
  testing.value = row.id
  try {
    const { code, data } = await testRelay(row.id) as any
    if (code === 200) {
      toast.success(`连通性正常，延迟 ${data?.latencyMs ?? '—'} ms`)
      await fetchList()
    } else {
      toast.error('连通性测试失败')
    }
  } catch {
    toast.error('连通性测试失败')
  } finally {
    testing.value = null
  }
}

// ── computed ──────────────────────────────────────────────────────────────────
const filteredRows = computed(() => {
  const q = searchValue.value.toLowerCase().trim()
  return rows.value.filter(r => {
    const matchSearch = !q
      || r.name?.toLowerCase().includes(q)
      || r.tcpUrl?.toLowerCase().includes(q)
      || r.quicUrl?.toLowerCase().includes(q)
    const matchStatus = statusFilter.value === 'all' || r.status === statusFilter.value
    return matchSearch && matchStatus
  })
})

const stats = computed(() => {
  const all = rows.value
  return {
    total: all.length,
    healthy: all.filter(r => r.status === 'healthy').length,
    degraded: all.filter(r => r.status === 'degraded').length,
    offline: all.filter(r => !r.enabled || r.status === 'offline').length,
    quicEnabled: all.filter(r => !!r.quicUrl).length,
  }
})

function setStatusFilter(val: typeof statusFilter.value) {
  statusFilter.value = val
  searchValue.value = ''
}

function onSearchInput() {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => { statusFilter.value = 'all' }, 300)
}

// ── helpers ───────────────────────────────────────────────────────────────────
function statusBadge(row: RelayServer) {
  if (!row.enabled) {
    return { label: '已禁用', cls: 'bg-zinc-500/10 text-zinc-500 ring-zinc-500/20' }
  }
  switch (row.status) {
    case 'healthy':  return { label: '正常',   cls: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 ring-emerald-500/20' }
    case 'degraded': return { label: '降级',   cls: 'bg-amber-500/10  text-amber-600  dark:text-amber-400  ring-amber-500/20' }
    case 'offline':  return { label: '离线',   cls: 'bg-rose-500/10   text-rose-600   dark:text-rose-400   ring-rose-500/20' }
    default:         return { label: '未知',   cls: 'bg-muted text-muted-foreground ring-border' }
  }
}

// function statusIcon(row: RelayServer) {
//   if (!row.enabled) return h(WifiOff,       { class: 'size-3.5 text-zinc-400' })
//   switch (row.status) {
//     case 'healthy':  return h(CheckCircle2, { class: 'size-3.5 text-emerald-500' })
//     case 'degraded': return h(AlertCircle,  { class: 'size-3.5 text-amber-500' })
//     case 'offline':  return h(WifiOff,      { class: 'size-3.5 text-rose-500' })
//     default:         return h(CircleDashed, { class: 'size-3.5 text-muted-foreground' })
//   }
// }

// ── table columns ─────────────────────────────────────────────────────────────
const columns: ColumnDef<RelayServer>[] = [
  {
    id: 'status',
    header: '状态',
    cell: ({ row }) => {
      const b = statusBadge(row.original)
      return h('span', {
        class: `text-xs font-medium px-2 py-0.5 rounded-full ring-1 ${b.cls}`,
      }, b.label)
    },
  },
  {
    id: 'name',
    header: '名称',
    cell: ({ row }) => {
      const relay = row.original
      return h('div', { class: 'flex items-center gap-3' }, [
        h('div', {
          class: 'size-9 rounded-lg flex items-center justify-center shrink-0 bg-primary/10 ring-1 ring-primary/20',
        }, h(Server, { class: 'size-4 text-primary' })),
        h('div', { class: 'min-w-0' }, [
          h('p', { class: 'font-semibold text-sm leading-none' }, relay.name),
          relay.description
            ? h('p', { class: 'text-[11px] text-muted-foreground mt-1 truncate max-w-[200px]' }, relay.description)
            : null,
        ]),
      ])
    },
  },
  {
    id: 'tcp',
    header: 'TCP 地址',
    cell: ({ row }) => {
      const url = row.original.tcpUrl
      return h('div', { class: 'flex items-center gap-1.5' }, [
        h(Globe, { class: 'size-3.5 text-muted-foreground shrink-0' }),
        h('span', { class: 'font-mono text-xs' }, url || '—'),
      ])
    },
  },
  {
    id: 'quic',
    header: 'QUIC 地址',
    cell: ({ row }) => {
      const url = row.original.quicUrl
      if (!url) return h('span', { class: 'text-[11px] text-muted-foreground/40' }, '—')
      return h('div', { class: 'flex items-center gap-1.5' }, [
        h(Zap, { class: 'size-3.5 text-amber-500 shrink-0' }),
        h('span', { class: 'font-mono text-xs' }, url),
      ])
    },
  },
  {
    id: 'latency',
    header: '延迟',
    cell: ({ row }) => {
      const ms = row.original.latencyMs
      if (ms == null) return h('span', { class: 'text-[11px] text-muted-foreground/40' }, '—')
      const cls = ms < 50 ? 'text-emerald-500' : ms < 150 ? 'text-amber-500' : 'text-rose-500'
      return h('span', { class: `text-xs font-medium tabular-nums ${cls}` }, `${ms} ms`)
    },
  },
  {
    id: 'peers',
    header: '已连接节点',
    cell: ({ row }) => {
      const n = row.original.connectedPeers ?? 0
      return h('div', { class: 'flex items-center gap-1.5' }, [
        h(Radio, { class: 'size-3.5 text-muted-foreground shrink-0' }),
        h('span', { class: 'text-sm tabular-nums' }, String(n)),
      ])
    },
  },
  {
    id: 'workspaces',
    header: '适用空间',
    cell: ({ row }) => {
      const ws = row.original.workspaces ?? []
      if (ws.length === 0) {
        return h('span', { class: 'text-xs text-muted-foreground/50' }, '全部')
      }
      return h('div', { class: 'flex flex-wrap gap-1' }, ws.slice(0, 3).map(w =>
        h('span', {
          class: 'text-[10px] font-medium px-1.5 py-0.5 rounded-md bg-muted text-muted-foreground',
        }, w)
      ).concat(ws.length > 3
        ? [h('span', { class: 'text-[10px] text-muted-foreground/50' }, `+${ws.length - 3}`)]
        : []
      ))
    },
  },
  {
    id: 'actions',
    header: '',
    cell: ({ row }) => {
      const relay = row.original
      return h(DropdownMenu, {}, {
        default: () => [
          h(DropdownMenuTrigger, { asChild: true }, () =>
            h(Button, { variant: 'ghost', size: 'sm', class: 'size-8 p-0' }, () =>
              h(MoreHorizontal, { class: 'size-4' })
            )
          ),
          h(DropdownMenuContent, { align: 'end', class: 'w-36' }, () => [
            h(DropdownMenuItem, {
              onClick: () => handleTest(relay),
              disabled: testing.value === relay.id,
            }, () => [
              h(ActivitySquare, { class: 'mr-2 size-3.5' }),
              testing.value === relay.id ? '测试中...' : '连通测试',
            ]),
            h(DropdownMenuItem, { onClick: () => openEdit(relay) }, () => [
              h(Pencil, { class: 'mr-2 size-3.5' }), '编辑',
            ]),
            h(DropdownMenuSeparator),
            h(DropdownMenuItem, {
              class: 'text-destructive focus:text-destructive',
              onClick: () => promptDelete(relay),
            }, () => [h(Trash2, { class: 'mr-2 size-3.5' }), '删除']),
          ]),
        ],
      })
    },
  },
]

const table = useVueTable({
  get data() { return filteredRows.value },
  columns,
  getCoreRowModel: getCoreRowModel(),
  manualPagination: true,
  manualFiltering: true,
})

const currentPage  = computed(() => page.value)
const totalPages   = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))
const visiblePages = computed(() => {
  const cur = currentPage.value, tp = totalPages.value
  const start = Math.max(1, Math.min(cur - 1, tp - 2))
  const end   = Math.min(tp, start + 2)
  return Array.from({ length: end - start + 1 }, (_, i) => start + i)
})

function goToPage(p: number) {
  if (p < 1 || p > totalPages.value) return
  fetchList({ page: p })
}
</script>

<template>
  <div class="flex flex-col gap-5 p-6 animate-in fade-in duration-300">

    <!-- stats cards -->
    <div class="grid grid-cols-2 sm:grid-cols-4 gap-4">
      <button
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left hover:shadow-md transition-shadow"
        :class="statusFilter === 'all' ? 'ring-2 ring-primary/20 border-primary/30' : ''"
        @click="setStatusFilter('all')"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">全部中继</span>
            <span class="text-2xl font-bold tracking-tight">{{ stats.total }}</span>
          </div>
          <div class="bg-muted rounded-lg p-2"><Server class="text-muted-foreground size-4" /></div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-sm">
          <Zap class="size-4 text-amber-500 shrink-0" />
          <span class="text-muted-foreground">
            其中 <span class="font-semibold text-foreground">{{ stats.quicEnabled }}</span> 个支持 QUIC
          </span>
        </div>
      </button>

      <button
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left hover:shadow-md transition-shadow"
        :class="statusFilter === 'healthy' ? 'ring-2 ring-emerald-500/20 border-emerald-500/30' : ''"
        @click="setStatusFilter('healthy')"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">运行正常</span>
            <span class="text-2xl font-bold tracking-tight">{{ stats.healthy }}</span>
          </div>
          <div class="bg-muted rounded-lg p-2"><Wifi class="text-muted-foreground size-4" /></div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-sm">
          <CheckCircle2 class="text-emerald-600 size-4 shrink-0" />
          <span class="text-muted-foreground">连通性正常</span>
        </div>
      </button>

      <button
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left hover:shadow-md transition-shadow"
        :class="statusFilter === 'degraded' ? 'ring-2 ring-amber-500/20 border-amber-500/30' : ''"
        @click="setStatusFilter('degraded')"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">性能降级</span>
            <span class="text-2xl font-bold tracking-tight">{{ stats.degraded }}</span>
          </div>
          <div class="bg-muted rounded-lg p-2"><AlertCircle class="text-muted-foreground size-4" /></div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-sm">
          <AlertCircle class="text-amber-500 size-4 shrink-0" />
          <span class="text-muted-foreground">延迟偏高或丢包</span>
        </div>
      </button>

      <button
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left hover:shadow-md transition-shadow"
        :class="statusFilter === 'offline' ? 'ring-2 ring-rose-500/20 border-rose-500/30' : ''"
        @click="setStatusFilter('offline')"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">离线/禁用</span>
            <span class="text-2xl font-bold tracking-tight">{{ stats.offline }}</span>
          </div>
          <div class="bg-muted rounded-lg p-2"><WifiOff class="text-muted-foreground size-4" /></div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-sm">
          <WifiOff class="text-rose-500 size-4 shrink-0" />
          <span class="text-muted-foreground">不可用或已禁用</span>
        </div>
      </button>
    </div>

    <!-- toolbar -->
    <div class="flex items-center gap-2">
      <div class="relative w-72">
        <Search class="absolute left-2.5 top-1/2 -translate-y-1/2 size-4 text-muted-foreground" />
        <Input
          v-model="searchValue"
          placeholder="搜索名称或地址..."
          class="pl-8 h-9"
          @input="onSearchInput"
        />
      </div>
      <div class="ml-auto flex items-center gap-2">
        <Button variant="outline" size="sm" class="gap-1.5" :disabled="loading" @click="fetchList()">
          <RefreshCw class="size-3.5" :class="loading ? 'animate-spin' : ''" />
          刷新
        </Button>
        <Button size="sm" class="gap-1.5" @click="openCreate">
          <Plus class="size-3.5" /> 添加中继
        </Button>
      </div>
    </div>

    <!-- table -->
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
            <TableRow v-for="row in table.getRowModel().rows" :key="row.id">
              <TableCell v-for="cell in row.getVisibleCells()" :key="cell.id">
                <FlexRender :render="cell.column.columnDef.cell" :props="cell.getContext()" />
              </TableCell>
            </TableRow>
          </template>
          <TableRow v-else>
            <TableCell :colspan="columns.length" class="h-32 text-center text-muted-foreground">
              {{ loading ? '加载中...' : '暂无中继服务器，点击「添加中继」创建一个' }}
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>

    <!-- pagination -->
    <div class="flex items-center justify-between text-sm text-muted-foreground">
      <span>共 {{ total }} 条 · 第 {{ currentPage }} / {{ totalPages }} 页</span>
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

    <!-- delete confirmation -->
    <AppAlertDialog
      v-model:open="deleteDialogOpen"
      title="删除中继服务器"
      :description="`确认删除「${deleteTarget?.name ?? ''}」？关联该中继的节点将在下次同步时切换至其他中继或直连。`"
      confirm-text="删除"
      variant="destructive"
      @confirm="confirmDelete"
    />

    <!-- create / edit dialog -->
    <Dialog v-model:open="dialogOpen">
      <DialogContent class="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{{ editingItem ? '编辑中继服务器' : '添加中继服务器' }}</DialogTitle>
          <DialogDescription>
            中继服务器地址保存后将同步到对应工作空间下的 WireflowPeer，节点启动时自动配置。
          </DialogDescription>
        </DialogHeader>

        <div class="space-y-4 py-1">
          <!-- name -->
          <div class="space-y-1.5">
            <label class="text-sm font-medium">名称 <span class="text-destructive">*</span></label>
            <Input v-model="form.name" placeholder="例：Asia-HK-01" />
          </div>

          <!-- description -->
          <div class="space-y-1.5">
            <label class="text-sm font-medium">备注说明</label>
            <Input v-model="form.description" placeholder="可选，帮助区分不同中继节点" />
          </div>

          <!-- tcp url -->
          <div class="space-y-1.5">
            <label class="text-sm font-medium flex items-center gap-1.5">
              <Globe class="size-3.5 text-muted-foreground" />
              TCP 地址 <span class="text-destructive">*</span>
            </label>
            <Input
              v-model="form.tcpUrl"
              placeholder="relay.example.com:6266"
              class="font-mono text-sm"
            />
            <p class="text-[11px] text-muted-foreground">
              对应节点配置中的 <code class="bg-muted px-1 rounded">--wrrp-url</code> 参数（WrrpUrl 字段）
            </p>
          </div>

          <!-- quic url -->
          <div class="space-y-1.5">
            <label class="text-sm font-medium flex items-center gap-1.5">
              <Zap class="size-3.5 text-amber-500" />
              QUIC 地址
              <span class="text-[10px] font-normal text-muted-foreground ml-1">（可选，推荐）</span>
            </label>
            <Input
              v-model="form.quicUrl"
              placeholder="relay.example.com:6267"
              class="font-mono text-sm"
            />
            <p class="text-[11px] text-muted-foreground">
              对应 <code class="bg-muted px-1 rounded">--wrrp-quic-url</code>，优先级高于 TCP，支持 0-RTT 和无 HoL 阻塞
            </p>
          </div>

          <!-- enabled -->
          <div class="flex items-center justify-between rounded-lg border px-4 py-3">
            <div>
              <p class="text-sm font-medium">启用该中继</p>
              <p class="text-[11px] text-muted-foreground mt-0.5">禁用后不会下发给任何节点</p>
            </div>
            <button
              type="button"
              class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              :class="form.enabled ? 'bg-primary' : 'bg-input'"
              @click="form.enabled = !form.enabled"
            >
              <span
                class="pointer-events-none inline-block size-5 rounded-full bg-background shadow-lg ring-0 transition-transform"
                :class="form.enabled ? 'translate-x-5' : 'translate-x-0'"
              />
            </button>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" @click="dialogOpen = false">取消</Button>
          <Button :disabled="saving" @click="handleSave">
            {{ saving ? '保存中...' : (editingItem ? '保存更改' : '创建') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

  </div>
</template>
