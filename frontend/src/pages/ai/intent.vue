<script setup lang="ts">
import { ref, computed, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useWorkspaceStore } from '@/stores/workspace'
import { Zap, Loader2, AlertCircle, CheckCircle2, ArrowLeft, ChevronDown, ChevronRight, Clock, History } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { toast } from 'vue-sonner'
import { planNetworkChange, applyNetworkChange, getIntentHistory, type IntentPlanView, type IntentHistoryItem } from '@/api/intent'

definePage({
  meta: { titleKey: 'common.ai.intent.title', descKey: 'common.ai.intent.desc' },
})

const { t } = useI18n()
const workspaceStore = useWorkspaceStore()

const intent = ref('')
const dryRun = ref(true)
const loading = ref(false)
const applying = ref(false)
const plan = ref<IntentPlanView | null>(null)
const error = ref('')
const errorIsWarning = ref(false)
const expandedChanges = ref<Set<number>>(new Set())
const history = ref<IntentHistoryItem[]>([])
const historyLoading = ref(false)

async function loadHistory() {
  if (!workspaceStore.currentWorkspace?.id) return
  historyLoading.value = true
  try {
    history.value = await getIntentHistory(workspaceStore.currentWorkspace.id)
  } catch {
    // history is non-critical, silently ignore
  } finally {
    historyLoading.value = false
  }
}

loadHistory()
const now = ref(Date.now())
const ticker = setInterval(() => { now.value = Date.now() }, 1000)
onUnmounted(() => clearInterval(ticker))

const expiresIn = computed(() => {
  if (!plan.value?.expiresAt) return null
  const diff = Math.floor((new Date(plan.value.expiresAt).getTime() - now.value) / 1000)
  if (diff <= 0) return t('common.ai.intent.expired')
  const m = Math.floor(diff / 60)
  const s = diff % 60
  return m > 0 ? `${m}m ${s}s` : `${s}s`
})

function toggleChange(i: number) {
  if (expandedChanges.value.has(i)) {
    expandedChanges.value.delete(i)
  } else {
    expandedChanges.value.add(i)
  }
}

async function handlePlan() {
  if (!intent.value.trim()) return
  if (!workspaceStore.currentWorkspace?.id) {
    error.value = t('common.ai.intent.noWorkspace')
    return
  }
  loading.value = true
  error.value = ''
  errorIsWarning.value = false
  plan.value = null
  expandedChanges.value = new Set()
  try {
    const res = await planNetworkChange({
      workspaceId: workspaceStore.currentWorkspace.id,
      intent: intent.value.trim(),
      dryRun: dryRun.value,
    })
    if (!res.changes || res.changes.length === 0) {
      error.value = t('common.ai.intent.noChanges')
      errorIsWarning.value = true
    } else {
      plan.value = res
    }
  } catch (e: any) {
    plan.value = null
    if (e?.response?.status === 402) {
      error.value = t('common.ai.intent.proRequired')
    } else {
      error.value = e?.message || t('common.ai.intent.planError')
    }
  } finally {
    loading.value = false
  }
}

async function handleApply() {
  if (!plan.value) return
  applying.value = true
  try {
    const res = await applyNetworkChange(plan.value.id)
    toast.success(t('common.ai.intent.applySuccess'), {
      description: (res as any)?.message || '',
    })
    plan.value = null
    intent.value = ''
    loadHistory()
  } catch (e: any) {
    toast.error(t('common.ai.intent.applyError'), {
      description: e?.message || '',
    })
  } finally {
    applying.value = false
  }
}

function handleCancel() {
  plan.value = null
  error.value = ''
}

const riskBadge = (level: string) => {
  const map: Record<string, string> = {
    low: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
    medium: 'bg-amber-500/10 text-amber-600 dark:text-amber-400',
    high: 'bg-rose-500/10 text-rose-600 dark:text-rose-400',
  }
  return map[level] || map.low
}

const changeIcon = (action: string) => {
  if (action === 'create') return '+'
  if (action === 'delete') return '-'
  return '~'
}

const changeColor = (action: string) => {
  if (action === 'create') return 'text-emerald-500'
  if (action === 'delete') return 'text-rose-500'
  return 'text-blue-500'
}
</script>

