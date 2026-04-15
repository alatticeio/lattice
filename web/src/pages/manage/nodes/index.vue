<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import {
  useVueTable, getCoreRowModel, FlexRender, type ColumnDef,
} from '@tanstack/vue-table'
import {
  Search, RefreshCw, MoreHorizontal, X, Tag,
  Server, Wifi, WifiOff, Clock, Network,
  KeyRound, ChevronRight, ChevronLeft, Trash2, Pencil,
  Globe, ArrowUpRight, ArrowDownRight, Copy, Check, Layers,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter,
} from '@/components/ui/dialog'
import {
  Card, CardContent,
} from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem,
  DropdownMenuSeparator, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import AppAlertDialog from '@/components/AlertDialog.vue'
import { usePeerPageStore } from '@/stores/peerPage'

definePage({
  meta: { title: 'Node 管理', description: '管理网络中的所有节点。' },
})

const store = usePeerPageStore()
onMounted(() => store.actions.refresh())

// ── Types ─────────────────────────────────────────────────────────
type PeerRow = (typeof store.rows)[number]
type NodeStatus = 'online' | 'offline' | 'pending'

// ── Style maps ────────────────────────────────────────────────────
const statusDot: Record<string, string> = {
  online: 'bg-emerald-500', offline: 'bg-rose-500', pending: 'bg-amber-400',
}
const statusBadge: Record<string, string> = {
  online:  'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 ring-1 ring-emerald-500/20',
  offline: 'bg-rose-500/10 text-rose-600 dark:text-rose-400 ring-1 ring-rose-500/20',
  pending: 'bg-amber-400/10 text-amber-600 dark:text-amber-400 ring-1 ring-amber-400/20',
}
const statusLabel: Record<string, string> = {
  online: '在线', offline: '离线', pending: '待接入',
}
const labelColors = [
  'bg-blue-500/10 text-blue-600 dark:text-blue-400 ring-1 ring-blue-500/20',
  'bg-violet-500/10 text-violet-600 dark:text-violet-400 ring-1 ring-violet-500/20',
  'bg-cyan-500/10 text-cyan-600 dark:text-cyan-400 ring-1 ring-cyan-500/20',
  'bg-orange-500/10 text-orange-600 dark:text-orange-400 ring-1 ring-orange-500/20',
]
function labelColor(label: string) {
  let h = 0
  for (const c of label) h = (h * 31 + c.charCodeAt(0)) & 0xff
  return labelColors[h % labelColors.length]
}

// Format an RFC3339 timestamp as a Chinese relative time string.
function formatLastSeen(isoStr: string | undefined | null): string {
  if (!isoStr) return '—'
  const diff = Date.now() - new Date(isoStr).getTime()
  if (diff < 60_000)        return '刚刚'
  if (diff < 3_600_000)     return `${Math.floor(diff / 60_000)} 分钟前`
  if (diff < 86_400_000)    return `${Math.floor(diff / 3_600_000)} 小时前`
  return `${Math.floor(diff / 86_400_000)} 天前`
}
const regionFlag: Record<string, string> = {
  'us-west-2': '🇺🇸', 'us-east-1': '🇺🇸',
  'eu-central-1': '🇩🇪', 'eu-west-1': '🇬🇧',
  'ap-southeast-1': '🇸🇬',
}

// helper: labels map → array of "k=v"
function labelsToArray(labels: any): string[] {
  if (!labels) return []
  if (Array.isArray(labels)) return labels
  return Object.entries(labels).map(([k, v]) => `${k}=${v}`)
}

// ── Stats ─────────────────────────────────────────────────────────
const statusFilter = ref<NodeStatus | 'all'>('all')

const stats = computed(() => {
  const rows    = store.rows as any[]
  const online  = rows.filter(n => n.status === 'online').length
  const offline = rows.filter(n => n.status === 'offline').length
  const pending = rows.filter(n => n.status === 'pending').length
  const total   = store.total || rows.length
  const regions = new Set(rows.map(n => n.region).filter(Boolean)).size
  return {
    total, online, offline, pending, regions,
    onlineRate: total ? Math.round((online / total) * 100) : 0,
  }
})

