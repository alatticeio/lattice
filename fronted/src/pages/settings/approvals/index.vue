<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Clock, CheckCircle2, XCircle, AlertCircle, Loader2,
  RefreshCw, ChevronLeft, ChevronRight, FileText, Search,
  Shield, Ban,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter,
} from '@/components/ui/dialog'
import { toast } from 'vue-sonner'
import {
  listWorkflowRequests, approveWorkflowRequest, rejectWorkflowRequest,
  type WorkflowRequestVo,
} from '@/api/workflow'
import { useTable } from '@/composables/useApi'
import { useUserStore } from '@/stores/user'

definePage({
  meta: { titleKey: 'settings.approvals.title', descKey: 'settings.approvals.desc' },
})

const { t, locale } = useI18n()
const userStore = useUserStore()

// ── Data ──────────────────────────────────────────────────────────
const { rows, total, loading, refresh } = useTable(listWorkflowRequests)
const page     = ref(1)
const pageSize = ref(10)
onMounted(() => doRefresh())

const statusFilter = ref('')
const keyword      = ref('')

// Client-side filter by requester name / resource name
const filteredRows = computed(() => {
  const kw  = keyword.value.trim().toLowerCase()
  const all = rows.value as WorkflowRequestVo[]
  if (!kw) return all
  return all.filter(r =>
    (r.requestedByName ?? '').toLowerCase().includes(kw) ||
    (r.requestedByEmail ?? '').toLowerCase().includes(kw) ||
    (r.resourceName ?? '').toLowerCase().includes(kw)
  )
})

