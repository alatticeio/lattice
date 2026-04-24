<script setup lang="ts">
import { ref, computed, onMounted, h } from 'vue'
import {
  useVueTable, getCoreRowModel,
  FlexRender, type ColumnDef,
} from '@tanstack/vue-table'
import {
  Search, RefreshCw, Clock, Shield, Activity,
  AlertTriangle, Users, ChevronDown, CheckCircle2,
  XCircle, ArrowUpRight, ArrowDownRight, FileText,
  ChevronLeft, ChevronRight,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import { listAuditLogs, type AuditLogVo } from '@/api/audit'
import { useTable } from '@/composables/useApi'

definePage({
  meta: { title: '审计日志', description: '查看 Workspace 内所有操作的完整记录。' },
})

// ── Data ──────────────────────────────────────────────────────────
const { rows: logs, total, loading, refresh } = useTable(listAuditLogs)
const page     = ref(1)
const pageSize = ref(10)
onMounted(() => refresh({ page: page.value, pageSize: pageSize.value }))

// ── Filters ───────────────────────────────────────────────────────
const searchValue  = ref('')
const actionFilter = ref('')
const resourceFilter = ref('')
const statusFilter = ref('')
const timeRange    = ref('7d')

const actions   = ['CREATE', 'UPDATE', 'DELETE', 'LOGIN', 'INVITE', 'REVOKE', 'EXPORT', 'ACCEPT']
const resources = ['member', 'workspace', 'policy', 'token', 'relay', 'invitation', 'peer', 'user']
const timeRanges = [
  { label: '今天',   value: '1d' },
  { label: '近 7 天', value: '7d' },
  { label: '近 30 天', value: '30d' },
]

function fromDate(range: string): string {
  const d = new Date()
  if (range === '1d')  d.setDate(d.getDate() - 1)
  if (range === '7d')  d.setDate(d.getDate() - 7)
  if (range === '30d') d.setDate(d.getDate() - 30)
  return d.toISOString()
}

function doRefresh(p = 1) {
  page.value = p
  refresh({
    action:   actionFilter.value || undefined,
    resource: resourceFilter.value || undefined,
    status:   statusFilter.value || undefined,
    keyword:  searchValue.value || undefined,
    from:     fromDate(timeRange.value),
    page:     page.value,
    pageSize: pageSize.value,
  })
}

const totalPages   = computed(() => Math.max(1, Math.ceil((total.value || 0) / pageSize.value)))
const visiblePages = computed(() => {
  const cur = page.value, tp = totalPages.value
  const start = Math.max(1, Math.min(cur - 1, tp - 2))
  const end   = Math.min(tp, start + 2)
  return Array.from({ length: end - start + 1 }, (_, i) => start + i)
})

// ── Style maps ────────────────────────────────────────────────────
const actionStyle: Record<string, string> = {
  CREATE: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 ring-1 ring-emerald-500/20',
  UPDATE: 'bg-blue-500/10 text-blue-600 dark:text-blue-400 ring-1 ring-blue-500/20',
  DELETE: 'bg-red-500/10 text-red-500 ring-1 ring-red-500/20',
  LOGIN:  'bg-violet-500/10 text-violet-600 dark:text-violet-400 ring-1 ring-violet-500/20',
  INVITE: 'bg-amber-400/10 text-amber-600 dark:text-amber-400 ring-1 ring-amber-400/20',
  REVOKE: 'bg-orange-500/10 text-orange-600 dark:text-orange-400 ring-1 ring-orange-500/20',
  EXPORT: 'bg-cyan-500/10 text-cyan-600 dark:text-cyan-400 ring-1 ring-cyan-500/20',
  ACCEPT: 'bg-teal-500/10 text-teal-600 dark:text-teal-400 ring-1 ring-teal-500/20',
}
const actionLabel: Record<string, string> = {
  CREATE: '创建', UPDATE: '更新', DELETE: '删除', LOGIN: '登录',
  INVITE: '邀请', REVOKE: '撤销', EXPORT: '导出', ACCEPT: '接受',
}
const resourceLabel: Record<string, string> = {
  member: '成员', workspace: '空间', policy: '策略', token: '令牌',
  relay: '中继', invitation: '邀请', peer: '节点', user: '用户',
}

function formatTime(iso: string): string {
  if (!iso) return '—'
  const diff = Date.now() - new Date(iso).getTime()
  if (diff < 60_000)     return '刚刚'
  if (diff < 3_600_000)  return `${Math.floor(diff / 60_000)} 分钟前`
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)} 小时前`
  return new Date(iso).toLocaleString('zh-CN', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
}

// ── Stats ─────────────────────────────────────────────────────────
const stats = computed(() => {
  const all = logs.value as AuditLogVo[]
  const failed   = all.filter(l => l.status === 'failed').length
  const users    = new Set(all.map(l => l.userId).filter(Boolean)).size
  const deletes  = all.filter(l => l.action === 'DELETE').length
  return { total: total.value || all.length, failed, users, deletes }
})

// ── Row expand ────────────────────────────────────────────────────
const expandedRow = ref<string | null>(null)
function toggleExpand(id: string) {
  expandedRow.value = expandedRow.value === id ? null : id
}

// ── Columns ───────────────────────────────────────────────────────
const columns: ColumnDef<AuditLogVo>[] = [
  {
    id: 'time',
    header: '时间',
    cell: ({ row }) => {
      const l = row.original
      return h('div', { class: 'flex items-center gap-1.5 text-xs text-muted-foreground whitespace-nowrap' }, [
        h(Clock, { class: 'size-3 shrink-0' }),
        h('span', { title: l.createdAt }, formatTime(l.createdAt)),
      ])
    },
  },
  {
    id: 'user',
    header: '操作者',
    cell: ({ row }) => {
      const l = row.original
      const displayName = l.userName || l.userEmail || '—'
      return h('div', { class: 'flex flex-col gap-0.5' }, [
        h('span', { class: 'text-sm font-medium leading-none' }, displayName),
        h('span', { class: 'font-mono text-[11px] text-muted-foreground/60' }, l.userIP),
      ])
    },
  },
  {
    id: 'action',
    header: '操作',
    cell: ({ row }) => {
      const action = row.original.action
      return h('span', {
        class: `text-[11px] font-bold px-2.5 py-1 rounded-full w-fit ${actionStyle[action] ?? 'bg-muted text-muted-foreground ring-1 ring-border'}`,
      }, actionLabel[action] ?? action)
    },
  },
  {
    id: 'resource',
    header: '资源',
    cell: ({ row }) => {
      const l = row.original
      return h('div', { class: 'flex flex-col gap-0.5' }, [
        h('span', { class: 'text-xs font-medium' }, resourceLabel[l.resource] ?? l.resource),
        l.resourceName && h('span', { class: 'text-[11px] text-muted-foreground/60 truncate max-w-32' }, l.resourceName),
      ])
    },
  },
  {
    id: 'scope',
    header: '影响范围',
    cell: ({ row }) => {
      const scope = row.original.scope
      if (!scope) return h('span', { class: 'text-[11px] text-muted-foreground/40 italic' }, '—')
      return h('span', { class: 'text-[11px] text-muted-foreground max-w-48 truncate block', title: scope }, scope)
    },
  },
  {
    id: 'status',
    header: '结果',
    cell: ({ row }) => {
      const s = row.original.status
      const ok = s === 'success'
      return h('div', { class: 'flex items-center gap-1.5' }, [
        h(ok ? CheckCircle2 : XCircle, {
          class: `size-4 ${ok ? 'text-emerald-500' : 'text-red-500'}`,
        }),
        h('span', {
          class: `text-[11px] font-medium ${ok ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-500'}`,
        }, ok ? '成功' : '失败'),
      ])
    },
  },
  {
    id: 'detail',
    header: '',
    cell: ({ row }) => {
      const l = row.original
      if (!l.detail) return h('span')
      const expanded = expandedRow.value === l.id
      return h(Button, {
        variant: 'ghost',
        size: 'sm',
        class: 'h-7 px-2 text-[11px] gap-1 text-muted-foreground',
        onClick: () => toggleExpand(l.id),
      }, () => [
        h(FileText, { class: 'size-3' }),
        '详情',
        h(ChevronDown, { class: `size-3 transition-transform ${expanded ? 'rotate-180' : ''}` }),
      ])
    },
  },
]

