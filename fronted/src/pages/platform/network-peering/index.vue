<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  ArrowLeftRight, Plus, RefreshCw, MoreHorizontal, Trash2,
  CheckCircle2, XCircle, Clock, AlertTriangle, Info,
  ChevronLeft, ChevronRight, Search, Zap,
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
import { listPeerings, createPeering, deletePeering } from '@/api/user'
import { useWorkspaceStore } from '@/stores/workspace'
import { toast } from 'vue-sonner'

definePage({
  meta: { titleKey: 'manage.networkPeering.title', descKey: 'manage.networkPeering.desc' },
})

// ── Types ─────────────────────────────────────────────────────────
type PeeringStatus = 'active' | 'pending' | 'failed'
type PeeringMode   = 'gateway' | 'mesh'

interface WorkspaceEndpoint {
  name:      string
  namespace: string
  cidr:      string
  nodeCount: number
}

interface PeeringConnection {
  name:        string
  local:       WorkspaceEndpoint
  remote:      WorkspaceEndpoint
  status:      PeeringStatus
  peeringMode: PeeringMode
  createdAt:   string
}

// ── Data ──────────────────────────────────────────────────────────
const workspaceStore = useWorkspaceStore()
const { t } = useI18n()
const connections    = ref<PeeringConnection[]>([])
const loading        = ref(false)

async function fetchList() {
  loading.value = true
  try {
    const res = await listPeerings()
    connections.value = (res.data?.data ?? res.data ?? []) as PeeringConnection[]
  } catch (e: any) {
    toast(t('manage.networkPeering.toast.loadFailed'), { description: e?.message })
  } finally {
    loading.value = false
  }
}

onMounted(fetchList)

// ── Style maps ────────────────────────────────────────────────────
const statusConfig = computed<Record<PeeringStatus, { label: string; badge: string; icon: any; dot: string }>>(() => ({
  active:  { label: t('manage.networkPeering.status.active'),  badge: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 ring-1 ring-emerald-500/20', icon: CheckCircle2, dot: 'bg-emerald-500' },
  pending: { label: t('manage.networkPeering.status.pending'), badge: 'bg-amber-400/10 text-amber-600 dark:text-amber-400 ring-1 ring-amber-400/20',         icon: Clock,        dot: 'bg-amber-400' },
  failed:  { label: t('manage.networkPeering.status.failed'),  badge: 'bg-rose-500/10 text-rose-600 dark:text-rose-400 ring-1 ring-rose-500/20',             icon: XCircle,      dot: 'bg-rose-500' },
}))

const modeConfig = computed<Record<PeeringMode, { label: string; badge: string; tip: string }>>(() => ({
  gateway: { label: 'Gateway', badge: 'bg-blue-500/10 text-blue-600 dark:text-blue-400 ring-1 ring-blue-500/20',        tip: t('manage.networkPeering.mode.gatewayTip') },
  mesh:    { label: 'Mesh',    badge: 'bg-violet-500/10 text-violet-600 dark:text-violet-400 ring-1 ring-violet-500/20', tip: t('manage.networkPeering.mode.meshTip') },
}))

// ── Stats ─────────────────────────────────────────────────────────
type StatusFilter = PeeringStatus | 'all'
const statusFilter = ref<StatusFilter>('all')
const searchValue  = ref('')

const stats = computed(() => ({
  total:   connections.value.length,
  active:  connections.value.filter(c => c.status === 'active').length,
  pending: connections.value.filter(c => c.status === 'pending').length,
  failed:  connections.value.filter(c => c.status === 'failed').length,
}))

// ── Filter / pagination ───────────────────────────────────────────
const filtered = computed(() => {
  const q = searchValue.value.toLowerCase().trim()
  return connections.value.filter(c => {
    const matchSearch = !q
      || c.name.toLowerCase().includes(q)
      || c.local.name.toLowerCase().includes(q)
      || c.remote.name.toLowerCase().includes(q)
      || c.local.namespace.toLowerCase().includes(q)
      || c.remote.namespace.toLowerCase().includes(q)
    const matchStatus = statusFilter.value === 'all' || c.status === statusFilter.value
    return matchSearch && matchStatus
  })
})

const PAGE_SIZE    = 10
const currentPage  = ref(1)
const totalPages   = computed(() => Math.max(1, Math.ceil(filtered.value.length / PAGE_SIZE)))
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
  searchValue.value  = ''
  currentPage.value  = 1
}

