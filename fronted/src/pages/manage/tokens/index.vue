<script setup lang="ts">
import { ref, computed, watch, onMounted, h } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  useVueTable, getCoreRowModel, FlexRender, type ColumnDef,
} from '@tanstack/vue-table'
import {
  Search, RefreshCw, MoreHorizontal, Trash2, KeyRound,
  ChevronLeft, ChevronRight, Plus, Copy, Check, Terminal,
  ShieldCheck, ShieldX, Infinity,
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
import { listTokens, create, rmToken } from '@/api/token'
import AppAlertDialog from '@/components/AlertDialog.vue'
import { toast } from 'vue-sonner'
import { useWorkspaceStore } from '@/stores/workspace'

definePage({
  meta: { titleKey: 'manage.tokens.title', descKey: 'manage.tokens.desc' },
})

const { t } = useI18n()

interface TokenRow {
  token: string
  namespace: string
  workspaceDisplayName?: string
  usageLimit: number
  expiry?: string
  boundPeers?: string[]
  usedCount?: number
  isExpired?: boolean
  phase?: string
}

const rows = ref<TokenRow[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(10)
const loading = ref(false)
const creating = ref(false)
const deleting = ref(false)

const createDialogOpen = ref(false)
const detailDialogOpen = ref(false)
const deleteDialogOpen = ref(false)
const selectedToken = ref<TokenRow | null>(null)
const deleteTarget = ref<TokenRow | null>(null)
const copiedKey = ref<string | null>(null)

const searchValue = ref('')
let searchTimer: ReturnType<typeof setTimeout>

const statusFilter = ref<'all' | 'valid' | 'expired' | 'permanent'>('all')

function isExpired(expiry?: string | null, rowExpired?: boolean) {
  if (rowExpired) return true
  if (!expiry) return false
  const date = new Date(expiry)
  if (Number.isNaN(date.getTime())) return false
  return date.getTime() < Date.now()
}

function isPermanent(expiry?: string | null) {
  return !expiry || expiry === '0001-01-01T00:00:00Z'
}

function formatExpiry(expiry?: string | null) {
  if (isPermanent(expiry)) return t('manage.tokens.expiryPermanent')
  if (!expiry) return '—'
  const date = new Date(expiry)
  if (Number.isNaN(date.getTime())) return '—'
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hours = String(date.getHours()).padStart(2, '0')
  const minutes = String(date.getMinutes()).padStart(2, '0')
  return `${year}-${month}-${day} ${hours}:${minutes}`
}

function maskToken(token: string) {
  if (!token) return '—'
  if (token.length <= 12) return token
  return `${token.slice(0, 8)}...${token.slice(-4)}`
}

function getTokenInitials(token: string) {
  return (token || 'TK').slice(0, 2).toUpperCase()
}

async function fetchList(params?: { page?: number, pageSize?: number, search?: string }) {
  loading.value = true
  if (params?.page) page.value = params.page
  if (params?.pageSize) pageSize.value = params.pageSize
  try {
    const { data, code } = await listTokens({
      page: page.value,
      pageSize: pageSize.value,
      keyword: params?.search ?? searchValue.value,
    }) as any
    if (code === 200) {
      rows.value = Array.isArray(data) ? data : (data?.list ?? data?.items ?? [])
      total.value = Array.isArray(data) ? rows.value.length : (data?.total ?? rows.value.length)
    }
  } catch {
    toast.error(t('manage.tokens.toast.fetchFailed'))
  } finally {
    loading.value = false
  }
}

onMounted(() => fetchList())

const workspaceStore = useWorkspaceStore()
watch(() => workspaceStore.currentWorkspace?.id, (newId, oldId) => {
  if (newId && newId !== oldId) fetchList()
})

const filteredRows = computed(() => {
  const q = searchValue.value.toLowerCase().trim()
  return rows.value.filter((t) => {
    const matchSearch = !q
      || t.token?.toLowerCase().includes(q)
      || t.namespace?.toLowerCase().includes(q)
    const expired = isExpired(t.expiry, t.isExpired)
    const permanent = isPermanent(t.expiry)
    const matchStatus = statusFilter.value === 'all'
      || (statusFilter.value === 'expired' && expired)
      || (statusFilter.value === 'permanent' && permanent)
      || (statusFilter.value === 'valid' && !permanent && !expired)
    return matchSearch && matchStatus
  })
})

const stats = computed(() => {
  const data = rows.value
  const expired = data.filter(t => isExpired(t.expiry, t.isExpired)).length
  const permanent = data.filter(t => isPermanent(t.expiry)).length
  const valid = data.filter(t => !isPermanent(t.expiry) && !isExpired(t.expiry, t.isExpired)).length
  const totalUsageLimit = data.reduce((sum, t) => sum + (t.usageLimit ?? 0), 0)
  return {
    total: data.length,
    expired,
    permanent,
    valid,
    totalUsageLimit,
  }
})

function setStatusFilter(val: typeof statusFilter.value) {
  statusFilter.value = val
  searchValue.value = ''
}

function onSearchInput() {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    statusFilter.value = 'all'
  }, 300)
}

