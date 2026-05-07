<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useWorkspaceStore } from '@/stores/workspace'
import {
  Clock, Send, Loader2, AlertCircle, Eye, Layers,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { listSnapshots, diffSnapshots, type NetworkSnapshot, type DiffResult } from '@/api/snapshot'
import { streamDebug } from '@/api/debug'

definePage({
  meta: { titleKey: 'common.ai.debug.title', descKey: 'common.ai.debug.desc' },
})

const { t } = useI18n()
const workspaceStore = useWorkspaceStore()

// Snapshots
const snapshots = ref<NetworkSnapshot[]>([])
const snapsLoading = ref(false)
const selectedId = ref<string | null>(null)
const compareIds = ref<string[]>([])

// AI debug
const question = ref('')
const debugLoading = ref(false)
const debugResult = ref('')
const debugError = ref('')

// Snapshot diff
const diffLoading = ref(false)
const diffResult = ref<DiffResult | null>(null)

async function handleDiff() {
  const wsId = workspaceStore.currentWorkspace?.id
  if (!wsId || compareIds.value.length < 2) return
  diffLoading.value = true
  try {
    const res = await diffSnapshots(wsId, compareIds.value[0], compareIds.value[1])
    diffResult.value = res
  } catch (e: any) {
    debugError.value = e?.message || t('common.ai.debug.error')
  } finally {
    diffLoading.value = false
  }
}

const triggerBadgeColor = (type: string) => {
  const map: Record<string, string> = {
    policy_change: 'bg-blue-500/10 text-blue-600 dark:text-blue-400',
    peer_online: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
    peer_offline: 'bg-amber-500/10 text-amber-600 dark:text-amber-400',
  }
  return map[type] || 'bg-muted text-muted-foreground'
}

async function fetchSnapshots() {
  const wsId = workspaceStore.currentWorkspace?.id
  if (!wsId) return
  snapsLoading.value = true
  try {
    const res = await listSnapshots(wsId)
    snapshots.value = res
    if (snapshots.value.length > 0 && !selectedId.value) {
      selectedId.value = snapshots.value[0].id
    }
  } catch {
    // ignore
  } finally {
    snapsLoading.value = false
  }
}

function toggleCompare(id: string) {
  const idx = compareIds.value.indexOf(id)
  if (idx >= 0) {
    compareIds.value.splice(idx, 1)
  } else if (compareIds.value.length < 2) {
    compareIds.value.push(id)
  }
}

async function handleDebug() {
  const wsId = workspaceStore.currentWorkspace?.id
  if (!wsId || !question.value.trim()) return
  debugLoading.value = true
  debugResult.value = ''
  debugError.value = ''
  const msg = question.value.trim()
  question.value = ''
  try {
    diffResult.value = null
    await streamDebug(wsId, msg, undefined, undefined, (event) => {
      if (event.type === 'token' && event.content) {
        debugResult.value += event.content
      } else if (event.type === 'error' && event.error) {
        debugError.value = event.error
      } else if (event.type === 'done') {
        debugLoading.value = false
      } else if (event.type === 'tool_use') {
        debugResult.value += `\n[使用工具: ${event.tool}]\n`
      }
    })
  } catch (e: any) {
    debugError.value = e?.message || t('common.ai.debug.error')
  } finally {
    debugLoading.value = false
  }
}

onMounted(fetchSnapshots)
</script>

<template>
  <div class="flex h-[calc(100vh-var(--header-height,56px)-var(--page-header-height,0px))] overflow-hidden">
    <!-- Left panel: snapshot timeline -->
    <div class="flex w-80 shrink-0 flex-col border-r border-border bg-card">
      <div class="border-b border-border p-4">
        <h3 class="text-sm font-semibold">{{ t('common.ai.debug.timeline') }}</h3>
      </div>
      <div v-if="snapsLoading" class="flex flex-col gap-2 p-4">
        <div v-for="i in 4" :key="i" class="h-16 animate-pulse rounded-lg bg-muted" />
      </div>
      <div v-else-if="snapshots.length === 0" class="flex flex-col items-center gap-2 p-8 text-sm text-muted-foreground">
        <Clock class="size-8 opacity-50" />
        <p>{{ t('common.ai.debug.noSnapshots') }}</p>
      </div>
      <div v-else class="flex-1 space-y-1 overflow-y-auto p-2">
        <div
          v-for="snap in snapshots"
          :key="snap.id"
          class="cursor-pointer rounded-lg p-3 text-sm transition-colors hover:bg-muted/50"
          :class="selectedId === snap.id ? 'bg-muted ring-1 ring-border' : ''"
          @click="selectedId = snap.id"
        >
          <div class="mb-1 flex items-center justify-between">
            <span class="font-medium">{{ new Date(snap.capturedAt).toLocaleTimeString() }}</span>
            <input
              type="checkbox"
              :checked="compareIds.includes(snap.id)"
              class="size-3.5"
              @click.stop="toggleCompare(snap.id)"
            />
          </div>
          <div class="flex items-center gap-2">
            <span class="rounded-full px-2 py-0.5 text-[10px] font-medium" :class="triggerBadgeColor(snap.triggerType)">
              {{ snap.triggerType }}
            </span>
            <span class="text-[11px] text-muted-foreground">{{ snap.triggerBy }}</span>
          </div>
        </div>
      </div>
      <div class="border-t border-border p-2">
        <Button
          variant="outline"
          size="sm"
          class="w-full text-xs"
          :disabled="compareIds.length < 2 || diffLoading"
          @click="handleDiff"
        >
          <Layers class="mr-1 size-3.5" />
          {{ t('common.ai.debug.compareSelected') }}
        </Button>
      </div>
    </div>

    <!-- Right panel: AI debug -->
    <div class="flex flex-1 flex-col">
      <div class="flex-1 overflow-y-auto p-4">
        <Alert v-if="debugError" variant="destructive" class="mb-4">
          <AlertCircle class="size-4" />
          <AlertTitle>{{ t('common.ai.debug.error') }}</AlertTitle>
          <AlertDescription>{{ debugError }}</AlertDescription>
        </Alert>

        <div v-if="!debugResult && !debugLoading && !debugError && !diffResult" class="flex h-full flex-col items-center justify-center text-sm text-muted-foreground">
          <Eye class="mb-2 size-10 opacity-40" />
          <p>{{ t('common.ai.debug.askQuestion') }}</p>
        </div>

        <div v-if="diffResult" class="rounded-lg border border-border bg-card p-4 mb-4">
          <div class="flex items-center justify-between mb-2">
            <h4 class="text-sm font-semibold">Snapshot Diff</h4>
            <Button variant="ghost" size="icon" class="size-6" @click="diffResult = null">&times;</Button>
          </div>
          <p class="text-xs text-muted-foreground">{{ diffResult.diffNotes }}</p>
        </div>

        <div v-if="debugResult" class="prose prose-sm max-w-none dark:prose-invert whitespace-pre-wrap">
          {{ debugResult }}
        </div>
      </div>

      <div class="border-t border-border p-4">
        <div class="flex gap-2">
          <Input
            v-model="question"
            :placeholder="t('common.ai.debug.inputPlaceholder')"
            :disabled="debugLoading"
            @keyup.enter="handleDebug"
          />
          <Button :disabled="debugLoading || !question.trim()" @click="handleDebug">
            <Loader2 v-if="debugLoading" class="size-4 animate-spin" />
            <Send v-else class="size-4" />
          </Button>
        </div>
      </div>
    </div>
  </div>
</template>
