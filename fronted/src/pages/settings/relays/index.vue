<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  useVueTable, getCoreRowModel, FlexRender, type ColumnDef,
} from '@tanstack/vue-table'
import {
  Search, RefreshCw, MoreHorizontal, Trash2, Pencil,
  ChevronLeft, ChevronRight, Plus, Wifi, WifiOff,
  Radio, Globe, Zap, Server, ActivitySquare, CheckCircle2,
  AlertCircle, UserCircle, MapPin, Tag, X, Clock,
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
  meta: { titleKey: 'settings.relays.title', descKey: 'settings.relays.desc' },
})

const { t } = useI18n()

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
  displayName: '',
  description: '',
  region: '',
  tcpUrl: '',
  quicUrl: '',
  enabled: true,
  workspaces: [],
  peerLabels: [],
})

const labelInput = ref('')

function addLabel() {
  const v = labelInput.value.trim()
  if (!v) return
  if (!form.value.peerLabels) form.value.peerLabels = []
  if (!form.value.peerLabels.includes(v)) form.value.peerLabels.push(v)
  labelInput.value = ''
}

function removeLabel(index: number) {
  form.value.peerLabels?.splice(index, 1)
}

function resetForm() {
  form.value = { name: '', displayName: '', description: '', region: '', tcpUrl: '', quicUrl: '', enabled: true, workspaces: [], peerLabels: [] }
  labelInput.value = ''
}

function openCreate() {
  editingItem.value = null
  resetForm()
  dialogOpen.value = true
}

function openEdit(row: RelayServer) {
  editingItem.value = row
  form.value = {
    name: row.id,
    displayName: row.name,
    description: row.description ?? '',
    region: row.region ?? '',
    tcpUrl: row.tcpUrl,
    quicUrl: row.quicUrl ?? '',
    enabled: row.enabled,
    workspaces: row.workspaces ?? [],
    peerLabels: row.peerLabels ? [...row.peerLabels] : [],
  }
  labelInput.value = ''
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
    toast.error(t('settings.relays.toast.fetchFailed'))
  } finally {
    loading.value = false
  }
}

onMounted(() => fetchList())

async function handleSave() {
  if (!form.value.displayName?.trim()) { toast.error(t('settings.relays.toast.nameRequired')); return }
  if (!editingItem.value && !form.value.name.trim()) { toast.error(t('settings.relays.toast.slugRequired')); return }
  if (!form.value.tcpUrl.trim()) { toast.error(t('settings.relays.toast.tcpRequired')); return }
  saving.value = true
  try {
    if (editingItem.value) {
      const { code } = await updateRelay(editingItem.value.id, form.value) as any
      if (code === 200) {
        toast.success(t('settings.relays.toast.updated'))
        dialogOpen.value = false
        await fetchList()
      }
    } else {
      const { code } = await createRelay(form.value) as any
      if (code === 200) {
        toast.success(t('settings.relays.toast.created'))
        dialogOpen.value = false
        await fetchList({ page: 1 })
      }
    }
  } catch {
    toast.error(t(editingItem.value ? 'settings.relays.toast.updateFailed' : 'settings.relays.toast.createFailed'))
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
      toast.success(t('settings.relays.toast.deleted'))
      deleteDialogOpen.value = false
      deleteTarget.value = null
      await fetchList()
    }
  } catch {
    toast.error(t('settings.relays.toast.deleteFailed'))
  } finally {
    deleting.value = false
  }
}

