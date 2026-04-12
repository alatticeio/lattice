<script setup lang="ts">
import { ref, computed } from 'vue'
import {
  ArrowLeftRight, Plus, RefreshCw, MoreHorizontal, Trash2,
  CheckCircle2, XCircle, Clock, AlertTriangle, Info,
  ChevronLeft, ChevronRight, Search, Zap, Globe, Route,
  Activity, Network,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle,
  DialogDescription, DialogFooter,
} from '@/components/ui/dialog'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem,
  DropdownMenuSeparator, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import AppAlertDialog from '@/components/AlertDialog.vue'

definePage({
  meta: { title: '对等连接', description: '管理跨空间网段互通的对等连接通道。' },
})

// ── Types ─────────────────────────────────────────────────────────
type PeerStatus = 'active' | 'pending' | 'failed' | 'disconnected'
type RouteMode  = 'nat' | 'overlay' | 'direct'

interface WorkspaceEndpoint {
  name: string
  namespace: string
  cidr: string
  nodeCount: number
}

interface PeeringConnection {
  id: string
  name: string
  local: WorkspaceEndpoint
  remote: WorkspaceEndpoint
  status: PeerStatus
  routeMode: RouteMode
  cidrConflict: boolean
  latencyMs?: number
  tunnelCidr?: string
  description: string
  createdAt: string
  acceptedAt?: string
}

// ── Mock data ─────────────────────────────────────────────────────
const connections = ref<PeeringConnection[]>([
  {
    id: '1',
    name: 'prod-to-staging',
    local:  { name: 'production',  namespace: 'wf-prod',    cidr: '10.0.1.0/24', nodeCount: 12 },
    remote: { name: 'staging',     namespace: 'wf-staging', cidr: '10.0.2.0/24', nodeCount: 5  },
    status: 'active',
    routeMode: 'direct',
    cidrConflict: false,
    latencyMs: 2,
    tunnelCidr: undefined,
    description: '生产环境与预发布环境直连，用于灰度验证',
    createdAt: '2025-01-15',
    acceptedAt: '2025-01-15',
  },
  {
    id: '2',
    name: 'dev-to-data',
    local:  { name: 'dev-team-a',  namespace: 'wf-dev-a',  cidr: '10.0.0.0/24', nodeCount: 8 },
    remote: { name: 'data-lake',   namespace: 'wf-data',   cidr: '10.0.0.0/24', nodeCount: 3 },
    status: 'active',
    routeMode: 'nat',
    cidrConflict: true,
    latencyMs: 5,
    tunnelCidr: '192.168.100.0/30',
    description: '两空间 CIDR 冲突，通过 NAT 隧道转换地址后互通',
    createdAt: '2025-02-03',
    acceptedAt: '2025-02-04',
  },
  {
    id: '3',
    name: 'test-to-infra',
    local:  { name: 'test-env',    namespace: 'wf-test',   cidr: '172.16.1.0/24', nodeCount: 4 },
    remote: { name: 'infra-shared',namespace: 'wf-infra',  cidr: '172.16.2.0/24', nodeCount: 6 },
    status: 'pending',
    routeMode: 'overlay',
    cidrConflict: false,
    latencyMs: undefined,
    tunnelCidr: undefined,
    description: '测试环境访问共享基础设施（DNS/NTP），等待对端确认',
    createdAt: '2025-04-08',
    acceptedAt: undefined,
  },
  {
    id: '4',
    name: 'legacy-to-prod',
    local:  { name: 'legacy-apps', namespace: 'wf-legacy', cidr: '192.168.0.0/16', nodeCount: 20 },
    remote: { name: 'production',  namespace: 'wf-prod',   cidr: '10.0.1.0/24',   nodeCount: 12 },
    status: 'failed',
    routeMode: 'nat',
    cidrConflict: true,
    latencyMs: undefined,
    tunnelCidr: undefined,
    description: 'NAT 规则配置失败，路由表冲突',
    createdAt: '2025-03-20',
    acceptedAt: undefined,
  },
  {
    id: '5',
    name: 'ci-to-registry',
    local:  { name: 'ci-pipeline', namespace: 'wf-ci',      cidr: '10.1.0.0/24', nodeCount: 3 },
    remote: { name: 'registry',    namespace: 'wf-registry', cidr: '10.2.0.0/24', nodeCount: 2 },
    status: 'disconnected',
    routeMode: 'direct',
    cidrConflict: false,
    latencyMs: undefined,
    tunnelCidr: undefined,
    description: '镜像仓库节点下线，连接中断',
    createdAt: '2024-11-10',
    acceptedAt: '2024-11-10',
  },
])

