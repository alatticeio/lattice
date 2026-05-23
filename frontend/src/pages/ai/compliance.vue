<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useWorkspaceStore } from '@/stores/workspace'
import {
  AlertTriangle, Info, RefreshCw,
  AlertCircle, Search,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { fetchAuditReport, type AuditReport } from '@/api/ai'

definePage({
  meta: { titleKey: 'common.ai.compliance.title', descKey: 'common.ai.compliance.desc' },
})

const { t } = useI18n()
const workspaceStore = useWorkspaceStore()

const loading = ref(false)
const report = ref<AuditReport | null>(null)
const error = ref('')
const searchQuery = ref('')

const severityColor = (s: string) => {
  const map: Record<string, string> = {
    high: 'bg-rose-500/10 text-rose-600 dark:text-rose-400',
    medium: 'bg-amber-500/10 text-amber-600 dark:text-amber-400',
    low: 'bg-blue-500/10 text-blue-600 dark:text-blue-400',
  }
  return map[s] || 'bg-muted text-muted-foreground'
}

const scoreColor = computed(() => {
  if (!report.value) return 'text-muted-foreground'
  const s = report.value.score
  if (s >= 80) return 'text-emerald-500'
  if (s >= 60) return 'text-amber-500'
  return 'text-rose-500'
})

const severitySummary = computed(() => {
  const counts = { high: 0, medium: 0, low: 0 }
  report.value?.findings.forEach(f => {
    if (f.severity in counts) counts[f.severity as keyof typeof counts]++
  })
  return counts
})

const filteredFindings = computed(() => {
  if (!report.value) return []
  const q = searchQuery.value.toLowerCase().trim()
  if (!q) return report.value.findings
  return report.value.findings.filter(f =>
    f.rule.toLowerCase().includes(q) ||
    f.resource.toLowerCase().includes(q) ||
    f.description.toLowerCase().includes(q)
  )
})

async function fetchReport() {
  const wsId = workspaceStore.currentWorkspace?.id
  if (!wsId) return
  loading.value = true
  error.value = ''
  try {
    const res = await fetchAuditReport(wsId)
    report.value = res
  } catch (e: any) {
    if (e?.response?.status === 402) {
      error.value = t('common.ai.compliance.proRequired')
    } else {
      error.value = e?.message || t('common.status.error')
    }
  } finally {
    loading.value = false
  }
}

onMounted(fetchReport)
</script>

<template>
  <div class="space-y-6 p-6">
    <!-- Score overview -->
    <div v-if="loading" class="flex gap-4">
      <div v-for="i in 4" :key="i" class="h-24 flex-1 animate-pulse rounded-xl bg-muted" />
    </div>

    <div v-else-if="report" class="flex gap-4">
      <div class="flex w-40 flex-col items-center justify-center rounded-xl border border-border bg-card p-6">
        <span class="text-4xl font-bold" :class="scoreColor">{{ report.score }}</span>
        <span class="text-xs text-muted-foreground">/ 100</span>
        <Badge :class="scoreColor" class="mt-2" variant="secondary">
          {{ report.score >= 80 ? t('common.ai.compliance.good') : report.score >= 60 ? t('common.ai.compliance.fair') : t('common.ai.compliance.poor') }}
        </Badge>
      </div>
      <div class="flex flex-1 gap-3">
        <div class="flex flex-1 items-center gap-3 rounded-xl border border-border bg-card p-4">
          <div class="rounded-lg bg-rose-500/10 p-2">
            <AlertTriangle class="size-5 text-rose-500" />
          </div>
          <div>
            <div class="text-2xl font-bold text-rose-500">{{ severitySummary.high }}</div>
            <div class="text-xs text-muted-foreground">{{ t('common.ai.compliance.highRisk') }}</div>
          </div>
        </div>
        <div class="flex flex-1 items-center gap-3 rounded-xl border border-border bg-card p-4">
          <div class="rounded-lg bg-amber-500/10 p-2">
            <AlertTriangle class="size-5 text-amber-500" />
          </div>
          <div>
            <div class="text-2xl font-bold text-amber-500">{{ severitySummary.medium }}</div>
            <div class="text-xs text-muted-foreground">{{ t('common.ai.compliance.mediumRisk') }}</div>
          </div>
        </div>
        <div class="flex flex-1 items-center gap-3 rounded-xl border border-border bg-card p-4">
          <div class="rounded-lg bg-blue-500/10 p-2">
            <Info class="size-5 text-blue-500" />
          </div>
          <div>
            <div class="text-2xl font-bold text-blue-500">{{ severitySummary.low }}</div>
            <div class="text-xs text-muted-foreground">{{ t('common.ai.compliance.lowRisk') }}</div>
          </div>
        </div>
      </div>
    </div>

    <!-- Error -->
    <Alert v-if="error && !report" variant="destructive">
      <AlertCircle class="size-4" />
      <AlertTitle>{{ t('common.ai.compliance.error') }}</AlertTitle>
      <AlertDescription>{{ error }}</AlertDescription>
      <Button variant="outline" size="sm" class="mt-2" @click="fetchReport">
        <RefreshCw class="mr-2 size-3.5" />
        {{ t('common.action.retry') }}
      </Button>
    </Alert>

    <!-- Findings table -->
    <div v-if="report" class="rounded-xl border border-border bg-card">
      <div class="flex items-center justify-between border-b border-border p-4">
        <div class="relative max-w-sm">
          <Search class="absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input v-model="searchQuery" :placeholder="t('common.action.search')" class="h-8 pl-8 text-sm" />
        </div>
        <div class="flex gap-2">
          <Button variant="outline" size="sm" @click="fetchReport">
            <RefreshCw class="mr-1.5 size-3.5" />
            {{ t('common.action.refresh') }}
          </Button>
        </div>
      </div>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{{ t('common.ai.compliance.severity') }}</TableHead>
            <TableHead>{{ t('common.ai.compliance.rule') }}</TableHead>
            <TableHead>{{ t('common.ai.compliance.resource') }}</TableHead>
            <TableHead>{{ t('common.ai.compliance.description') }}</TableHead>
            <TableHead>{{ t('common.ai.compliance.suggestion') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow v-for="(f, i) in filteredFindings" :key="i">
            <TableCell>
              <span class="rounded-full px-2.5 py-0.5 text-xs font-medium" :class="severityColor(f.severity)">
                {{ f.severity.toUpperCase() }}
              </span>
            </TableCell>
            <TableCell class="font-mono text-xs">{{ f.rule }}</TableCell>
            <TableCell class="text-sm">{{ f.resource }}</TableCell>
            <TableCell class="text-sm text-muted-foreground">{{ f.description }}</TableCell>
            <TableCell class="text-sm text-muted-foreground">{{ f.suggestion }}</TableCell>
          </TableRow>
          <TableRow v-if="filteredFindings.length === 0">
            <TableCell colspan="5" class="py-8 text-center text-sm text-muted-foreground">
              {{ searchQuery ? t('common.status.empty') : t('common.ai.compliance.noFindings') }}
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </div>
  </div>
</template>