async function handleTest(row: RelayServer) {
  testing.value = row.id
  try {
    const { code, data } = await testRelay(row.id) as any
    if (code === 200) {
      toast.success(t('settings.relays.toast.testSuccess', { ms: data?.latencyMs ?? '—' }))
      await fetchList()
    } else {
      toast.error(t('settings.relays.toast.testFailed'))
    }
  } catch {
    toast.error(t('settings.relays.toast.testFailed'))
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
    return { label: t('settings.relays.status.disabled'), cls: 'bg-zinc-500/10 text-zinc-500 ring-zinc-500/20' }
  }
  switch (row.status) {
    case 'healthy':  return { label: t('settings.relays.status.healthy'),  cls: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 ring-emerald-500/20' }
    case 'degraded': return { label: t('settings.relays.status.degraded'), cls: 'bg-amber-500/10  text-amber-600  dark:text-amber-400  ring-amber-500/20' }
    case 'offline':  return { label: t('settings.relays.status.offline'),  cls: 'bg-rose-500/10   text-rose-600   dark:text-rose-400   ring-rose-500/20' }
    default:         return { label: t('settings.relays.status.unknown'),  cls: 'bg-muted text-muted-foreground ring-border' }
  }
}

// ── table columns ─────────────────────────────────────────────────────────────
const columns: ColumnDef<RelayServer>[] = [
  {
    id: 'status',
    header: () => t('settings.relays.col.status'),
    cell: ({ row }) => {
      const b = statusBadge(row.original)
      return h('span', {
        class: `text-xs font-medium px-2 py-0.5 rounded-full ring-1 ${b.cls}`,
      }, b.label)
    },
  },
  {
    id: 'name',
    header: () => t('settings.relays.col.name'),
    cell: ({ row }) => {
      const relay = row.original
      return h('div', { class: 'flex items-center gap-3' }, [
        h('div', {
          class: 'size-9 rounded-lg flex items-center justify-center shrink-0 bg-primary/10 ring-1 ring-primary/20',
        }, h(Server, { class: 'size-4 text-primary' })),
        h('div', { class: 'min-w-0' }, [
          h('p', { class: 'font-semibold text-sm leading-none' }, relay.name || relay.id),
          relay.name
            ? h('p', { class: 'font-mono text-[10px] text-muted-foreground/50 mt-0.5' }, relay.id)
            : null,
          relay.description
            ? h('p', { class: 'text-[11px] text-muted-foreground mt-1 truncate max-w-[200px]' }, relay.description)
            : null,
        ]),
      ])
    },
  },
  {
    id: 'tcp',
    header: () => t('settings.relays.col.tcp'),
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
    header: () => t('settings.relays.col.quic'),
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
    header: () => t('settings.relays.col.latency'),
    cell: ({ row }) => {
      const ms = row.original.latencyMs
      if (ms == null) return h('span', { class: 'text-[11px] text-muted-foreground/40' }, '—')
      const cls = ms < 50 ? 'text-emerald-500' : ms < 150 ? 'text-amber-500' : 'text-rose-500'
      return h('span', { class: `text-xs font-medium tabular-nums ${cls}` }, `${ms} ms`)
    },
  },
  {
    id: 'peers',
    header: () => t('settings.relays.col.peers'),
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
    header: () => t('settings.relays.col.workspaces'),
    cell: ({ row }) => {
      const ws = row.original.workspaces ?? []
      if (ws.length === 0) {
        return h('span', { class: 'text-xs text-muted-foreground/50' }, t('settings.relays.allWorkspaces'))
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
    id: 'creator',
    header: () => t('settings.relays.col.creator'),
    cell: ({ row }) => {
      const name = row.original.createdBy
      if (!name) return h('span', { class: 'text-[11px] text-muted-foreground/40' }, '—')
      return h('div', { class: 'flex items-center gap-1.5' }, [
        h(UserCircle, { class: 'size-3.5 text-muted-foreground shrink-0' }),
        h('span', { class: 'text-sm' }, name),
      ])
    },
  },
  {
    id: 'createdAt',
    header: () => t('settings.relays.col.createdAt'),
    cell: ({ row }) => {
      const ts = row.original.createdAt
      if (!ts) return h('span', { class: 'text-[11px] text-muted-foreground/40' }, '—')
      const d = new Date(ts)
      return h('div', { class: 'flex items-center gap-1.5' }, [
        h(Clock, { class: 'size-3.5 text-muted-foreground shrink-0' }),
        h('span', { class: 'text-xs tabular-nums' }, d.toLocaleDateString()),
      ])
    },
  },
  {
    id: 'updatedBy',
    header: () => t('settings.relays.col.updatedBy'),
    cell: ({ row }) => {
      const name = row.original.updatedBy
      if (!name) return h('span', { class: 'text-[11px] text-muted-foreground/40' }, '—')
      return h('div', { class: 'flex items-center gap-1.5' }, [
        h(UserCircle, { class: 'size-3.5 text-muted-foreground shrink-0' }),
        h('span', { class: 'text-sm' }, name),
      ])
    },
  },
  {
    id: 'updatedAt',
    header: () => t('settings.relays.col.updatedAt'),
    cell: ({ row }) => {
      const ts = row.original.updatedAt
      if (!ts) return h('span', { class: 'text-[11px] text-muted-foreground/40' }, '—')
      const d = new Date(ts)
      return h('div', { class: 'flex items-center gap-1.5' }, [
        h(Clock, { class: 'size-3.5 text-muted-foreground shrink-0' }),
        h('span', { class: 'text-xs tabular-nums' }, d.toLocaleDateString()),
      ])
    },
  },
  {
    id: 'actions',
    header: () => t('settings.relays.col.actions'),
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
              testing.value === relay.id ? t('settings.relays.menu.testing') : t('settings.relays.menu.test'),
            ]),
            h(DropdownMenuItem, { onClick: () => openEdit(relay) }, () => [
              h(Pencil, { class: 'mr-2 size-3.5' }), t('settings.relays.menu.edit'),
            ]),
            h(DropdownMenuSeparator),
            h(DropdownMenuItem, {
              class: 'text-destructive focus:text-destructive',
              onClick: () => promptDelete(relay),
            }, () => [h(Trash2, { class: 'mr-2 size-3.5' }), t('settings.relays.menu.delete')]),
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
            <span class="text-muted-foreground text-sm font-medium">{{ t('settings.relays.stats.all') }}</span>
            <span class="text-2xl font-bold tracking-tight">{{ stats.total }}</span>
          </div>
          <div class="bg-muted rounded-lg p-2"><Server class="text-muted-foreground size-4" /></div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-sm">
          <Zap class="size-4 text-amber-500 shrink-0" />
          <span class="text-muted-foreground">{{ t('settings.relays.stats.allSub', { n: stats.quicEnabled }) }}</span>
        </div>
      </button>

      <button
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left hover:shadow-md transition-shadow"
        :class="statusFilter === 'healthy' ? 'ring-2 ring-emerald-500/20 border-emerald-500/30' : ''"
        @click="setStatusFilter('healthy')"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">{{ t('settings.relays.stats.healthy') }}</span>
            <span class="text-2xl font-bold tracking-tight">{{ stats.healthy }}</span>
          </div>
          <div class="bg-muted rounded-lg p-2"><Wifi class="text-muted-foreground size-4" /></div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-sm">
          <CheckCircle2 class="text-emerald-600 size-4 shrink-0" />
          <span class="text-muted-foreground">{{ t('settings.relays.stats.healthySub') }}</span>
        </div>
      </button>

      <button
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left hover:shadow-md transition-shadow"
        :class="statusFilter === 'degraded' ? 'ring-2 ring-amber-500/20 border-amber-500/30' : ''"
        @click="setStatusFilter('degraded')"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">{{ t('settings.relays.stats.degraded') }}</span>
            <span class="text-2xl font-bold tracking-tight">{{ stats.degraded }}</span>
          </div>
          <div class="bg-muted rounded-lg p-2"><AlertCircle class="text-muted-foreground size-4" /></div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-sm">
          <AlertCircle class="text-amber-500 size-4 shrink-0" />
          <span class="text-muted-foreground">{{ t('settings.relays.stats.degradedSub') }}</span>
        </div>
      </button>

      <button
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left hover:shadow-md transition-shadow"
        :class="statusFilter === 'offline' ? 'ring-2 ring-rose-500/20 border-rose-500/30' : ''"
        @click="setStatusFilter('offline')"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">{{ t('settings.relays.stats.offline') }}</span>
            <span class="text-2xl font-bold tracking-tight">{{ stats.offline }}</span>
          </div>
          <div class="bg-muted rounded-lg p-2"><WifiOff class="text-muted-foreground size-4" /></div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-sm">
          <WifiOff class="text-rose-500 size-4 shrink-0" />
          <span class="text-muted-foreground">{{ t('settings.relays.stats.offlineSub') }}</span>
        </div>
      </button>
    </div>

    <!-- toolbar -->
    <div class="flex items-center gap-2">
      <div class="relative w-72">
        <Search class="absolute left-2.5 top-1/2 -translate-y-1/2 size-4 text-muted-foreground" />
        <Input
          v-model="searchValue"
          :placeholder="t('settings.relays.searchPlaceholder')"
          class="pl-8 h-9"
          @input="onSearchInput"
        />
      </div>
      <div class="ml-auto flex items-center gap-2">
        <Button variant="outline" size="sm" class="gap-1.5" :disabled="loading" @click="fetchList()">
          <RefreshCw class="size-3.5" :class="loading ? 'animate-spin' : ''" />
          {{ t('common.action.refresh') }}
        </Button>
        <Button size="sm" class="gap-1.5" @click="openCreate">
          <Plus class="size-3.5" /> {{ t('settings.relays.createBtn') }}
        </Button>
      </div>
    </div>

    <!-- table -->
    <div class="rounded-md border min-w-0">
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
              {{ loading ? t('common.status.loading') : t('settings.relays.empty') }}
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>

    <!-- pagination -->
    <div class="flex items-center justify-between text-sm text-muted-foreground">
      <span>{{ t('common.pagination.total', { total, page: currentPage, totalPages }) }}</span>
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
      :title="t('settings.relays.deleteDialog.title')"
      :description="t('settings.relays.deleteDialog.desc', { name: deleteTarget?.name ?? '' })"
      :confirm-text="t('common.action.delete')"
      variant="destructive"
      @confirm="confirmDelete"
    />

    <!-- create / edit dialog -->
    <Dialog v-model:open="dialogOpen">
      <DialogContent class="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{{ t(editingItem ? 'settings.relays.dialog.editTitle' : 'settings.relays.dialog.createTitle') }}</DialogTitle>
          <DialogDescription>{{ t('settings.relays.dialog.desc') }}</DialogDescription>
        </DialogHeader>

        <div class="space-y-4 py-1">
          <!-- display name -->
          <div class="space-y-1.5">
            <label class="text-sm font-medium">{{ t('settings.relays.dialog.displayNameLabel') }} <span class="text-destructive">*</span></label>
            <Input v-model="form.displayName" :placeholder="t('settings.relays.dialog.displayNamePlaceholder')" />
          </div>

          <!-- resource name (slug) -->
          <div class="space-y-1.5">
            <label class="text-sm font-medium">{{ t('settings.relays.dialog.nameLabel') }} <span class="text-destructive">*</span></label>
            <Input
              v-model="form.name"
              :placeholder="t('settings.relays.dialog.namePlaceholder')"
              :disabled="!!editingItem"
              class="font-mono text-sm"
              :class="editingItem ? 'opacity-60 cursor-not-allowed' : ''"
            />
            <p class="text-[11px] text-muted-foreground">{{ t('settings.relays.dialog.nameHint') }}</p>
          </div>

          <!-- description -->
          <div class="space-y-1.5">
            <label class="text-sm font-medium">{{ t('settings.relays.dialog.descLabel') }}</label>
            <Input v-model="form.description" :placeholder="t('settings.relays.dialog.descLabel')" />
          </div>

          <!-- region -->
          <div class="space-y-1.5">
            <label class="text-sm font-medium flex items-center gap-1.5">
              <MapPin class="size-3.5 text-muted-foreground" />
              {{ t('settings.relays.dialog.regionLabel') }}
            </label>
            <Input v-model="form.region" :placeholder="t('settings.relays.dialog.regionPlaceholder')" />
          </div>

          <!-- tcp url -->
          <div class="space-y-1.5">
            <label class="text-sm font-medium flex items-center gap-1.5">
              <Globe class="size-3.5 text-muted-foreground" />
              {{ t('settings.relays.dialog.tcpLabel') }} <span class="text-destructive">*</span>
            </label>
            <Input
              v-model="form.tcpUrl"
              placeholder="relay.example.com:6266"
              class="font-mono text-sm"
            />
            <p class="text-[11px] text-muted-foreground">{{ t('settings.relays.dialog.tcpHint') }}</p>
          </div>

          <!-- quic url -->
          <div class="space-y-1.5">
            <label class="text-sm font-medium flex items-center gap-1.5">
              <Zap class="size-3.5 text-amber-500" />
              {{ t('settings.relays.dialog.quicLabel') }}
              <span class="text-[10px] font-normal text-muted-foreground ml-1">{{ t('settings.relays.dialog.quicOptional') }}</span>
            </label>
            <Input
              v-model="form.quicUrl"
              placeholder="relay.example.com:6267"
              class="font-mono text-sm"
            />
            <p class="text-[11px] text-muted-foreground">{{ t('settings.relays.dialog.quicHint') }}</p>
          </div>

          <!-- peer labels -->
          <div class="space-y-1.5">
            <label class="text-sm font-medium flex items-center gap-1.5">
              <Tag class="size-3.5 text-muted-foreground" />
              {{ t('settings.relays.dialog.peerLabelsLabel') }}
            </label>
            <div class="flex gap-2">
              <Input
                v-model="labelInput"
                :placeholder="t('settings.relays.dialog.peerLabelsPlaceholder')"
                class="font-mono text-sm"
                @keyup.enter="addLabel"
              />
              <Button type="button" variant="outline" size="sm" class="shrink-0 px-3" @click="addLabel">
                {{ t('settings.relays.dialog.peerLabelsAdd') }}
              </Button>
            </div>
            <div v-if="form.peerLabels && form.peerLabels.length" class="flex flex-wrap gap-1.5 pt-0.5">
              <span
                v-for="(lbl, i) in form.peerLabels" :key="i"
                class="inline-flex items-center gap-1 text-[11px] font-mono font-medium px-2 py-0.5 rounded-md bg-primary/10 text-primary ring-1 ring-primary/20"
              >
                {{ lbl }}
                <button type="button" class="ml-0.5 hover:text-destructive transition-colors" @click="removeLabel(i)">
                  <X class="size-3" />
                </button>
              </span>
            </div>
            <p class="text-[11px] text-muted-foreground">{{ t('settings.relays.dialog.peerLabelsHint') }}</p>
          </div>

          <!-- enabled -->
          <div class="flex items-center justify-between rounded-lg border px-4 py-3">
            <div>
              <p class="text-sm font-medium">{{ t('settings.relays.dialog.enabledLabel') }}</p>
              <p class="text-[11px] text-muted-foreground mt-0.5">{{ t('settings.relays.dialog.enabledDesc') }}</p>
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
          <Button variant="outline" @click="dialogOpen = false">{{ t('common.action.cancel') }}</Button>
          <Button :disabled="saving" @click="handleSave">
            {{ saving ? t('settings.relays.dialog.saving') : t(editingItem ? 'settings.relays.dialog.save' : 'settings.relays.dialog.create') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

  </div>
</template>
