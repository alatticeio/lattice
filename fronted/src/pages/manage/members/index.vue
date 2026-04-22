<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import {
  useVueTable, getCoreRowModel, FlexRender, type ColumnDef,
} from '@tanstack/vue-table'
import {
  Users, Plus, RefreshCw, MoreHorizontal, Pencil,
  Trash2, ChevronLeft, ChevronRight, Search,
  Shield, UserCheck, Clock, Key, Server,
  CheckCircle2, AlertCircle,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle,
  DialogDescription, DialogFooter,
} from '@/components/ui/dialog'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem,
  DropdownMenuSeparator, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import AppAlertDialog from '@/components/AlertDialog.vue'
import { add, listUser, deleteUser } from '@/api/user'
import { listWs } from '@/api/workspace'
import { useTable, useAction } from '@/composables/useApi'

definePage({
  meta: { title: '用户管理', description: '管理平台成员与 RBAC 权限。' },
})

// ── API ───────────────────────────────────────────────────────────
const { rows: members, total, loading, params, refresh } = useTable(listUser)
const { rows: workspaces } = useTable(listWs)
const { loading: addLoading, execute: runAdd } = useAction(add, {
  successMsg: '成员添加成功',
  onSuccess: () => { dialogOpen.value = false; refresh() },
})

onMounted(() => refresh())

// ── Style helpers ─────────────────────────────────────────────────
const roleStyle: Record<string, string> = {
  admin:  'bg-primary/10 text-primary ring-1 ring-primary/20',
  editor: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 ring-1 ring-emerald-500/20',
  viewer: 'bg-muted text-muted-foreground ring-1 ring-border',
}
const roleLabel: Record<string, string> = {
  admin: '管理员', editor: '编辑者', viewer: '访客',
}
const providerStyle: Record<string, string> = {
  local: 'bg-blue-500/10 text-blue-600 dark:text-blue-400 ring-1 ring-blue-500/20',
  dex:   'bg-violet-500/10 text-violet-600 dark:text-violet-400 ring-1 ring-violet-500/20',
}
const statusStyle: Record<string, string> = {
  active:  'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 ring-1 ring-emerald-500/20',
  pending: 'bg-amber-400/10 text-amber-600 dark:text-amber-400 ring-1 ring-amber-400/20',
}

const avatarColors = [
  'bg-blue-500', 'bg-violet-500', 'bg-emerald-500',
  'bg-orange-500', 'bg-rose-500', 'bg-cyan-500', 'bg-indigo-500',
]
function avatarColor(name: string) {
  let hash = 0
  for (const c of (name ?? '')) hash = (hash * 31 + c.charCodeAt(0)) & 0xff
  return avatarColors[hash % avatarColors.length]
}
function firstChar(name: string) {
  return name?.trim().charAt(0).toUpperCase() ?? '?'
}
function nsBadgeStyle(name: string) {
  const hues = [210, 160, 260, 40, 0, 190, 230]
  let hash = 0
  for (const c of (name ?? '')) hash = (hash * 31 + c.charCodeAt(0)) & 0xff
  const hue = hues[hash % hues.length]
  return {
    backgroundColor: `hsla(${hue}, 70%, 50%, 0.12)`,
    color: `hsla(${hue}, 80%, 60%, 1)`,
    outline: `1px solid hsla(${hue}, 70%, 50%, 0.2)`,
  }
}

// ── Stats ─────────────────────────────────────────────────────────
type StatusFilter = 'all' | 'active' | 'pending'
const statusFilter = ref<StatusFilter>('all')
const searchValue  = ref('')

