<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import {
  useVueTable, getCoreRowModel, FlexRender, type ColumnDef,
} from '@tanstack/vue-table'
import {
  ShieldCheck, Search, RefreshCw, MoreHorizontal, Plus, Trash2, Pencil,
  Eye, ChevronLeft, ChevronRight, Fingerprint,
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
import { useAgentIdentityStore } from '@/stores/useAgentIdentityStore'
import type { AgentIdentity } from '@/api/agent-identity'

definePage({
  meta: { titleKey: 'manage.agentIdentities.title', descKey: 'manage.agentIdentities.desc' },
})

const { t } = useI18n()
const store = useAgentIdentityStore()

// ── Stats / filter ──────────────────────────────────────────────
type PhaseFilter = 'all' | 'active' | 'sandboxed'
const phaseFilter = ref<PhaseFilter>('all')
const searchValue = ref('')

function setPhaseFilter(val: PhaseFilter) {
  phaseFilter.value = val
  searchValue.value = ''
}

const filtered = computed(() => {
  const q = searchValue.value.toLowerCase().trim()
  return store.rows.filter(i => {
    const matchSearch = !q
      || i.name.toLowerCase().includes(q)
      || i.peer_ref.toLowerCase().includes(q)
      || (i.description ?? '').toLowerCase().includes(q)
    const matchPhase = phaseFilter.value === 'all'
      || (phaseFilter.value === 'active' && i.phase === 'Active')
      || (phaseFilter.value === 'sandboxed' && i.sandbox && i.sandbox !== 'none')
    return matchSearch && matchPhase
  })
})

// ── Pagination ─────────────────────────────────────────────────
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

// ── Columns ─────────────────────────────────────────────────────
const columns: ColumnDef<AgentIdentity, any>[] = [
  {
    accessorKey: 'name',
    header: t('manage.agentIdentities.colName'),
    cell: ({ row }) => row.original.name,
  },
  {
    accessorKey: 'peer_ref',
    header: t('manage.agentIdentities.colPeerRef'),
    cell: ({ row }) => row.original.peer_ref,
  },
  {
    accessorKey: 'sandbox',
    header: t('manage.agentIdentities.colSandbox'),
    cell: ({ row }) => row.original.sandbox,
  },
  {
    accessorKey: 'audit_level',
    header: t('manage.agentIdentities.colAuditLevel'),
    cell: ({ row }) => row.original.audit_level,
  },
  {
    accessorKey: 'enforcement_mode',
    header: t('manage.agentIdentities.colEnforcementMode'),
    cell: ({ row }) => row.original.enforcement_mode,
  },
  {
    accessorKey: 'phase',
    header: t('manage.agentIdentities.colPhase'),
    cell: ({ row }) => row.original.phase,
  },
  {
    id: 'actions',
    header: '',
    cell: ({ row }) => row.original,
  },
]

const table = useVueTable({
  get data() { return paged.value },
  columns,
  getCoreRowModel: getCoreRowModel(),
})

// ── Helpers ─────────────────────────────────────────────────────
function parseJSON(s?: string): string[] {
  if (!s) return []
  try { return JSON.parse(s) } catch { return [] }
}

function phaseColor(phase: string): string {
  if (phase === 'Active') return 'text-emerald-600 bg-emerald-50 dark:bg-emerald-950/30'
  if (phase === 'Expired' || phase === 'Suspended') return 'text-red-600 bg-red-50 dark:bg-red-950/30'
  return 'text-yellow-600 bg-yellow-50 dark:bg-yellow-950/30'
}

function sandboxColor(sandbox: string): string {
  if (!sandbox || sandbox === 'none') return 'text-muted-foreground bg-muted'
  return 'text-violet-600 bg-violet-50 dark:bg-violet-950/30'
}

// ── Tag input helpers ──────────────────────────────────────────
const newTool = ref('')
const newNamespace = ref('')
const newRole = ref('')
function addTag(arr: string[] | undefined, val: string): string[] {
  const list = arr ?? []
  const trimmed = val.trim()
  if (trimmed && !list.includes(trimmed)) return [...list, trimmed]
  return list
}
function removeTag(arr: string[] | undefined, idx: number): string[] {
  return (arr ?? []).filter((_, i) => i !== idx)
}

// ── Actions ─────────────────────────────────────────────────────
async function handleCreateOrUpdate() {
  const ok = await store.handleCreateOrUpdate()
  if (ok) {
    toast.success(t(store.drawerType === 'create' ? 'manage.agentIdentities.createSuccess' : 'manage.agentIdentities.editSuccess'))
  } else {
    toast.error(t('manage.agentIdentities.saveFailed'))
  }
}

async function handleDelete() {
  const ok = await store.handleDelete()
  if (ok) toast.success(t('manage.agentIdentities.deleteSuccess'))
  else toast.error(t('manage.agentIdentities.deleteError'))
}

onMounted(() => store.refresh())
</script>

<template>
  <div class="flex flex-col gap-6">
    <!-- Header -->
    <div class="flex flex-col gap-1">
      <h1 class="text-2xl font-bold tracking-tight">{{ t('manage.agentIdentities.title') }}</h1>
      <p class="text-muted-foreground text-sm">{{ t('manage.agentIdentities.desc') }}</p>
    </div>

    <!-- Stat cards -->
    <div class="grid grid-cols-3 gap-4">
      <Card class="cursor-pointer" :class="phaseFilter === 'all' && 'ring-2 ring-primary'" @click="setPhaseFilter('all')">
        <CardContent class="flex items-center gap-3 p-4">
          <div class="p-2 rounded-lg bg-muted">
            <Fingerprint class="size-5 text-muted-foreground" />
          </div>
          <div>
            <div class="text-2xl font-bold">{{ store.stats.total }}</div>
            <div class="text-xs text-muted-foreground">{{ t('manage.agentIdentities.statTotal') }}</div>
          </div>
        </CardContent>
      </Card>
      <Card class="cursor-pointer" :class="phaseFilter === 'active' && 'ring-2 ring-primary'" @click="setPhaseFilter('active')">
        <CardContent class="flex items-center gap-3 p-4">
          <div class="p-2 rounded-lg bg-emerald-50 dark:bg-emerald-950/30">
            <ShieldCheck class="size-5 text-emerald-600" />
          </div>
          <div>
            <div class="text-2xl font-bold">{{ store.stats.active }}</div>
            <div class="text-xs text-muted-foreground">{{ t('manage.agentIdentities.statActive') }}</div>
          </div>
        </CardContent>
      </Card>
      <Card class="cursor-pointer" :class="phaseFilter === 'sandboxed' && 'ring-2 ring-primary'" @click="setPhaseFilter('sandboxed')">
        <CardContent class="flex items-center gap-3 p-4">
          <div class="p-2 rounded-lg bg-violet-50 dark:bg-violet-950/30">
            <ShieldCheck class="size-5 text-violet-600" />
          </div>
          <div>
            <div class="text-2xl font-bold">{{ store.stats.sandboxed }}</div>
            <div class="text-xs text-muted-foreground">{{ t('manage.agentIdentities.statSandboxed') }}</div>
          </div>
        </CardContent>
      </Card>
    </div>

    <!-- Toolbar -->
    <div class="flex items-center gap-2">
      <div class="relative flex-1 max-w-sm">
        <Search class="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-muted-foreground" />
        <Input v-model="searchValue" :placeholder="t('manage.agentIdentities.searchPlaceholder')" class="pl-9" />
      </div>
      <Button variant="outline" size="icon" @click="store.refresh()">
        <RefreshCw class="size-4" />
      </Button>
      <Button class="ml-auto" @click="store.openDrawer('create')">
        <Plus class="size-4 mr-1" />
        {{ t('manage.agentIdentities.create') }}
      </Button>
    </div>

    <!-- Table -->
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
          <template v-if="table.getRowModel().rows.length">
            <TableRow v-for="row in table.getRowModel().rows" :key="row.id">
              <TableCell v-for="cell in row.getVisibleCells()" :key="cell.id">
                <template v-if="cell.column.id === 'name'">
                  <span class="font-medium">{{ cell.getValue() }}</span>
                </template>
                <template v-else-if="cell.column.id === 'peer_ref'">
                  <span class="font-mono text-xs">{{ cell.getValue() }}</span>
                </template>
                <template v-else-if="cell.column.id === 'sandbox'">
                  <span class="inline-block px-2 py-0.5 rounded-full text-xs font-medium" :class="sandboxColor(cell.getValue() as string)">
                    {{ cell.getValue() }}
                  </span>
                </template>
                <template v-else-if="cell.column.id === 'phase'">
                  <span class="inline-block px-2 py-0.5 rounded-full text-xs font-medium" :class="phaseColor(cell.getValue() as string)">
                    {{ cell.getValue() }}
                  </span>
                </template>
                <template v-else-if="cell.column.id === 'actions'">
                  <DropdownMenu>
                    <DropdownMenuTrigger as-child>
                      <Button variant="ghost" size="icon" class="size-8">
                        <MoreHorizontal class="size-4" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end" class="w-40">
                      <DropdownMenuItem @click="store.openDrawer('view', cell.getValue() as AgentIdentity)">
                        <Eye class="size-4 mr-2" />
                        {{ t('manage.agentIdentities.actions.view') }}
                      </DropdownMenuItem>
                      <DropdownMenuItem @click="store.openDrawer('edit', cell.getValue() as AgentIdentity)">
                        <Pencil class="size-4 mr-2" />
                        {{ t('manage.agentIdentities.actions.edit') }}
                      </DropdownMenuItem>
                      <DropdownMenuSeparator />
                      <DropdownMenuItem class="text-destructive focus:text-destructive" @click="store.openDeleteDialog(cell.getValue() as AgentIdentity)">
                        <Trash2 class="size-4 mr-2" />
                        {{ t('common.action.delete') }}
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </template>
                <template v-else>
                  <FlexRender :render="cell.column.columnDef.cell" :props="cell.getContext()" />
                </template>
              </TableCell>
            </TableRow>
          </template>
          <template v-else>
            <TableRow>
              <TableCell :colspan="columns.length" class="h-32 text-center text-muted-foreground">
                {{ t('manage.agentIdentities.empty') }}
              </TableCell>
            </TableRow>
          </template>
        </TableBody>
      </Table>
    </div>

    <!-- Pagination -->
    <div v-if="totalPages > 1" class="flex items-center justify-center gap-1">
      <Button variant="outline" size="icon" class="size-8" :disabled="currentPage === 1" @click="goToPage(currentPage - 1)">
        <ChevronLeft class="size-4" />
      </Button>
      <Button
        v-for="p in visiblePages" :key="p"
        variant="outline" size="icon" class="size-8"
        :class="p === currentPage && 'bg-primary text-primary-foreground'"
        @click="goToPage(p)"
      >
        {{ p }}
      </Button>
      <Button variant="outline" size="icon" class="size-8" :disabled="currentPage === totalPages" @click="goToPage(currentPage + 1)">
        <ChevronRight class="size-4" />
      </Button>
    </div>

    <!-- Delete confirmation -->
    <AppAlertDialog
      v-model:open="store.deleteDialogOpen"
      :title="t('manage.agentIdentities.deleteTitle')"
      :description="t('manage.agentIdentities.confirmDelete', { name: store.deleteTarget?.name ?? '' })"
      :confirm-text="t('common.action.delete')"
      :loading="store.loading"
      @confirm="handleDelete"
    />

    <!-- Create / Edit / View dialog -->
    <Dialog :open="store.isDrawerOpen" @update:open="v => { if (!v) store.isDrawerOpen = false }">
      <DialogContent class="sm:max-w-lg max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>
            {{ store.drawerType === 'create' ? t('manage.agentIdentities.createTitle') : store.drawerType === 'edit' ? t('manage.agentIdentities.editTitle') : t('manage.agentIdentities.viewTitle') }}
          </DialogTitle>
        </DialogHeader>

        <div class="space-y-4 py-2">
          <!-- Name -->
          <div class="space-y-2">
            <Label>{{ t('manage.agentIdentities.formName') }}</Label>
            <Input
              v-model="store.form.name"
              :placeholder="t('manage.agentIdentities.formNamePlaceholder')"
              :disabled="store.drawerType === 'view'"
            />
          </div>

          <!-- Peer Ref -->
          <div class="space-y-2">
            <Label>{{ t('manage.agentIdentities.formPeerRef') }}</Label>
            <Input
              v-model="store.form.peer_ref"
              :placeholder="t('manage.agentIdentities.formPeerRefPlaceholder')"
              :disabled="store.drawerType === 'view'"
            />
          </div>

          <!-- Sandbox -->
          <div class="space-y-2">
            <Label>{{ t('manage.agentIdentities.formSandbox') }}</Label>
            <select
              v-model="store.form.sandbox"
              :disabled="store.drawerType === 'view'"
              class="w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus:outline-none focus:ring-2 focus:ring-ring"
            >
              <option value="none">none</option>
              <option value="eBPF">eBPF</option>
              <option value="gVisor">gVisor</option>
            </select>
          </div>

          <!-- Audit Level -->
          <div class="space-y-2">
            <Label>{{ t('manage.agentIdentities.formAuditLevel') }}</Label>
            <select
              v-model="store.form.audit_level"
              :disabled="store.drawerType === 'view'"
              class="w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus:outline-none focus:ring-2 focus:ring-ring"
            >
              <option value="none">none</option>
              <option value="read">read</option>
              <option value="write">write</option>
              <option value="all">all</option>
            </select>
          </div>

          <!-- Enforcement Mode -->
          <div class="space-y-2">
            <Label>{{ t('manage.agentIdentities.formEnforcementMode') }}</Label>
            <select
              v-model="store.form.enforcement_mode"
              :disabled="store.drawerType === 'view'"
              class="w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus:outline-none focus:ring-2 focus:ring-ring"
            >
              <option value="monitor">monitor</option>
              <option value="enforce">enforce</option>
            </select>
          </div>

          <!-- Parent Ref -->
          <div class="space-y-2">
            <Label>{{ t('manage.agentIdentities.formParentRef') }}</Label>
            <Input
              v-model="store.form.parent_ref"
              :placeholder="t('manage.agentIdentities.formParentRefPlaceholder')"
              :disabled="store.drawerType === 'view'"
            />
          </div>

          <!-- Allowed Tools -->
          <div class="space-y-2">
            <Label>{{ t('manage.agentIdentities.formAllowedTools') }}</Label>
            <div class="flex gap-1">
              <Input v-model="newTool" placeholder="tool-name" class="flex-1"
                :disabled="store.drawerType === 'view'"
                @keyup.enter="store.form.allowed_tools = addTag(store.form.allowed_tools, newTool); newTool = ''" />
              <Button v-if="store.drawerType !== 'view'" variant="outline" size="sm"
                @click="store.form.allowed_tools = addTag(store.form.allowed_tools, newTool); newTool = ''">
                <Plus class="size-3" />
              </Button>
            </div>
            <div class="flex flex-wrap gap-1">
              <span
                v-for="(tool, i) in store.form.allowed_tools" :key="i"
                class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-secondary cursor-pointer hover:bg-destructive/20"
                @click="store.drawerType !== 'view' && (store.form.allowed_tools = removeTag(store.form.allowed_tools, i))"
              >
                {{ tool }}
                <span v-if="store.drawerType !== 'view'" class="text-muted-foreground">&times;</span>
              </span>
            </div>
          </div>

          <!-- Allowed Namespaces -->
          <div class="space-y-2">
            <Label>{{ t('manage.agentIdentities.formAllowedNamespaces') }}</Label>
            <div class="flex gap-1">
              <Input v-model="newNamespace" placeholder="namespace" class="flex-1"
                :disabled="store.drawerType === 'view'"
                @keyup.enter="store.form.allowed_namespaces = addTag(store.form.allowed_namespaces, newNamespace); newNamespace = ''" />
              <Button v-if="store.drawerType !== 'view'" variant="outline" size="sm"
                @click="store.form.allowed_namespaces = addTag(store.form.allowed_namespaces, newNamespace); newNamespace = ''">
                <Plus class="size-3" />
              </Button>
            </div>
            <div class="flex flex-wrap gap-1">
              <span
                v-for="(ns, i) in store.form.allowed_namespaces" :key="i"
                class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-secondary cursor-pointer hover:bg-destructive/20"
                @click="store.drawerType !== 'view' && (store.form.allowed_namespaces = removeTag(store.form.allowed_namespaces, i))"
              >
                {{ ns }}
                <span v-if="store.drawerType !== 'view'" class="text-muted-foreground">&times;</span>
              </span>
            </div>
          </div>

          <!-- Spawnable Roles -->
          <div class="space-y-2">
            <Label>{{ t('manage.agentIdentities.formSpawnableRoles') }}</Label>
            <div class="flex gap-1">
              <Input v-model="newRole" placeholder="role-name" class="flex-1"
                :disabled="store.drawerType === 'view'"
                @keyup.enter="store.form.spawnable_roles = addTag(store.form.spawnable_roles, newRole); newRole = ''" />
              <Button v-if="store.drawerType !== 'view'" variant="outline" size="sm"
                @click="store.form.spawnable_roles = addTag(store.form.spawnable_roles, newRole); newRole = ''">
                <Plus class="size-3" />
              </Button>
            </div>
            <div class="flex flex-wrap gap-1">
              <span
                v-for="(role, i) in store.form.spawnable_roles" :key="i"
                class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-secondary cursor-pointer hover:bg-destructive/20"
                @click="store.drawerType !== 'view' && (store.form.spawnable_roles = removeTag(store.form.spawnable_roles, i))"
              >
                {{ role }}
                <span v-if="store.drawerType !== 'view'" class="text-muted-foreground">&times;</span>
              </span>
            </div>
          </div>

          <!-- Description -->
          <div class="space-y-2">
            <Label>{{ t('manage.agentIdentities.formDescription') }}</Label>
            <Input
              v-model="store.form.description"
              :placeholder="t('manage.agentIdentities.formDescriptionPlaceholder')"
              :disabled="store.drawerType === 'view'"
            />
          </div>

          <!-- View-only: resolved info -->
          <template v-if="store.drawerType === 'view' && store.selectedIdentity">
            <Separator />
            <div class="grid grid-cols-2 gap-4 text-sm">
              <div>
                <div class="text-muted-foreground">{{ t('manage.agentIdentities.detailPeerIP') }}</div>
                <div class="font-mono">{{ store.selectedIdentity.peer_ip || '-' }}</div>
              </div>
              <div>
                <div class="text-muted-foreground">{{ t('manage.agentIdentities.detailParentRef') }}</div>
                <div>{{ store.selectedIdentity.parent_ref || '-' }}</div>
              </div>
              <div>
                <div class="text-muted-foreground">{{ t('manage.agentIdentities.detailAllowedTools') }}</div>
                <div class="flex flex-wrap gap-1 mt-1">
                  <span v-for="(tool, i) in parseJSON(store.selectedIdentity.allowed_tools)" :key="i"
                    class="inline-block px-2 py-0.5 rounded-full text-xs font-medium bg-secondary">
                    {{ tool }}
                  </span>
                  <span v-if="!parseJSON(store.selectedIdentity.allowed_tools).length" class="text-muted-foreground">-</span>
                </div>
              </div>
              <div>
                <div class="text-muted-foreground">{{ t('manage.agentIdentities.detailAllowedNamespaces') }}</div>
                <div class="flex flex-wrap gap-1 mt-1">
                  <span v-for="(ns, i) in parseJSON(store.selectedIdentity.allowed_namespaces)" :key="i"
                    class="inline-block px-2 py-0.5 rounded-full text-xs font-medium bg-secondary">
                    {{ ns }}
                  </span>
                  <span v-if="!parseJSON(store.selectedIdentity.allowed_namespaces).length" class="text-muted-foreground">-</span>
                </div>
              </div>
              <div>
                <div class="text-muted-foreground">{{ t('manage.agentIdentities.detailSpawnableRoles') }}</div>
                <div class="flex flex-wrap gap-1 mt-1">
                  <span v-for="(role, i) in parseJSON(store.selectedIdentity.spawnable_roles)" :key="i"
                    class="inline-block px-2 py-0.5 rounded-full text-xs font-medium bg-secondary">
                    {{ role }}
                  </span>
                  <span v-if="!parseJSON(store.selectedIdentity.spawnable_roles).length" class="text-muted-foreground">-</span>
                </div>
              </div>
              <div>
                <div class="text-muted-foreground">{{ t('manage.agentIdentities.detailLastSeenAt') }}</div>
                <div>{{ store.selectedIdentity.last_seen_at || '-' }}</div>
              </div>
              <div>
                <div class="text-muted-foreground">{{ t('manage.agentIdentities.detailCreatedAt') }}</div>
                <div>{{ store.selectedIdentity.created_at || '-' }}</div>
              </div>
            </div>
          </template>
        </div>

        <DialogFooter v-if="store.drawerType !== 'view'">
          <Button variant="outline" @click="store.isDrawerOpen = false">
            {{ t('common.action.cancel') }}
          </Button>
          <Button :disabled="store.loading || !store.form.name?.trim() || !store.form.peer_ref?.trim()" @click="handleCreateOrUpdate">
            <ShieldCheck class="size-4 mr-1" />
            {{ t('common.action.save') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
