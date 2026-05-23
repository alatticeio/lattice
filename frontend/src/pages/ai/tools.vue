<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useWorkspaceStore } from '@/stores/workspace'
import {
  Puzzle, Search, Play, Loader2, Terminal,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter,
} from '@/components/ui/dialog'
import { listTools, callTool, type MCPTool } from '@/api/tools'

definePage({
  meta: { titleKey: 'common.ai.tools.title', descKey: 'common.ai.tools.desc' },
})

const { t } = useI18n()
const workspaceStore = useWorkspaceStore()

const tools = ref<MCPTool[]>([])
const loading = ref(false)
const searchQuery = ref('')

const filtered = computed(() => {
  const q = searchQuery.value.toLowerCase().trim()
  if (!q) return tools.value
  return tools.value.filter(t =>
    t.name.toLowerCase().includes(q) || t.description.toLowerCase().includes(q)
  )
})

// Invoke dialog
const invokeDialog = ref(false)
const invokeTool = ref<MCPTool | null>(null)
const invokeParams = ref<Record<string, string>>({})
const invokeLoading = ref(false)
const invokeResult = ref('')

function openInvoke(tool: MCPTool) {
  invokeTool.value = tool
  invokeParams.value = {}
  invokeResult.value = ''
  invokeDialog.value = true
}

async function handleInvoke() {
  const wsId = workspaceStore.currentWorkspace?.id
  if (!invokeTool.value || !wsId) return
  invokeLoading.value = true
  invokeResult.value = ''
  try {
    const input: Record<string, unknown> = {}
    for (const key in invokeParams.value) {
      input[key] = invokeParams.value[key]
    }
    const res = await callTool(wsId, invokeTool.value.name, input)
    invokeResult.value = JSON.stringify(res, null, 2)
  } catch (e: any) {
    invokeResult.value = `Error: ${e?.message || 'Unknown error'}`
  } finally {
    invokeLoading.value = false
  }
}

async function fetchTools() {
  loading.value = true
  try {
    const res = await listTools()
    tools.value = res
  } catch {
    // ignore
  } finally {
    loading.value = false
  }
}

onMounted(fetchTools)
</script>

<template>
  <div class="space-y-4 p-6">
    <!-- Search bar -->
    <div class="relative max-w-md">
      <Search class="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
      <Input
        v-model="searchQuery"
        :placeholder="t('common.ai.tools.searchPlaceholder')"
        class="h-9 pl-9"
      />
    </div>

    <!-- Loading skeleton -->
    <div v-if="loading" class="grid gap-4 md:grid-cols-2">
      <div v-for="i in 4" :key="i" class="h-28 animate-pulse rounded-xl border border-border bg-card p-4">
        <div class="mb-2 h-4 w-32 rounded bg-muted" />
        <div class="h-3 w-full rounded bg-muted" />
      </div>
    </div>

    <!-- Empty -->
    <div v-else-if="tools.length === 0 && !loading" class="flex flex-col items-center gap-2 py-16 text-sm text-muted-foreground">
      <Puzzle class="size-10 opacity-40" />
      <p>{{ t('common.ai.tools.noTools') }}</p>
    </div>

    <!-- Tool cards -->
    <div v-else class="grid gap-4 md:grid-cols-2">
      <div
        v-for="tool in filtered"
        :key="tool.name"
        class="rounded-xl border border-border bg-card p-4 transition-colors hover:border-muted-foreground/20"
      >
        <div class="mb-2 flex items-start justify-between">
          <div>
            <h4 class="font-mono text-sm font-semibold">{{ tool.name }}</h4>
            <p class="mt-0.5 text-xs text-muted-foreground">{{ tool.description }}</p>
          </div>
          <Badge variant="secondary" class="shrink-0 text-[10px]">
            {{ tool.parameters?.length || 0 }} params
          </Badge>
        </div>
        <div class="mt-3 flex justify-end">
          <Button variant="outline" size="sm" @click="openInvoke(tool)">
            <Play class="mr-1.5 size-3.5" />
            {{ t('common.ai.tools.invoke') }}
          </Button>
        </div>
      </div>

      <div v-if="filtered.length === 0" class="col-span-full py-8 text-center text-sm text-muted-foreground">
        {{ t('common.ai.tools.noMatch') }}
      </div>
    </div>

    <!-- Invoke dialog -->
    <Dialog v-model:open="invokeDialog">
      <DialogContent class="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle class="font-mono text-base">{{ invokeTool?.name }}</DialogTitle>
          <DialogDescription>{{ invokeTool?.description }}</DialogDescription>
        </DialogHeader>

        <div v-if="invokeTool?.parameters?.length" class="space-y-3">
          <div v-for="param in invokeTool.parameters" :key="param.name" class="space-y-1">
            <label class="text-xs font-medium">
              {{ param.name }}
              <span v-if="param.required" class="text-rose-500">*</span>
              <span class="ml-1 text-[10px] text-muted-foreground">({{ param.type }})</span>
            </label>
            <Input
              v-model="invokeParams[param.name]"
              :placeholder="param.description"
              size="sm"
            />
          </div>
        </div>
        <div v-else class="py-4 text-center text-sm text-muted-foreground">
          {{ t('common.ai.tools.noParams') }}
        </div>

        <div v-if="invokeResult" class="rounded-lg border border-border bg-muted/50 p-3">
          <div class="mb-1 flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
            <Terminal class="size-3.5" />
            {{ t('common.ai.tools.result') }}
          </div>
          <pre class="overflow-auto text-xs leading-relaxed">{{ invokeResult }}</pre>
        </div>

        <DialogFooter>
          <Button variant="outline" @click="invokeDialog = false">
            {{ t('common.action.close') }}
          </Button>
          <Button :disabled="invokeLoading" @click="handleInvoke">
            <Loader2 v-if="invokeLoading" class="mr-2 size-4 animate-spin" />
            <Play v-else class="mr-2 size-4" />
            {{ t('common.ai.tools.execute') }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</template>