async function handleCreateToken() {
  creating.value = true
  try {
    const { code } = await create({}) as any
    if (code === 200) {
      toast.success(t('manage.tokens.toast.created'))
      createDialogOpen.value = false
      await fetchList({ page: 1 })
    }
  } catch {
    toast.error(t('manage.tokens.toast.createFailed'))
  } finally {
    creating.value = false
  }
}

function promptDelete(token: TokenRow) {
  deleteTarget.value = token
  deleteDialogOpen.value = true
}

async function confirmDelete() {
  if (!deleteTarget.value) return
  deleting.value = true
  try {
    const { code } = await rmToken(deleteTarget.value.token) as any
    if (code === 200) {
      toast.success(t('manage.tokens.toast.deleted'))
      deleteDialogOpen.value = false
      deleteTarget.value = null
      await fetchList()
    }
  } catch {
    toast.error(t('manage.tokens.toast.deleteFailed'))
  } finally {
    deleting.value = false
  }
}

function openDetail(token: TokenRow) {
  selectedToken.value = token
  detailDialogOpen.value = true
}

const installCommand = computed(() => selectedToken.value
  ? `wireflow join --token ${selectedToken.value.token}`
  : '')

async function copyText(text: string, key: string) {
  await navigator.clipboard.writeText(text)
  copiedKey.value = key
  setTimeout(() => { copiedKey.value = null }, 1500)
}