<template>
  <div class="mx-auto max-w-3xl space-y-6 p-6">
    <!-- Input area -->
    <div class="rounded-xl border border-border bg-card p-6">
      <h3 class="mb-1 font-semibold">{{ t('common.ai.intent.inputTitle') }}</h3>
      <p class="text-muted-foreground mb-4 text-sm">{{ t('common.ai.intent.inputDesc') }}</p>
      <div class="space-y-3">
        <Textarea
          v-model="intent"
          :placeholder="t('common.ai.intent.placeholder')"
          :disabled="loading"
          :rows="3"
          class="resize-none"
        />
        <div class="flex items-center justify-between">
          <label class="flex cursor-pointer items-center gap-2 text-sm text-muted-foreground select-none">
            <input type="checkbox" v-model="dryRun" class="accent-primary" />
            {{ t('common.ai.intent.dryRun') }}
          </label>
          <div class="flex gap-2">
            <Button variant="outline" :disabled="loading" @click="intent = ''">
              {{ t('common.action.clear') }}
            </Button>
            <Button :disabled="loading || !intent.trim()" @click="handlePlan">
              <Loader2 v-if="loading" class="mr-2 size-4 animate-spin" />
              <Zap v-else class="mr-2 size-4" />
              {{ t('common.ai.intent.generatePlan') }}
            </Button>
          </div>
        </div>
      </div>
    </div>

    <!-- Error / Warning -->
    <Alert v-if="error" :variant="errorIsWarning ? 'default' : 'destructive'">
      <AlertCircle class="size-4" />
      <AlertDescription>{{ error }}</AlertDescription>
    </Alert>

    <!-- Plan result -->
    <div v-if="plan" class="rounded-xl border border-border bg-card p-6">
      <div class="mb-4 flex items-center justify-between">
        <h3 class="font-semibold">{{ t('common.ai.intent.planTitle') }}</h3>
        <div class="flex items-center gap-2">
          <span v-if="expiresIn" class="flex items-center gap-1 text-xs text-muted-foreground">
            <Clock class="size-3" />
            {{ expiresIn }}
          </span>
          <span class="rounded-full px-3 py-1 text-xs font-medium" :class="riskBadge(plan.riskLevel)">
            {{ t(`common.ai.intent.risk.${plan.riskLevel}`) }}
          </span>
        </div>
      </div>

      <div class="mb-4 rounded-lg bg-muted/50 p-4 text-sm leading-relaxed">
        <div v-html="plan.summary" />
      </div>

      <!-- Changes with expandable YAML diff -->
      <div class="mb-4 space-y-1">
        <div v-for="(change, i) in plan.changes" :key="i" class="rounded-lg border border-border overflow-hidden">
          <!-- Change header row -->
          <button
            class="flex w-full items-center gap-2 px-3 py-2 text-sm hover:bg-muted/50 transition-colors"
            @click="toggleChange(i)"
          >
            <span :class="changeColor(change.action)" class="font-bold font-mono text-base w-4 shrink-0">
              {{ changeIcon(change.action) }}
            </span>
            <span class="flex-1 text-left font-mono">{{ change.resource }}</span>
            <ChevronDown v-if="expandedChanges.has(i)" class="size-3.5 text-muted-foreground shrink-0" />
            <ChevronRight v-else class="size-3.5 text-muted-foreground shrink-0" />
          </button>

          <!-- YAML diff -->
          <div v-if="expandedChanges.has(i)" class="border-t border-border">
            <!-- Before (delete / update) -->
            <div v-if="change.before" class="border-b border-border last:border-0">
              <div class="bg-rose-500/5 px-3 py-1 text-xs font-medium text-rose-500">Before</div>
              <pre class="overflow-x-auto bg-rose-500/5 px-3 py-2 text-xs leading-relaxed text-rose-700 dark:text-rose-300">{{ change.before }}</pre>
            </div>
            <!-- After (create / update) -->
            <div v-if="change.after">
              <div class="bg-emerald-500/5 px-3 py-1 text-xs font-medium text-emerald-600">After</div>
              <pre class="overflow-x-auto bg-emerald-500/5 px-3 py-2 text-xs leading-relaxed text-emerald-700 dark:text-emerald-300">{{ change.after }}</pre>
            </div>
          </div>
        </div>
      </div>

      <div class="flex items-center justify-between border-t border-border pt-4">
        <p v-if="dryRun" class="text-xs text-muted-foreground">
          {{ t('common.ai.intent.dryRunHint') }}
        </p>
        <div class="ml-auto flex gap-2">
          <Button variant="outline" :disabled="applying" @click="handleCancel">
            <ArrowLeft class="mr-2 size-4" />
            {{ t('common.action.cancel') }}
          </Button>
          <Button v-if="!dryRun" :disabled="applying" @click="handleApply">
            <Loader2 v-if="applying" class="mr-2 size-4 animate-spin" />
            <CheckCircle2 v-else class="mr-2 size-4" />
            {{ t('common.ai.intent.submitApply') }}
          </Button>
        </div>
      </div>
    </div>

    <!-- History -->
    <div class="rounded-xl border border-border bg-card p-6">
      <div class="mb-4 flex items-center gap-2">
        <History class="size-4 text-muted-foreground" />
        <h3 class="font-semibold">{{ t('common.ai.intent.historyTitle') }}</h3>
        <Loader2 v-if="historyLoading" class="ml-auto size-3.5 animate-spin text-muted-foreground" />
      </div>

      <div v-if="!historyLoading && history.length === 0" class="py-6 text-center text-sm text-muted-foreground">
        {{ t('common.ai.intent.historyEmpty') }}
      </div>

      <div v-else class="space-y-3">
        <div
          v-for="item in history"
          :key="item.id"
          class="rounded-lg border border-border p-4"
        >
          <div class="mb-2 flex items-start justify-between gap-2">
            <p class="text-sm font-medium leading-snug">{{ item.intent }}</p>
            <span class="shrink-0 rounded-full px-2 py-0.5 text-xs font-medium" :class="riskBadge(item.riskLevel)">
              {{ t(`common.ai.intent.risk.${item.riskLevel}`) }}
            </span>
          </div>
          <p class="text-xs text-muted-foreground line-clamp-2">{{ item.summary }}</p>
          <div class="mt-2 flex items-center gap-3 text-xs text-muted-foreground">
            <span>{{ t('common.ai.intent.historyBy') }} {{ item.appliedBy }}</span>
            <span v-if="item.appliedAt">{{ new Date(item.appliedAt).toLocaleString() }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
