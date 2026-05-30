<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import {
  useVueTable, getCoreRowModel, FlexRender, type ColumnDef,
} from '@tanstack/vue-table'
import {
  Search, RefreshCw, MoreHorizontal, Plus, Trash2, Pencil,
  Eye, ChevronLeft, ChevronRight, Copy, Check, Fingerprint,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter,
} from '@/components/ui/dialog'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem,
  DropdownMenuSeparator, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Card, CardContent } from '@/components/ui/card'
import AppAlertDialog from '@/components/AlertDialog.vue'
import { usePeerIdentityStore } from '@/stores/usePeerIdentityStore'
import type { PeerIdentity } from '@/api/peer-identity'

definePage({
  meta: { titleKey: 'manage.peerIdentities.title', descKey: 'manage.peerIdentities.desc' },
})

const { t } = useI18n()
const store = usePeerIdentityStore()

// ── Stats / filter ────────────────────────────────────────────────
type StatusFilter = 'all' | 'bound' | 'grace'
const statusFilter = ref<StatusFilter>('all')
const searchValue = ref('')

function setStatusFilter(val: StatusFilter) {
  statusFilter.value = val
  searchValue.value = ''
}

const filtered = computed(() => {
  const q = searchValue.value.toLowerCase().trim()
  const now = new Date()
  return store.rows.filter(i => {
    const matchSearch = !q
      || i.name.toLowerCase().includes(q)
      || i.peer_ref.toLowerCase().includes(q)
      || (i.description ?? '').toLowerCase().includes(q)
    const matchStatus = statusFilter.value === 'all'
      || (statusFilter.value === 'bound' && i.resolved_peer_ip)
      || (statusFilter.value === 'grace' && i.grace_period_expires_at && new Date(i.grace_period_expires_at) > now)
    return matchSearch && matchStatus
  })
})

// ── Pagination ─────────────────────────────────────────────────────
const PAGE_SIZE = 10
const currentPage = ref(1)
const totalPages = computed(() => Math.max(1, Math.ceil(filtered.value.length / PAGE_SIZE)))
const paged = computed(() => {
  const start = (currentPage.value - 1) * PAGE_SIZE
  return filtered.value.slice(start, start + PAGE_SIZE)
})
const visiblePages = computed(() => {
  const cur = currentPage.value
  const total = totalPages.value
  const start = Math.max(1, Math.min(cur - 1, total - 2))
  const end = Math.min(total, start + 2)
  return Array.from({ length: end - start + 1 }, (_, i) => start + i)
})
function goToPage(p: number) {
  if (p < 1 || p > totalPages.value) return
  currentPage.value = p
}

// ── Copy helper ───────────────────────────────────────────────────
const copiedKey = ref<string | null>(null)
async function copyText(text: string, key: string) {
  await navigator.clipboard.writeText(text)
  copiedKey.value = key
  setTimeout(() => { copiedKey.value = null }, 1500)
}

// ── Grace period helper ───────────────────────────────────────────
function isGraceActive(identity: PeerIdentity): boolean {
  return !!identity.grace_period_expires_at && new Date(identity.grace_period_expires_at) > new Date()
}