// ── Search / filter (client-side over loaded page) ─────────────────
const searchValue = ref('')

const filtered = computed(() => {
  const q = searchValue.value.toLowerCase().trim()
  return store.rows.filter((n: any) => {
    const matchSearch = !q
      || n.name?.toLowerCase().includes(q)
      || n.appId?.toLowerCase().includes(q)
      || n.address?.toLowerCase().includes(q)
      || n.region?.toLowerCase().includes(q)
    const matchStatus = statusFilter.value === 'all' || n.status === statusFilter.value
    return matchSearch && matchStatus
  })
})

function setStatusFilter(val: typeof statusFilter.value) {
  statusFilter.value = val
  searchValue.value = ''
}

let searchTimer: ReturnType<typeof setTimeout>
function onSearchInput() {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => { statusFilter.value = 'all' }, 400)
}

// ── Pagination (server-side) ───────────────────────────────────────
const PAGE_SIZE   = store.params.pageSize ?? 10
const currentPage = computed(() => store.params.page ?? 1)
const totalPages  = computed(() => Math.max(1, Math.ceil(store.total / PAGE_SIZE)))
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

// ── Delete confirm ─────────────────────────────────────────────────
const deleteTarget     = ref<PeerRow | null>(null)
const deleteDialogOpen = ref(false)

function promptDelete(node: PeerRow) {
  deleteTarget.value = node
  deleteDialogOpen.value = true
}
async function confirmDelete() {
  if (deleteTarget.value) {
    await store.actions.handleDelete(deleteTarget.value, () => Promise.resolve(true))
  }
  deleteTarget.value = null
}

// ── Column definitions ────────────────────────────────────────────
const columns: ColumnDef<PeerRow>[] = [
  {
    id: 'status',
    header: '状态',
    cell: ({ row }) => {
      const s: string = (row.original as any).status ?? 'pending'
      return h('div', { class: 'flex items-center gap-2' }, [
        h('span', { class: 'relative flex size-2 shrink-0' }, [
          s === 'online' && h('span', { class: `absolute inline-flex h-full w-full animate-ping rounded-full opacity-60 ${statusDot[s]}` }),
          h('span', { class: `relative inline-flex size-2 rounded-full ${statusDot[s] ?? 'bg-muted-foreground'}` }),
        ]),
        h('span', { class: `text-xs font-medium px-2 py-0.5 rounded-full ${statusBadge[s] ?? statusBadge.pending}` }, statusLabel[s] ?? s),
      ])
    },
  },
  {
    id: 'node',
    header: '节点',
    cell: ({ row }) => {
      const n = row.original as any
      return h('div', { class: 'flex flex-col gap-0.5' }, [
        h('span', { class: 'font-semibold text-sm leading-none' }, n.name ?? n.appId),
        h('span', { class: 'font-mono text-[11px] text-muted-foreground/60' }, n.appId),
      ])
    },
  },
  {
    id: 'region',
    header: '区域',
    cell: ({ row }) => {
      const region: string = (row.original as any).region ?? ''
      if (!region) return h('span', { class: 'text-[11px] text-muted-foreground/40' }, '—')
      return h('div', { class: 'flex items-center gap-1.5' }, [
        h('span', { class: 'text-base leading-none' }, regionFlag[region] ?? '🌐'),
        h('span', { class: 'text-xs text-muted-foreground' }, region),
      ])
    },
  },
  {
    id: 'workspace',
    header: '所属空间',
    cell: ({ row }) => {
      const n = row.original as any
      const workspaceName = n.workspaceDisplayName ?? n.namespace ?? ''
      if (!workspaceName) return h('span', { class: 'text-[11px] text-muted-foreground/40' }, '—')

      return h('div', { class: 'flex items-center gap-1.5' }, [
        h(Layers, { class: 'size-3 shrink-0 text-muted-foreground' }),
        h('span', { class: 'text-xs font-medium' }, workspaceName),
      ])
    },
  },
  {
    id: 'network',
    header: '网络 / 地址',
    cell: ({ row }) => {
      const n = row.original as any
      const network = n.network ?? n.namespace ?? '—'
      const address = n.address ?? ''
      return h('div', { class: 'flex flex-col gap-0.5' }, [
        h('span', { class: 'text-xs text-muted-foreground' }, network),
        address && h('span', { class: 'font-mono text-[11px] text-muted-foreground/60' }, address),
      ])
    },
  },
  {
    id: 'labels',
    header: '标签',
    cell: ({ row }) => {
      const labels = labelsToArray((row.original as any).labels)
      if (!labels.length) return h('span', { class: 'text-[11px] text-muted-foreground/40' }, '—')
      return h('div', { class: 'flex flex-wrap gap-1' },
        labels.map(label =>
          h('span', { class: `text-[11px] font-medium px-2 py-0.5 rounded-full ${labelColor(label)}` }, label)
        )
      )
    },
  },
  {
    id: 'lastSeen',
    header: '最后在线',
    cell: ({ row }) => {
      const n = row.original as any
      const text = formatLastSeen(n.lastSeen)
      if (text === '—') return h('span', { class: 'text-[11px] text-muted-foreground/40' }, '—')
      return h('span', {
        class: `text-xs ${n.status === 'offline' ? 'text-rose-500/70' : 'text-muted-foreground'}`,
        title: n.lastSeen ?? '',
      }, text)
    },
  },
  {
    id: 'actions',
    header: '',
    cell: ({ row }) => {
      const node = row.original
      return h(DropdownMenu, {}, {
        default: () => [
          h(DropdownMenuTrigger, { asChild: true }, () =>
            h(Button, { variant: 'ghost', size: 'sm', class: 'size-8 p-0' }, () =>
              h(MoreHorizontal, { class: 'size-4' })
            )
          ),
          h(DropdownMenuContent, { align: 'end', class: 'w-36' }, () => [
            h(DropdownMenuItem, { onClick: () => store.actions.openDrawer('view', node) }, () => [
              h(ChevronRight, { class: 'mr-2 size-3.5' }), '查看详情',
            ]),
            h(DropdownMenuItem, { onClick: () => store.actions.openDrawer('edit', node) }, () => [
              h(Pencil, { class: 'mr-2 size-3.5' }), '编辑标签',
            ]),
            h(DropdownMenuSeparator),
            h(DropdownMenuItem, {
              class: 'text-destructive focus:text-destructive',
              onClick: () => promptDelete(node),
            }, () => [h(Trash2, { class: 'mr-2 size-3.5' }), '删除节点']),
          ]),
        ],
      })
    },
  },
]

