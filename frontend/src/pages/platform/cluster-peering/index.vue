<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  ArrowLeftRight, Plus, RefreshCw, Trash2,
  CheckCircle2, XCircle, Clock, Globe,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle,
  DialogDescription, DialogFooter,
} from '@/components/ui/dialog'
import AppAlertDialog from '@/components/AlertDialog.vue'
import { listClusterPeerings, createClusterPeering, deleteClusterPeering, type ClusterPeeringItem } from '@/api/user'
import { toast } from 'vue-sonner'

definePage({
  meta: { titleKey: 'manage.clusterPeering.title', descKey: 'manage.clusterPeering.desc' },
})

// ── Types ─────────────────────────────────────────────────────────
type Phase = 'Ready' | 'Pending' | 'Error'

// ── Data ──────────────────────────────────────────────────────────
const { t } = useI18n()
const connections = ref<ClusterPeeringItem[]>([])
const loading = ref(false)

async function fetchList() {
  loading.value = true
  try {
    const res = await listClusterPeerings()
    connections.value = (res ?? []) as ClusterPeeringItem[]
  } catch (e: any) {
    toast(t('manage.clusterPeering.toast.loadFailed'), { description: e?.message })
  } finally {
    loading.value = false
  }
}

onMounted(fetchList)

// ── Style maps ────────────────────────────────────────────────────
const phaseConfig = computed<Record<string, { label: string; badge: string; icon: any; dot: string }>>(() => ({
  Ready:  { label: t('manage.clusterPeering.stats.active'),  badge: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 ring-1 ring-emerald-500/20', icon: CheckCircle2, dot: 'bg-emerald-500' },
  Pending:{ label: t('manage.clusterPeering.stats.pending'), badge: 'bg-amber-400/10 text-amber-600 dark:text-amber-400 ring-1 ring-amber-400/20',         icon: Clock,        dot: 'bg-amber-400' },
  Error:  { label: t('manage.clusterPeering.stats.failed'),  badge: 'bg-rose-500/10 text-rose-600 dark:text-rose-400 ring-1 ring-rose-500/20',             icon: XCircle,      dot: 'bg-rose-500' },
}))

// ── Stats ─────────────────────────────────────────────────────────
type Filter = Phase | 'all'
const statusFilter = ref<Filter>('all')

const stats = computed(() => ({
  total:   connections.value.length,
  ready:   connections.value.filter(c => c.phase === 'Ready').length,
  pending: connections.value.filter(c => c.phase === 'Pending').length,
  failed:  connections.value.filter(c => c.phase === 'Error').length,
}))

const filtered = computed(() => {
  if (statusFilter.value === 'all') return connections.value
  return connections.value.filter(c => c.phase === statusFilter.value)
})

function setFilter(val: Filter) {
  statusFilter.value = val
}

// ── Create dialog ─────────────────────────────────────────────────
const createOpen = ref(false)
const createLoading = ref(false)
const createForm = ref({
  remoteCluster: '',
  localNamespace: '',
  remoteNamespace: '',
  localNetwork: '',
  remoteNetwork: '',
})

function openCreate() {
  createForm.value = { remoteCluster: '', localNamespace: '', remoteNamespace: '', localNetwork: '', remoteNetwork: '' }
  createOpen.value = true
}

async function handleCreate() {
  if (!createForm.value.remoteCluster.trim() || !createForm.value.localNamespace.trim() || !createForm.value.remoteNamespace.trim()) {
    toast(t('manage.clusterPeering.toast.createFailed'), { description: 'Please fill in required fields' })
    return
  }
  createLoading.value = true
  try {
    await createClusterPeering({
      remoteCluster: createForm.value.remoteCluster.trim(),
      localNamespace: createForm.value.localNamespace.trim(),
      remoteNamespace: createForm.value.remoteNamespace.trim(),
      localNetwork: createForm.value.localNetwork.trim() || 'lattice-default-net',
      remoteNetwork: createForm.value.remoteNetwork.trim() || 'lattice-default-net',
    })
    toast(t('manage.clusterPeering.toast.created'))
    createOpen.value = false
    fetchList()
  } catch (e: any) {
    toast(t('manage.clusterPeering.toast.createFailed'), { description: e?.message })
  } finally {
    createLoading.value = false
  }
}

// ── Delete ────────────────────────────────────────────────────────
const deleteTarget = ref<ClusterPeeringItem | null>(null)
const deleteDialogOpen = ref(false)

function promptDelete(conn: ClusterPeeringItem) {
  deleteTarget.value = conn
  deleteDialogOpen.value = true
}

async function confirmDelete() {
  if (!deleteTarget.value) return
  try {
    await deleteClusterPeering(deleteTarget.value.name)
    toast(t('manage.clusterPeering.toast.deleted'))
    fetchList()
  } catch (e: any) {
    toast(t('manage.clusterPeering.toast.deleteFailed'), { description: e?.message })
  } finally {
    deleteTarget.value = null
  }
}
</script>

<template>
  <div class="flex flex-col gap-5 p-6 animate-in fade-in duration-300">

    <!-- ── Stat cards ─────────────────────────────────────────────── -->
    <div class="grid grid-cols-2 sm:grid-cols-4 gap-4">

      <!-- All -->
      <button
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left hover:shadow-md transition-all"
        :class="statusFilter === 'all' ? 'ring-2 ring-blue-500/20 border-blue-500/30' : ''"
        @click="setFilter('all')"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">{{ t('manage.clusterPeering.stats.all') }}</span>
            <span class="text-2xl font-bold tracking-tight">{{ stats.total }}</span>
          </div>
          <div class="bg-blue-500/10 rounded-lg p-2">
            <Globe class="text-blue-500 size-4" />
          </div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-xs text-muted-foreground">
          <Globe class="size-3.5 shrink-0 text-blue-500" />
          <span>{{ t('manage.clusterPeering.stats.allDesc') }}</span>
        </div>
      </button>

      <!-- Ready -->
      <button
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left hover:shadow-md transition-all"
        :class="statusFilter === 'Ready' ? 'ring-2 ring-emerald-500/20 border-emerald-500/30' : ''"
        @click="setFilter('Ready')"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">{{ t('manage.clusterPeering.stats.active') }}</span>
            <span class="text-2xl font-bold tracking-tight text-emerald-600 dark:text-emerald-400">{{ stats.ready }}</span>
          </div>
          <div class="bg-emerald-500/10 rounded-lg p-2">
            <CheckCircle2 class="text-emerald-500 size-4" />
          </div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-xs text-muted-foreground">
          <CheckCircle2 class="size-3.5 shrink-0 text-emerald-500" />
          <span>{{ t('manage.clusterPeering.stats.activeDesc') }}</span>
        </div>
      </button>

      <!-- Pending -->
      <button
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left hover:shadow-md transition-all"
        :class="statusFilter === 'Pending' ? 'ring-2 ring-amber-500/20 border-amber-500/30' : ''"
        @click="setFilter('Pending')"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">{{ t('manage.clusterPeering.stats.pending') }}</span>
            <span class="text-2xl font-bold tracking-tight text-amber-600 dark:text-amber-400">{{ stats.pending }}</span>
          </div>
          <div class="bg-amber-500/10 rounded-lg p-2">
            <Clock class="text-amber-500 size-4" />
          </div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-xs text-muted-foreground">
          <Clock class="size-3.5 shrink-0 text-amber-500" />
          <span>{{ t('manage.clusterPeering.stats.pendingDesc') }}</span>
        </div>
      </button>

      <!-- Failed -->
      <button
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left hover:shadow-md transition-all"
        :class="statusFilter === 'Error' ? 'ring-2 ring-rose-500/20 border-rose-500/30' : ''"
        @click="setFilter('Error')"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">{{ t('manage.clusterPeering.stats.failed') }}</span>
            <span class="text-2xl font-bold tracking-tight text-rose-600 dark:text-rose-400">{{ stats.failed }}</span>
          </div>
          <div class="bg-rose-500/10 rounded-lg p-2">
            <XCircle class="text-rose-500 size-4" />
          </div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-xs text-muted-foreground">
          <XCircle class="size-3.5 shrink-0 text-rose-500" />
          <span>{{ stats.failed === 0 ? t('manage.clusterPeering.stats.failedOk') : t('manage.clusterPeering.stats.failedDesc') }}</span>
        </div>
      </button>

    </div>

    <!-- ── Toolbar ────────────────────────────────────────────────── -->
    <div class="flex items-center gap-2">
      <div class="ml-auto flex items-center gap-2">
        <Button variant="outline" size="sm" class="gap-1.5" :disabled="loading" @click="fetchList">
          <RefreshCw class="size-3.5" :class="loading ? 'animate-spin' : ''" />
          {{ t('common.action.refresh') }}
        </Button>
        <Button size="sm" class="gap-1.5" @click="openCreate">
          <Plus class="size-3.5" /> {{ t('manage.clusterPeering.createBtn') }}
        </Button>
      </div>
    </div>

    <!-- ── Connection cards ───────────────────────────────────────── -->
    <div v-if="loading" class="flex items-center justify-center py-28 text-muted-foreground text-sm">
      <RefreshCw class="size-4 animate-spin mr-2" /> 加载中...
    </div>

    <div v-else-if="filtered.length" class="grid gap-3 lg:grid-cols-2">
      <div
        v-for="conn in filtered"
        :key="conn.name"
        class="group bg-card border border-border rounded-xl overflow-hidden hover:shadow-md hover:border-primary/20 transition-all"
      >
        <!-- Card header -->
        <div class="flex items-start justify-between px-4 pt-4 pb-3 gap-3">
          <div class="flex items-center gap-3 min-w-0">
            <div class="relative shrink-0 size-9 rounded-xl flex items-center justify-center"
              :class="conn.phase === 'Ready' ? 'bg-emerald-500/10' : conn.phase === 'Pending' ? 'bg-amber-400/10' : 'bg-rose-500/10'">
              <component :is="phaseConfig[conn.phase]?.icon ?? Globe"
                class="size-4"
                :class="conn.phase === 'Ready' ? 'text-emerald-500' : conn.phase === 'Pending' ? 'text-amber-400' : 'text-rose-500'"
              />
            </div>
            <div class="min-w-0">
              <p class="font-bold text-sm leading-none truncate">{{ conn.name }}</p>
              <p class="text-[11px] text-muted-foreground/60 mt-0.5 font-mono truncate">
                {{ conn.localNamespace }} ↔ {{ conn.remoteCluster }}/{{ conn.remoteNamespace }}
              </p>
            </div>
          </div>

          <div class="flex items-center gap-1.5 shrink-0">
            <span class="text-[10px] font-semibold px-2 py-0.5 rounded-full flex items-center gap-1"
              :class="phaseConfig[conn.phase]?.badge">
              <span class="size-1.5 rounded-full" :class="phaseConfig[conn.phase]?.dot" />
              {{ phaseConfig[conn.phase]?.label }}
            </span>
            <Button variant="ghost" size="sm" class="size-7 p-0 opacity-0 group-hover:opacity-100 transition-opacity" @click="promptDelete(conn)">
              <Trash2 class="size-4" />
            </Button>
          </div>
        </div>

        <!-- CIDR info -->
        <div class="flex items-center gap-2 px-4 py-3 bg-muted/30 border-y border-border/60">
          <div class="flex-1 min-w-0">
            <p class="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/50 mb-1">{{ t('manage.clusterPeering.local') }}</p>
            <p class="text-xs font-bold truncate">{{ conn.localNamespace }}/{{ conn.localNetwork }}</p>
            <p class="font-mono text-[10px] text-muted-foreground/60">{{ conn.localCIDR || '—' }}</p>
          </div>
          <div class="flex flex-col items-center gap-0.5 shrink-0">
            <ArrowLeftRight class="size-3.5 text-muted-foreground/40" />
          </div>
          <div class="flex-1 min-w-0 text-right">
            <p class="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/50 mb-1">{{ t('manage.clusterPeering.remote') }}</p>
            <p class="text-xs font-bold truncate">{{ conn.remoteCluster }}/{{ conn.remoteNamespace }}</p>
            <p class="font-mono text-[10px] text-muted-foreground/60">{{ conn.remoteCIDR || '—' }}</p>
          </div>
        </div>

        <!-- Error display -->
        <div v-if="conn.phase === 'Error' && conn.errorMessage"
          class="px-4 py-2 text-xs text-destructive bg-destructive/5">
          {{ conn.errorMessage }}
        </div>
      </div>
    </div>

    <!-- Empty state -->
    <div v-else class="flex flex-col items-center justify-center py-28 text-center">
      <div class="size-16 rounded-2xl bg-muted/40 flex items-center justify-center mb-4">
        <Globe class="size-7 text-muted-foreground/30" />
      </div>
      <p class="text-sm font-semibold text-muted-foreground">{{ t('manage.clusterPeering.empty') }}</p>
      <p class="text-xs text-muted-foreground/50 mt-1">{{ t('manage.clusterPeering.emptyDesc') }}</p>
      <Button size="sm" class="mt-4 gap-1.5" @click="openCreate">
        <Plus class="size-3.5" /> {{ t('manage.clusterPeering.createBtn') }}
      </Button>
    </div>

    <!-- ── Delete confirm ─────────────────────────────────────────── -->
    <AppAlertDialog
      v-model:open="deleteDialogOpen"
      :title="t('manage.clusterPeering.deleteDialog.title')"
      :description="t('manage.clusterPeering.deleteDialog.desc', { name: deleteTarget?.name })"
      :confirm-text="t('common.action.delete')"
      variant="destructive"
      @confirm="confirmDelete"
      @cancel="deleteTarget = null"
    />
  </div>

  <!-- ── Create Dialog ───────────────────────────────────────────── -->
  <Dialog v-model:open="createOpen">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>{{ t('manage.clusterPeering.createDialog.title') }}</DialogTitle>
        <DialogDescription>{{ t('manage.clusterPeering.createDialog.desc') }}</DialogDescription>
      </DialogHeader>

      <div class="space-y-4 py-2">
        <!-- Remote cluster -->
        <div class="space-y-1.5">
          <label class="text-xs font-medium">{{ t('manage.clusterPeering.createDialog.remoteClusterLabel') }} <span class="text-destructive">*</span></label>
          <Input
            v-model="createForm.remoteCluster"
            :placeholder="t('manage.clusterPeering.createDialog.remoteClusterPlaceholder')"
            class="font-mono text-xs h-9"
          />
          <p class="text-[10px] text-muted-foreground/60">{{ t('manage.clusterPeering.createDialog.remoteClusterHint') }}</p>
        </div>

        <!-- Local namespace -->
        <div class="space-y-1.5">
          <label class="text-xs font-medium">{{ t('manage.clusterPeering.createDialog.localNamespaceLabel') }} <span class="text-destructive">*</span></label>
          <Input
            v-model="createForm.localNamespace"
            :placeholder="t('manage.clusterPeering.createDialog.localNamespacePlaceholder')"
            class="font-mono text-xs h-9"
          />
        </div>

        <!-- Remote namespace -->
        <div class="space-y-1.5">
          <label class="text-xs font-medium">{{ t('manage.clusterPeering.createDialog.remoteNamespaceLabel') }} <span class="text-destructive">*</span></label>
          <Input
            v-model="createForm.remoteNamespace"
            :placeholder="t('manage.clusterPeering.createDialog.remoteNamespacePlaceholder')"
            class="font-mono text-xs h-9"
          />
        </div>

        <!-- Local network (optional) -->
        <div class="space-y-1.5">
          <label class="text-xs font-medium">{{ t('manage.clusterPeering.createDialog.localNetLabel') }} <span class="text-muted-foreground font-normal">(lattice-default-net)</span></label>
          <Input
            v-model="createForm.localNetwork"
            placeholder="lattice-default-net"
            class="font-mono text-xs h-9"
          />
        </div>

        <!-- Remote network (optional) -->
        <div class="space-y-1.5">
          <label class="text-xs font-medium">{{ t('manage.clusterPeering.createDialog.remoteNetLabel') }} <span class="text-muted-foreground font-normal">(lattice-default-net)</span></label>
          <Input
            v-model="createForm.remoteNetwork"
            placeholder="lattice-default-net"
            class="font-mono text-xs h-9"
          />
        </div>
      </div>

      <DialogFooter>
        <Button variant="outline" @click="createOpen = false">{{ t('common.action.cancel') }}</Button>
        <Button
          :disabled="!createForm.remoteCluster.trim() || !createForm.localNamespace.trim() || !createForm.remoteNamespace.trim() || createLoading"
          @click="handleCreate"
        >
          <RefreshCw v-if="createLoading" class="size-3.5 mr-1.5 animate-spin" />
          <ArrowLeftRight v-else class="size-3.5 mr-1.5" />
          {{ t('manage.clusterPeering.createDialog.submit') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