// ── Column definitions ────────────────────────────────────────────
const columns: ColumnDef<PeerIdentity, any>[] = [
  {
    accessorKey: 'name',
    header: t('manage.peerIdentities.colName'),
    cell: ({ row }) => h('div', { class: 'flex items-center gap-2' }, [
      h(Fingerprint, { class: 'h-4 w-4 text-muted-foreground' }),
      h('span', { class: 'font-medium' }, row.original.name),
    ]),
  },
  {
    accessorKey: 'peer_ref',
    header: t('manage.peerIdentities.colPeerRef'),
    cell: ({ row }) => h('code', { class: 'text-xs bg-muted px-1.5 py-0.5 rounded' }, row.original.peer_ref),
  },
  {
    accessorKey: 'resolved_peer_ip',
    header: t('manage.peerIdentities.colResolvedIP'),
    cell: ({ row }) => {
      const ip = row.original.resolved_peer_ip
      if (!ip) return h('span', { class: 'text-muted-foreground text-xs' }, '-')
      return h('div', { class: 'flex items-center gap-1' }, [
        h('code', { class: 'text-xs' }, ip),
        h('button', {
          class: 'p-0.5 rounded hover:bg-muted',
          onClick: () => copyText(ip, `ip-${row.original.id}`),
        }, copiedKey.value === `ip-${row.original.id}`
          ? h(Check, { class: 'h-3 w-3 text-emerald-500' })
          : h(Copy, { class: 'h-3 w-3 text-muted-foreground' }),
        ),
      ])
    },
  },
  {
    accessorKey: 'grace_period_expires_at',
    header: t('manage.peerIdentities.colGracePeriod'),
    cell: ({ row }) => {
      if (!row.original.previous_peer_ref) return h('span', { class: 'text-muted-foreground text-xs' }, '-')
      const active = isGraceActive(row.original)
      return h('span', {
        class: `inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs ${active ? 'bg-amber-500/10 text-amber-600 dark:text-amber-400' : 'bg-muted text-muted-foreground'}`,
      }, active ? t('manage.peerIdentities.graceActive') : t('manage.peerIdentities.graceExpired'))
    },
  },
  {
    accessorKey: 'description',
    header: t('manage.peerIdentities.colDescription'),
    cell: ({ row }) => h('span', { class: 'text-sm text-muted-foreground truncate max-w-[200px]' }, row.original.description || '-'),
  },
  {
    id: 'actions',
    header: '',
    cell: ({ row }) => h(DropdownMenu, null, {
      default: () => [
        h(DropdownMenuTrigger, { asChild: true }, () =>
          h(Button, { variant: 'ghost', size: 'sm', class: 'h-8 w-8 p-0' }, () =>
            h(MoreHorizontal, { class: 'h-4 w-4' }),
          ),
        ),
        h(DropdownMenuContent, { align: 'end' }, () => [
          h(DropdownMenuItem, { onClick: () => store.openDrawer('view', row.original) }, () => [
            h(Eye, { class: 'mr-2 h-4 w-4' }),
            t('manage.peerIdentities.actions.view'),
          ]),
          h(DropdownMenuItem, { onClick: () => store.openDrawer('edit', row.original) }, () => [
            h(Pencil, { class: 'mr-2 h-4 w-4' }),
            t('manage.peerIdentities.actions.edit'),
          ]),
          h(DropdownMenuSeparator),
          h(DropdownMenuItem, { class: 'text-destructive', onClick: () => store.openDeleteDialog(row.original) }, () => [
            h(Trash2, { class: 'mr-2 h-4 w-4' }),
            t('common.action.delete'),
          ]),
        ]),
      ],
    }),
  },
]

const table = useVueTable({
  get data() { return paged.value },
  columns,
  getCoreRowModel: getCoreRowModel(),
})

// ── Toast helpers ──────────────────────────────────────────────────
async function handleCreateOrUpdate() {
  const ok = await store.handleCreateOrUpdate()
  if (ok) {
    toast.success(store.drawerType === 'create' ? t('manage.peerIdentities.createSuccess') : t('manage.peerIdentities.editSuccess'))
  } else {
    toast.error(t('manage.peerIdentities.saveFailed'))
  }
}

async function handleDelete() {
  const ok = await store.handleDelete()
  if (ok) {
    toast.success(t('manage.peerIdentities.deleteSuccess'))
  } else {
    toast.error(t('manage.peerIdentities.deleteError'))
  }
}

onMounted(() => store.refresh())
</script>

