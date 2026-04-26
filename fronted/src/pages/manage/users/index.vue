<script setup lang="ts">
import { ref, computed, onMounted, watch, h } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  useVueTable, getCoreRowModel,
  FlexRender, type ColumnDef,
} from '@tanstack/vue-table'
import {
  RefreshCw, Search, Shield, User as UserIcon,
  ChevronLeft, ChevronRight, Building2, MoreHorizontal, Users,
  Globe, Mail, Github, UserPlus, Clock,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem,
  DropdownMenuSeparator, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle,
  DialogDescription, DialogFooter,
} from '@/components/ui/dialog'
import { toast } from 'vue-sonner'
import { listUser, updateSystemRole, type UserVo } from '@/api/user'
import {
  addMemberToWorkspace,
  getUserWorkspaces,
  removeMemberFromWorkspace,
  updateMemberRoleInWorkspace,
} from '@/api/member'
import { useTable } from '@/composables/useApi'
import { useUserStore } from '@/stores/user'
import { useWorkspaceStore } from '@/stores/workspace'

const { t } = useI18n()

definePage({
  meta: { titleKey: 'manage.users.title', descKey: 'manage.users.desc' },
})

const userStore = useUserStore()
const workspaceStore = useWorkspaceStore()

const { rows, total, loading, refresh } = useTable(listUser)
const page     = ref(1)
const pageSize = ref(20)
const keyword  = ref('')

onMounted(() => {
  doRefresh()
  workspaceStore.fetchAll()
})

function doRefresh(p = 1) {
  page.value = p
  refresh({ page: page.value, pageSize: pageSize.value, keyword: keyword.value || undefined })
}

const totalPages   = computed(() => Math.max(1, Math.ceil((total.value || 0) / pageSize.value)))
const visiblePages = computed(() => {
  const cur = page.value, tp = totalPages.value
  const start = Math.max(1, Math.min(cur - 1, tp - 2))
  const end   = Math.min(tp, start + 2)
  return Array.from({ length: end - start + 1 }, (_, i) => start + i)
})

// ── Role filter ────────────────────────────────────────────────────
const roleFilter = ref<'all' | 'platform_admin' | 'user'>('all')

const filteredRows = computed<UserVo[]>(() => {
  const all = rows.value as UserVo[]
  if (roleFilter.value === 'all') return all
  if (roleFilter.value === 'platform_admin') return all.filter(u => u.role === 'platform_admin')
  return all.filter(u => u.role !== 'platform_admin')
})

// ── Stat card counts ───────────────────────────────────────────────
const statAdminCount = computed(() => (rows.value as UserVo[]).filter(u => u.role === 'platform_admin').length)
const statUserCount  = computed(() => (rows.value as UserVo[]).filter(u => u.role !== 'platform_admin').length)

// ── System role change ─────────────────────────────────────────────
const updating = ref<string | null>(null)

async function changeRole(user: UserVo, newRole: 'platform_admin' | 'user') {
  if (user.role === newRole) return
  updating.value = user.id
  try {
    await updateSystemRole(user.id, newRole)
    toast.success(t('manage.users.toast.roleUpdated', { name: user.name || user.email, role: roleLabel.value[newRole] }))
    doRefresh(page.value)
  } catch {
    toast.error(t('manage.users.toast.roleFailed'))
  } finally {
    updating.value = null
  }
}

// ── Workspace permissions dialog ───────────────────────────────────
const wsPermOpen    = ref(false)
const wsPermTarget  = ref<UserVo | null>(null)
const wsPermLoading = ref(false)
const wsPermSaving  = ref(false)

interface WsRow {
  workspaceId: string
  workspaceName: string
  checked: boolean
  role: string
  originalChecked: boolean
  originalRole: string | null
}
const wsRows = ref<WsRow[]>([])

const pendingChanges = computed(() => wsRows.value.filter(r => {
  const addNew     = r.checked && !r.originalChecked
  const remove     = !r.checked && r.originalChecked
  const roleChange = r.checked && r.originalChecked && r.role !== r.originalRole
  return addNew || remove || roleChange
}).length)

