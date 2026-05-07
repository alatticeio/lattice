<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useWorkspaceStore } from '@/stores/workspace'
import { Zap, Loader2, AlertCircle, CheckCircle2, ArrowLeft } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { toast } from 'vue-sonner'
import { planNetworkChange, applyNetworkChange, type IntentPlanView } from '@/api/intent'

definePage({
  meta: { titleKey: 'common.ai.intent.title', descKey: 'common.ai.intent.desc' },
})

const { t } = useI18n()
const workspaceStore = useWorkspaceStore()

const intent = ref('')
const loading = ref(false)
const applying = ref(false)
const plan = ref<IntentPlanView | null>(null)
const error = ref('')

async function handlePlan() {
  if (!intent.value.trim() || !workspaceStore.currentWorkspace?.id) return
  loading.value = true
  error.value = ''
  plan.value = null
  try {
    const res = await planNetworkChange({
      workspaceId: workspaceStore.currentWorkspace.id,
      intent: intent.value.trim(),
      dryRun: true,
    })
    plan.value = res
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
        <div class="flex justify-end gap-2">
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

    <!-- Error -->
    <Alert v-if="error" variant="destructive">
      <AlertCircle class="size-4" />
      <AlertTitle>{{ t('common.ai.intent.planError') }}</AlertTitle>
      <AlertDescription>{{ error }}</AlertDescription>
    </Alert>

    <!-- Plan result -->
    <div v-if="plan" class="rounded-xl border border-border bg-card p-6">
      <div class="mb-4 flex items-center justify-between">
        <h3 class="font-semibold">{{ t('common.ai.intent.planTitle') }}</h3>
        <span class="rounded-full px-3 py-1 text-xs font-medium" :class="riskBadge(plan.riskLevel)">
          {{ t(`common.ai.intent.risk.${plan.riskLevel}`) }}
        </span>
      </div>

      <div class="mb-4 rounded-lg bg-muted/50 p-4 text-sm leading-relaxed">
        <div v-html="plan.summary" />
      </div>

      <div class="mb-4 space-y-2">
        <div
          v-for="(change, i) in plan.changes"
          :key="i"
          class="flex items-center gap-2 font-mono text-sm"
        >
          <span :class="changeColor(change.action)" class="font-bold text-base w-4 shrink-0">
            {{ changeIcon(change.action) }}
          </span>
          <span>{{ change.resource }}</span>
        </div>
      </div>

      <div class="flex justify-end gap-2 border-t border-border pt-4">
        <Button variant="outline" :disabled="applying" @click="handleCancel">
          <ArrowLeft class="mr-2 size-4" />
          {{ t('common.action.cancel') }}
        </Button>
        <Button :disabled="applying" @click="handleApply">
          <Loader2 v-if="applying" class="mr-2 size-4 animate-spin" />
          <CheckCircle2 v-else class="mr-2 size-4" />
          {{ t('common.ai.intent.submitApply') }}
        </Button>
      </div>
    </div>
  </div>
</template>