// ── Copy helper ──────────────────────────────────────────────────
const copiedKey = ref<string | null>(null)
async function copyText(text: string, key: string) {
  await navigator.clipboard.writeText(text)
  copiedKey.value = key
  setTimeout(() => { copiedKey.value = null }, 1500)
}

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
    <div class="grid grid-cols-2 sm:grid-cols-4 gap-4">

      <!-- 全部节点 -->
      <button
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left hover:shadow-md transition-shadow"
        :class="statusFilter === 'all' ? 'ring-2 ring-primary/20 border-primary/30' : ''"
        @click="setStatusFilter('all')"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">全部节点</span>
            <span class="text-2xl font-bold tracking-tight">{{ stats.total }}</span>
          </div>
          <div class="bg-muted rounded-lg p-2">
            <Server class="text-muted-foreground size-4" />
          </div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-sm">
          <Globe class="text-muted-foreground size-4 shrink-0" />
          <span class="text-muted-foreground">覆盖 <span class="font-semibold text-foreground">{{ stats.regions }}</span> 个地域</span>
        </div>
      </button>

      <!-- 在线 -->
      <button
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left hover:shadow-md transition-shadow"
        :class="statusFilter === 'online' ? 'ring-2 ring-emerald-500/20 border-emerald-500/30' : ''"
        @click="setStatusFilter('online')"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">在线节点</span>
            <span class="text-2xl font-bold tracking-tight">{{ stats.online }}</span>
          </div>
          <div class="bg-muted rounded-lg p-2">
            <Wifi class="text-muted-foreground size-4" />
          </div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-sm">
          <ArrowUpRight class="text-emerald-600 size-4 shrink-0" />
          <span class="text-emerald-600 font-semibold">{{ stats.onlineRate }}%</span>
          <span class="text-muted-foreground">在线率</span>
        </div>
      </button>

      <!-- 离线 -->
      <button
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left hover:shadow-md transition-shadow"
        :class="statusFilter === 'offline' ? 'ring-2 ring-red-500/20 border-red-500/30' : ''"
        @click="setStatusFilter('offline')"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">离线节点</span>
            <span class="text-2xl font-bold tracking-tight">{{ stats.offline }}</span>
          </div>
          <div class="bg-muted rounded-lg p-2">
            <WifiOff class="text-muted-foreground size-4" />
          </div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-sm">
          <component
            :is="stats.offline === 0 ? ArrowUpRight : ArrowDownRight"
            :class="stats.offline === 0 ? 'text-emerald-600' : 'text-red-500'"
            class="size-4 shrink-0"
          />
          <span :class="stats.offline === 0 ? 'text-emerald-600 font-semibold' : 'text-red-500 font-semibold'">
            {{ stats.offline === 0 ? '全部在线' : stats.offline + ' 台异常' }}
          </span>
          <span class="text-muted-foreground">{{ stats.offline === 0 ? '网络健康' : '需要检查' }}</span>
        </div>
      </button>

      <!-- 待接入 -->
      <button
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left hover:shadow-md transition-shadow"
        :class="statusFilter === 'pending' ? 'ring-2 ring-amber-400/20 border-amber-400/30' : ''"
        @click="setStatusFilter('pending')"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">待接入</span>
            <span class="text-2xl font-bold tracking-tight">{{ stats.pending }}</span>
          </div>
          <div class="bg-muted rounded-lg p-2">
            <Clock class="text-muted-foreground size-4" />
          </div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-sm">
          <component
            :is="stats.pending === 0 ? ArrowUpRight : ArrowDownRight"
            :class="stats.pending === 0 ? 'text-emerald-600' : 'text-amber-500'"
            class="size-4 shrink-0"
          />
          <span :class="stats.pending === 0 ? 'text-emerald-600 font-semibold' : 'text-amber-500 font-semibold'">
            {{ stats.pending === 0 ? '全部已接入' : stats.pending + ' 台待配置' }}
          </span>
          <span class="text-muted-foreground">{{ stats.pending === 0 ? '接入完成' : '等待配置' }}</span>
        </div>
      </button>

    </div>

    <!-- ── Toolbar ────────────────────────────────────────────────── -->
    <div class="flex items-center gap-2">
      <div class="relative w-72">
        <Search class="absolute left-2.5 top-1/2 -translate-y-1/2 size-4 text-muted-foreground" />
        <Input
          v-model="searchValue"
          placeholder="搜索名称、AppID、地址、区域..."
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
              @click="store.actions.openDrawer('view', row.original)"
            >
              <TableCell
                v-for="cell in row.getVisibleCells()" :key="cell.id"
                @click.stop="cell.column.id === 'actions' ? undefined : store.actions.openDrawer('view', row.original)"
              >
                <FlexRender :render="cell.column.columnDef.cell" :props="cell.getContext()" />
              </TableCell>
            </TableRow>
          </template>
          <TableRow v-else>
            <TableCell :colspan="columns.length" class="h-32 text-center text-muted-foreground">
              {{ store.loading ? '加载中...' : '暂无节点数据' }}
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
      title="删除节点"
      :description="`确认删除节点「${(deleteTarget as any)?.name ?? (deleteTarget as any)?.appId}」？该操作不可撤销。`"
      confirm-text="删除"
      variant="destructive"
      @confirm="confirmDelete"
      @cancel="deleteTarget = null"
    />

  </div>

  <!-- ── Node Detail / Edit Dialog ──────────────────────────────── -->
  <Dialog :open="store.isDrawerOpen" @update:open="v => { if (!v) store.isDrawerOpen = false }">
    <DialogContent class="sm:max-w-md p-0 gap-0 overflow-hidden">

      <!-- ── Header ─────────────────────────────────────────────── -->
      <DialogHeader class="px-6 pt-6 pb-5 border-b gap-0">
        <div class="flex items-start gap-3 pr-6">
          <!-- Status icon -->
          <div class="relative shrink-0 mt-0.5">
            <div
              class="size-10 rounded-lg border flex items-center justify-center"
              :class="store.selectedNode?.status === 'online'
                ? 'bg-emerald-500/10 border-emerald-500/20 text-emerald-500'
                : store.selectedNode?.status === 'offline'
                  ? 'bg-rose-500/10 border-rose-500/20 text-rose-500'
                  : 'bg-muted border-border text-muted-foreground'"
            >
              <Server class="size-4" />
            </div>
            <!-- live pulse dot -->
            <span
              class="absolute -bottom-1 -right-1 size-3 rounded-full border-2 border-background"
              :class="statusDot[store.selectedNode?.status ?? 'pending']"
            >
              <span
                v-if="store.selectedNode?.status === 'online'"
                class="absolute inset-0 rounded-full animate-ping opacity-75"
                :class="statusDot['online']"
              />
            </span>
          </div>

          <!-- Name + meta -->
          <div class="flex-1 min-w-0">
            <DialogTitle class="text-sm font-semibold leading-snug truncate">
              {{ store.selectedNode?.name ?? store.selectedNode?.appId }}
            </DialogTitle>
            <p v-if="store.selectedNode?.namespace" class="text-xs text-muted-foreground font-mono mt-0.5 truncate">
              {{ store.selectedNode.namespace }}
            </p>
            <div class="flex items-center gap-2 mt-2 flex-wrap">
              <span
                v-if="store.selectedNode?.status"
                class="inline-flex items-center text-[11px] font-medium px-2 py-0.5 rounded-md"
                :class="statusBadge[store.selectedNode.status] ?? statusBadge.pending"
              >
                {{ statusLabel[store.selectedNode.status] ?? store.selectedNode.status }}
              </span>
              <span
                v-if="store.selectedNode?.lastSeen"
                class="inline-flex items-center gap-1 text-[11px] text-muted-foreground"
                :title="store.selectedNode.lastSeen"
              >
                <Clock class="size-3" />
                <span :class="store.selectedNode.status === 'offline' ? 'text-destructive/70' : ''">
                  {{ store.selectedNode.status === 'online' ? '在线中' : formatLastSeen(store.selectedNode.lastSeen) }}
                </span>
              </span>
            </div>
          </div>
        </div>
      </DialogHeader>

      <!-- ── Body ─────────────────────────────────────────────────── -->
      <div v-if="store.selectedNode" class="px-6 py-5 space-y-4 max-h-[55vh] overflow-y-auto">

        <!-- IP highlight (Card component) -->
        <Card v-if="store.selectedNode.address" class="rounded-lg shadow-none py-0">
          <CardContent class="px-4 py-3 flex items-center justify-between gap-3">
            <div class="flex items-center gap-3 min-w-0">
              <div class="size-8 rounded-md bg-muted flex items-center justify-center shrink-0">
                <Network class="size-3.5 text-muted-foreground" />
              </div>
              <div class="min-w-0">
                <p class="text-[10px] text-muted-foreground leading-none mb-1">分配 IP</p>
                <p class="font-mono text-sm font-semibold leading-none truncate">
                  {{ store.selectedNode.address }}
                </p>
              </div>
            </div>
            <Button
              variant="ghost"
              size="icon"
              class="size-7 shrink-0 text-muted-foreground"
              :title="copiedKey === 'ip' ? '已复制' : '复制 IP'"
              @click="copyText(store.selectedNode.address!, 'ip')"
            >
              <Check v-if="copiedKey === 'ip'" class="size-3.5 text-emerald-500" />
              <Copy v-else class="size-3.5" />
            </Button>
          </CardContent>
        </Card>

        <Separator />

        <!-- Identity -->
        <div class="space-y-1">
          <p class="text-[11px] font-medium text-muted-foreground mb-2 flex items-center gap-1.5">
            <KeyRound class="size-3" /> 身份信息
          </p>

          <!-- Workspace -->
          <div v-if="store.selectedNode.namespace || store.selectedNode.workspaceDisplayName" class="flex items-center justify-between rounded-md bg-muted px-3 py-2 gap-3">
            <span class="text-xs text-muted-foreground shrink-0">所属空间</span>
            <div class="flex items-center gap-1.5 min-w-0">
              <Layers class="size-3 text-muted-foreground" />
              <span class="text-xs truncate">{{ store.selectedNode.workspaceDisplayName ?? store.selectedNode.namespace }}</span>
            </div>
          </div>

          <!-- App ID -->
          <div class="flex items-center justify-between rounded-md bg-muted px-3 py-2 gap-3">
            <span class="text-xs text-muted-foreground shrink-0">App ID</span>
            <div class="flex items-center gap-1.5 min-w-0">
              <span class="font-mono text-xs truncate">{{ store.selectedNode.appId }}</span>
              <Button
                variant="ghost"
                size="icon"
                class="size-5 shrink-0 text-muted-foreground hover:text-foreground"
                @click="copyText(store.selectedNode.appId, 'appId')"
              >
                <Check v-if="copiedKey === 'appId'" class="size-3 text-emerald-500" />
                <Copy v-else class="size-3" />
              </Button>
            </div>
          </div>

          <!-- Public key -->
          <div v-if="store.selectedNode.publicKey" class="rounded-md bg-muted px-3 py-2">
            <div class="flex items-center justify-between mb-1.5">
              <span class="text-xs text-muted-foreground">公钥</span>
              <Button
                variant="ghost"
                size="icon"
                class="size-5 text-muted-foreground hover:text-foreground"
                @click="copyText(store.selectedNode.publicKey, 'pubkey')"
              >
                <Check v-if="copiedKey === 'pubkey'" class="size-3 text-emerald-500" />
                <Copy v-else class="size-3" />
              </Button>
            </div>
            <p class="font-mono text-[11px] text-muted-foreground break-all leading-relaxed">
              {{ store.selectedNode.publicKey }}
            </p>
          </div>
        </div>

        <Separator />

        <!-- Labels -->
        <div class="space-y-2">
          <p class="text-[11px] font-medium text-muted-foreground flex items-center gap-1.5">
            <Tag class="size-3" /> 标签
          </p>

          <div class="flex flex-wrap gap-1.5 min-h-7">
            <span
              v-for="(label, i) in store.selectedNode.labels" :key="i"
              class="inline-flex items-center gap-1 text-[11px] font-medium px-2 py-0.5 rounded-md"
              :class="labelColor(label)"
            >
              {{ label }}
              <button
                v-if="store.drawerType === 'edit'"
                class="opacity-50 hover:opacity-100 hover:text-destructive transition-colors"
                type="button"
                @click="store.actions.removeLabel(i)"
              >
                <X class="size-2.5" />
              </button>
            </span>
            <span v-if="!store.selectedNode.labels.length" class="text-xs text-muted-foreground/50 py-0.5">
              暂无标签
            </span>
          </div>

          <div v-if="store.drawerType === 'edit'" class="flex gap-2 pt-1">
            <Input
              v-model="store.newLabelInput"
              placeholder="key=value，回车添加"
              class="h-8 text-xs"
              @keydown.enter="store.actions.addLabel()"
            />
            <Button size="sm" variant="outline" class="shrink-0" @click="store.actions.addLabel()">
              添加
            </Button>
          </div>
        </div>

      </div>

      <!-- ── Footer ────────────────────────────────────────────────── -->
      <DialogFooter class="px-6 py-4 border-t bg-muted/30 sm:justify-between">
        <Button variant="ghost" size="sm" @click="store.isDrawerOpen = false">
          {{ store.drawerType === 'view' ? '关闭' : '取消' }}
        </Button>
        <Button
          v-if="store.drawerType === 'edit'"
          size="sm"
          :disabled="store.isUpdating"
          @click="store.actions.handleSave()"
        >
          <RefreshCw v-if="store.isUpdating" class="size-3.5 animate-spin mr-1.5" />
          保存更改
        </Button>
        <Button
          v-else
          size="sm"
          variant="outline"
          @click="store.actions.openDrawer('edit', store.selectedNode)"
        >
          <Pencil class="size-3.5 mr-1.5" /> 编辑标签
        </Button>
      </DialogFooter>

    </DialogContent>
  </Dialog>
</template>