async function openWsPermissions(u: UserVo) {
  wsPermTarget.value  = u
  wsPermOpen.value    = true
  wsPermLoading.value = true
  wsRows.value        = []
  try {
    const { data: memberships } = await getUserWorkspaces(u.id)
    const memberMap = new Map<string, string>()
    for (const m of (memberships ?? [])) {
      memberMap.set(m.workspaceId, m.role)
    }
    wsRows.value = workspaceStore.allRows.map(ws => {
      const currentRole = memberMap.get(ws.id) ?? null
      return {
        workspaceId:     ws.id,
        workspaceName:   ws.displayName,
        checked:         currentRole !== null,
        role:            currentRole ?? 'member',
        originalChecked: currentRole !== null,
        originalRole:    currentRole,
      }
    })
  } catch {
    toast.error(t('manage.users.toast.wsLoadFailed'))
  } finally {
    wsPermLoading.value = false
  }
}

async function saveWsPermissions() {
  if (!wsPermTarget.value) return
  wsPermSaving.value = true
  const userID = wsPermTarget.value.id
  let successCount = 0
  let errorCount   = 0

  for (const row of wsRows.value) {
    const addNew     = row.checked && !row.originalChecked
    const remove     = !row.checked && row.originalChecked
    const roleChange = row.checked && row.originalChecked && row.role !== row.originalRole

    try {
      if (addNew) {
        await addMemberToWorkspace(row.workspaceId, userID, row.role)
        successCount++
      } else if (remove) {
        await removeMemberFromWorkspace(row.workspaceId, userID)
        successCount++
      } else if (roleChange) {
        await updateMemberRoleInWorkspace(row.workspaceId, userID, row.role)
        successCount++
      }
    } catch {
      errorCount++
    }
  }

  wsPermSaving.value = false
  if (successCount > 0) toast.success(t('manage.users.toast.wsSaved', { n: successCount }))
  if (errorCount > 0)   toast.error(t('manage.users.toast.wsFailed', { n: errorCount }))
  wsPermOpen.value = false
}

// ── Workspace membership cache (for the table column) ─────────────
interface WsMembership { workspaceId: string; workspaceName: string; role: string }
const userWsMap = ref<Map<string, WsMembership[]>>(new Map())

watch(rows, async (newRows) => {
  const users = (newRows as UserVo[]).filter(u => u.role !== 'platform_admin')
  await Promise.allSettled(users.map(async u => {
    if (userWsMap.value.has(u.id)) return
    try {
      const { data } = await getUserWorkspaces(u.id)
      userWsMap.value = new Map(userWsMap.value).set(u.id, data ?? [])
    } catch { /* ignore */ }
  }))
}, { immediate: false })

// ── Styles ────────────────────────────────────────────────────────
const roleLabel = computed<Record<string, string>>(() => ({
  platform_admin: t('common.role.platform_admin'),
  user: t('common.role.user'),
  '': t('common.role.user'),
}))
const roleStyle: Record<string, string> = {
  platform_admin: 'bg-primary/10 text-primary ring-1 ring-primary/20',
  user:           'bg-muted text-muted-foreground ring-1 ring-border',
  '':             'bg-muted text-muted-foreground ring-1 ring-border',
}

function avatarColor(name: string) {
  const colors = ['bg-blue-500','bg-violet-500','bg-emerald-500','bg-orange-500','bg-rose-500','bg-cyan-500']
  let hash = 0
  for (const c of (name ?? '')) hash = (hash * 31 + c.charCodeAt(0)) & 0xff
  return colors[hash % colors.length]
}

function initials(name: string) {
  const trimmed = (name ?? '').trim()
  if (!trimmed) return '?'
  const parts = trimmed.split(/\s+/)
  if (parts.length >= 2) return (parts[0][0] + parts[1][0]).toUpperCase()
  return trimmed.slice(0, 2).toUpperCase()
}