<template>
  <div class="space-y-6">
    <!-- Stat cards -->
    <div class="grid grid-cols-2 sm:grid-cols-3 gap-4">
      <Card
        class="cursor-pointer hover:border-primary/50 transition-colors"
        :class="statusFilter === 'all' ? 'border-primary' : ''"
        @click="setStatusFilter('all')"
      >
        <CardContent class="p-4">
          <div class="text-2xl font-bold">{{ store.stats.total }}</div>
          <div class="text-xs text-muted-foreground">{{ t('manage.peerIdentities.statTotal') }}</div>
        </CardContent>
      </Card>
      <Card
        class="cursor-pointer hover:border-primary/50 transition-colors"
        :class="statusFilter === 'bound' ? 'border-primary' : ''"
        @click="setStatusFilter('bound')"
      >
        <CardContent class="p-4">
          <div class="text-2xl font-bold">{{ store.stats.bound }}</div>
          <div class="text-xs text-muted-foreground">{{ t('manage.peerIdentities.statBound') }}</div>
        </CardContent>
      </Card>
      <Card
        class="cursor-pointer hover:border-primary/50 transition-colors"
        :class="statusFilter === 'grace' ? 'border-primary' : ''"
        @click="setStatusFilter('grace')"
      >
        <CardContent class="p-4">
          <div class="text-2xl font-bold">{{ store.stats.gracePeriod }}</div>
          <div class="text-xs text-muted-foreground">{{ t('manage.peerIdentities.statGracePeriod') }}</div>
        </CardContent>
      </Card>
    </div>

    <!-- Toolbar -->
    <div class="flex items-center gap-3">
      <div class="relative flex-1 max-w-sm">
        <Search class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
        <Input
          v-model="searchValue"
          :placeholder="t('manage.peerIdentities.searchPlaceholder')"
          class="pl-9"
        />
      </div>
      <Button variant="outline" size="sm" @click="store.refresh()">
        <RefreshCw class="h-4 w-4 mr-1" />
        {{ t('common.action.refresh') }}
      </Button>
      <Button size="sm" @click="store.openDrawer('create')">
        <Plus class="h-4 w-4 mr-1" />
        {{ t('manage.peerIdentities.create') }}
      </Button>
    </div>

    <!-- Data table -->
    <div class="rounded-md border">
      <Table>
        <TableHeader>
          <TableRow v-for="headerGroup in table.getHeaderGroups()" :key="headerGroup.id">
            <TableHead v-for="header in headerGroup.headers" :key="header.id">
              <FlexRender v-if="!header.isPlaceholder" :render="header.column.columnDef.header" :props="header.getContext()" />
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <template v-if="paged.length">
            <TableRow v-for="row in table.getRowModel().rows" :key="row.id">
              <TableCell v-for="cell in row.getVisibleCells()" :key="cell.id">
                <FlexRender :render="cell.column.columnDef.cell" :props="cell.getContext()" />
              </TableCell>
            </TableRow>
          </template>
          <template v-else>
            <TableRow>
              <TableCell :colspan="columns.length" class="h-24 text-center text-muted-foreground">
                {{ t('manage.peerIdentities.empty') }}
              </TableCell>
            </TableRow>
          </template>
        </TableBody>
      </Table>
    </div>

    <!-- Pagination -->
    <div v-if="totalPages > 1" class="flex items-center justify-center gap-1">
      <Button variant="outline" size="sm" :disabled="currentPage <= 1" @click="goToPage(currentPage - 1)">
        <ChevronLeft class="h-4 w-4" />
      </Button>
      <Button
        v-for="p in visiblePages" :key="p"
        variant="outline" size="sm"
        :class="p === currentPage ? 'bg-primary text-primary-foreground' : ''"
        @click="goToPage(p)"
      >
        {{ p }}
      </Button>
      <Button variant="outline" size="sm" :disabled="currentPage >= totalPages" @click="goToPage(currentPage + 1)">
        <ChevronRight class="h-4 w-4" />
      </Button>
    </div>

    <!-- Delete confirmation -->
    <AppAlertDialog
      v-model:open="store.deleteDialogOpen"
      :title="t('manage.peerIdentities.deleteTitle')"
      :description="t('manage.peerIdentities.confirmDelete', { name: store.deleteTarget?.name ?? '' })"
      :confirm-text="t('common.action.delete')"
      :loading="store.loading"
      @confirm="handleDelete"
    />

    <!-- Create / Edit dialog -->
    <Dialog :open="store.isDrawerOpen" @update:open="v => { if (!v) store.isDrawerOpen = false }">
      <DialogContent class="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>
            {{ store.drawerType === 'create' ? t('manage.peerIdentities.createTitle') : store.drawerType === 'edit' ? t('manage.peerIdentities.editTitle') : t('manage.peerIdentities.viewTitle') }}
          </DialogTitle>
        </DialogHeader>

        <div class="space-y-4 py-2">
          <!-- Name -->
          <div class="space-y-2">
            <Label>{{ t('manage.peerIdentities.formName') }}</Label>
            <Input
              v-model="store.form.name"
              :placeholder="t('manage.peerIdentities.formNamePlaceholder')"
              :disabled="store.drawerType === 'view'"
            />
          </div>

          <!-- Peer Ref -->
          <div class="space-y-2">
            <Label>{{ t('manage.peerIdentities.formPeerRef') }}</Label>
            <Input
              v-model="store.form.peer_ref"
              :placeholder="t('manage.peerIdentities.formPeerRefPlaceholder')"
              :disabled="store.drawerType === 'view'"
            />
          </div>

          <!-- Previous Peer Ref -->
          <div class="space-y-2">
            <Label>{{ t('manage.peerIdentities.formPreviousPeerRef') }}</Label>
            <Input
              v-model="store.form.previous_peer_ref"
              :placeholder="t('manage.peerIdentities.formPreviousPeerRefPlaceholder')"
              :disabled="store.drawerType === 'view'"
            />
            <p class="text-xs text-muted-foreground">{{ t('manage.peerIdentities.formPreviousPeerRefHelp') }}</p>
          </div>

          <!-- Grace Period -->
          <div class="space-y-2">
            <Label>{{ t('manage.peerIdentities.formGracePeriod') }}</Label>
            <Input
              v-model.number="store.form.grace_period_seconds"
              type="number"
              min="0"
              :disabled="store.drawerType === 'view'"
            />
            <p class="text-xs text-muted-foreground">{{ t('manage.peerIdentities.formGracePeriodHelp') }}</p>
          </div>

          <!-- Description -->
          <div class="space-y-2">
            <Label>{{ t('manage.peerIdentities.formDescription') }}</Label>
            <Input
              v-model="store.form.description"
              :placeholder="t('manage.peerIdentities.formDescriptionPlaceholder')"
              :disabled="store.drawerType === 'view'"
            />
          </div>

          <!-- View-only: resolved info -->
          <template v-if="store.drawerType === 'view' && store.selectedIdentity">
            <Separator />
            <div class="grid grid-cols-2 gap-4 text-sm">
              <div>
                <div class="text-muted-foreground">{{ t('manage.peerIdentities.detailResolvedIP') }}</div>
                <div class="font-mono">{{ store.selectedIdentity.resolved_peer_ip || '-' }}</div>
              </div>
              <div>
                <div class="text-muted-foreground">{{ t('manage.peerIdentities.detailPreviousIP') }}</div>
                <div class="font-mono">{{ store.selectedIdentity.previous_peer_ip || '-' }}</div>
              </div>
              <div>
                <div class="text-muted-foreground">{{ t('manage.peerIdentities.detailCreatedAt') }}</div>
                <div>{{ store.selectedIdentity.created_at || '-' }}</div>
              </div>
              <div>
                <div class="text-muted-foreground">{{ t('manage.peerIdentities.detailGraceExpires') }}</div>
                <div>{{ store.selectedIdentity.grace_period_expires_at || '-' }}</div>
              </div>
            </div>
          </template>
        </div>

        <DialogFooter v-if="store.drawerType !== 'view'">
          <Button variant="outline" @click="store.isDrawerOpen = false">
            {{ t('common.action.cancel') }}
          </Button>
          <Button :disabled="store.loading" @click="handleCreateOrUpdate">
            {{ t('common.action.save') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