// ── Detail dialog ─────────────────────────────────────────────────
const detailOpen = ref(false)
const selected   = ref<PeeringConnection | null>(null)

function openDetail(conn: PeeringConnection) {
  selected.value  = conn
  detailOpen.value = true
}

// ── Create dialog ─────────────────────────────────────────────────
const createOpen    = ref(false)
const createLoading = ref(false)
const createForm    = ref({
  name:        '',
  namespaceB:  '',
  networkB:    '',
  peeringMode: 'gateway' as PeeringMode,
})

function openCreate() {
  createForm.value = { name: '', namespaceB: '', networkB: '', peeringMode: 'gateway' }
  createOpen.value = true
}

async function handleCreate() {
  if (!createForm.value.namespaceB.trim()) {
    toast(t('manage.networkPeering.toast.nsRequired'))
    return
  }
  createLoading.value = true
  try {
    await createPeering({
      name:        createForm.value.name.trim() || undefined,
      namespaceB:  createForm.value.namespaceB.trim(),
      networkB:    createForm.value.networkB.trim() || undefined,
      peeringMode: createForm.value.peeringMode,
    })
    toast(t('manage.networkPeering.toast.created'))
    createOpen.value = false
    fetchList()
  } catch (e: any) {
    toast(t('manage.networkPeering.toast.createFailed'), { description: e?.message })
  } finally {
    createLoading.value = false
  }
}

// ── Delete ────────────────────────────────────────────────────────
const deleteTarget     = ref<PeeringConnection | null>(null)
const deleteDialogOpen = ref(false)

function promptDelete(conn: PeeringConnection) {
  deleteTarget.value     = conn
  deleteDialogOpen.value = true
}

async function confirmDelete() {
  if (!deleteTarget.value) return
  try {
    await deletePeering(deleteTarget.value.name)
    toast(t('manage.networkPeering.toast.deleted'))
    fetchList()
  } catch (e: any) {
    toast(t('manage.networkPeering.toast.deleteFailed'), { description: e?.message })
  } finally {
    deleteTarget.value = null
  }
}
</script>