const stats = computed(() => {
  const all     = members.value as any[]
  const tot     = total.value || all.length
  const admin   = all.filter(m => m.role === 'admin').length
  const editor  = all.filter(m => m.role === 'editor').length
  const viewer  = all.filter(m => !m.role || m.role === 'viewer').length
  const active  = all.filter(m => m.status === 'active').length
  const pending = all.filter(m => m.status !== 'active').length
  return {
    total: tot,
    admin, editor, viewer, active, pending,
    activeRate:  tot ? Math.round((active  / tot) * 100) : 0,
    adminRate:   tot ? Math.round((admin   / tot) * 100) : 0,
    // first 4 member names for avatar stack
    recentNames: all.slice(0, 4).map(m => m.name ?? '?'),
  }
})

// ── Pagination (server-side) ───────────────────────────────────────
const PAGE_SIZE   = params.pageSize ?? 10
const currentPage = computed(() => params.page ?? 1)
const totalPages  = computed(() => Math.max(1, Math.ceil(total.value / PAGE_SIZE)))
const visiblePages = computed(() => {
  const cur   = currentPage.value
  const tot   = totalPages.value
  const start = Math.max(1, Math.min(cur - 1, tot - 2))
  const end   = Math.min(tot, start + 2)
  return Array.from({ length: end - start + 1 }, (_, i) => start + i)
})

function goToPage(p: number) {
  if (p < 1 || p > totalPages.value) return
  params.page = p
  refresh()
}

let searchTimer: ReturnType<typeof setTimeout>
function onSearchInput() {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => { params.page = 1; refresh() }, 400)
}

function setStatusFilter(val: StatusFilter) {
  statusFilter.value = val
  searchValue.value = ''
}

// ── Delete ─────────────────────────────────────────────────────────
const deleteTarget     = ref<any>(null)
const deleteDialogOpen = ref(false)

function promptDelete(m: any) {
  deleteTarget.value = m
  deleteDialogOpen.value = true
}
async function confirmDelete() {
  if (deleteTarget.value) { await deleteUser(deleteTarget.value.id); refresh() }
  deleteTarget.value = null
}

// ── Create / Edit dialog ───────────────────────────────────────────
const dialogOpen     = ref(false)
const dialogType     = ref<'invite' | 'config'>('invite')
const selectedMember = ref<any>(null)
const form = ref({
  username: '', password: '', role: 'viewer', namespace: '',
  provider: 'local' as 'local' | 'dex',
})

function openInvite() {
  dialogType.value = 'invite'
  form.value = { username: '', password: '', role: 'viewer', namespace: '', provider: 'local' }
  dialogOpen.value = true
}
function openConfig(m: any) {
  selectedMember.value = JSON.parse(JSON.stringify(m))
  dialogType.value = 'config'
  dialogOpen.value = true
}

// ── Client-side filter (over loaded page) ─────────────────────────
const filteredRows = computed(() => {
  const q = searchValue.value.toLowerCase().trim()
  return members.value.filter((m: any) => {
    const matchSearch = !q || m.name?.toLowerCase().includes(q) || m.email?.toLowerCase().includes(q)
    const matchStatus = statusFilter.value === 'all' || m.status === statusFilter.value
    return matchSearch && matchStatus
  })
})

// ── Column definitions ────────────────────────────────────────────
type MemberRow = (typeof members.value)[number]

