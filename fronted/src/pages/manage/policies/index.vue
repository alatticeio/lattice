<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  useVueTable, getCoreRowModel, FlexRender, type ColumnDef,
} from '@tanstack/vue-table'
import {
  Shield, Plus, RefreshCw, MoreHorizontal, Search,
  Pencil, Trash2, ArrowDown, ArrowUp, ChevronLeft, ChevronRight,
  Info, CheckCircle2, XCircle, X, Zap,
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
  meta: { titleKey: 'manage.policies.title', descKey: 'manage.policies.desc' },
})

const { t } = useI18n()
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
  { key: 'isolate',  label: t('manage.policies.templates.isolate'),  desc: 'Deny All In/Out' },
  { key: 'db',       label: t('manage.policies.templates.db'),       desc: 'Postgres Ingress' },
  { key: 'internet', label: t('manage.policies.templates.internet'), desc: 'Allow HTTPS Out' },
]

// ── Column definitions ────────────────────────────────────────────
const columns: ColumnDef<Policy>[] = [
  {
    accessorKey: 'name',
    header: () => t('manage.policies.col.name'),
    cell: ({ row }) => {
      const p = row.original
      const isDeny = p.action === 'Deny'
      return h('div', { class: 'flex items-center gap-3' }, [
        h('div', {
          class: `size-9 rounded-lg flex items-center justify-center shrink-0 ${isDeny ? 'bg-rose-500/10' : 'bg-emerald-500/10'}`,
        }, h(Shield, { class: `size-4 ${isDeny ? 'text-rose-500' : 'text-emerald-500'}` })),
        h('div', { class: 'min-w-0' }, [
          h('p', { class: 'font-semibold text-sm leading-none' }, p.name),
          h('p', { class: 'text-[11px] text-muted-foreground mt-1 truncate max-w-48' }, p.description || t('manage.policies.noDesc')),
        ]),
      ])
    },
  },
  {
    accessorKey: 'action',
    header: () => t('manage.policies.col.action'),
    cell: ({ row }) => {
      const action = row.original.action ?? 'Allow'
      return h('span', {
        class: `text-xs font-semibold px-2 py-0.5 rounded-full ${actionBadge[action] ?? actionBadge.Allow}`,
      }, action)
    },
  },
  {
    accessorKey: 'policyTypes',
    header: () => t('manage.policies.col.direction'),
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
    header: () => t('manage.policies.col.selector'),
    cell: ({ row }) => {
      const labels = row.original.peerSelector?.matchLabels ?? {}
      const entries = Object.entries(labels)
      if (!entries.length) return h('span', { class: 'text-[11px] text-muted-foreground/40 italic' }, t('manage.policies.notSet'))
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
    header: () => t('manage.policies.col.rules'),
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
    id: 'creator',
    header: () => t('manage.policies.col.creator'),
    cell: ({ row }) => {
      const name = (row.original as any).createdByName as string
      if (!name) return h('span', { class: 'text-[11px] text-muted-foreground/40' }, '—')
      return h('span', { class: 'text-xs text-muted-foreground' }, name)
    },
  },
  {
    id: 'status',
    header: () => t('manage.policies.col.status'),
    cell: ({ row }) => {
      const s = (row.original as any).status as string
      const map: Record<string, { label: string; cls: string }> = {
        active:   { label: t('manage.policies.policyStatus.active'),   cls: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 ring-1 ring-emerald-500/20' },
        pending:  { label: t('manage.policies.policyStatus.pending'),  cls: 'bg-amber-500/10 text-amber-600 dark:text-amber-400 ring-1 ring-amber-500/20' },
        approved: { label: t('manage.policies.policyStatus.approved'), cls: 'bg-blue-500/10 text-blue-600 dark:text-blue-400 ring-1 ring-blue-500/20' },
        failed:   { label: t('manage.policies.policyStatus.failed'),   cls: 'bg-red-500/10 text-red-500 ring-1 ring-red-500/20' },
      }
      const { label, cls } = map[s] ?? { label: s || t('manage.policies.policyStatus.unknown'), cls: 'bg-muted text-muted-foreground ring-1 ring-border' }
      return h('span', { class: `text-[11px] font-bold px-2.5 py-1 rounded-full ${cls}` }, label)
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
            h(DropdownMenuItem, {
              class: (policy as any).status === 'pending' ? 'opacity-50 pointer-events-none' : '',
              onClick: () => store.actions.openDrawer('edit', policy),
            }, () => [
              h(Pencil, { class: 'mr-2 size-3.5' }), t('common.action.edit'),
            ]),
            h(DropdownMenuSeparator),
            h(DropdownMenuItem, {
              class: 'text-destructive focus:text-destructive',
              onClick: () => promptDelete(policy),
            }, () => [h(Trash2, { class: 'mr-2 size-3.5' }), t('common.action.delete')]),
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

      <!-- All Policies -->
      <button
        class="bg-card border border-border rounded-xl p-4 text-left hover:border-primary/30 hover:shadow-sm transition-all"
        :class="actionFilter === 'all' ? 'border-primary/40 ring-1 ring-primary/10' : ''"
        @click="setActionFilter('all')"
      >
        <div>
          <div class="flex items-center justify-between mb-3">
            <span class="text-xs font-semibold text-muted-foreground uppercase tracking-wider">{{ t('manage.policies.stats.total') }}</span>
            <div class="bg-muted rounded-lg p-2">
              <Shield class="size-4 text-muted-foreground" />
            </div>
          </div>
          <p class="text-3xl font-black tracking-tighter tabular-nums">{{ stats.total }}</p>
          <p class="text-[11px] text-muted-foreground/60 mt-1">{{ t('manage.policies.totalRulesLabel') }} <span class="font-bold text-foreground">{{ stats.totalRules }}</span></p>
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
        class="bg-card border border-border rounded-xl p-4 text-left hover:border-emerald-500/30 hover:shadow-sm transition-all"
        :class="actionFilter === 'Allow' ? 'border-emerald-500/40 ring-1 ring-emerald-500/10' : ''"
        @click="setActionFilter('Allow')"
      >
        <div>
          <div class="flex items-center justify-between mb-3">
            <span class="text-xs font-semibold text-muted-foreground uppercase tracking-wider">Allow</span>
            <div class="bg-muted rounded-lg p-2">
              <CheckCircle2 class="size-4 text-muted-foreground" />
            </div>
          </div>
          <p class="text-3xl font-black tracking-tighter tabular-nums text-emerald-500">{{ stats.allow }}</p>
          <p class="text-[11px] text-muted-foreground/60 mt-1">
            {{ stats.allowRate }}%
          </p>
          <div class="mt-3 space-y-1.5">
            <div class="flex h-1.5 rounded-full overflow-hidden bg-muted/50">
              <div class="bg-emerald-500 rounded-full transition-all duration-700" :style="{ width: `${stats.allowRate}%` }" />
            </div>
            <p class="text-[10px] text-muted-foreground/50">{{ t('manage.policies.allowTrafficDesc') }}</p>
          </div>
        </div>
      </button>

      <!-- Deny -->
      <button
        class="bg-card border border-border rounded-xl p-4 text-left hover:border-rose-500/30 hover:shadow-sm transition-all"
        :class="actionFilter === 'Deny' ? 'border-rose-500/40 ring-1 ring-rose-500/10' : ''"
        @click="setActionFilter('Deny')"
      >
        <div>
          <div class="flex items-center justify-between mb-3">
            <span class="text-xs font-semibold text-muted-foreground uppercase tracking-wider">Deny</span>
            <div class="bg-muted rounded-lg p-2">
              <XCircle class="size-4 text-muted-foreground" />
            </div>
          </div>
          <p class="text-3xl font-black tracking-tighter tabular-nums text-rose-500">{{ stats.deny }}</p>
          <p class="text-[11px] text-muted-foreground/60 mt-1">
            {{ stats.denyRate }}%
          </p>
          <div class="mt-3 space-y-1.5">
            <div class="flex h-1.5 rounded-full overflow-hidden bg-muted/50">
              <div class="bg-rose-500 rounded-full transition-all duration-700" :style="{ width: `${stats.denyRate}%` }" />
            </div>
            <p class="text-[10px] text-muted-foreground/50">{{ t('manage.policies.denyTrafficDesc') }}</p>
          </div>
        </div>
      </button>

      <!-- Total Rules -->
      <div class="bg-card border border-border rounded-xl p-4 text-left">
        <div>
          <div class="flex items-center justify-between mb-3">
            <span class="text-xs font-semibold text-muted-foreground uppercase tracking-wider">{{ t('manage.policies.totalRulesLabel') }}</span>
            <div class="bg-muted rounded-lg p-2">
              <ArrowDown class="size-4 text-muted-foreground" />
            </div>
          </div>
          <p class="text-3xl font-black tracking-tighter tabular-nums text-primary">{{ stats.totalRules }}</p>
          <p class="text-[11px] text-muted-foreground/60 mt-1">
            {{ t('manage.policies.avgRulesLabel', { n: stats.avgRules }) }}
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
          :placeholder="t('manage.policies.searchPlaceholder')"
          class="pl-8 h-9"
          @input="onSearchInput"
        />
      </div>
      <div class="ml-auto flex items-center gap-2">
        <Button variant="outline" size="sm" class="gap-1.5"
          :disabled="store.loading" @click="store.actions.refresh()">
          <RefreshCw class="size-3.5" :class="store.loading ? 'animate-spin' : ''" />
          {{ t('common.action.refresh') }}
        </Button>
        <Button size="sm" class="gap-1.5" @click="store.actions.openDrawer('create')">
          <Plus class="size-3.5" /> {{ t('manage.policies.createBtn') }}
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
              {{ store.loading ? t('common.status.loading') : t('manage.policies.empty') }}
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>

    <!-- ── Pagination ─────────────────────────────────────────────── -->
    <div class="flex items-center justify-between text-sm text-muted-foreground">
      <span>{{ t('common.pagination.total', { total: store.total, page: currentPage, totalPages }) }}</span>
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
      :title="t('manage.policies.deleteDialog.title')"
      :description="t('manage.policies.deleteDialog.desc', { name: deleteTarget?.name })"
      :confirm-text="t('common.action.delete')"
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
        <DialogDescription>{{ detailPolicy?.description || t('manage.policies.noDesc') }}</DialogDescription>
      </DialogHeader>

      <div class="space-y-4 py-1 max-h-[60vh] overflow-y-auto pr-1">

        <!-- Direction -->
        <div class="space-y-1.5">
          <p class="text-xs font-semibold text-muted-foreground uppercase tracking-wider">{{ t('manage.policies.detailDialog.direction') }}</p>
          <div class="flex gap-1.5">
            <template v-if="detailPolicy?.policyTypes?.length">
              <span
                v-for="type in detailPolicy.policyTypes" :key="type"
                :class="`flex items-center gap-1 text-xs font-bold px-2.5 py-1 rounded-md ${typeBadge[type] ?? 'bg-muted text-muted-foreground'}`"
              >
                <ArrowDown v-if="type === 'Ingress'" class="size-3" />
                <ArrowUp v-else class="size-3" />
                {{ type }}
              </span>
            </template>
            <span v-else class="text-xs text-muted-foreground/40 italic">{{ t('manage.policies.notSet') }}</span>
          </div>
        </div>

        <!-- Peer selector -->
        <div class="space-y-1.5">
          <p class="text-xs font-semibold text-muted-foreground uppercase tracking-wider">{{ t('manage.policies.detailDialog.targetSelector') }}</p>
          <div class="flex flex-wrap gap-1.5">
            <template v-if="Object.keys(detailPolicy?.peerSelector?.matchLabels ?? {}).length">
              <span
                v-for="[k, v] in Object.entries(detailPolicy!.peerSelector!.matchLabels!)" :key="k"
                class="font-mono text-xs px-2 py-0.5 rounded bg-muted/60 text-muted-foreground ring-1 ring-border"
              >{{ k }}={{ v }}</span>
            </template>
            <span v-else class="text-xs text-muted-foreground/40 italic">{{ t('manage.policies.notSet') }}</span>
          </div>
        </div>

        <!-- Ingress rules -->
        <div v-if="detailPolicy?.ingress?.length" class="space-y-2">
          <p class="text-xs font-semibold flex items-center gap-1.5 text-blue-600 dark:text-blue-400">
            <ArrowDown class="size-3.5" /> Ingress
          </p>
          <div
            v-for="(rule, i) in detailPolicy.ingress" :key="i"
            class="grid grid-cols-2 gap-2 p-3 rounded-lg border border-border bg-muted/20 text-xs"
          >
            <div class="space-y-0.5">
              <p class="text-[10px] text-muted-foreground/50 uppercase font-semibold">{{ t('manage.policies.detailDialog.source') }}</p>
              <p class="font-mono">
                {{ (() => { const ml = rule.from?.[0]?.peerSelector?.matchLabels ?? {}; const k = Object.keys(ml)[0]; return k ? `${k}=${ml[k]}` : '—' })() }}
              </p>
            </div>
            <div class="space-y-0.5">
              <p class="text-[10px] text-muted-foreground/50 uppercase font-semibold">{{ t('manage.policies.detailDialog.port') }}</p>
              <p class="font-mono">{{ rule.ports?.[0]?.port || '—' }} {{ rule.ports?.[0]?.protocol || '' }}</p>
            </div>
          </div>
        </div>

        <!-- Egress rules -->
        <div v-if="detailPolicy?.egress?.length" class="space-y-2">
          <p class="text-xs font-semibold flex items-center gap-1.5 text-violet-600 dark:text-violet-400">
            <ArrowUp class="size-3.5" /> Egress
          </p>
          <div
            v-for="(rule, i) in detailPolicy.egress" :key="i"
            class="grid grid-cols-2 gap-2 p-3 rounded-lg border border-border bg-muted/20 text-xs"
          >
            <div class="space-y-0.5">
              <p class="text-[10px] text-muted-foreground/50 uppercase font-semibold">{{ t('manage.policies.detailDialog.destination') }}</p>
              <p class="font-mono">
                {{ (() => { const ml = rule.to?.[0]?.peerSelector?.matchLabels ?? {}; const k = Object.keys(ml)[0]; return k ? `${k}=${ml[k]}` : '—' })() }}
              </p>
            </div>
            <div class="space-y-0.5">
              <p class="text-[10px] text-muted-foreground/50 uppercase font-semibold">{{ t('manage.policies.detailDialog.port') }}</p>
              <p class="font-mono">{{ rule.ports?.[0]?.port || '—' }} {{ rule.ports?.[0]?.protocol || '' }}</p>
            </div>
          </div>
        </div>

        <!-- No rules hint -->
        <p
          v-if="!detailPolicy?.ingress?.length && !detailPolicy?.egress?.length"
          class="text-xs text-muted-foreground/40 italic"
        >{{ t('manage.policies.detailDialog.noRules') }}</p>

      </div>

      <DialogFooter>
        <Button variant="outline" @click="detailOpen = false">{{ t('common.action.close') }}</Button>
        <Button @click="() => { detailOpen = false; store.actions.openDrawer('edit', detailPolicy) }">
          <Pencil class="size-3.5 mr-1.5" /> {{ t('manage.policies.detailDialog.edit') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>

  <!-- ── Create / Edit Dialog ───────────────────────────────────── -->
  <Dialog :open="store.isDrawerOpen" @update:open="v => { if (!v) store.isDrawerOpen = false }">
    <DialogContent class="sm:max-w-lg">
      <DialogHeader>
        <DialogTitle>{{ store.drawerType === 'create' ? t('manage.policies.createDialog.createTitle') : t('manage.policies.createDialog.editTitle') }}</DialogTitle>
        <DialogDescription>
          {{ store.drawerType === 'create' ? t('manage.policies.createDialog.createDesc') : t('manage.policies.createDialog.editDesc') }}
        </DialogDescription>
      </DialogHeader>

      <div class="space-y-4 py-1 max-h-[65vh] overflow-y-auto pr-1">

        <!-- Quick templates (create only) -->
        <div v-if="store.drawerType === 'create'" class="space-y-2">
          <!-- Allow All -->
          <button
            class="w-full flex items-center gap-3 p-3 rounded-lg border-2 border-emerald-500/30 bg-emerald-500/5 hover:border-emerald-500/60 hover:bg-emerald-500/10 transition-all text-left group"
            @click="store.actions.applyTemplate('allowAll')"
          >
            <div class="size-8 rounded-lg bg-emerald-500/15 flex items-center justify-center shrink-0 group-hover:bg-emerald-500/25 transition-colors">
              <Zap class="size-4 text-emerald-500" />
            </div>
            <div class="min-w-0 flex-1">
              <p class="text-sm font-bold text-emerald-600 dark:text-emerald-400">{{ t('manage.policies.createDialog.allowAllLabel') }}</p>
              <p class="text-[11px] text-muted-foreground/60 mt-0.5">{{ t('manage.policies.createDialog.allowAllDesc') }}</p>
            </div>
            <span class="text-[10px] font-bold px-1.5 py-0.5 rounded bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 shrink-0">{{ t('manage.policies.createDialog.recommended') }}</span>
          </button>

          <!-- Other templates -->
          <div class="grid grid-cols-3 gap-2">
            <button
              v-for="tpl in templates" :key="tpl.key"
              class="p-2.5 rounded-lg border border-border bg-muted/20 hover:border-primary/40 hover:bg-primary/5 transition-all text-left"
              @click="store.actions.applyTemplate(tpl.key)"
            >
              <p class="text-xs font-bold">{{ tpl.label }}</p>
              <p class="text-[10px] text-muted-foreground/60 mt-0.5">{{ tpl.desc }}</p>
            </button>
          </div>
        </div>

        <div class="grid grid-cols-2 gap-3">
          <!-- Name -->
          <div class="space-y-1.5 col-span-2">
            <label class="text-xs font-medium">{{ t('manage.policies.createDialog.nameLabel') }}</label>
            <Input v-model="store.form.name" placeholder="例如：deny-all-egress" class="font-mono text-xs" />
          </div>

          <!-- Target label -->
          <div class="space-y-1.5 col-span-2">
            <label class="text-xs font-medium">
              {{ t('manage.policies.createDialog.selectorLabel') }}
              <span class="text-muted-foreground font-normal ml-1 font-mono text-[10px]">key=value</span>
            </label>
            <Input v-model="store.form._targetLabel" placeholder="app=web" class="font-mono text-xs" />
          </div>

          <!-- Action -->
          <div class="space-y-1.5">
            <label class="text-xs font-medium">{{ t('manage.policies.createDialog.actionLabel') }}</label>
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
            <label class="text-xs font-medium">{{ t('manage.policies.createDialog.directionLabel') }}</label>
            <div class="flex gap-2 h-9 items-center">
              <label
                v-for="type in ['Ingress', 'Egress']" :key="type"
                class="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border cursor-pointer transition-all select-none text-xs font-semibold"
                :class="store.form.policyTypes?.includes(type)
                  ? (type === 'Ingress' ? 'border-blue-500/50 bg-blue-500/8 text-blue-600 dark:text-blue-400' : 'border-violet-500/50 bg-violet-500/8 text-violet-600 dark:text-violet-400')
                  : 'border-border text-muted-foreground'"
              >
                <input
                  type="checkbox"
                  :checked="store.form.policyTypes?.includes(type)"
                  class="sr-only"
                  @change="store.form.policyTypes?.includes(type)
                    ? store.form.policyTypes.splice(store.form.policyTypes.indexOf(type), 1)
                    : store.form.policyTypes.push(type)"
                />
                <ArrowDown v-if="type === 'Ingress'" class="size-3.5" />
                <ArrowUp v-else class="size-3.5" />
                {{ type }}
              </label>
            </div>
          </div>

          <!-- Description -->
          <div class="space-y-1.5 col-span-2">
            <label class="text-xs font-medium">{{ t('manage.policies.createDialog.descLabel') }} <span class="text-muted-foreground font-normal">{{ t('manage.policies.createDialog.descOptional') }}</span></label>
            <Input v-model="store.form.description" :placeholder="t('manage.policies.noDesc')" />
          </div>
        </div>

        <!-- Ingress rules -->
        <div v-if="store.form.policyTypes?.includes('Ingress')" class="space-y-2">
          <div class="flex items-center justify-between">
            <p class="text-xs font-semibold flex items-center gap-1.5 text-blue-600 dark:text-blue-400">
              <ArrowDown class="size-3.5" /> {{ t('manage.policies.createDialog.ingressTitle') }}
            </p>
            <Button variant="ghost" size="sm" class="h-6 text-[11px] text-primary font-bold px-2"
              @click="store.actions.addRule('ingress')">
              {{ t('manage.policies.createDialog.addRule') }}
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
              <p class="text-[10px] text-muted-foreground/50 uppercase font-semibold">{{ t('manage.policies.createDialog.srcSelector') }}</p>
              <Input v-model="rule._rawLabel" placeholder="app=frontend" class="h-7 text-xs font-mono" />
            </div>
            <div class="space-y-1">
              <p class="text-[10px] text-muted-foreground/50 uppercase font-semibold">{{ t('manage.policies.createDialog.portLabel') }}</p>
              <Input v-model="rule.ports[0].port" placeholder="80" class="h-7 text-xs font-mono" />
            </div>
          </div>
          <p v-if="!store.form.ingress.length" class="text-xs text-muted-foreground/40 italic">{{ t('manage.policies.createDialog.noIngressRules') }}</p>
        </div>

        <!-- Egress rules -->
        <div v-if="store.form.policyTypes?.includes('Egress')" class="space-y-2">
          <div class="flex items-center justify-between">
            <p class="text-xs font-semibold flex items-center gap-1.5 text-violet-600 dark:text-violet-400">
              <ArrowUp class="size-3.5" /> {{ t('manage.policies.createDialog.egressTitle') }}
            </p>
            <Button variant="ghost" size="sm" class="h-6 text-[11px] text-primary font-bold px-2"
              @click="store.actions.addRule('egress')">
              {{ t('manage.policies.createDialog.addRule') }}
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
              <p class="text-[10px] text-muted-foreground/50 uppercase font-semibold">{{ t('manage.policies.createDialog.dstSelector') }}</p>
              <Input v-model="rule._rawLabel" placeholder="app=db" class="h-7 text-xs font-mono" />
            </div>
            <div class="space-y-1">
              <p class="text-[10px] text-muted-foreground/50 uppercase font-semibold">{{ t('manage.policies.createDialog.portLabel') }}</p>
              <Input v-model="rule.ports[0].port" placeholder="5432" class="h-7 text-xs font-mono" />
            </div>
          </div>
          <p v-if="!store.form.egress.length" class="text-xs text-muted-foreground/40 italic">{{ t('manage.policies.createDialog.noEgressRules') }}</p>
        </div>

        <!-- Hint -->
        <div class="flex gap-2 rounded-lg bg-primary/5 border border-primary/10 p-3">
          <Info class="size-4 text-primary shrink-0 mt-0.5" />
          <p class="text-xs text-muted-foreground leading-relaxed">
            {{ t('manage.policies.createDialog.crdHint') }}
          </p>
        </div>
      </div>

      <DialogFooter>
        <Button variant="outline" @click="store.isDrawerOpen = false">{{ t('common.action.cancel') }}</Button>
        <Button :disabled="store.loading" @click="store.actions.handleCreateOrUpdate(toast)">
          <RefreshCw v-if="store.loading" class="size-3.5 animate-spin mr-2" />
          {{ store.drawerType === 'create' ? t('manage.policies.createDialog.publish') : t('manage.policies.createDialog.save') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