const columns: ColumnDef<TokenRow>[] = [
  {
    id: 'status',
    header: () => t('manage.tokens.col.status'),
    cell: ({ row }) => {
      const token = row.original
      const expired = isExpired(token.expiry, token.isExpired)
      const permanent = isPermanent(token.expiry)
      if (expired) {
        return h('span', { class: 'text-xs font-medium px-2 py-0.5 rounded-full bg-rose-500/10 text-rose-600 dark:text-rose-400 ring-1 ring-rose-500/20' }, token.phase || t('manage.tokens.status.expired'))
      }
      if (permanent) {
        return h('span', { class: 'text-xs font-medium px-2 py-0.5 rounded-full bg-blue-500/10 text-blue-600 dark:text-blue-400 ring-1 ring-blue-500/20' }, token.phase || t('manage.tokens.status.permanent'))
      }
      return h('span', { class: 'text-xs font-medium px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 ring-1 ring-emerald-500/20' }, token.phase || t('manage.tokens.status.valid'))
    },
  },
  {
    id: 'token',
    header: () => t('manage.tokens.col.tokenId'),
    cell: ({ row }) => {
      const token = row.original
      return h('div', { class: 'flex items-center gap-3' }, [
        h('div', {
          class: 'size-9 rounded-lg flex items-center justify-center shrink-0 text-xs font-bold bg-primary/10 text-primary ring-1 ring-primary/20',
        }, getTokenInitials(token.token)),
        h('div', { class: 'min-w-0' }, [
          h('p', { class: 'font-semibold text-sm leading-none font-mono' }, maskToken(token.token)),
          h('p', { class: 'text-[11px] text-muted-foreground mt-1' }, token.namespace || '—'),
        ]),
      ])
    },
  },
  {
    id: 'workspace',
    header: () => t('manage.tokens.col.workspace'),
    cell: ({ row }) => {
      const token = row.original
      const name = token.workspaceDisplayName ?? token.namespace ?? ''
      if (!name) return h('span', { class: 'text-[11px] text-muted-foreground/40' }, '—')
      return h('span', { class: 'text-xs font-medium' }, name)
    },
  },
  {
    id: 'tokenContent',
    header: () => t('manage.tokens.col.tokenContent'),
    cell: ({ row }) => {
      const token = row.original.token || ''
      if (!token) return h('span', { class: 'text-[11px] text-muted-foreground/40' }, '—')
      return h('div', { class: 'flex items-center gap-2 min-w-0' }, [
        h('span', {
          class: 'font-mono text-xs bg-muted px-2 py-1 rounded truncate max-w-[220px]',
          title: token,
        }, token),
        h(Button, {
          variant: 'ghost',
          size: 'sm',
          class: 'size-7 p-0 shrink-0',
          onClick: () => copyText(token, `inline-token-${token}`),
        }, () => copiedKey.value === `inline-token-${token}`
          ? h(Check, { class: 'size-3.5 text-emerald-500' })
          : h(Copy, { class: 'size-3.5' })
        ),
      ])
    },
  },
  {
    accessorKey: 'usageLimit',
    header: () => t('manage.tokens.col.usage'),
    cell: ({ row }) => {
      const used = row.original.usedCount ?? 0
      const limit = row.original.usageLimit ?? 0
      return h('div', { class: 'flex flex-col gap-0.5' }, [
        h('div', { class: 'flex items-baseline gap-1' }, [
          h('span', { class: 'text-sm font-medium tabular-nums' }, String(used)),
          h('span', { class: 'text-[11px] text-muted-foreground/60' }, `/ ${limit}`),
        ]),
        limit > 0
          ? h('span', { class: 'text-[10px] text-muted-foreground/50' }, t('manage.tokens.pctUsed', { pct: Math.round((used / limit) * 100) }))
          : null,
      ])
    },
  },
  {
    id: 'boundPeers',
    header: () => t('manage.tokens.col.boundPeers'),
    cell: ({ row }) => {
      const count = row.original.boundPeers?.length ?? row.original.usedCount ?? 0
      return h('span', { class: 'text-sm text-muted-foreground tabular-nums' }, String(count))
    },
  },
  {
    accessorKey: 'expiry',
    header: () => t('manage.tokens.col.expiry'),
    cell: ({ row }) => {
      const token = row.original
      if (isPermanent(token.expiry)) {
        return h('div', { class: 'flex items-center gap-1.5 text-xs text-muted-foreground' }, [
          h(Infinity, { class: 'size-3.5 shrink-0' }),
          h('span', t('manage.tokens.expiryPermanent')),
        ])
      }
      const text = formatExpiry(token.expiry)
      return h('div', { class: 'flex flex-col gap-0.5' }, [
        h('span', {
          class: `text-xs ${isExpired(token.expiry) ? 'text-rose-500' : 'text-muted-foreground'}`,
          title: token.expiry ?? '',
        }, text),
      ])
    },
  },
  {
    id: 'actions',
    header: '',
    cell: ({ row }) => {
      const token = row.original
      return h(DropdownMenu, {}, {
        default: () => [
          h(DropdownMenuTrigger, { asChild: true }, () =>
            h(Button, { variant: 'ghost', size: 'sm', class: 'size-8 p-0' }, () =>
              h(MoreHorizontal, { class: 'size-4' })
            )
          ),
          h(DropdownMenuContent, { align: 'end', class: 'w-36' }, () => [
            h(DropdownMenuItem, { onClick: () => openDetail(token) }, () => [
              h(Terminal, { class: 'mr-2 size-3.5' }), t('manage.tokens.menu.viewDetail'),
            ]),
            h(DropdownMenuItem, { onClick: () => copyText(token.token, `token-${token.token}`) }, () => [
              h(Copy, { class: 'mr-2 size-3.5' }), t('manage.tokens.menu.copyToken'),
            ]),
            h(DropdownMenuSeparator),
            h(DropdownMenuItem, {
              class: 'text-destructive focus:text-destructive',
              onClick: () => promptDelete(token),
            }, () => [h(Trash2, { class: 'mr-2 size-3.5' }), t('manage.tokens.menu.delete')]),
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

const currentPage = computed(() => page.value)
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))
const visiblePages = computed(() => {
  const cur = currentPage.value
  const totalPageCount = totalPages.value
  const start = Math.max(1, Math.min(cur - 1, totalPageCount - 2))
  const end = Math.min(totalPageCount, start + 2)
  return Array.from({ length: end - start + 1 }, (_, i) => start + i)
})

function goToPage(p: number) {
  if (p < 1 || p > totalPages.value) return
  fetchList({ page: p })
}
</script>

<template>
  <div class="flex flex-col gap-5 p-6 animate-in fade-in duration-300">
    <div class="grid grid-cols-2 sm:grid-cols-4 gap-4">
      <button
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left hover:shadow-md transition-all"
        :class="statusFilter === 'all' ? 'ring-2 ring-blue-500/20 border-blue-500/30' : ''"
        @click="setStatusFilter('all')"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">{{ t('manage.tokens.stats.all') }}</span>
            <span class="text-2xl font-bold tracking-tight">{{ stats.total }}</span>
          </div>
          <div class="bg-blue-500/10 rounded-lg p-2">
            <KeyRound class="text-blue-500 size-4" />
          </div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-xs text-muted-foreground">
          <KeyRound class="size-3.5 shrink-0 text-blue-500" />
          <span>{{ t('manage.tokens.stats.allSub', { n: stats.totalUsageLimit }) }}</span>
        </div>
      </button>

      <button
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left hover:shadow-md transition-all"
        :class="statusFilter === 'valid' ? 'ring-2 ring-emerald-500/20 border-emerald-500/30' : ''"
        @click="setStatusFilter('valid')"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">{{ t('manage.tokens.stats.valid') }}</span>
            <span class="text-2xl font-bold tracking-tight text-emerald-600 dark:text-emerald-400">{{ stats.valid }}</span>
          </div>
          <div class="bg-emerald-500/10 rounded-lg p-2">
            <ShieldCheck class="text-emerald-500 size-4" />
          </div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-xs text-muted-foreground">
          <ShieldCheck class="size-3.5 shrink-0 text-emerald-500" />
          <span>{{ t('manage.tokens.stats.validSub') }}</span>
        </div>
      </button>

      <button
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left hover:shadow-md transition-all"
        :class="statusFilter === 'expired' ? 'ring-2 ring-rose-500/20 border-rose-500/30' : ''"
        @click="setStatusFilter('expired')"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">{{ t('manage.tokens.stats.expired') }}</span>
            <span class="text-2xl font-bold tracking-tight text-rose-600 dark:text-rose-400">{{ stats.expired }}</span>
          </div>
          <div class="bg-rose-500/10 rounded-lg p-2">
            <ShieldX class="text-rose-500 size-4" />
          </div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-xs text-muted-foreground">
          <ShieldX class="size-3.5 shrink-0 text-rose-500" />
          <span>{{ t('manage.tokens.stats.expiredSub') }}</span>
        </div>
      </button>

      <button
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left hover:shadow-md transition-all"
        :class="statusFilter === 'permanent' ? 'ring-2 ring-violet-500/20 border-violet-500/30' : ''"
        @click="setStatusFilter('permanent')"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">{{ t('manage.tokens.stats.permanent') }}</span>
            <span class="text-2xl font-bold tracking-tight text-violet-600 dark:text-violet-400">{{ stats.permanent }}</span>
          </div>
          <div class="bg-violet-500/10 rounded-lg p-2">
            <Infinity class="text-violet-500 size-4" />
          </div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-xs text-muted-foreground">
          <Infinity class="size-3.5 shrink-0 text-violet-500" />
          <span>{{ t('manage.tokens.stats.permanentSub') }}</span>
        </div>
      </button>
    </div>

    <div class="flex items-center gap-2">
      <div class="relative w-72">
        <Search class="absolute left-2.5 top-1/2 -translate-y-1/2 size-4 text-muted-foreground" />
        <Input
          v-model="searchValue"
          :placeholder="t('manage.tokens.searchPlaceholder')"
          class="pl-8 h-9"
          @input="onSearchInput"
        />
      </div>
      <div class="ml-auto flex items-center gap-2">
        <Button variant="outline" size="sm" class="gap-1.5" :disabled="loading" @click="fetchList()">
          <RefreshCw class="size-3.5" :class="loading ? 'animate-spin' : ''" />
          {{ t('common.action.refresh') }}
        </Button>
        <Button size="sm" class="gap-1.5" @click="createDialogOpen = true">
          <Plus class="size-3.5" /> {{ t('manage.tokens.createBtn') }}
        </Button>
      </div>
    </div>

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
              <TableCell
                v-for="cell in row.getVisibleCells()"
                :key="cell.id"
                @click.stop="cell.column.id === 'actions' ? undefined : openDetail(row.original)"
              >
                <FlexRender :render="cell.column.columnDef.cell" :props="cell.getContext()" />
              </TableCell>
            </TableRow>
          </template>
          <TableRow v-else>
            <TableCell :colspan="columns.length" class="h-32 text-center text-muted-foreground">
              {{ loading ? t('common.status.loading') : t('manage.tokens.empty') }}
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>

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

    <AppAlertDialog
      v-model:open="deleteDialogOpen"
      :title="t('manage.tokens.deleteDialog.title')"
      :description="t('manage.tokens.deleteDialog.desc', { token: maskToken(deleteTarget?.token || '') })"
      :confirm-text="t('common.action.delete')"
      variant="destructive"
      @confirm="confirmDelete"
    />

    <Dialog v-model:open="createDialogOpen">
      <DialogContent class="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{{ t('manage.tokens.createDialog.title') }}</DialogTitle>
          <DialogDescription>
            {{ t('manage.tokens.createDialog.desc') }}
          </DialogDescription>
        </DialogHeader>
        <div class="py-2 text-sm text-muted-foreground">
          {{ t('manage.tokens.createDialog.hint') }}
        </div>
        <DialogFooter>
          <Button variant="outline" @click="createDialogOpen = false">{{ t('common.action.cancel') }}</Button>
          <Button :disabled="creating" @click="handleCreateToken">
            {{ creating ? t('common.status.saving') : t('common.action.create') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <Dialog v-model:open="detailDialogOpen">
      <DialogContent class="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle class="flex items-center gap-2">
            <Terminal class="size-4" /> {{ t('manage.tokens.detailDialog.title') }}
          </DialogTitle>
          <DialogDescription>
            {{ t('manage.tokens.detailDialog.desc') }}
          </DialogDescription>
        </DialogHeader>

        <div v-if="selectedToken" class="space-y-4 py-2">
          <div class="rounded-lg border bg-muted/30 p-4 space-y-3">
            <div class="flex items-center justify-between gap-3">
              <span class="text-xs text-muted-foreground">{{ t('manage.tokens.detailDialog.tokenContent') }}</span>
              <Button variant="ghost" size="icon" class="size-7" @click="copyText(selectedToken.token, 'detail-token')">
                <Check v-if="copiedKey === 'detail-token'" class="size-3.5 text-emerald-500" />
                <Copy v-else class="size-3.5" />
              </Button>
            </div>
            <p class="font-mono text-sm break-all">{{ selectedToken.token }}</p>
          </div>

          <div class="grid grid-cols-2 gap-3 text-sm">
            <div class="rounded-md bg-muted px-3 py-2">
              <p class="text-xs text-muted-foreground mb-1">{{ t('manage.tokens.detailDialog.namespace') }}</p>
              <p class="font-mono">{{ selectedToken.namespace }}</p>
            </div>
            <div class="rounded-md bg-muted px-3 py-2">
              <p class="text-xs text-muted-foreground mb-1">{{ t('manage.tokens.detailDialog.usage') }}</p>
              <p>{{ selectedToken.usedCount ?? 0 }} / {{ selectedToken.usageLimit }}</p>
            </div>
            <div class="rounded-md bg-muted px-3 py-2">
              <p class="text-xs text-muted-foreground mb-1">{{ t('manage.tokens.detailDialog.status') }}</p>
              <p>{{ selectedToken.phase || (selectedToken.isExpired ? 'Expired' : 'Active') }}</p>
            </div>
            <div class="rounded-md bg-muted px-3 py-2">
              <p class="text-xs text-muted-foreground mb-1">{{ t('manage.tokens.detailDialog.expiry') }}</p>
              <p>{{ formatExpiry(selectedToken.expiry) }}</p>
            </div>
            <div class="rounded-md bg-muted px-3 py-2">
              <p class="text-xs text-muted-foreground mb-1">{{ t('manage.tokens.detailDialog.boundPeers') }}</p>
              <p>{{ selectedToken.boundPeers?.length ?? selectedToken.usedCount ?? 0 }}</p>
            </div>
          </div>

          <div class="space-y-2">
            <p class="text-sm font-medium">{{ t('manage.tokens.detailDialog.cmdLabel') }}</p>
            <div class="relative">
              <div class="bg-zinc-950 dark:bg-zinc-900 rounded-lg p-4 pr-12 font-mono text-sm text-emerald-400 border border-zinc-800">
                <span class="text-zinc-500 select-none">$ </span>{{ installCommand }}
              </div>
              <button
                @click="copyText(installCommand, 'install-cmd')"
                class="absolute right-2 top-2 p-2 rounded-md text-zinc-400 hover:text-zinc-100 hover:bg-zinc-800 transition-colors"
              >
                <Check v-if="copiedKey === 'install-cmd'" class="size-4 text-emerald-400" />
                <Copy v-else class="size-4" />
              </button>
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" @click="detailDialogOpen = false">{{ t('common.action.close') }}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