// ── Style maps ────────────────────────────────────────────────────
const statusConfig: Record<PeerStatus, { label: string; badge: string; icon: any; dot: string }> = {
  active:       { label: '已连接',   badge: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 ring-1 ring-emerald-500/20', icon: CheckCircle2, dot: 'bg-emerald-500' },
  pending:      { label: '待确认',   badge: 'bg-amber-400/10 text-amber-600 dark:text-amber-400 ring-1 ring-amber-400/20',         icon: Clock,         dot: 'bg-amber-400' },
  failed:       { label: '连接失败', badge: 'bg-rose-500/10 text-rose-600 dark:text-rose-400 ring-1 ring-rose-500/20',             icon: XCircle,       dot: 'bg-rose-500' },
  disconnected: { label: '已断开',   badge: 'bg-muted text-muted-foreground ring-1 ring-border',                                   icon: AlertTriangle,  dot: 'bg-muted-foreground/50' },
}

const routeModeConfig: Record<RouteMode, { label: string; badge: string; tip: string }> = {
  direct:  { label: 'Direct',  badge: 'bg-blue-500/10 text-blue-600 dark:text-blue-400 ring-1 ring-blue-500/20',     tip: '无 CIDR 冲突，直接路由' },
  nat:     { label: 'NAT',     badge: 'bg-orange-500/10 text-orange-600 dark:text-orange-400 ring-1 ring-orange-500/20', tip: 'CIDR 冲突，通过地址转换互通' },
  overlay: { label: 'Overlay', badge: 'bg-violet-500/10 text-violet-600 dark:text-violet-400 ring-1 ring-violet-500/20', tip: '覆盖网络封装，适合复杂拓扑' },
}

// ── Stats ─────────────────────────────────────────────────────────
type StatusFilter = PeerStatus | 'all'
const statusFilter = ref<StatusFilter>('all')
const searchValue  = ref('')

const stats = computed(() => ({
  total:        connections.value.length,
  active:       connections.value.filter(c => c.status === 'active').length,
  pending:      connections.value.filter(c => c.status === 'pending').length,
  failed:       connections.value.filter(c => c.status === 'failed').length,
}))

// ── Filter / pagination ───────────────────────────────────────────
const filtered = computed(() => {
  const q = searchValue.value.toLowerCase().trim()
  return connections.value.filter(c => {
    const matchSearch = !q
      || c.name.includes(q)
      || c.local.name.includes(q)
      || c.remote.name.includes(q)
      || c.local.cidr.includes(q)
      || c.remote.cidr.includes(q)
    const matchStatus = statusFilter.value === 'all' || c.status === statusFilter.value
    return matchSearch && matchStatus
  })
})

const PAGE_SIZE   = 10
const currentPage = ref(1)
const totalPages  = computed(() => Math.max(1, Math.ceil(filtered.value.length / PAGE_SIZE)))
const visiblePages = computed(() => {
  const cur   = currentPage.value
  const total = totalPages.value
  const start = Math.max(1, Math.min(cur - 1, total - 2))
  const end   = Math.min(total, start + 2)
  return Array.from({ length: end - start + 1 }, (_, i) => start + i)
})
const paginated = computed(() => {
  const start = (currentPage.value - 1) * PAGE_SIZE
  return filtered.value.slice(start, start + PAGE_SIZE)
})

function setFilter(val: StatusFilter) {
  statusFilter.value = val
  searchValue.value = ''
  currentPage.value = 1
}

// ── Detail dialog ─────────────────────────────────────────────────
const detailOpen = ref(false)
const selected   = ref<PeeringConnection | null>(null)

function openDetail(conn: PeeringConnection) {
  selected.value = conn
  detailOpen.value = true
}

// ── Create dialog ─────────────────────────────────────────────────
const createOpen = ref(false)
const createForm = ref({
  name: '',
  localWorkspace:  '',
  remoteWorkspace: '',
  routeMode: 'direct' as RouteMode,
  description: '',
})

function handleCreate() {
  // TODO: call API
  createOpen.value = false
}

// ── Delete ────────────────────────────────────────────────────────
const deleteTarget     = ref<PeeringConnection | null>(null)
const deleteDialogOpen = ref(false)

function promptDelete(conn: PeeringConnection) {
  deleteTarget.value = conn
  deleteDialogOpen.value = true
}
function confirmDelete() {
  if (deleteTarget.value) {
    connections.value = connections.value.filter(c => c.id !== deleteTarget.value!.id)
  }
  deleteTarget.value = null
}

// ── Refresh (mock) ────────────────────────────────────────────────
const isRefreshing = ref(false)
function handleRefresh() {
  isRefreshing.value = true
  setTimeout(() => (isRefreshing.value = false), 800)
}
</script>

<template>
  <div class="flex flex-col gap-5 p-6 animate-in fade-in duration-300">

    <!-- ── Stat cards ─────────────────────────────────────────────── -->
    <div class="grid grid-cols-2 sm:grid-cols-4 gap-3">
      <button
        class="bg-card border border-border rounded-xl p-4 text-left hover:border-primary/30 transition-colors group"
        :class="statusFilter === 'all' ? 'border-primary/40 ring-1 ring-primary/10' : ''"
        @click="setFilter('all')"
      >
        <div class="flex items-center justify-between mb-2">
          <span class="text-xs font-medium text-muted-foreground">全部连接</span>
          <ArrowLeftRight class="size-4 text-muted-foreground/50 group-hover:text-muted-foreground transition-colors" />
        </div>
        <p class="text-2xl font-black tracking-tighter">{{ stats.total }}</p>
      </button>

      <button
        class="bg-card border border-border rounded-xl p-4 text-left hover:border-emerald-500/30 transition-colors group"
        :class="statusFilter === 'active' ? 'border-emerald-500/40 ring-1 ring-emerald-500/10' : ''"
        @click="setFilter('active')"
      >
        <div class="flex items-center justify-between mb-2">
          <span class="text-xs font-medium text-muted-foreground">已连接</span>
          <CheckCircle2 class="size-4 text-emerald-500/60 group-hover:text-emerald-500 transition-colors" />
        </div>
        <p class="text-2xl font-black tracking-tighter text-emerald-500">{{ stats.active }}</p>
      </button>

      <button
        class="bg-card border border-border rounded-xl p-4 text-left hover:border-amber-400/30 transition-colors group"
        :class="statusFilter === 'pending' ? 'border-amber-400/40 ring-1 ring-amber-400/10' : ''"
        @click="setFilter('pending')"
      >
        <div class="flex items-center justify-between mb-2">
          <span class="text-xs font-medium text-muted-foreground">待确认</span>
          <Clock class="size-4 text-amber-400/60 group-hover:text-amber-400 transition-colors" />
        </div>
        <p class="text-2xl font-black tracking-tighter text-amber-400">{{ stats.pending }}</p>
      </button>

      <button
        class="bg-card border border-border rounded-xl p-4 text-left hover:border-rose-500/30 transition-colors group"
        :class="statusFilter === 'failed' ? 'border-rose-500/40 ring-1 ring-rose-500/10' : ''"
        @click="setFilter('failed')"
      >
        <div class="flex items-center justify-between mb-2">
          <span class="text-xs font-medium text-muted-foreground">连接失败</span>
          <XCircle class="size-4 text-rose-500/60 group-hover:text-rose-500 transition-colors" />
        </div>
        <p class="text-2xl font-black tracking-tighter text-rose-500">{{ stats.failed }}</p>
      </button>
    </div>

    <!-- ── Toolbar ────────────────────────────────────────────────── -->
    <div class="flex items-center gap-2">
      <div class="relative w-72">
        <Search class="absolute left-2.5 top-1/2 -translate-y-1/2 size-4 text-muted-foreground" />
        <Input v-model="searchValue" placeholder="搜索连接名称、空间、网段..." class="pl-8 h-9" />
      </div>
      <div class="ml-auto flex items-center gap-2">
        <Button variant="outline" size="sm" class="gap-1.5" :disabled="isRefreshing" @click="handleRefresh">
          <RefreshCw class="size-3.5" :class="isRefreshing ? 'animate-spin' : ''" />
          刷新
        </Button>
        <Button size="sm" class="gap-1.5" @click="createOpen = true">
          <Plus class="size-3.5" /> 新建连接
        </Button>
      </div>
    </div>

    <!-- ── Connection cards ───────────────────────────────────────── -->
    <div v-if="paginated.length" class="grid gap-3 lg:grid-cols-2">
      <div
        v-for="conn in paginated"
        :key="conn.id"
        class="group bg-card border border-border rounded-xl overflow-hidden hover:shadow-md hover:border-primary/20 transition-all cursor-pointer"
        @click="openDetail(conn)"
      >
        <!-- Card header -->
        <div class="flex items-start justify-between px-4 pt-4 pb-3 gap-3">
          <div class="flex items-center gap-3 min-w-0">
            <!-- Status dot -->
            <div class="relative shrink-0 size-9 rounded-xl flex items-center justify-center"
              :class="conn.status === 'active' ? 'bg-emerald-500/10' : conn.status === 'pending' ? 'bg-amber-400/10' : conn.status === 'failed' ? 'bg-rose-500/10' : 'bg-muted'">
              <component :is="statusConfig[conn.status].icon"
                class="size-4"
                :class="conn.status === 'active' ? 'text-emerald-500' : conn.status === 'pending' ? 'text-amber-400' : conn.status === 'failed' ? 'text-rose-500' : 'text-muted-foreground'"
              />
            </div>
            <div class="min-w-0">
              <div class="flex items-center gap-2">
                <p class="font-bold text-sm leading-none truncate">{{ conn.name }}</p>
                <!-- CIDR conflict badge -->
                <span v-if="conn.cidrConflict"
                  class="text-[10px] font-bold px-1.5 py-0.5 rounded bg-orange-500/10 text-orange-500 ring-1 ring-orange-500/20 shrink-0">
                  CIDR冲突
                </span>
              </div>
              <p class="text-[11px] text-muted-foreground/60 mt-0.5 truncate">{{ conn.description }}</p>
            </div>
          </div>

          <div class="flex items-center gap-1.5 shrink-0" @click.stop>
            <!-- Status badge -->
            <span class="text-[10px] font-semibold px-2 py-0.5 rounded-full flex items-center gap-1"
              :class="statusConfig[conn.status].badge">
              <span class="size-1.5 rounded-full" :class="statusConfig[conn.status].dot"
                :style="conn.status === 'active' ? 'animation: pulse 2s infinite' : ''" />
              {{ statusConfig[conn.status].label }}
            </span>
            <DropdownMenu>
              <DropdownMenuTrigger as-child>
                <Button variant="ghost" size="sm" class="size-7 p-0 opacity-0 group-hover:opacity-100 transition-opacity">
                  <MoreHorizontal class="size-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" class="w-36">
                <DropdownMenuItem @click="openDetail(conn)">
                  <Info class="mr-2 size-3.5" /> 查看详情
                </DropdownMenuItem>
                <DropdownMenuItem v-if="conn.status === 'pending'">
                  <CheckCircle2 class="mr-2 size-3.5" /> 接受请求
                </DropdownMenuItem>
                <DropdownMenuItem v-if="conn.status === 'failed'">
                  <RefreshCw class="mr-2 size-3.5" /> 重试连接
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  class="text-destructive focus:text-destructive"
                  @click="promptDelete(conn)"
                >
                  <Trash2 class="mr-2 size-3.5" /> 删除
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </div>

        <!-- Workspace path visualization -->
        <div class="flex items-center gap-2 px-4 py-3 bg-muted/30 border-y border-border/60">
          <!-- Local -->
          <div class="flex-1 min-w-0">
            <p class="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/50 mb-1">本端空间</p>
            <p class="text-xs font-bold truncate">{{ conn.local.name }}</p>
            <p class="font-mono text-[10px] text-muted-foreground/60">{{ conn.local.cidr }}</p>
          </div>

          <!-- Connector -->
          <div class="flex flex-col items-center gap-0.5 shrink-0">
            <div class="flex items-center gap-1">
              <div class="w-6 h-px bg-border" />
              <span class="text-[10px] font-bold px-1.5 py-0.5 rounded"
                :class="routeModeConfig[conn.routeMode].badge">
                {{ routeModeConfig[conn.routeMode].label }}
              </span>
              <div class="w-6 h-px bg-border" />
            </div>
            <ArrowLeftRight class="size-3.5 text-muted-foreground/40" />
          </div>

          <!-- Remote -->
          <div class="flex-1 min-w-0 text-right">
            <p class="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/50 mb-1">对端空间</p>
            <p class="text-xs font-bold truncate">{{ conn.remote.name }}</p>
            <p class="font-mono text-[10px] text-muted-foreground/60">{{ conn.remote.cidr }}</p>
          </div>
        </div>

        <!-- Footer stats -->
        <div class="flex items-center divide-x divide-border/60 px-4 py-2.5 text-center">
          <div class="flex-1 flex items-center justify-center gap-1.5 text-xs text-muted-foreground">
            <Activity class="size-3 shrink-0" />
            <span v-if="conn.latencyMs !== undefined" class="font-mono font-semibold" :class="conn.latencyMs < 5 ? 'text-emerald-500' : 'text-amber-500'">
              {{ conn.latencyMs }}ms
            </span>
            <span v-else class="text-muted-foreground/40">—</span>
          </div>
          <div class="flex-1 flex items-center justify-center gap-1.5 text-xs text-muted-foreground">
            <Network class="size-3 shrink-0" />
            {{ conn.local.nodeCount + conn.remote.nodeCount }} 节点
          </div>
          <div class="flex-1 flex items-center justify-center gap-1.5 text-xs text-muted-foreground">
            <Globe class="size-3 shrink-0" />
            <span v-if="conn.tunnelCidr" class="font-mono text-[10px]">{{ conn.tunnelCidr }}</span>
            <span v-else class="text-muted-foreground/40">原生路由</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Empty state -->
    <div v-else class="flex flex-col items-center justify-center py-28 text-center">
      <div class="size-16 rounded-2xl bg-muted/40 flex items-center justify-center mb-4">
        <ArrowLeftRight class="size-7 text-muted-foreground/30" />
      </div>
      <p class="text-sm font-semibold text-muted-foreground">暂无对等连接</p>
      <p class="text-xs text-muted-foreground/50 mt-1">创建连接以打通不同空间之间的网络</p>
      <Button size="sm" class="mt-4 gap-1.5" @click="createOpen = true">
        <Plus class="size-3.5" /> 新建连接
      </Button>
    </div>

    <!-- ── Pagination ─────────────────────────────────────────────── -->
    <div v-if="totalPages > 1" class="flex items-center justify-between text-sm text-muted-foreground">
      <span>共 {{ filtered.length }} 条 · 第 {{ currentPage }} / {{ totalPages }} 页</span>
      <div class="flex items-center gap-1">
        <Button variant="outline" size="sm" class="size-8 p-0" :disabled="currentPage <= 1" @click="currentPage--">
          <ChevronLeft class="size-4" />
        </Button>
        <Button
          v-for="p in visiblePages" :key="p"
          variant="outline" size="sm" class="size-8 p-0 text-xs"
          :class="p === currentPage ? 'bg-primary text-primary-foreground border-primary' : ''"
          @click="currentPage = p"
        >{{ p }}</Button>
        <Button variant="outline" size="sm" class="size-8 p-0" :disabled="currentPage >= totalPages" @click="currentPage++">
          <ChevronRight class="size-4" />
        </Button>
      </div>
    </div>

    <!-- ── Delete confirm ─────────────────────────────────────────── -->
    <AppAlertDialog
      v-model:open="deleteDialogOpen"
      title="删除对等连接"
      :description="`确认删除「${deleteTarget?.name}」？双端路由规则将被清除，通信立即中断。`"
      confirm-text="删除"
      variant="destructive"
      @confirm="confirmDelete"
      @cancel="deleteTarget = null"
    />
  </div>

  <!-- ── Detail Dialog ───────────────────────────────────────────── -->
  <Dialog v-model:open="detailOpen">
    <DialogContent class="sm:max-w-lg">
      <DialogHeader>
        <DialogTitle class="flex items-center gap-2">
          <ArrowLeftRight class="size-4" />
          {{ selected?.name }}
        </DialogTitle>
        <DialogDescription>{{ selected?.description }}</DialogDescription>
      </DialogHeader>

      <div v-if="selected" class="space-y-4 pt-1 max-h-[65vh] overflow-y-auto pr-1">

        <!-- Status + mode -->
        <div class="flex items-center gap-2">
          <span class="text-xs font-semibold px-2.5 py-1 rounded-full flex items-center gap-1.5"
            :class="statusConfig[selected.status].badge">
            <component :is="statusConfig[selected.status].icon" class="size-3" />
            {{ statusConfig[selected.status].label }}
          </span>
          <span class="text-xs font-semibold px-2.5 py-1 rounded-full flex items-center gap-1.5"
            :class="routeModeConfig[selected.routeMode].badge">
            <Route class="size-3" />
            {{ routeModeConfig[selected.routeMode].label }} — {{ routeModeConfig[selected.routeMode].tip }}
          </span>
          <span v-if="selected.cidrConflict"
            class="text-xs font-semibold px-2.5 py-1 rounded-full bg-orange-500/10 text-orange-500 ring-1 ring-orange-500/20 flex items-center gap-1.5">
            <AlertTriangle class="size-3" />
            CIDR 冲突
          </span>
        </div>

        <!-- Endpoints -->
        <div class="rounded-lg border border-border overflow-hidden">
          <div class="grid grid-cols-2 divide-x divide-border/60">
            <!-- Local -->
            <div class="p-4 space-y-2">
              <p class="text-[10px] font-bold uppercase tracking-widest text-muted-foreground/50">本端空间</p>
              <p class="font-bold text-sm">{{ selected.local.name }}</p>
              <div class="space-y-1 text-xs text-muted-foreground">
                <div class="flex items-center justify-between">
                  <span>Namespace</span>
                  <span class="font-mono text-[11px]">{{ selected.local.namespace }}</span>
                </div>
                <div class="flex items-center justify-between">
                  <span>CIDR</span>
                  <span class="font-mono text-[11px] font-semibold text-foreground">{{ selected.local.cidr }}</span>
                </div>
                <div class="flex items-center justify-between">
                  <span>节点数</span>
                  <span class="font-semibold text-foreground">{{ selected.local.nodeCount }}</span>
                </div>
              </div>
            </div>
            <!-- Remote -->
            <div class="p-4 space-y-2">
              <p class="text-[10px] font-bold uppercase tracking-widest text-muted-foreground/50">对端空间</p>
              <p class="font-bold text-sm">{{ selected.remote.name }}</p>
              <div class="space-y-1 text-xs text-muted-foreground">
                <div class="flex items-center justify-between">
                  <span>Namespace</span>
                  <span class="font-mono text-[11px]">{{ selected.remote.namespace }}</span>
                </div>
                <div class="flex items-center justify-between">
                  <span>CIDR</span>
                  <span class="font-mono text-[11px] font-semibold text-foreground">{{ selected.remote.cidr }}</span>
                </div>
                <div class="flex items-center justify-between">
                  <span>节点数</span>
                  <span class="font-semibold text-foreground">{{ selected.remote.nodeCount }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Tunnel / metrics -->
        <div class="rounded-lg border border-border overflow-hidden divide-y divide-border/60">
          <div v-if="selected.tunnelCidr" class="flex items-center justify-between px-4 py-2.5">
            <span class="text-xs text-muted-foreground flex items-center gap-1.5"><Route class="size-3" /> 隧道 CIDR</span>
            <span class="font-mono text-xs font-semibold">{{ selected.tunnelCidr }}</span>
          </div>
          <div class="flex items-center justify-between px-4 py-2.5">
            <span class="text-xs text-muted-foreground flex items-center gap-1.5"><Activity class="size-3" /> 延迟</span>
            <span v-if="selected.latencyMs !== undefined" class="font-mono text-xs font-semibold"
              :class="selected.latencyMs < 5 ? 'text-emerald-500' : 'text-amber-500'">
              {{ selected.latencyMs }} ms
            </span>
            <span v-else class="text-xs text-muted-foreground/40">未知</span>
          </div>
          <div class="flex items-center justify-between px-4 py-2.5">
            <span class="text-xs text-muted-foreground">创建时间</span>
            <span class="text-xs">{{ selected.createdAt }}</span>
          </div>
          <div v-if="selected.acceptedAt" class="flex items-center justify-between px-4 py-2.5">
            <span class="text-xs text-muted-foreground">接受时间</span>
            <span class="text-xs">{{ selected.acceptedAt }}</span>
          </div>
        </div>

        <!-- Pending tip -->
        <div v-if="selected.status === 'pending'"
          class="flex gap-2 rounded-lg bg-amber-400/5 border border-amber-400/20 p-3">
          <Clock class="size-4 text-amber-400 shrink-0 mt-0.5" />
          <p class="text-xs text-muted-foreground leading-relaxed">
            等待对端空间管理员确认此连接请求。确认后双端路由将自动配置生效。
          </p>
        </div>

        <!-- Failed tip -->
        <div v-if="selected.status === 'failed'"
          class="flex gap-2 rounded-lg bg-rose-500/5 border border-rose-500/20 p-3">
          <XCircle class="size-4 text-rose-500 shrink-0 mt-0.5" />
          <p class="text-xs text-muted-foreground leading-relaxed">
            连接建立失败。请检查两端空间的网络策略与路由表配置，确认无冲突后重试。
          </p>
        </div>
      </div>

      <DialogFooter>
        <Button variant="outline" @click="detailOpen = false">关闭</Button>
        <Button v-if="selected?.status === 'pending'">
          <CheckCircle2 class="size-3.5 mr-1.5" /> 接受请求
        </Button>
        <Button v-else-if="selected?.status === 'failed'" variant="secondary">
          <RefreshCw class="size-3.5 mr-1.5" /> 重试连接
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>

  <!-- ── Create Dialog ───────────────────────────────────────────── -->
  <Dialog v-model:open="createOpen">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>新建对等连接</DialogTitle>
        <DialogDescription>将两个工作空间通过加密隧道互通，支持 CIDR 冲突场景</DialogDescription>
      </DialogHeader>

      <div class="space-y-4 py-2">
        <div class="space-y-1.5">
          <label class="text-xs font-medium">连接名称</label>
          <Input v-model="createForm.name" placeholder="例如：prod-to-staging" class="font-mono text-xs" />
        </div>

        <div class="grid grid-cols-2 gap-3">
          <div class="space-y-1.5">
            <label class="text-xs font-medium">本端空间</label>
            <select
              v-model="createForm.localWorkspace"
              class="w-full h-9 rounded-md border border-input bg-background px-3 text-sm focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50 transition-[color,box-shadow]"
            >
              <option value="">请选择</option>
              <option value="production">production</option>
              <option value="staging">staging</option>
              <option value="dev-team-a">dev-team-a</option>
              <option value="test-env">test-env</option>
            </select>
          </div>
          <div class="space-y-1.5">
            <label class="text-xs font-medium">对端空间</label>
            <select
              v-model="createForm.remoteWorkspace"
              class="w-full h-9 rounded-md border border-input bg-background px-3 text-sm focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50 transition-[color,box-shadow]"
            >
              <option value="">请选择</option>
              <option value="data-lake">data-lake</option>
              <option value="infra-shared">infra-shared</option>
              <option value="registry">registry</option>
              <option value="ci-pipeline">ci-pipeline</option>
            </select>
          </div>
        </div>

        <!-- Route mode -->
        <div class="space-y-2">
          <label class="text-xs font-medium">路由模式</label>
          <div class="grid grid-cols-3 gap-2">
            <button
              v-for="mode in (['direct', 'nat', 'overlay'] as RouteMode[])" :key="mode"
              class="p-2.5 rounded-lg border text-left transition-all"
              :class="createForm.routeMode === mode
                ? 'border-primary bg-primary/5'
                : 'border-border hover:border-primary/30'"
              @click="createForm.routeMode = mode"
            >
              <p class="text-xs font-bold" :class="createForm.routeMode === mode ? 'text-primary' : ''">
                {{ routeModeConfig[mode].label }}
              </p>
              <p class="text-[10px] text-muted-foreground/60 mt-0.5 leading-tight">{{ routeModeConfig[mode].tip }}</p>
            </button>
          </div>
        </div>

        <div class="space-y-1.5">
          <label class="text-xs font-medium">描述 <span class="text-muted-foreground font-normal">(可选)</span></label>
          <Input v-model="createForm.description" placeholder="说明此连接的用途..." />
        </div>

        <!-- Info tip -->
        <div class="flex gap-2 rounded-lg bg-primary/5 border border-primary/10 p-3">
          <Zap class="size-4 text-primary shrink-0 mt-0.5" />
          <p class="text-xs text-muted-foreground leading-relaxed">
            发起请求后，对端空间管理员需在其管理界面确认。若两端 CIDR 冲突，请选择 <strong>NAT</strong> 模式。
          </p>
        </div>
      </div>

      <DialogFooter>
        <Button variant="outline" @click="createOpen = false">取消</Button>
        <Button :disabled="!createForm.name || !createForm.localWorkspace || !createForm.remoteWorkspace"
          @click="handleCreate">
          <ArrowLeftRight class="size-3.5 mr-1.5" /> 发起连接
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