function doRefresh(p = 1) {
  page.value = p
  refresh({
    status:   statusFilter.value || undefined,
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

// ── Stat cards (computed from filtered rows) ──────────────────────
const stats = computed(() => {
  const all = filteredRows.value
  return {
    total:    all.length,
    pending:  all.filter(r => r.status === 'pending').length,
    executed: all.filter(r => r.status === 'executed').length,
    failed:   all.filter(r => r.status === 'failed').length,
  }
})

// ── Status tabs ───────────────────────────────────────────────────
const statusTabs = computed(() => [
  { value: '',         label: t('settings.approvals.tabs.all') },
  { value: 'pending',  label: t('settings.approvals.tabs.pending') },
  { value: 'approved', label: t('settings.approvals.tabs.approved') },
  { value: 'rejected', label: t('settings.approvals.tabs.rejected') },
  { value: 'executed', label: t('settings.approvals.tabs.executed') },
  { value: 'failed',   label: t('settings.approvals.tabs.failed') },
])

function setTab(val: string) {
  statusFilter.value = val
  doRefresh(1)
}

// ── Review dialog ─────────────────────────────────────────────────
const reviewOpen   = ref(false)
const reviewTarget = ref<WorkflowRequestVo | null>(null)
const reviewAction = ref<'approve' | 'reject'>('approve')
const reviewNote   = ref('')
const reviewing    = ref(false)

function openReview(row: WorkflowRequestVo, action: 'approve' | 'reject') {
  reviewTarget.value = row
  reviewAction.value = action
  reviewNote.value   = ''
  reviewOpen.value   = true
}

async function submitReview() {
  if (!reviewTarget.value) return
  reviewing.value = true
  try {
    if (reviewAction.value === 'approve') {
      await approveWorkflowRequest(reviewTarget.value.id, reviewNote.value || undefined)
      toast.success(t('settings.approvals.toast.approved'))
    } else {
      await rejectWorkflowRequest(reviewTarget.value.id, reviewNote.value || undefined)
      toast.success(t('settings.approvals.toast.rejected'))
    }
    reviewOpen.value = false
    doRefresh(page.value)
  } catch (e: any) {
    toast.error(e?.message ?? t('settings.approvals.toast.failed'))
  } finally {
    reviewing.value = false
  }
}

// ── Style helpers ─────────────────────────────────────────────────
const statusStyle: Record<string, string> = {
  pending:  'bg-amber-500/10 text-amber-600 dark:text-amber-400 ring-1 ring-amber-500/20',
  approved: 'bg-blue-500/10 text-blue-600 dark:text-blue-400 ring-1 ring-blue-500/20',
  rejected: 'bg-red-500/10 text-red-500 ring-1 ring-red-500/20',
  executed: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 ring-1 ring-emerald-500/20',
  failed:   'bg-rose-500/10 text-rose-600 dark:text-rose-400 ring-1 ring-rose-500/20',
}
const actionStyle: Record<string, string> = {
  create: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 ring-1 ring-emerald-500/20',
  update: 'bg-blue-500/10 text-blue-600 dark:text-blue-400 ring-1 ring-blue-500/20',
  delete: 'bg-red-500/10 text-red-500 ring-1 ring-red-500/20',
}

const statusLabel = computed<Record<string, string>>(() => ({
  pending:  t('settings.approvals.status.pending'),
  approved: t('settings.approvals.status.approved'),
  rejected: t('settings.approvals.status.rejected'),
  executed: t('settings.approvals.status.executed'),
  failed:   t('settings.approvals.status.failed'),
}))

const actionLabel = computed<Record<string, string>>(() => ({
  create: t('settings.approvals.actionType.create'),
  update: t('settings.approvals.actionType.update'),
  delete: t('settings.approvals.actionType.delete'),
}))

const resourceLabel = computed<Record<string, string>>(() => ({
  policy: t('settings.approvals.resourceType.policy'),
  member: t('settings.approvals.resourceType.member'),
  relay:  t('settings.approvals.resourceType.relay'),
  token:  t('settings.approvals.resourceType.token'),
}))

function formatTime(iso?: string): string {
  if (!iso) return '—'
  const diff = Date.now() - new Date(iso).getTime()
  if (diff < 60_000)     return t('common.time.justNow')
  if (diff < 3_600_000)  return t('common.time.minutesAgo', { n: Math.floor(diff / 60_000) })
  if (diff < 86_400_000) return t('common.time.hoursAgo', { n: Math.floor(diff / 3_600_000) })
  return new Date(iso).toLocaleString(locale.value, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
}

// ── Avatar helpers ────────────────────────────────────────────────
const AVATAR_COLORS = [
  'bg-violet-500',
  'bg-blue-500',
  'bg-emerald-500',
  'bg-amber-500',
  'bg-rose-500',
  'bg-cyan-500',
]

function avatarInitials(name: string): string {
  if (!name) return '?'
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map(w => w[0].toUpperCase())
    .join('')
}

function avatarColor(name: string): string {
  if (!name) return AVATAR_COLORS[0]
  let hash = 0
  for (let i = 0; i < name.length; i++) hash = (hash * 31 + name.charCodeAt(i)) >>> 0
  return AVATAR_COLORS[hash % AVATAR_COLORS.length]
}

// ── Row expand ────────────────────────────────────────────────────
const expandedRow = ref<string | null>(null)
function toggleExpand(id: string) {
  expandedRow.value = expandedRow.value === id ? null : id
}
</script>

<template>
  <div class="flex flex-col gap-5 p-6 animate-in fade-in duration-300">

    <!-- ── Stat bar ────────────────────────────────────────────────── -->
    <div class="grid grid-cols-2 sm:grid-cols-4 gap-4">

      <!-- 全部 -->
      <button
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left hover:shadow-md transition-all"
        :class="statusFilter === '' ? 'ring-2 ring-primary/20 border-primary/30' : ''"
        @click="setTab('')"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">{{ t('settings.approvals.stats.total') }}</span>
            <span class="text-2xl font-bold tracking-tight">{{ stats.total }}</span>
          </div>
          <div class="bg-muted rounded-lg p-2">
            <FileText class="text-muted-foreground size-4" />
          </div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-xs text-muted-foreground">
          <Clock class="size-3.5 shrink-0" />
          <span>{{ t('settings.approvals.stats.totalDesc') }}</span>
        </div>
      </button>

      <!-- 待审批 -->
      <button
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left hover:shadow-md transition-all"
        :class="statusFilter === 'pending' ? 'ring-2 ring-amber-500/20 border-amber-500/30' : ''"
        @click="setTab('pending')"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">{{ t('settings.approvals.stats.pending') }}</span>
            <span class="text-2xl font-bold tracking-tight text-amber-600 dark:text-amber-400">{{ stats.pending }}</span>
          </div>
          <div class="bg-amber-500/10 rounded-lg p-2">
            <Clock class="text-amber-500 size-4" />
          </div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-xs text-muted-foreground">
          <AlertCircle class="size-3.5 shrink-0 text-amber-500" />
          <span>{{ t('settings.approvals.stats.pendingDesc') }}</span>
        </div>
      </button>

      <!-- 已执行 -->
      <button
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left hover:shadow-md transition-all"
        :class="statusFilter === 'executed' ? 'ring-2 ring-emerald-500/20 border-emerald-500/30' : ''"
        @click="setTab('executed')"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">{{ t('settings.approvals.stats.executed') }}</span>
            <span class="text-2xl font-bold tracking-tight text-emerald-600 dark:text-emerald-400">{{ stats.executed }}</span>
          </div>
          <div class="bg-emerald-500/10 rounded-lg p-2">
            <CheckCircle2 class="text-emerald-500 size-4" />
          </div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-xs text-muted-foreground">
          <CheckCircle2 class="size-3.5 shrink-0 text-emerald-500" />
          <span>{{ t('settings.approvals.stats.executedDesc') }}</span>
        </div>
      </button>

      <!-- 执行失败 -->
      <button
        class="border-border bg-card text-card-foreground rounded-xl border p-5 shadow-sm text-left hover:shadow-md transition-all"
        :class="statusFilter === 'failed' ? 'ring-2 ring-rose-500/20 border-rose-500/30' : ''"
        @click="setTab('failed')"
      >
        <div class="flex items-start justify-between">
          <div class="flex flex-col gap-1">
            <span class="text-muted-foreground text-sm font-medium">{{ t('settings.approvals.stats.failed') }}</span>
            <span class="text-2xl font-bold tracking-tight text-rose-600 dark:text-rose-400">{{ stats.failed }}</span>
          </div>
          <div class="bg-rose-500/10 rounded-lg p-2">
            <XCircle class="text-rose-500 size-4" />
          </div>
        </div>
        <div class="mt-3 flex items-center gap-1 text-xs text-muted-foreground">
          <XCircle class="size-3.5 shrink-0 text-rose-500" />
          <span>{{ stats.failed === 0 ? t('settings.approvals.stats.allGood') : t('settings.approvals.stats.failedDesc') }}</span>
        </div>
      </button>

    </div>

    <!-- ── Toolbar ────────────────────────────────────────────────── -->
    <div class="flex items-center gap-3">
      <!-- pill tabs -->
      <div class="flex bg-muted/50 rounded-lg p-1 border border-border gap-0.5 flex-wrap">
        <button
          v-for="tab in statusTabs"
          :key="tab.value"
          class="px-3 py-1.5 rounded-md text-xs font-semibold transition-all"
          :class="statusFilter === tab.value
            ? 'bg-background text-foreground shadow-sm ring-1 ring-border'
            : 'text-muted-foreground hover:text-foreground'"
          @click="setTab(tab.value)"
        >{{ tab.label }}</button>
      </div>

      <div class="ml-auto flex items-center gap-2">
        <div class="relative w-52">
          <Search class="absolute left-2.5 top-1/2 -translate-y-1/2 size-4 text-muted-foreground pointer-events-none" />
          <Input v-model="keyword" :placeholder="t('settings.approvals.searchPlaceholder')" class="pl-8 h-9" />
        </div>
        <Button variant="outline" size="sm" class="gap-1.5" :disabled="loading" @click="doRefresh()">
          <RefreshCw class="size-3.5" :class="loading ? 'animate-spin' : ''" />
          {{ t('common.action.refresh') }}
        </Button>
      </div>
    </div>

    <!-- ── Pending card layout ─────────────────────────────────────── -->
    <template v-if="statusFilter === 'pending'">
      <div v-if="loading" class="flex items-center justify-center h-40 text-muted-foreground text-sm gap-2">
        <Loader2 class="size-4 animate-spin" />
        {{ t('common.status.loading') }}
      </div>

      <div v-else-if="(rows as WorkflowRequestVo[]).filter(r => r.status === 'pending').length === 0"
        class="flex flex-col items-center justify-center h-40 gap-2 text-muted-foreground text-sm">
        <CheckCircle2 class="size-8 text-emerald-500/50" />
        {{ t('settings.approvals.noPending') }}
      </div>

      <div v-else class="flex flex-col gap-3">
        <div
          v-for="item in filteredRows.filter(r => r.status === 'pending')"
          :key="item.id"
          class="rounded-xl border border-border bg-card shadow-sm flex items-center gap-4 px-5 py-4 border-l-4 border-l-amber-400 hover:shadow-md transition-shadow"
        >
          <!-- Avatar + requester -->
          <div class="flex items-center gap-3 min-w-0 w-52 shrink-0">
            <div
              class="size-10 rounded-full flex items-center justify-center shrink-0 text-white text-sm font-bold select-none"
              :class="avatarColor(item.requestedByName || item.requestedByEmail || item.requestedBy)"
            >
              {{ avatarInitials(item.requestedByName || item.requestedByEmail || item.requestedBy) }}
            </div>
            <div class="min-w-0 flex flex-col gap-0.5">
              <span class="text-sm font-semibold leading-none truncate">
                {{ item.requestedByName || item.requestedBy }}
              </span>
              <span v-if="item.requestedByEmail" class="text-[11px] text-muted-foreground/70 truncate">
                {{ item.requestedByEmail }}
              </span>
            </div>
          </div>

          <!-- Resource + action + time -->
          <div class="flex-1 min-w-0 flex flex-wrap items-center gap-2">
            <!-- resource chip -->
            <div class="flex items-center gap-1.5 bg-muted/60 rounded-lg px-2.5 py-1.5 text-xs font-medium shrink-0">
              <Shield class="size-3.5 text-muted-foreground shrink-0" />
              <span class="text-muted-foreground">{{ resourceLabel[item.resourceType] ?? item.resourceType }}</span>
              <span class="text-muted-foreground/50">/</span>
              <span class="text-foreground truncate max-w-32">{{ item.resourceName || '—' }}</span>
            </div>
            <!-- action badge -->
            <span
              class="text-[11px] font-bold px-2.5 py-1 rounded-full shrink-0"
              :class="actionStyle[item.action] ?? 'bg-muted text-muted-foreground ring-1 ring-border'"
            >{{ actionLabel[item.action] ?? item.action }}</span>
            <!-- time -->
            <div class="flex items-center gap-1 text-[11px] text-muted-foreground shrink-0">
              <Clock class="size-3 shrink-0" />
              <span :title="item.createdAt">{{ formatTime(item.createdAt) }}</span>
            </div>
          </div>

          <!-- Approve / Reject buttons -->
          <div v-if="userStore.isPlatformAdmin" class="flex items-center gap-2 shrink-0">
            <Button
              size="sm"
              variant="outline"
              class="h-8 px-3 text-xs gap-1.5 text-emerald-600 border-emerald-500/30 hover:bg-emerald-50 dark:hover:bg-emerald-950 hover:border-emerald-500/50"
              @click="openReview(item, 'approve')"
            >
              <CheckCircle2 class="size-3.5" />
              {{ t('common.action.approve') }}
            </Button>
            <Button
              size="sm"
              variant="outline"
              class="h-8 px-3 text-xs gap-1.5 text-red-500 border-red-500/30 hover:bg-red-50 dark:hover:bg-red-950 hover:border-red-500/50"
              @click="openReview(item, 'reject')"
            >
              <XCircle class="size-3.5" />
              {{ t('common.action.reject') }}
            </Button>
          </div>
        </div>
      </div>
    </template>

    <!-- ── Table layout (all other tabs) ─────────────────────────── -->
    <template v-else>
      <div class="rounded-xl border border-border overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead class="text-left align-middle"><div class="flex w-full items-center justify-start text-left">{{ t('settings.approvals.col.time') }}</div></TableHead>
              <TableHead class="text-left align-middle"><div class="flex w-full items-center justify-start text-left">{{ t('settings.approvals.col.requester') }}</div></TableHead>
              <TableHead class="text-left align-middle"><div class="flex w-full items-center justify-start text-left">{{ t('settings.approvals.col.resource') }}</div></TableHead>
              <TableHead class="text-left align-middle"><div class="flex w-full items-center justify-start text-left">{{ t('settings.approvals.col.action') }}</div></TableHead>
              <TableHead class="text-left align-middle"><div class="flex w-full items-center justify-start text-left">{{ t('settings.approvals.col.status') }}</div></TableHead>
              <TableHead class="text-left align-middle"><div class="flex w-full items-center justify-start text-left">{{ t('settings.approvals.col.reviewer') }}</div></TableHead>
              <TableHead v-if="statusFilter === ''" class="text-left align-middle" />
            </TableRow>
          </TableHeader>
          <TableBody>
            <template v-if="(rows as WorkflowRequestVo[]).length && !loading">
              <template v-for="item in filteredRows" :key="item.id">
                <TableRow
                  class="cursor-pointer hover:bg-muted/30 transition-colors"
                  @click="toggleExpand(item.id)"
                >
                  <!-- 提交时间 -->
                  <TableCell class="pl-5">
                    <div class="flex items-center gap-1.5 text-xs text-muted-foreground whitespace-nowrap">
                      <Clock class="size-3 shrink-0" />
                      <span :title="item.createdAt">{{ formatTime(item.createdAt) }}</span>
                    </div>
                  </TableCell>

                  <!-- 申请人 -->
                  <TableCell>
                    <div class="flex items-center gap-2.5">
                      <div
                        class="size-7 rounded-full flex items-center justify-center shrink-0 text-white text-[10px] font-bold select-none"
                        :class="avatarColor(item.requestedByName || item.requestedByEmail || item.requestedBy)"
                      >
                        {{ avatarInitials(item.requestedByName || item.requestedByEmail || item.requestedBy) }}
                      </div>
                      <div class="flex flex-col gap-0.5">
                        <span class="text-sm font-medium leading-none">
                          {{ item.requestedByName || item.requestedBy }}
                        </span>
                        <span v-if="item.requestedByEmail" class="text-[11px] text-muted-foreground/60">
                          {{ item.requestedByEmail }}
                        </span>
                      </div>
                    </div>
                  </TableCell>

                  <!-- 资源 -->
                  <TableCell>
                    <div class="flex items-center gap-1.5">
                      <span class="text-[11px] font-medium px-2 py-0.5 rounded-md bg-muted text-muted-foreground ring-1 ring-border">
                        {{ resourceLabel[item.resourceType] ?? item.resourceType }}
                      </span>
                      <span class="text-xs text-foreground truncate max-w-32" :title="item.resourceName">
                        {{ item.resourceName || '—' }}
                      </span>
                    </div>
                  </TableCell>

                  <!-- 操作 -->
                  <TableCell>
                    <span
                      class="text-[11px] font-bold px-2.5 py-1 rounded-full"
                      :class="actionStyle[item.action] ?? 'bg-muted text-muted-foreground ring-1 ring-border'"
                    >{{ actionLabel[item.action] ?? item.action }}</span>
                  </TableCell>

                  <!-- 状态 -->
                  <TableCell>
                    <span
                      class="text-[11px] font-bold px-2.5 py-1 rounded-full"
                      :class="statusStyle[item.status] ?? 'bg-muted text-muted-foreground ring-1 ring-border'"
                    >{{ statusLabel[item.status] ?? item.status }}</span>
                  </TableCell>

                  <!-- 审批人 -->
                  <TableCell>
                    <template v-if="item.reviewedByName || item.reviewedBy">
                      <div class="flex flex-col gap-0.5">
                        <span class="text-xs font-medium">{{ item.reviewedByName || item.reviewedBy }}</span>
                        <span v-if="item.reviewedAt" class="text-[11px] text-muted-foreground/60">
                          {{ formatTime(item.reviewedAt) }}
                        </span>
                      </div>
                    </template>
                    <span v-else class="text-[11px] text-muted-foreground/40 italic">—</span>
                  </TableCell>

                  <!-- Inline approve/reject for "all" tab on pending rows -->
                  <TableCell v-if="statusFilter === ''" @click.stop>
                    <div v-if="item.status === 'pending' && userStore.isPlatformAdmin" class="flex items-center gap-1.5">
                      <Button
                        size="sm"
                        variant="outline"
                        class="h-7 px-2.5 text-xs gap-1 text-emerald-600 border-emerald-500/30 hover:bg-emerald-50 dark:hover:bg-emerald-950"
                        @click="openReview(item, 'approve')"
                      >
                        <CheckCircle2 class="size-3.5" />
                        {{ t('common.action.approve') }}
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        class="h-7 px-2.5 text-xs gap-1 text-red-500 border-red-500/30 hover:bg-red-50 dark:hover:bg-red-950"
                        @click="openReview(item, 'reject')"
                      >
                        <XCircle class="size-3.5" />
                        {{ t('common.action.reject') }}
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>

                <!-- Expanded detail row -->
                <TableRow
                  v-if="expandedRow === item.id && (item.reviewNote || item.errorMessage)"
                  class="bg-muted/20 hover:bg-muted/20"
                >
                  <TableCell :colspan="statusFilter === '' ? 7 : 6" class="py-3 px-5 space-y-2">
                    <div v-if="item.reviewNote" class="text-xs text-muted-foreground flex items-start gap-1.5">
                      <FileText class="size-3.5 shrink-0 mt-0.5 text-muted-foreground/60" />
                      <span>
                        <span class="font-semibold text-foreground">{{ t('settings.approvals.reviewNoteLabel') }}</span>
                        {{ item.reviewNote }}
                      </span>
                    </div>
                    <div v-if="item.errorMessage" class="text-xs text-rose-600 dark:text-rose-400 flex items-center gap-1.5">
                      <AlertCircle class="size-3.5 shrink-0" />
                      {{ item.errorMessage }}
                    </div>
                  </TableCell>
                </TableRow>
              </template>
            </template>

            <TableRow v-else>
              <TableCell :colspan="statusFilter === '' ? 7 : 6" class="h-40 text-center text-muted-foreground">
                <div v-if="loading" class="flex items-center justify-center gap-2">
                  <Loader2 class="size-4 animate-spin" />
                  {{ t('common.status.loading') }}
                </div>
                <div v-else class="flex flex-col items-center gap-2 text-sm">
                  <Ban class="size-8 text-muted-foreground/30" />
                  {{ t('settings.approvals.noRequests') }}
                </div>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>
    </template>

    <!-- ── Pagination ─────────────────────────────────────────────── -->
    <div class="flex items-center justify-between text-sm text-muted-foreground">
      <span>{{ t('common.pagination.total', { total, page, totalPages }) }}</span>
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

  <!-- ── Review dialog ──────────────────────────────────────────── -->
  <Dialog v-model:open="reviewOpen">
    <DialogContent class="max-w-md">
      <DialogHeader>
        <DialogTitle class="flex items-center gap-2">
          <component
            :is="reviewAction === 'approve' ? CheckCircle2 : XCircle"
            :class="reviewAction === 'approve' ? 'size-5 text-emerald-500' : 'size-5 text-red-500'"
          />
          {{ reviewAction === 'approve' ? t('settings.approvals.reviewDialog.approveTitle') : t('settings.approvals.reviewDialog.rejectTitle') }}
        </DialogTitle>
      </DialogHeader>

      <div v-if="reviewTarget" class="space-y-4 text-sm">
        <!-- Summary card -->
        <div class="rounded-lg border border-border bg-muted/30 p-4 space-y-3">

          <!-- Requester -->
          <div class="flex items-center gap-3">
            <div
              class="size-9 rounded-full flex items-center justify-center shrink-0 text-white text-sm font-bold select-none"
              :class="avatarColor(reviewTarget.requestedByName || reviewTarget.requestedByEmail || reviewTarget.requestedBy)"
            >
              {{ avatarInitials(reviewTarget.requestedByName || reviewTarget.requestedByEmail || reviewTarget.requestedBy) }}
            </div>
            <div class="flex flex-col gap-0.5 min-w-0">
              <span class="text-sm font-semibold leading-none">
                {{ reviewTarget.requestedByName || reviewTarget.requestedBy }}
              </span>
              <span v-if="reviewTarget.requestedByEmail" class="text-[11px] text-muted-foreground/70">
                {{ reviewTarget.requestedByEmail }}
              </span>
            </div>
          </div>

          <div class="border-t border-border/60" />

          <!-- Resource + action -->
          <div class="flex flex-col gap-2">
            <div class="flex items-center gap-2 text-xs">
              <Shield class="size-3.5 text-muted-foreground shrink-0" />
              <span class="text-muted-foreground font-medium">{{ resourceLabel[reviewTarget.resourceType] ?? reviewTarget.resourceType }}</span>
              <span class="text-muted-foreground/40">/</span>
              <span class="font-semibold truncate">{{ reviewTarget.resourceName || '—' }}</span>
              <span
                class="ml-auto text-[11px] font-bold px-2 py-0.5 rounded-full shrink-0"
                :class="actionStyle[reviewTarget.action] ?? 'bg-muted text-muted-foreground ring-1 ring-border'"
              >{{ actionLabel[reviewTarget.action] ?? reviewTarget.action }}</span>
            </div>
            <div class="flex items-center gap-1.5 text-[11px] text-muted-foreground">
              <Clock class="size-3 shrink-0" />
              <span>{{ t('settings.approvals.reviewDialog.submittedAtText', { time: formatTime(reviewTarget.createdAt) }) }}</span>
            </div>
          </div>
        </div>

        <!-- Review note -->
        <div>
          <label class="text-xs font-semibold text-muted-foreground mb-1.5 block">
            {{ t('settings.approvals.reviewDialog.noteLabel') }}
          </label>
          <textarea
            v-model="reviewNote"
            rows="3"
            :placeholder="t('settings.approvals.reviewDialog.notePlaceholder')"
            class="w-full rounded-md border border-input bg-background px-3 py-2 text-xs resize-none focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50 transition-[color,box-shadow] placeholder:text-muted-foreground/50"
          />
        </div>
      </div>

      <DialogFooter class="gap-2">
        <Button variant="outline" size="sm" @click="reviewOpen = false">{{ t('common.action.cancel') }}</Button>
        <Button
          :variant="reviewAction === 'approve' ? 'default' : 'destructive'"
          size="sm"
          :disabled="reviewing"
          @click="submitReview"
        >
          <Loader2 v-if="reviewing" class="size-3.5 mr-1.5 animate-spin" />
          {{ reviewAction === 'approve' ? t('settings.approvals.reviewDialog.confirmApprove') : t('settings.approvals.reviewDialog.confirmReject') }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