const columns: ColumnDef<MemberRow>[] = [
  {
    id: 'member',
    header: '成员',
    cell: ({ row }) => {
      const m = row.original as any
      return h('div', { class: 'flex items-center gap-3' }, [
        h('div', {
          class: `size-9 rounded-xl flex items-center justify-center text-white text-xs font-black shrink-0 ${avatarColor(m.name)}`,
        }, firstChar(m.name)),
        h('div', { class: 'min-w-0' }, [
          h('p', { class: 'font-semibold text-sm leading-none' }, m.name),
          h('p', { class: 'font-mono text-[11px] text-muted-foreground/60 mt-1 truncate max-w-40' }, m.email),
        ]),
      ])
    },
  },
  {
    accessorKey: 'role',
    header: '角色',
    cell: ({ row }) => {
      const role: string = (row.original as any).role ?? 'viewer'
      const icon = role === 'admin' ? Shield : role === 'editor' ? UserCheck : Users
      return h('span', {
        class: `text-[11px] font-bold px-2.5 py-1 rounded-full flex items-center gap-1.5 w-fit ${roleStyle[role] ?? roleStyle.viewer}`,
      }, [
        h(icon, { class: 'size-3' }),
        roleLabel[role] ?? role,
      ])
    },
  },
  {
    accessorKey: 'status',
    header: '状态',
    cell: ({ row }) => {
      const status: string = (row.original as any).status ?? 'pending'
      const active = status === 'active'
      return h('div', { class: 'flex items-center gap-1.5' }, [
        h('span', { class: 'relative flex size-1.5 shrink-0' }, [
          active && h('span', { class: 'absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-500 opacity-60' }),
          h('span', { class: `relative inline-flex size-1.5 rounded-full ${active ? 'bg-emerald-500' : 'bg-amber-400'}` }),
        ]),
        h('span', {
          class: `text-xs font-medium px-2 py-0.5 rounded-full ${statusStyle[status] ?? statusStyle.pending}`,
        }, active ? '活跃' : '待激活'),
      ])
    },
  },
  {
    accessorKey: 'provider',
    header: 'Provider',
    cell: ({ row }) => {
      const provider: string = (row.original as any).provider ?? 'local'
      return h('span', {
        class: `text-[10px] font-bold px-2 py-0.5 rounded uppercase tracking-wider ${providerStyle[provider] ?? providerStyle.local}`,
      }, provider)
    },
  },
  {
    id: 'bindings',
    header: 'Namespace Bindings',
    cell: ({ row }) => {
      const bindings: any[] = (row.original as any).bindings ?? []
      if (!bindings.length) return h('span', { class: 'text-[11px] text-muted-foreground/40 italic' }, '未分配')
      const chips = bindings.slice(0, 3).map((b: any) =>
        h('span', {
          class: 'text-[10px] font-bold px-1.5 py-0.5 rounded-md',
          style: nsBadgeStyle(b.ns),
        }, b.ns)
      )
      if (bindings.length > 3) chips.push(
        h('span', { class: 'text-[10px] text-muted-foreground/60 px-1' }, `+${bindings.length - 3}`)
      )
      return h('div', { class: 'flex flex-wrap gap-1' }, chips)
    },
  },
  {
    id: 'spaces',
    header: '空间数',
    cell: ({ row }) => {
      const count = ((row.original as any).bindings ?? []).length
      return h('div', { class: 'flex items-center gap-1.5 text-xs text-muted-foreground' }, [
        h(Server, { class: 'size-3 shrink-0' }),
        h('span', { class: 'tabular-nums font-medium' }, String(count)),
      ])
    },
  },
  {
    accessorKey: 'lastActive',
    header: '最后活跃',
    cell: ({ row }) => {
      const t = (row.original as any).lastActive
      return h('div', { class: 'flex items-center gap-1.5 text-xs text-muted-foreground' }, [
        h(Clock, { class: 'size-3 shrink-0' }),
        t ?? '从未登录',
      ])
    },
  },
  {
    id: 'actions',
    header: '',
    cell: ({ row }) => {
      const m = row.original
      return h(DropdownMenu, {}, {
        default: () => [
          h(DropdownMenuTrigger, { asChild: true }, () =>
            h(Button, { variant: 'ghost', size: 'sm', class: 'size-8 p-0' }, () =>
              h(MoreHorizontal, { class: 'size-4' })
            )
          ),
          h(DropdownMenuContent, { align: 'end', class: 'w-36' }, () => [
            h(DropdownMenuItem, { onClick: () => openConfig(m) }, () => [
              h(Pencil, { class: 'mr-2 size-3.5' }), '编辑权限',
            ]),
            h(DropdownMenuSeparator),
            h(DropdownMenuItem, {
              class: 'text-destructive focus:text-destructive',
              onClick: () => promptDelete(m),
            }, () => [h(Trash2, { class: 'mr-2 size-3.5' }), '移除']),
          ]),
        ],
      })
    },
  },
]