// ── Table ─────────────────────────────────────────────────────────
const table = useVueTable({
  get data() { return logs.value as AuditLogVo[] },
  columns,
  getCoreRowModel: getCoreRowModel(),
  manualPagination: true,
})
</script>

<template>
  <div class="flex flex-col gap-5 p-6 animate-in fade-in duration-300">

    <!-- ── Stat cards ─────────────────────────────────────────────── -->
    <div class="grid grid-cols-2 sm:grid-cols-4 gap-4">

      <!-- 总操作数 -->
      <div class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm">
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">操作总数</span>
            <span class="text-2xl font-bold tracking-tight">{{ stats.total }}</span>
          </div>
          <div class="bg-muted rounded-lg p-2">
            <Activity class="text-muted-foreground size-4" />
          </div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-sm">
          <ArrowUpRight class="text-muted-foreground size-4 shrink-0" />
          <span class="text-muted-foreground">近 <span class="font-semibold text-foreground">{{ timeRanges.find(t => t.value === timeRange)?.label }}</span> 记录</span>
        </div>
      </div>

      <!-- 失败操作 -->
      <div class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm">
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">失败操作</span>
            <span class="text-2xl font-bold tracking-tight">{{ stats.failed }}</span>
          </div>
          <div class="bg-muted rounded-lg p-2">
            <AlertTriangle class="text-muted-foreground size-4" />
          </div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-sm">
          <component
            :is="stats.failed === 0 ? ArrowUpRight : ArrowDownRight"
            :class="stats.failed === 0 ? 'text-emerald-600' : 'text-red-500'"
            class="size-4 shrink-0"
          />
          <span :class="stats.failed === 0 ? 'text-emerald-600 font-semibold' : 'text-red-500 font-semibold'">
            {{ stats.failed === 0 ? '全部成功' : stats.failed + ' 次异常' }}
          </span>
          <span class="text-muted-foreground">{{ stats.failed === 0 ? '运行正常' : '需要关注' }}</span>
        </div>
      </div>

      <!-- 活跃用户 -->
      <div class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm">
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">活跃用户</span>
            <span class="text-2xl font-bold tracking-tight">{{ stats.users }}</span>
          </div>
          <div class="bg-muted rounded-lg p-2">
            <Users class="text-muted-foreground size-4" />
          </div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-sm">
          <ArrowUpRight class="text-muted-foreground size-4 shrink-0" />
          <span class="text-muted-foreground">产生操作的独立用户</span>
        </div>
      </div>

      <!-- 删除操作 -->
      <div class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm">
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">删除操作</span>
            <span class="text-2xl font-bold tracking-tight">{{ stats.deletes }}</span>
          </div>
          <div class="bg-muted rounded-lg p-2">
            <Shield class="text-muted-foreground size-4" />
          </div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-sm">
          <component
            :is="stats.deletes === 0 ? ArrowUpRight : ArrowDownRight"
            :class="stats.deletes === 0 ? 'text-emerald-600' : 'text-amber-500'"
            class="size-4 shrink-0"
          />
          <span :class="stats.deletes === 0 ? 'text-emerald-600 font-semibold' : 'text-amber-500 font-semibold'">
            {{ stats.deletes === 0 ? '无删除记录' : stats.deletes + ' 次删除' }}
          </span>
          <span class="text-muted-foreground">{{ stats.deletes === 0 ? '数据安全' : '需确认' }}</span>
        </div>
      </div>

    </div>

    <!-- ── Toolbar ────────────────────────────────────────────────── -->
    <div class="flex flex-wrap items-center gap-2">

      <!-- 时间范围 -->
      <div class="flex bg-muted/50 rounded-lg p-1 border border-border gap-0.5">
        <button
          v-for="t in timeRanges" :key="t.value"
          class="px-3 py-1.5 rounded-md text-xs font-semibold transition-all"
          :class="timeRange === t.value
            ? 'bg-background text-foreground shadow-sm ring-1 ring-border'
            : 'text-muted-foreground hover:text-foreground'"
          @click="timeRange = t.value; doRefresh()"
        >{{ t.label }}</button>
      </div>

      <!-- 操作类型 -->
      <select
        v-model="actionFilter"
        class="h-9 rounded-md border border-input bg-background px-3 text-xs focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50 transition-[color,box-shadow]"
        @change="() => doRefresh()"
      >
        <option value="">全部操作</option>
        <option v-for="a in actions" :key="a" :value="a">{{ actionLabel[a] ?? a }}</option>
      </select>

      <!-- 资源类型 -->
      <select
        v-model="resourceFilter"
        class="h-9 rounded-md border border-input bg-background px-3 text-xs focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50 transition-[color,box-shadow]"
        @change="() => doRefresh()"
      >
        <option value="">全部资源</option>
        <option v-for="r in resources" :key="r" :value="r">{{ resourceLabel[r] ?? r }}</option>
      </select>

      <!-- 结果 -->
      <select
        v-model="statusFilter"
        class="h-9 rounded-md border border-input bg-background px-3 text-xs focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50 transition-[color,box-shadow]"
        @change="() => doRefresh()"
      >
        <option value="">全部结果</option>
        <option value="success">成功</option>
        <option value="failed">失败</option>
      </select>

      <!-- 搜索 -->
      <div class="relative w-56 ml-auto">
        <Search class="absolute left-2.5 top-1/2 -translate-y-1/2 size-4 text-muted-foreground" />
        <Input
          v-model="searchValue"
          placeholder="搜索用户或资源..."
          class="pl-8 h-9"
          @keyup.enter="doRefresh"
        />
      </div>

      <Button variant="outline" size="sm" class="gap-1.5" :disabled="loading" @click="doRefresh">
        <RefreshCw class="size-3.5" :class="loading ? 'animate-spin' : ''" />
        刷新
      </Button>
    </div>

    <!-- ── Table ──────────────────────────────────────────────────── -->
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
            <template v-for="row in table.getRowModel().rows" :key="row.id">
              <TableRow class="cursor-pointer hover:bg-muted/30" @click="toggleExpand(row.original.id)">
                <TableCell v-for="cell in row.getVisibleCells()" :key="cell.id" @click.stop="cell.column.id === 'detail' ? undefined : toggleExpand(row.original.id)">
                  <FlexRender :render="cell.column.columnDef.cell" :props="cell.getContext()" />
                </TableCell>
              </TableRow>
              <!-- 展开的 detail 行 -->
              <TableRow v-if="expandedRow === row.original.id && row.original.detail" class="bg-muted/20 hover:bg-muted/20">
                <TableCell :colspan="columns.length" class="py-3 px-6">
                  <pre class="font-mono text-[11px] text-muted-foreground whitespace-pre-wrap break-all leading-relaxed">{{ (() => { try { return JSON.stringify(JSON.parse(row.original.detail!), null, 2) } catch { return row.original.detail } })() }}</pre>
                </TableCell>
              </TableRow>
            </template>
          </template>
          <TableRow v-else>
            <TableCell :colspan="columns.length" class="h-32 text-center text-muted-foreground">
              {{ loading ? '加载中...' : '暂无审计记录' }}
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>

    <!-- pagination -->
    <div class="flex items-center justify-between text-sm text-muted-foreground">
      <span>共 {{ total }} 条 · 第 {{ page }} / {{ totalPages }} 页</span>
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
</template>
