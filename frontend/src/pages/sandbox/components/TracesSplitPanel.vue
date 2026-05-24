<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Copy } from 'lucide-vue-next'
import { Skeleton } from '@/components/ui/skeleton'
import { Button } from '@/components/ui/button'
import { useDrawer } from '@/composables/useAgentDetailDrawer'
import { toast } from 'vue-sonner'

const { t } = useI18n()
const drawer = useDrawer()

function statusColor(status: string) {
  switch (status) {
    case 'ok': return 'bg-green-500'
    case 'error': return 'bg-red-500'
    case 'blocked': return 'bg-orange-500'
    default: return 'bg-gray-400'
  }
}

function timeAgo(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime()
  const sec = Math.floor(diff / 1000)
  if (sec < 60) return `${sec}s ago`
  const min = Math.floor(sec / 60)
  if (min < 60) return `${min}m ago`
  const hr = Math.floor(min / 60)
  if (hr < 24) return `${hr}h ago`
  return `${Math.floor(hr / 24)}d ago`
}

function formatTime(iso: string): string {
  return new Date(iso).toLocaleString()
}

async function copyText(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    toast.success('Copied')
  } catch {
    toast.error('Copy failed')
  }
}
</script>

<template>
  <div class="flex gap-0 h-full">
    <!-- 左侧列表 -->
    <div class="w-1/2 border-r pr-3 flex flex-col gap-2">
      <!-- 时间筛选 -->
      <div class="flex gap-2">
        <input
          type="datetime-local"
          class="flex-1 rounded-md border px-2 py-1 text-xs bg-background"
          :placeholder="'From'"
        />
        <input
          type="datetime-local"
          class="flex-1 rounded-md border px-2 py-1 text-xs bg-background"
          :placeholder="'To'"
        />
      </div>

      <!-- Loading -->
      <div v-if="drawer.tracesLoading.value" class="space-y-2">
        <Skeleton v-for="i in 5" :key="i" class="h-8 w-full" />
      </div>

      <!-- Error -->
      <div v-else-if="drawer.tracesError.value" class="flex flex-col items-center gap-2 py-8">
        <p class="text-sm text-red-500">{{ drawer.tracesError.value }}</p>
        <Button variant="ghost" size="sm" @click="drawer.openDrawer(drawer.agent.value!)">Retry</Button>
      </div>

      <!-- Empty -->
      <div v-else-if="drawer.traces.value.length === 0" class="flex items-center justify-center py-12">
        <p class="text-sm text-muted-foreground">{{ t('agent.noTraces') }}</p>
      </div>

      <!-- List -->
      <div v-else class="flex-1 overflow-y-auto space-y-0.5">
        <div
          v-for="trace in drawer.traces.value"
          :key="trace.traceId"
          class="flex items-center gap-2 py-2 px-3 cursor-pointer rounded-md hover:bg-muted/50"
          :class="{ 'bg-muted': drawer.selectedTraceId.value === trace.traceId }"
          @click="drawer.selectTrace(trace.traceId)"
        >
          <span class="w-2 h-2 rounded-full flex-shrink-0" :class="statusColor(trace.status)" />
          <span class="flex-1 truncate text-sm font-medium">{{ trace.tool }}</span>
          <span class="text-xs text-muted-foreground">{{ timeAgo(trace.startedAt) }}</span>
          <span class="text-xs text-muted-foreground tabular-nums w-10 text-right">{{ trace.durationMs }}ms</span>
        </div>
      </div>
    </div>

    <!-- 右侧详情 -->
    <div class="w-1/2 pl-3 overflow-y-auto">
      <template v-if="drawer.selectedTrace.value">
        <div class="space-y-3">
          <!-- Trace ID -->
          <div>
            <p class="text-xs font-medium text-muted-foreground mb-1">{{ t('agent.traceId') }}</p>
            <div class="flex items-center gap-1">
              <code class="text-xs bg-muted px-2 py-1 rounded">{{ drawer.selectedTrace.value.traceId }}</code>
              <Button variant="ghost" size="icon" class="size-6" @click="copyText(drawer.selectedTrace.value!.traceId)">
                <Copy class="size-3" />
              </Button>
            </div>
          </div>

          <!-- Key fields -->
          <div class="grid grid-cols-2 gap-2">
            <div>
              <p class="text-xs font-medium text-muted-foreground">Tool</p>
              <p class="text-sm">{{ drawer.selectedTrace.value.tool }}</p>
            </div>
            <div>
              <p class="text-xs font-medium text-muted-foreground">Status</p>
              <span
                v-if="drawer.selectedTrace.value.status === 'ok'"
                class="lattice-badge lattice-badge-stable"
              >ok</span>
              <span
                v-else-if="drawer.selectedTrace.value.status === 'error'"
                class="lattice-badge lattice-badge-roadmap"
              >error</span>
              <span
                v-else
                class="lattice-badge lattice-badge-pro"
              >blocked</span>
            </div>
            <div>
              <p class="text-xs font-medium text-muted-foreground">Duration</p>
              <p class="text-sm tabular-nums">{{ drawer.selectedTrace.value.durationMs }}ms</p>
            </div>
            <div>
              <p class="text-xs font-medium text-muted-foreground">Namespace</p>
              <p class="text-sm">{{ drawer.selectedTrace.value.namespace }}</p>
            </div>
          </div>

          <div>
            <p class="text-xs font-medium text-muted-foreground mb-1">Started At</p>
            <p class="text-sm">{{ formatTime(drawer.selectedTrace.value.startedAt) }}</p>
          </div>

          <!-- Error message -->
          <div
            v-if="drawer.selectedTrace.value.errorMsg"
            class="bg-red-50 border border-red-200 rounded-md p-3 dark:bg-red-950 dark:border-red-800"
          >
            <p class="text-xs font-medium text-red-700 dark:text-red-400 mb-1">Error</p>
            <p class="text-sm text-red-600 dark:text-red-300 font-mono">{{ drawer.selectedTrace.value.errorMsg }}</p>
          </div>

          <!-- Parent link -->
          <div v-if="drawer.selectedTrace.value.parentId" class="text-sm text-muted-foreground flex items-center gap-1">
            <span>{{ t('agent.parentAgent') }}:</span>
            <code class="text-xs bg-muted px-1.5 py-0.5 rounded">{{ drawer.selectedTrace.value.parentId }}</code>
          </div>
        </div>
      </template>

      <template v-else>
        <div class="flex items-center justify-center h-full text-sm text-muted-foreground">
          {{ t('agent.selectTrace') }}
        </div>
      </template>
    </div>
  </div>
</template>