// ── TanStack Table ────────────────────────────────────────────────
const table = useVueTable({
  get data() { return filteredRows.value },
  columns,
  getCoreRowModel: getCoreRowModel(),
  manualPagination: true,
  manualFiltering: true,
  get rowCount() { return total.value },
})
</script>

<template>
  <div class="flex flex-col gap-5 p-6 animate-in fade-in duration-300">

    <!-- ── Stat cards ─────────────────────────────────────────────── -->
    <div class="grid grid-cols-2 sm:grid-cols-4 gap-3">

      <!-- 全部成员 -->
      <button
        class="relative bg-card border border-border rounded-xl p-4 text-left hover:border-primary/30 hover:shadow-sm transition-all group overflow-hidden"
        :class="statusFilter === 'all' ? 'border-primary/40 ring-1 ring-primary/10' : ''"
        @click="setStatusFilter('all')"
      >
        <!-- decorative bg blob -->
        <div class="absolute -right-3 -top-3 size-16 rounded-full bg-primary/5 group-hover:bg-primary/8 transition-colors" />
        <div class="relative">
          <div class="flex items-center justify-between mb-3">
            <span class="text-xs font-semibold text-muted-foreground uppercase tracking-wider">全部成员</span>
            <div class="size-7 rounded-lg bg-primary/10 flex items-center justify-center">
              <Users class="size-3.5 text-primary" />
            </div>
          </div>
          <p class="text-3xl font-black tracking-tighter tabular-nums">{{ stats.total }}</p>

          <!-- Avatar stack -->
          <div class="flex items-center gap-2 mt-3">
            <div class="flex -space-x-2">
              <div
                v-for="(name, i) in stats.recentNames" :key="i"
                class="size-6 rounded-full ring-2 ring-card flex items-center justify-center text-[9px] font-black text-white shrink-0"
                :class="avatarColor(name)"
              >{{ firstChar(name) }}</div>
            </div>
            <span v-if="stats.total > 4" class="text-[10px] text-muted-foreground/60">
              +{{ stats.total - 4 }} 人
            </span>
          </div>

          <!-- Role distribution bar -->
          <div class="mt-3 space-y-1.5">
            <div class="flex h-1.5 rounded-full overflow-hidden bg-muted/50 gap-px">
              <div
                class="bg-primary transition-all"
                :style="{ width: `${stats.total ? (stats.admin / stats.total) * 100 : 0}%` }"
              />
              <div
                class="bg-emerald-500 transition-all"
                :style="{ width: `${stats.total ? (stats.editor / stats.total) * 100 : 0}%` }"
              />
              <div
                class="bg-muted-foreground/30 transition-all"
                :style="{ width: `${stats.total ? (stats.viewer / stats.total) * 100 : 0}%` }"
              />
            </div>
            <div class="flex items-center gap-2.5 text-[10px] text-muted-foreground/60">
              <span class="flex items-center gap-1"><span class="size-1.5 rounded-full bg-primary inline-block" />{{ stats.admin }}</span>
              <span class="flex items-center gap-1"><span class="size-1.5 rounded-full bg-emerald-500 inline-block" />{{ stats.editor }}</span>
              <span class="flex items-center gap-1"><span class="size-1.5 rounded-full bg-muted-foreground/30 inline-block" />{{ stats.viewer }}</span>
            </div>
          </div>
        </div>
      </button>

      <!-- 管理员 -->
      <div class="relative bg-card border border-border rounded-xl p-4 text-left overflow-hidden">
        <div class="absolute -right-3 -top-3 size-16 rounded-full bg-primary/5" />
        <div class="relative">
          <div class="flex items-center justify-between mb-3">
            <span class="text-xs font-semibold text-muted-foreground uppercase tracking-wider">管理员</span>
            <div class="size-7 rounded-lg bg-primary/10 flex items-center justify-center">
              <Shield class="size-3.5 text-primary" />
            </div>
          </div>
          <p class="text-3xl font-black tracking-tighter tabular-nums text-primary">{{ stats.admin }}</p>
          <p class="text-[11px] text-muted-foreground/60 mt-1">
            占全体成员 <span class="font-bold text-primary">{{ stats.adminRate }}%</span>
          </p>

          <!-- Admin proportion bar -->
          <div class="mt-3 space-y-1.5">
            <div class="flex h-1.5 rounded-full overflow-hidden bg-muted/50">
              <div
                class="bg-primary rounded-full transition-all duration-700"
                :style="{ width: `${stats.adminRate}%` }"
              />
            </div>
            <p class="text-[10px] text-muted-foreground/50">拥有完整管理权限</p>
          </div>
        </div>
      </div>

      <!-- 活跃 -->
      <button
        class="relative bg-card border border-border rounded-xl p-4 text-left hover:border-emerald-500/30 hover:shadow-sm transition-all group overflow-hidden"
        :class="statusFilter === 'active' ? 'border-emerald-500/40 ring-1 ring-emerald-500/10' : ''"
        @click="setStatusFilter('active')"
      >
        <div class="absolute -right-3 -top-3 size-16 rounded-full bg-emerald-500/5 group-hover:bg-emerald-500/10 transition-colors" />
        <div class="relative">
          <div class="flex items-center justify-between mb-3">
            <span class="text-xs font-semibold text-muted-foreground uppercase tracking-wider">活跃成员</span>
            <div class="size-7 rounded-lg bg-emerald-500/10 flex items-center justify-center">
              <span class="relative flex size-2">
                <span class="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-60" />
                <span class="relative inline-flex size-2 rounded-full bg-emerald-500" />
              </span>
            </div>
          </div>
          <p class="text-3xl font-black tracking-tighter tabular-nums text-emerald-500">{{ stats.active }}</p>
          <p class="text-[11px] text-muted-foreground/60 mt-1">
            活跃率 <span class="font-bold text-emerald-500">{{ stats.activeRate }}%</span>
          </p>

          <!-- Active rate bar -->
          <div class="mt-3 space-y-1.5">
            <div class="flex h-1.5 rounded-full overflow-hidden bg-muted/50">
              <div
                class="bg-emerald-500 rounded-full transition-all duration-700"
                :style="{ width: `${stats.activeRate}%` }"
              />
            </div>
            <div class="flex items-center justify-between text-[10px] text-muted-foreground/50">
              <span>{{ stats.active }} 已激活</span>
              <span>{{ stats.pending }} 待处理</span>
            </div>
          </div>
        </div>
      </button>

      <!-- 待激活 -->
      <button
        class="relative bg-card border border-border rounded-xl p-4 text-left hover:border-amber-400/30 hover:shadow-sm transition-all group overflow-hidden"
        :class="statusFilter === 'pending' ? 'border-amber-400/40 ring-1 ring-amber-400/10' : ''"
        @click="setStatusFilter('pending')"
      >
        <div class="absolute -right-3 -top-3 size-16 rounded-full bg-amber-400/5 group-hover:bg-amber-400/10 transition-colors" />
        <div class="relative">
          <div class="flex items-center justify-between mb-3">
            <span class="text-xs font-semibold text-muted-foreground uppercase tracking-wider">待激活</span>
            <div class="size-7 rounded-lg bg-amber-400/10 flex items-center justify-center">
              <Clock class="size-3.5 text-amber-400" />
            </div>
          </div>
          <p class="text-3xl font-black tracking-tighter tabular-nums text-amber-400">{{ stats.pending }}</p>

          <!-- Status hint -->
          <div class="mt-3">
            <div v-if="stats.pending > 0"
              class="inline-flex items-center gap-1.5 text-[10px] font-semibold px-2 py-1 rounded-lg bg-amber-400/10 text-amber-600 dark:text-amber-400 ring-1 ring-amber-400/20"
            >
              <AlertCircle class="size-3" />
              需要处理
            </div>
            <div v-else
              class="inline-flex items-center gap-1.5 text-[10px] font-semibold px-2 py-1 rounded-lg bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 ring-1 ring-emerald-500/20"
            >
              <CheckCircle2 class="size-3" />
              全部已激活
            </div>
          </div>

          <p class="text-[10px] text-muted-foreground/50 mt-2">
            {{ stats.pending > 0 ? '等待完成邮箱验证' : '团队成员状态良好' }}
          </p>
        </div>
      </button>

    </div>

    <!-- ── Toolbar ────────────────────────────────────────────────── -->
    <div class="flex items-center gap-2">
      <div class="relative w-64">
        <Search class="absolute left-2.5 top-1/2 -translate-y-1/2 size-4 text-muted-foreground" />
        <Input
          v-model="searchValue"
          placeholder="搜索名称或邮箱..."
          class="pl-8 h-9"
          @input="onSearchInput"
        />
      </div>
      <div class="ml-auto flex items-center gap-2">
        <Button variant="outline" size="sm" class="gap-1.5" :disabled="loading" @click="refresh">
          <RefreshCw class="size-3.5" :class="loading ? 'animate-spin' : ''" />
          刷新
        </Button>
        <Button size="sm" class="gap-1.5" @click="openInvite">
          <Plus class="size-3.5" /> 添加成员
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
            <TableRow v-for="row in table.getRowModel().rows" :key="row.id">
              <TableCell v-for="cell in row.getVisibleCells()" :key="cell.id">
                <FlexRender :render="cell.column.columnDef.cell" :props="cell.getContext()" />
              </TableCell>
            </TableRow>
          </template>
          <TableRow v-else>
            <TableCell :colspan="columns.length" class="h-32 text-center text-muted-foreground">
              {{ loading ? '加载中...' : '暂无成员' }}
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>

    <!-- ── Pagination ─────────────────────────────────────────────── -->
    <div class="flex items-center justify-between text-sm text-muted-foreground">
      <span>共 {{ total }} 条 · 第 {{ currentPage }} / {{ totalPages }} 页</span>
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
      title="移除成员"
      :description="`确认从团队中移除「${deleteTarget?.name}」？此操作将同步撤销其 RBAC 绑定。`"
      confirm-text="确认移除"
      variant="destructive"
      @confirm="confirmDelete"
      @cancel="deleteTarget = null"
    />

  </div>

  <!-- ── Dialog ─────────────────────────────────────────────────── -->
  <Dialog v-model:open="dialogOpen">
    <DialogContent class="sm:max-w-md">

      <!-- Invite -->
      <template v-if="dialogType === 'invite'">
        <DialogHeader>
          <DialogTitle>添加成员</DialogTitle>
          <DialogDescription>向平台添加新用户并分配初始权限</DialogDescription>
        </DialogHeader>

        <div class="space-y-4 py-2">
          <div class="flex bg-muted/50 rounded-lg p-1 border border-border">
            <button
              class="flex-1 py-1.5 rounded-md text-xs font-bold uppercase tracking-widest transition-all"
              :class="form.provider === 'local' ? 'bg-background text-primary shadow-sm' : 'text-muted-foreground hover:text-foreground'"
              @click="form.provider = 'local'"
            >Local</button>
            <button
              class="flex-1 py-1.5 rounded-md text-xs font-bold uppercase tracking-widest transition-all"
              :class="form.provider === 'dex' ? 'bg-background text-primary shadow-sm' : 'text-muted-foreground hover:text-foreground'"
              @click="form.provider = 'dex'"
            >OIDC / Dex</button>
          </div>

          <div class="space-y-1.5">
            <label class="text-xs font-medium">用户名 / Email</label>
            <Input v-model="form.username" placeholder="ops@example.com" />
          </div>

          <div v-if="form.provider === 'local'" class="space-y-1.5">
            <label class="text-xs font-medium">初始密码</label>
            <Input v-model="form.password" type="password" />
          </div>

          <div class="grid grid-cols-2 gap-3">
            <div class="space-y-1.5">
              <label class="text-xs font-medium">系统角色</label>
              <select
                v-model="form.role"
                class="w-full h-9 rounded-md border border-input bg-background px-3 text-sm focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50 transition-[color,box-shadow]"
              >
                <option value="admin">Admin</option>
                <option value="editor">Editor</option>
                <option value="viewer">Viewer</option>
              </select>
            </div>
            <div class="space-y-1.5">
              <label class="text-xs font-medium">初始空间</label>
              <select
                v-model="form.namespace"
                class="w-full h-9 rounded-md border border-input bg-background px-3 text-sm focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50 transition-[color,box-shadow]"
              >
                <option value="">不指定</option>
                <option v-for="ws in workspaces" :key="(ws as any).id" :value="(ws as any).id">
                  {{ (ws as any).displayName }}
                </option>
              </select>
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" @click="dialogOpen = false">取消</Button>
          <Button :disabled="addLoading || !form.username" @click="runAdd(form)">
            <RefreshCw v-if="addLoading" class="size-3.5 animate-spin mr-2" />
            添加成员
          </Button>
        </DialogFooter>
      </template>

      <!-- Config -->
      <template v-else-if="selectedMember">
        <DialogHeader>
          <DialogTitle>编辑权限</DialogTitle>
          <DialogDescription>管理 {{ selectedMember.name }} 的命名空间绑定与角色</DialogDescription>
        </DialogHeader>

        <div class="space-y-4 py-2">
          <div class="flex items-center gap-3 p-3 bg-muted/30 rounded-lg border border-border">
            <div class="size-10 rounded-xl flex items-center justify-center text-white text-sm font-black shrink-0"
              :class="avatarColor(selectedMember.name)">
              {{ firstChar(selectedMember.name) }}
            </div>
            <div>
              <p class="text-sm font-bold">{{ selectedMember.name }}</p>
              <p class="text-xs font-mono text-muted-foreground/60">{{ selectedMember.email }}</p>
            </div>
          </div>

          <div class="space-y-1.5">
            <label class="text-xs font-medium">系统角色</label>
            <select
              v-model="selectedMember.role"
              class="w-full h-9 rounded-md border border-input bg-background px-3 text-sm focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50 transition-[color,box-shadow]"
            >
              <option value="admin">Admin</option>
              <option value="editor">Editor</option>
              <option value="viewer">Viewer</option>
            </select>
          </div>

          <div class="space-y-2">
            <label class="text-xs font-medium">Namespace Bindings</label>
            <div class="space-y-2 max-h-44 overflow-y-auto">
              <div
                v-for="(b, i) in selectedMember.bindings" :key="i"
                class="flex items-center justify-between p-3 rounded-lg border border-border bg-muted/20"
              >
                <div class="flex items-center gap-2">
                  <Server class="size-3.5 text-muted-foreground/50" />
                  <span class="text-xs font-bold">{{ b.ns }}</span>
                </div>
                <select class="h-7 rounded-md border border-input bg-background px-2 text-xs focus-visible:outline-none w-24">
                  <option>Admin</option>
                  <option>Editor</option>
                  <option>Viewer</option>
                </select>
              </div>
              <p v-if="!selectedMember.bindings?.length"
                class="text-xs text-muted-foreground/40 italic py-2 text-center">
                暂无命名空间绑定
              </p>
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" @click="dialogOpen = false">取消</Button>
          <Button @click="dialogOpen = false">
            <Key class="size-3.5 mr-2" /> 保存权限
          </Button>
        </DialogFooter>
      </template>

    </DialogContent>
  </Dialog>
</template>