// ── Columns ───────────────────────────────────────────────────────
const columns = computed((): ColumnDef<UserVo>[] => [
  {
    id: 'user',
    header: () => t('manage.users.col.user'),
    cell: ({ row }) => {
      const u = row.original
      const displayName = u.name || u.email || '—'
      return h('div', { class: 'flex items-center gap-3' }, [
        u.avatar
          ? h('img', { src: u.avatar, class: 'size-10 rounded-full object-cover shrink-0 ring-2 ring-border' })
          : h('div', {
              class: `size-10 rounded-full flex items-center justify-center text-white text-sm font-bold shrink-0 ring-2 ring-white/10 ${avatarColor(displayName)}`,
            }, initials(displayName)),
        h('div', { class: 'flex flex-col gap-0.5 min-w-0' }, [
          h('span', { class: 'text-sm font-semibold leading-tight truncate' }, u.name || u.email || '—'),
          u.name && u.email
            ? h('span', { class: 'text-xs text-muted-foreground truncate' }, u.email)
            : null,
        ]),
      ])
    },
  },
  {
    id: 'role',
    header: () => t('manage.users.col.platformRole'),
    cell: ({ row }) => {
      const u = row.original
      const isAdmin = u.role === 'platform_admin'
      const label = roleLabel.value[u.role ?? ''] ?? t('common.role.user')
      const style = roleStyle[u.role ?? ''] ?? roleStyle['']
      return h('span', {
        class: `inline-flex items-center gap-1.5 text-xs font-semibold px-2.5 py-1 rounded-full ${style}`,
      }, [
        isAdmin
          ? h(Shield, { class: 'size-3.5 shrink-0' })
          : h(UserIcon, { class: 'size-3.5 shrink-0' }),
        label,
      ])
    },
  },
  {
    id: 'workspaces',
    header: () => t('manage.users.col.workspaces'),
    cell: ({ row }) => {
      const u = row.original
      if (u.role === 'platform_admin') {
        return h('span', {
          class: 'inline-flex items-center gap-1.5 text-xs font-semibold px-2.5 py-1 rounded-full bg-primary/10 text-primary ring-1 ring-primary/20',
        }, [
          h(Shield, { class: 'size-3 shrink-0' }),
          t('manage.users.allPermissions'),
        ])
      }
      const memberships = userWsMap.value.get(u.id)
      if (!memberships) {
        return h('span', { class: 'text-xs text-muted-foreground' }, '—')
      }
      if (memberships.length === 0) {
        return h('span', { class: 'text-xs text-muted-foreground' }, t('manage.users.noWorkspace'))
      }
      const wsRoleLabel: Record<string, string> = {
        admin:  t('common.role.admin'),
        editor: t('common.role.editor'),
        member: t('common.role.member'),
        viewer: t('common.role.viewer'),
      }
      const wsRoleStyle: Record<string, string> = {
        admin:  'bg-primary/10 text-primary ring-primary/20',
        editor: 'bg-blue-500/10 text-blue-600 ring-blue-500/20',
        member: 'bg-muted text-muted-foreground ring-border',
        viewer: 'bg-muted text-muted-foreground ring-border',
      }
      const MAX_SHOW = 2
      const shown = memberships.slice(0, MAX_SHOW)
      const rest  = memberships.length - MAX_SHOW
      return h('div', { class: 'flex flex-wrap gap-1' }, [
        ...shown.map(m =>
          h('span', {
            key: m.workspaceId,
            title: m.workspaceName,
            class: `text-[11px] font-semibold px-2 py-0.5 rounded-full ring-1 max-w-[120px] truncate inline-block ${wsRoleStyle[m.role] ?? wsRoleStyle.member}`,
          }, `${m.workspaceName} · ${wsRoleLabel[m.role] ?? m.role}`)
        ),
        ...(rest > 0 ? [
          h('span', {
            class: 'text-[11px] px-2 py-0.5 rounded-full bg-muted text-muted-foreground ring-1 ring-border',
          }, `+${rest}`)
        ] : []),
      ])
    },
  },
  {
    id: 'source',
    header: () => t('manage.users.col.source'),
    cell: ({ row }) => {
      const u = row.original
      const source = u.source ?? ''
      const sourceMap: Record<string, { label: string; icon: any; cls: string }> = {
        local:      { label: t('manage.users.source.local'),      icon: Mail,     cls: 'bg-muted text-muted-foreground ring-border' },
        invitation: { label: t('manage.users.source.invitation'), icon: UserPlus, cls: 'bg-blue-500/10 text-blue-600 ring-blue-500/20' },
        github:     { label: t('manage.users.source.github'),     icon: Github,   cls: 'bg-gray-500/10 text-gray-700 ring-gray-500/20 dark:text-gray-300' },
        dex:        { label: t('manage.users.source.dex'),        icon: Globe,    cls: 'bg-violet-500/10 text-violet-600 ring-violet-500/20' },
      }
      const info = sourceMap[source] ?? { label: source || '—', icon: Globe, cls: 'bg-muted text-muted-foreground ring-border' }
      return h('div', { class: 'flex flex-col gap-1' }, [
        h('span', {
          class: `inline-flex items-center gap-1 text-[11px] font-semibold px-2 py-0.5 rounded-full ring-1 w-fit ${info.cls}`,
        }, [h(info.icon, { class: 'size-3 shrink-0' }), info.label]),
        u.inviterName
          ? h('span', { class: 'text-[11px] text-muted-foreground pl-0.5' }, t('manage.users.inviter', { name: u.inviterName }))
          : null,
      ])
    },
  },
  {
    id: 'registeredAt',
    header: () => t('manage.users.col.registeredAt'),
    cell: ({ row }) => {
      const t = row.original.registeredAt
      if (!t) return h('span', { class: 'text-xs text-muted-foreground' }, '—')
      const d = new Date(t)
      const formatted = d.toLocaleString(undefined, { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
      return h('div', { class: 'flex items-center gap-1 text-xs text-muted-foreground whitespace-nowrap' }, [
        h(Clock, { class: 'size-3 shrink-0' }),
        h('span', {}, formatted),
      ])
    },
  },
  {
    id: 'actions',
    header: '',
    cell: ({ row }) => {
      const u = row.original
      if (!userStore.isPlatformAdmin) return h('span')

      const isSelf     = u.id === String(userStore.userInfo?.id)
      const isUpdating = updating.value === u.id

      return h(DropdownMenu, {}, {
        default: () => [
          h(DropdownMenuTrigger, { asChild: true }, () =>
            h(Button, {
              variant: 'ghost',
              size: 'sm',
              class: 'size-8 p-0',
              disabled: isUpdating,
            }, () =>
              isUpdating
                ? h('span', { class: 'size-4 animate-spin border-2 border-current border-t-transparent rounded-full' })
                : h(MoreHorizontal, { class: 'size-4' })
            )
          ),
          h(DropdownMenuContent, { align: 'end', class: 'w-48' }, () => [
            h(DropdownMenuItem, {
              onClick: () => openWsPermissions(u),
            }, () => [
              h(Building2, { class: 'size-3.5 mr-2 text-blue-500' }),
              t('manage.users.menu.manageWs'),
            ]),
            ...(!isSelf ? [
              h(DropdownMenuSeparator),
              h(DropdownMenuItem, {
                class: u.role === 'platform_admin' ? 'opacity-50 pointer-events-none' : '',
                onClick: () => changeRole(u, 'platform_admin'),
              }, () => [
                h(Shield, { class: 'size-3.5 mr-2 text-primary' }),
                t('manage.users.menu.setAdmin'),
              ]),
              h(DropdownMenuItem, {
                class: u.role !== 'platform_admin' ? 'opacity-50 pointer-events-none' : '',
                onClick: () => changeRole(u, 'user'),
              }, () => [
                h(UserIcon, { class: 'size-3.5 mr-2' }),
                t('manage.users.menu.setUser'),
              ]),
            ] : []),
          ]),
        ],
      })
    },
  },
])

const table = useVueTable({
  get data() { return filteredRows.value },
  get columns() { return columns.value },
  getCoreRowModel: getCoreRowModel(),
  manualPagination: true,
})
</script>

<template>
  <div class="flex flex-col gap-6 p-6 animate-in fade-in duration-300">

    <!-- ── Stat cards ─────────────────────────────────────────────── -->
    <div class="grid grid-cols-3 gap-4">
      <!-- Total users -->
      <div class="rounded-xl border bg-card p-4 flex items-center gap-4 shadow-sm">
        <div class="size-11 rounded-full bg-blue-500/10 flex items-center justify-center shrink-0">
          <Users class="size-5 text-blue-500" />
        </div>
        <div>
          <p class="text-2xl font-bold leading-none">{{ total ?? 0 }}</p>
          <p class="text-xs text-muted-foreground mt-1">{{ t('manage.users.totalUsers') }}</p>
        </div>
      </div>
      <!-- Platform admins -->
      <div class="rounded-xl border bg-card p-4 flex items-center gap-4 shadow-sm">
        <div class="size-11 rounded-full bg-primary/10 flex items-center justify-center shrink-0">
          <Shield class="size-5 text-primary" />
        </div>
        <div>
          <p class="text-2xl font-bold leading-none">{{ statAdminCount }}</p>
          <p class="text-xs text-muted-foreground mt-1">{{ t('manage.users.platformAdmins') }}</p>
        </div>
      </div>
      <!-- Regular users -->
      <div class="rounded-xl border bg-card p-4 flex items-center gap-4 shadow-sm">
        <div class="size-11 rounded-full bg-emerald-500/10 flex items-center justify-center shrink-0">
          <UserIcon class="size-5 text-emerald-500" />
        </div>
        <div>
          <p class="text-2xl font-bold leading-none">{{ statUserCount }}</p>
          <p class="text-xs text-muted-foreground mt-1">{{ t('manage.users.regularUsers') }}</p>
        </div>
      </div>
    </div>

    <!-- ── Toolbar: filter tabs + search + refresh ───────────────── -->
    <div class="flex items-center gap-3">
      <div class="flex items-center gap-1.5 p-1 rounded-lg bg-muted">
        <button
          v-for="tab in ([
            { value: 'all',            label: t('manage.users.filterAll') },
            { value: 'platform_admin', label: t('manage.users.filterAdmin') },
            { value: 'user',           label: t('manage.users.filterUser') },
          ] as const)"
          :key="tab.value"
          class="px-3.5 py-1.5 rounded-md text-sm font-medium transition-all"
          :class="roleFilter === tab.value
            ? 'bg-background text-foreground shadow-sm'
            : 'text-muted-foreground hover:text-foreground'"
          @click="roleFilter = tab.value"
        >
          {{ tab.label }}
        </button>
      </div>
      <div class="ml-auto flex items-center gap-2">
        <div class="relative w-56">
          <Search class="absolute left-2.5 top-1/2 -translate-y-1/2 size-4 text-muted-foreground pointer-events-none" />
          <Input
            v-model="keyword"
            :placeholder="t('manage.users.searchPlaceholder')"
            class="pl-8 h-9"
            @keyup.enter="doRefresh()"
          />
        </div>
        <Button variant="outline" size="sm" class="gap-1.5 h-9" :disabled="loading" @click="doRefresh()">
          <RefreshCw class="size-3.5" :class="loading ? 'animate-spin' : ''" />
          {{ t('common.action.refresh') }}
        </Button>
      </div>
    </div>

    <!-- ── Table ──────────────────────────────────────────────────── -->
    <div class="rounded-xl border overflow-hidden shadow-sm">
      <Table>
        <TableHeader>
          <TableRow class="bg-muted/30 hover:bg-muted/30" v-for="hg in table.getHeaderGroups()" :key="hg.id">
            <TableHead
              v-for="header in hg.headers"
              :key="header.id"
              class="text-xs font-semibold uppercase tracking-wide text-muted-foreground h-10"
            >
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
              class="hover:bg-muted/20 transition-colors"
            >
              <TableCell v-for="cell in row.getVisibleCells()" :key="cell.id" class="py-3">
                <FlexRender :render="cell.column.columnDef.cell" :props="cell.getContext()" />
              </TableCell>
            </TableRow>
          </template>
          <!-- Empty state -->
          <TableRow v-else>
            <TableCell :colspan="columns.length" class="h-48 text-center">
              <div class="flex flex-col items-center justify-center gap-3 text-muted-foreground py-8">
                <div class="size-14 rounded-full bg-muted flex items-center justify-center">
                  <Users class="size-6" />
                </div>
                <p class="text-sm font-medium">
                  {{ loading ? t('common.status.loading') : t('common.status.empty') }}
                </p>
              </div>
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>

    <!-- ── Pagination ─────────────────────────────────────────────── -->
    <div class="flex items-center justify-between text-sm text-muted-foreground">
      <span>{{ t('common.pagination.totalUsers', { total, page, totalPages }) }}</span>
      <div class="flex items-center gap-1">
        <Button variant="outline" size="sm" class="size-8 p-0"
          :disabled="page <= 1" @click="doRefresh(page - 1)">
          <ChevronLeft class="size-4" />
        </Button>
        <Button
          v-for="p in visiblePages" :key="p"
          variant="outline" size="sm" class="size-8 p-0 text-xs"
          :class="p === page ? 'bg-primary text-primary-foreground border-primary hover:bg-primary/90 hover:text-primary-foreground' : ''"
          @click="doRefresh(p)"
        >{{ p }}</Button>
        <Button variant="outline" size="sm" class="size-8 p-0"
          :disabled="page >= totalPages" @click="doRefresh(page + 1)">
          <ChevronRight class="size-4" />
        </Button>
      </div>
    </div>

  </div>

  <!-- ── Workspace Permissions Dialog ───────────────────────────── -->
  <Dialog v-model:open="wsPermOpen">
    <DialogContent class="sm:max-w-lg">
      <DialogHeader>
        <DialogTitle>{{ t('manage.users.wsDialog.title') }}</DialogTitle>
        <DialogDescription>
          {{ t('manage.users.wsDialog.desc', { name: wsPermTarget?.name || wsPermTarget?.email }) }}
        </DialogDescription>
      </DialogHeader>

      <div class="max-h-[55vh] overflow-y-auto -mx-6 px-6 space-y-1 py-1">
        <!-- Loading -->
        <div v-if="wsPermLoading" class="flex items-center justify-center h-32 text-muted-foreground text-sm">
          {{ t('common.status.loading') }}
        </div>
        <!-- Empty -->
        <div v-else-if="wsRows.length === 0" class="flex items-center justify-center h-32 text-muted-foreground text-sm">
          {{ t('manage.users.wsDialog.noWorkspaces') }}
        </div>
        <!-- Workspace rows -->
        <div
          v-else
          v-for="row in wsRows"
          :key="row.workspaceId"
          class="flex items-center gap-3 rounded-lg px-3 py-2.5 transition-colors"
          :class="row.checked ? 'bg-muted/40' : 'hover:bg-muted/20'"
        >
          <!-- Checkbox -->
          <input
            type="checkbox"
            :checked="row.checked"
            class="size-4 rounded border-border accent-primary cursor-pointer shrink-0"
            @change="row.checked = ($event.target as HTMLInputElement).checked"
          />

          <!-- Workspace name -->
          <div class="flex-1 min-w-0">
            <p class="text-sm font-medium truncate">{{ row.workspaceName }}</p>
            <p v-if="row.originalChecked && !row.checked" class="text-[11px] text-destructive">{{ t('manage.users.wsDialog.willRemove') }}</p>
            <p v-else-if="row.checked && !row.originalChecked" class="text-[11px] text-emerald-500">{{ t('manage.users.wsDialog.willAdd') }}</p>
            <p v-else-if="row.originalChecked && row.role !== row.originalRole" class="text-[11px] text-amber-500">{{ t('manage.users.wsDialog.roleChanged') }}</p>
          </div>

          <!-- Role selector -->
          <select
            v-model="row.role"
            :disabled="!row.checked"
            class="h-7 rounded-md border border-input bg-background px-2 text-xs focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:opacity-40 disabled:cursor-not-allowed w-28"
          >
            <option value="admin">{{ t('common.role.admin') }}</option>
            <option value="editor">{{ t('common.role.editor') }}</option>
            <option value="member">{{ t('common.role.member') }}</option>
            <option value="viewer">{{ t('common.role.viewer') }}</option>
          </select>

          <!-- Already member badge -->
          <span
            v-if="row.originalChecked"
            class="text-[10px] font-semibold px-1.5 py-0.5 rounded bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 ring-1 ring-emerald-500/20 shrink-0"
          >{{ t('manage.users.wsDialog.alreadyMember') }}</span>
          <span v-else class="w-[42px] shrink-0" />
        </div>
      </div>

      <DialogFooter>
        <Button variant="outline" @click="wsPermOpen = false">{{ t('common.action.cancel') }}</Button>
        <Button
          :disabled="wsPermSaving || pendingChanges === 0"
          @click="saveWsPermissions"
        >
          {{ wsPermSaving ? t('common.status.saving') : pendingChanges > 0 ? t('manage.users.wsDialog.saveCount', { n: pendingChanges }) : t('manage.users.wsDialog.noChanges') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