<template>
  <div class="flex flex-col gap-5 p-6 animate-in fade-in duration-300">

    <!-- ── Stat cards ─────────────────────────────────────────────── -->
    <div class="grid grid-cols-2 sm:grid-cols-4 gap-4">

      <!-- 全部连接 -->
      <button
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left hover:shadow-md transition-all"
        :class="statusFilter === 'all' ? 'ring-2 ring-blue-500/20 border-blue-500/30' : ''"
        @click="setFilter('all')"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">{{ t('manage.networkPeering.stats.all') }}</span>
            <span class="text-2xl font-bold tracking-tight">{{ stats.total }}</span>
          </div>
          <div class="bg-blue-500/10 rounded-lg p-2">
            <ArrowLeftRight class="text-blue-500 size-4" />
          </div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-xs text-muted-foreground">
          <ArrowLeftRight class="size-3.5 shrink-0 text-blue-500" />
          <span>{{ t('manage.networkPeering.stats.allDesc') }}</span>
        </div>
      </button>

      <!-- 已连接 -->
      <button
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left hover:shadow-md transition-all"
        :class="statusFilter === 'active' ? 'ring-2 ring-emerald-500/20 border-emerald-500/30' : ''"
        @click="setFilter('active')"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">{{ t('manage.networkPeering.stats.active') }}</span>
            <span class="text-2xl font-bold tracking-tight text-emerald-600 dark:text-emerald-400">{{ stats.active }}</span>
          </div>
          <div class="bg-emerald-500/10 rounded-lg p-2">
            <CheckCircle2 class="text-emerald-500 size-4" />
          </div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-xs text-muted-foreground">
          <CheckCircle2 class="size-3.5 shrink-0 text-emerald-500" />
          <span>{{ t('manage.networkPeering.stats.activeDesc') }}</span>
        </div>
      </button>

      <!-- 建立中 -->
      <button
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left hover:shadow-md transition-all"
        :class="statusFilter === 'pending' ? 'ring-2 ring-amber-500/20 border-amber-500/30' : ''"
        @click="setFilter('pending')"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">{{ t('manage.networkPeering.stats.pending') }}</span>
            <span class="text-2xl font-bold tracking-tight text-amber-600 dark:text-amber-400">{{ stats.pending }}</span>
          </div>
          <div class="bg-amber-500/10 rounded-lg p-2">
            <Clock class="text-amber-500 size-4" />
          </div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-xs text-muted-foreground">
          <Clock class="size-3.5 shrink-0 text-amber-500" />
          <span>{{ t('manage.networkPeering.stats.pendingDesc') }}</span>
        </div>
      </button>

      <!-- 连接失败 -->
      <button
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left hover:shadow-md transition-all"
        :class="statusFilter === 'failed' ? 'ring-2 ring-rose-500/20 border-rose-500/30' : ''"
        @click="setFilter('failed')"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">{{ t('manage.networkPeering.stats.failed') }}</span>
            <span class="text-2xl font-bold tracking-tight text-rose-600 dark:text-rose-400">{{ stats.failed }}</span>
          </div>
          <div class="bg-rose-500/10 rounded-lg p-2">
            <XCircle class="text-rose-500 size-4" />
          </div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-xs text-muted-foreground">
          <XCircle class="size-3.5 shrink-0 text-rose-500" />
          <span>{{ stats.failed === 0 ? t('manage.networkPeering.stats.failedOk') : t('manage.networkPeering.stats.failedDesc') }}</span>
        </div>
      </button>

    </div>

    <!-- ── Toolbar ────────────────────────────────────────────────── -->
    <div class="flex items-center gap-2">
      <div class="relative w-72">
        <Search class="absolute left-2.5 top-1/2 -translate-y-1/2 size-4 text-muted-foreground" />
        <Input v-model="searchValue" :placeholder="t('manage.networkPeering.searchPlaceholder')" class="pl-8 h-9" />
      </div>
      <div class="ml-auto flex items-center gap-2">
        <Button variant="outline" size="sm" class="gap-1.5" :disabled="loading" @click="fetchList">
          <RefreshCw class="size-3.5" :class="loading ? 'animate-spin' : ''" />
          {{ t('common.action.refresh') }}
        </Button>
        <Button size="sm" class="gap-1.5" @click="openCreate">
          <Plus class="size-3.5" /> {{ t('manage.networkPeering.createBtn') }}
        </Button>
      </div>
    </div>

    <!-- ── Connection cards ───────────────────────────────────────── -->
    <div v-if="loading" class="flex items-center justify-center py-28 text-muted-foreground text-sm">
      <RefreshCw class="size-4 animate-spin mr-2" /> 加载中...
    </div>

    <div v-else-if="paginated.length" class="grid gap-3 lg:grid-cols-2">
      <div
        v-for="conn in paginated"
        :key="conn.name"
        class="group bg-card border border-border rounded-xl overflow-hidden hover:shadow-md hover:border-primary/20 transition-all cursor-pointer"
        @click="openDetail(conn)"
      >
        <!-- Card header -->
        <div class="flex items-start justify-between px-4 pt-4 pb-3 gap-3">
          <div class="flex items-center gap-3 min-w-0">
            <div class="relative shrink-0 size-9 rounded-xl flex items-center justify-center"
              :class="conn.status === 'active' ? 'bg-emerald-500/10' : conn.status === 'pending' ? 'bg-amber-400/10' : 'bg-rose-500/10'">
              <component :is="statusConfig[conn.status]?.icon ?? AlertTriangle"
                class="size-4"
                :class="conn.status === 'active' ? 'text-emerald-500' : conn.status === 'pending' ? 'text-amber-400' : 'text-rose-500'"
              />
            </div>
            <div class="min-w-0">
              <p class="font-bold text-sm leading-none truncate">{{ conn.name }}</p>
              <p class="text-[11px] text-muted-foreground/60 mt-0.5 font-mono truncate">
                {{ conn.local.namespace }} ↔ {{ conn.remote.namespace }}
              </p>
            </div>
          </div>

          <div class="flex items-center gap-1.5 shrink-0" @click.stop>
            <span class="text-[10px] font-semibold px-2 py-0.5 rounded-full flex items-center gap-1"
              :class="statusConfig[conn.status]?.badge">
              <span class="size-1.5 rounded-full" :class="statusConfig[conn.status]?.dot" />
              {{ statusConfig[conn.status]?.label }}
            </span>
            <DropdownMenu>
              <DropdownMenuTrigger as-child>
                <Button variant="ghost" size="sm" class="size-7 p-0 opacity-0 group-hover:opacity-100 transition-opacity">
                  <MoreHorizontal class="size-4" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" class="w-36">
                <DropdownMenuItem @click="openDetail(conn)">
                  <Info class="mr-2 size-3.5" /> {{ t('manage.networkPeering.actions.viewDetail') }}
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem class="text-destructive focus:text-destructive" @click="promptDelete(conn)">
                  <Trash2 class="mr-2 size-3.5" /> {{ t('manage.networkPeering.actions.delete') }}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </div>

        <!-- Workspace path -->
        <div class="flex items-center gap-2 px-4 py-3 bg-muted/30 border-y border-border/60">
          <div class="flex-1 min-w-0">
            <p class="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/50 mb-1">{{ t('manage.networkPeering.localWorkspace') }}</p>
            <p class="text-xs font-bold truncate">{{ conn.local.name }}</p>
            <p class="font-mono text-[10px] text-muted-foreground/60">{{ conn.local.cidr || '—' }}</p>
          </div>
          <div class="flex flex-col items-center gap-0.5 shrink-0">
            <span class="text-[10px] font-bold px-1.5 py-0.5 rounded" :class="modeConfig[conn.peeringMode]?.badge ?? modeConfig.gateway.badge">
              {{ modeConfig[conn.peeringMode]?.label ?? conn.peeringMode }}
            </span>
            <ArrowLeftRight class="size-3.5 text-muted-foreground/40" />
          </div>
          <div class="flex-1 min-w-0 text-right">
            <p class="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/50 mb-1">{{ t('manage.networkPeering.remoteWorkspace') }}</p>
            <p class="text-xs font-bold truncate">{{ conn.remote.name }}</p>
            <p class="font-mono text-[10px] text-muted-foreground/60">{{ conn.remote.cidr || '—' }}</p>
          </div>
        </div>

        <!-- Footer stats -->
        <div class="flex items-center divide-x divide-border/60 px-4 py-2.5 text-center">
          <div class="flex-1 flex items-center justify-center gap-1.5 text-xs text-muted-foreground">
            <Network class="size-3 shrink-0" />
            {{ t('manage.networkPeering.nodes', { n: conn.local.nodeCount + conn.remote.nodeCount }) }}
          </div>
          <div class="flex-1 flex items-center justify-center gap-1.5 text-xs text-muted-foreground">
            <Activity class="size-3 shrink-0" />
            <span class="font-mono text-[10px]">{{ conn.local.cidr || t('manage.networkPeering.allocating') }}</span>
          </div>
          <div class="flex-1 flex items-center justify-center gap-1.5 text-xs text-muted-foreground">
            <span class="text-[10px]">{{ new Date(conn.createdAt).toLocaleDateString('zh-CN') }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Empty state -->
    <div v-else class="flex flex-col items-center justify-center py-28 text-center">
      <div class="size-16 rounded-2xl bg-muted/40 flex items-center justify-center mb-4">
        <ArrowLeftRight class="size-7 text-muted-foreground/30" />
      </div>
      <p class="text-sm font-semibold text-muted-foreground">{{ t('manage.networkPeering.empty') }}</p>
      <p class="text-xs text-muted-foreground/50 mt-1">{{ t('manage.networkPeering.emptyDesc') }}</p>
      <Button size="sm" class="mt-4 gap-1.5" @click="openCreate">
        <Plus class="size-3.5" /> {{ t('manage.networkPeering.createBtn') }}
      </Button>
    </div>

    <!-- ── Pagination ─────────────────────────────────────────────── -->
    <div v-if="totalPages > 1" class="flex items-center justify-between text-sm text-muted-foreground">
      <span>{{ t('manage.networkPeering.pagination', { total: filtered.length, page: currentPage, totalPages }) }}</span>
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
      :title="t('manage.networkPeering.deleteDialog.title')"
      :description="t('manage.networkPeering.deleteDialog.desc', { name: deleteTarget?.name })"
      :confirm-text="t('common.action.delete')"
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
        <DialogDescription>{{ selected?.local.namespace }} ↔ {{ selected?.remote.namespace }}</DialogDescription>
      </DialogHeader>

      <div v-if="selected" class="space-y-4 pt-1 max-h-[65vh] overflow-y-auto pr-1">

        <!-- Status + mode -->
        <div class="flex items-center gap-2 flex-wrap">
          <span class="text-xs font-semibold px-2.5 py-1 rounded-full flex items-center gap-1.5"
            :class="statusConfig[selected.status]?.badge">
            <component :is="statusConfig[selected.status]?.icon" class="size-3" />
            {{ statusConfig[selected.status]?.label }}
          </span>
          <span class="text-xs font-semibold px-2.5 py-1 rounded-full flex items-center gap-1.5"
            :class="modeConfig[selected.peeringMode]?.badge ?? modeConfig.gateway.badge">
            {{ modeConfig[selected.peeringMode]?.label ?? selected.peeringMode }}
            — {{ modeConfig[selected.peeringMode]?.tip ?? '' }}
          </span>
        </div>

        <!-- Endpoints -->
        <div class="rounded-lg border border-border overflow-hidden">
          <div class="grid grid-cols-2 divide-x divide-border/60">
            <div class="p-4 space-y-2">
              <p class="text-[10px] font-bold uppercase tracking-widest text-muted-foreground/50">{{ t('manage.networkPeering.localWorkspace') }}</p>
              <p class="font-bold text-sm">{{ selected.local.name }}</p>
              <div class="space-y-1 text-xs text-muted-foreground">
                <div class="flex items-center justify-between">
                  <span>{{ t('manage.networkPeering.detailDialog.namespace') }}</span>
                  <span class="font-mono text-[11px]">{{ selected.local.namespace }}</span>
                </div>
                <div class="flex items-center justify-between">
                  <span>{{ t('manage.networkPeering.detailDialog.cidr') }}</span>
                  <span class="font-mono text-[11px] font-semibold text-foreground">{{ selected.local.cidr || t('manage.networkPeering.allocating') }}</span>
                </div>
                <div class="flex items-center justify-between">
                  <span>{{ t('manage.networkPeering.detailDialog.nodeCount') }}</span>
                  <span class="font-semibold text-foreground">{{ selected.local.nodeCount }}</span>
                </div>
              </div>
            </div>
            <div class="p-4 space-y-2">
              <p class="text-[10px] font-bold uppercase tracking-widest text-muted-foreground/50">{{ t('manage.networkPeering.remoteWorkspace') }}</p>
              <p class="font-bold text-sm">{{ selected.remote.name }}</p>
              <div class="space-y-1 text-xs text-muted-foreground">
                <div class="flex items-center justify-between">
                  <span>{{ t('manage.networkPeering.detailDialog.namespace') }}</span>
                  <span class="font-mono text-[11px]">{{ selected.remote.namespace }}</span>
                </div>
                <div class="flex items-center justify-between">
                  <span>{{ t('manage.networkPeering.detailDialog.cidr') }}</span>
                  <span class="font-mono text-[11px] font-semibold text-foreground">{{ selected.remote.cidr || t('manage.networkPeering.allocating') }}</span>
                </div>
                <div class="flex items-center justify-between">
                  <span>{{ t('manage.networkPeering.detailDialog.nodeCount') }}</span>
                  <span class="font-semibold text-foreground">{{ selected.remote.nodeCount }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Metadata -->
        <div class="rounded-lg border border-border overflow-hidden divide-y divide-border/60">
          <div class="flex items-center justify-between px-4 py-2.5">
            <span class="text-xs text-muted-foreground">{{ t('manage.networkPeering.detailDialog.createdAt') }}</span>
            <span class="text-xs">{{ new Date(selected.createdAt).toLocaleString('zh-CN') }}</span>
          </div>
        </div>

        <!-- Pending tip -->
        <div v-if="selected.status === 'pending'"
          class="flex gap-2 rounded-lg bg-amber-400/5 border border-amber-400/20 p-3">
          <Clock class="size-4 text-amber-400 shrink-0 mt-0.5" />
          <p class="text-xs text-muted-foreground leading-relaxed">
            {{ t('manage.networkPeering.detailDialog.pendingTip') }}
          </p>
        </div>

        <!-- Failed tip -->
        <div v-if="selected.status === 'failed'"
          class="flex gap-2 rounded-lg bg-rose-500/5 border border-rose-500/20 p-3">
          <XCircle class="size-4 text-rose-500 shrink-0 mt-0.5" />
          <p class="text-xs text-muted-foreground leading-relaxed">
            {{ t('manage.networkPeering.detailDialog.failedTip1') }}<code class="font-mono">wireflow.run/gateway=true</code>{{ t('manage.networkPeering.detailDialog.failedTip2') }}
          </p>
        </div>
      </div>

      <DialogFooter>
        <Button variant="outline" @click="detailOpen = false">{{ t('common.action.close') }}</Button>
        <Button variant="destructive" size="sm" @click="() => { detailOpen = false; promptDelete(selected!) }">
          <Trash2 class="size-3.5 mr-1.5" /> {{ t('manage.networkPeering.detailDialog.deleteBtn') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>

  <!-- ── Create Dialog ───────────────────────────────────────────── -->
  <Dialog v-model:open="createOpen">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>{{ t('manage.networkPeering.createDialog.title') }}</DialogTitle>
        <DialogDescription>{{ t('manage.networkPeering.createDialog.desc') }}</DialogDescription>
      </DialogHeader>

      <div class="space-y-4 py-2">

        <!-- Local (read-only) -->
        <div class="space-y-1.5">
          <label class="text-xs font-medium">{{ t('manage.networkPeering.createDialog.localLabel') }}</label>
          <div class="h-9 rounded-md border border-input bg-muted/40 px-3 flex items-center text-sm text-muted-foreground">
            {{ workspaceStore.currentWorkspace?.displayName || workspaceStore.currentWorkspace?.namespace || '—' }}
            <span v-if="workspaceStore.currentWorkspace?.namespace" class="ml-2 font-mono text-[11px] text-muted-foreground/60">
              ({{ workspaceStore.currentWorkspace.namespace }})
            </span>
          </div>
        </div>

        <!-- Remote namespace -->
        <div class="space-y-1.5">
          <label class="text-xs font-medium">{{ t('manage.networkPeering.createDialog.remoteNsLabel') }} <span class="text-destructive">*</span></label>
          <Input
            v-model="createForm.namespaceB"
            :placeholder="t('manage.networkPeering.createDialog.remoteNsPlaceholder')"
            class="font-mono text-xs h-9"
          />
          <p class="text-[10px] text-muted-foreground/60">{{ t('manage.networkPeering.createDialog.remoteNsHint') }}</p>
        </div>

        <!-- Remote network (optional) -->
        <div class="space-y-1.5">
          <label class="text-xs font-medium">{{ t('manage.networkPeering.createDialog.remoteNetLabel') }} <span class="text-muted-foreground font-normal">{{ t('manage.networkPeering.createDialog.remoteNetOptional') }}</span></label>
          <Input
            v-model="createForm.networkB"
            :placeholder="t('manage.networkPeering.createDialog.remoteNetPlaceholder')"
            class="font-mono text-xs h-9"
          />
        </div>

        <!-- Connection name (optional) -->
        <div class="space-y-1.5">
          <label class="text-xs font-medium">{{ t('manage.networkPeering.createDialog.nameLabel') }} <span class="text-muted-foreground font-normal">{{ t('manage.networkPeering.createDialog.nameOptional') }}</span></label>
          <Input v-model="createForm.name" :placeholder="t('manage.networkPeering.createDialog.namePlaceholder')" class="font-mono text-xs h-9" />
        </div>

        <!-- Mode selector -->
        <div class="space-y-2">
          <label class="text-xs font-medium">{{ t('manage.networkPeering.createDialog.modeLabel') }}</label>
          <div class="grid grid-cols-2 gap-2">
            <button
              v-for="mode in (['gateway', 'mesh'] as PeeringMode[])" :key="mode"
              class="p-3 rounded-lg border text-left transition-all"
              :class="createForm.peeringMode === mode
                ? 'border-primary bg-primary/5'
                : 'border-border hover:border-primary/30'"
              @click="createForm.peeringMode = mode"
            >
              <p class="text-xs font-bold" :class="createForm.peeringMode === mode ? 'text-primary' : ''">
                {{ modeConfig[mode].label }}
              </p>
              <p class="text-[10px] text-muted-foreground/60 mt-0.5 leading-tight">{{ modeConfig[mode].tip }}</p>
            </button>
          </div>
        </div>

        <!-- Info tip -->
        <div class="flex gap-2 rounded-lg bg-primary/5 border border-primary/10 p-3">
          <Zap class="size-4 text-primary shrink-0 mt-0.5" />
          <p class="text-xs text-muted-foreground leading-relaxed">
            {{ t('manage.networkPeering.createDialog.zapTip1') }}<code class="font-mono">wireflow.run/gateway=true</code>{{ t('manage.networkPeering.createDialog.zapTip2') }}
          </p>
        </div>
      </div>

      <DialogFooter>
        <Button variant="outline" @click="createOpen = false">{{ t('common.action.cancel') }}</Button>
        <Button
          :disabled="!createForm.namespaceB.trim() || createLoading"
          @click="handleCreate"
        >
          <RefreshCw v-if="createLoading" class="size-3.5 mr-1.5 animate-spin" />
          <ArrowLeftRight v-else class="size-3.5 mr-1.5" />
          {{ t('manage.networkPeering.createDialog.submit') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
