<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Plus, Copy } from 'lucide-vue-next'
import { Skeleton } from '@/components/ui/skeleton'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter,
} from '@/components/ui/dialog'
import { useDrawer } from '@/composables/useAgentDetailDrawer'
import { toast } from 'vue-sonner'

const { t } = useI18n()
const drawer = useDrawer()

// ── 委托 Dialog 本地状态 ──
const agentName = ref('')
const selectedTools = ref<string[]>([])
const ttlSeconds = ref(900)

const allSelected = computed(() => {
  const parent = drawer.agent.value?.allowedTools ?? []
  return parent.length > 0 && selectedTools.value.length === parent.length
})

const canSubmit = computed(() =>
  agentName.value.trim().length > 0 &&
  selectedTools.value.length > 0 &&
  !drawer.delegateSubmitting.value
)

function toggleAll() {
  const parent = drawer.agent.value?.allowedTools ?? []
  if (allSelected.value) {
    selectedTools.value = []
  } else {
    selectedTools.value = [...parent]
  }
}

function handleSubmit() {
  drawer.submitDelegate({
    agentName: agentName.value.trim(),
    allowedTools: [...selectedTools.value],
    ttlSeconds: ttlSeconds.value,
  })
}

function handleClose() {
  agentName.value = ''
  selectedTools.value = []
  ttlSeconds.value = 900
  drawer.delegateDialogOpen.value = false
}

async function copyToken() {
  if (drawer.delegateResult.value) {
    try {
      await navigator.clipboard.writeText(drawer.delegateResult.value.token)
      toast.success('Copied')
    } catch {
      toast.error('Copy failed')
    }
  }
}

const statusClass = (status: string) =>
  status === 'online'
    ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400'
    : 'bg-muted text-muted-foreground'
</script>

<template>
  <div class="space-y-4">
    <!-- 头部 -->
    <div class="flex items-center justify-between">
      <span class="text-sm text-muted-foreground">
        {{ drawer.subAgents.value.length }} sub-agent{{ drawer.subAgents.value.length !== 1 ? 's' : '' }}
      </span>
      <Button variant="outline" size="sm" @click="drawer.openDelegateDialog()">
        <Plus class="w-4 h-4 mr-1" />
        {{ t('agent.delegate') }}
      </Button>
    </div>

    <!-- 加载 / 错误 -->
    <div v-if="drawer.subAgentsLoading.value" class="space-y-2">
      <Skeleton v-for="i in 2" :key="i" class="h-16 w-full" />
    </div>
    <div v-else-if="drawer.subAgentsError.value" class="text-sm text-red-500 py-4 flex items-center gap-2">
      {{ drawer.subAgentsError.value }}
      <Button variant="link" size="sm" @click="drawer.loadSubAgents()">Retry</Button>
    </div>
    <div v-else-if="drawer.subAgents.value.length === 0" class="text-sm text-muted-foreground py-8 text-center">
      {{ t('agent.noSubAgents') }}
    </div>

    <!-- 卡片列表 -->
    <div v-else class="space-y-2">
      <div
        v-for="sub in drawer.subAgents.value"
        :key="sub.name"
        class="border rounded-lg p-3 space-y-2 cursor-pointer hover:bg-muted/30"
        @click="drawer.openDrawer(sub)"
      >
        <div class="flex items-center justify-between">
          <span class="text-sm font-medium font-mono">{{ sub.name }}</span>
          <Badge :class="statusClass(sub.status)" variant="secondary">
            {{ sub.status }}
          </Badge>
        </div>
        <div class="flex flex-wrap gap-1">
          <Badge v-for="tool in sub.allowedTools" :key="tool" variant="secondary" class="text-xs">
            {{ tool }}
          </Badge>
        </div>
      </div>
    </div>

    <!-- 委托 Dialog -->
    <Dialog :open="drawer.delegateDialogOpen.value" @update:open="(v: boolean) => !v && handleClose()">
      <DialogContent class="sm:max-w-[480px]">
        <template v-if="!drawer.delegateResult.value">
          <!-- 阶段 1: 表单 -->
          <DialogHeader>
            <DialogTitle>{{ t('agent.delegate') }}</DialogTitle>
            <DialogDescription>
              Create a one-time enrollment token for a sub-agent.
            </DialogDescription>
          </DialogHeader>

          <div class="space-y-4 py-2">
            <div class="space-y-2">
              <Label>{{ t('agent.delegateName') }}</Label>
              <Input v-model="agentName" placeholder="sub-agent-01" :maxlength="63" />
            </div>

            <div class="space-y-2">
              <div class="flex items-center justify-between">
                <Label>{{ t('agent.delegateTools') }}</Label>
                <Button variant="ghost" size="sm" class="text-xs" @click="toggleAll">
                  {{ allSelected ? 'Deselect all' : 'Select all' }}
                </Button>
              </div>
              <div class="grid grid-cols-2 gap-1.5">
                <label
                  v-for="tool in drawer.agent.value?.allowedTools ?? []"
                  :key="tool"
                  class="flex items-center gap-2 rounded-md border px-3 py-2 cursor-pointer hover:bg-muted/50 text-sm"
                  :class="selectedTools.includes(tool) ? 'border-primary bg-primary/5' : ''"
                >
                  <input
                    type="checkbox"
                    :value="tool"
                    v-model="selectedTools"
                    class="rounded"
                  />
                  {{ tool }}
                </label>
              </div>
            </div>

            <div class="space-y-2">
              <Label>{{ t('agent.delegateTtl') }}</Label>
              <select v-model="ttlSeconds" class="w-full rounded-md border px-3 py-2 text-sm bg-background">
                <option :value="900">15 min</option>
                <option :value="3600">1 hour</option>
              </select>
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" @click="handleClose">Cancel</Button>
            <Button :disabled="!canSubmit" @click="handleSubmit">
              {{ drawer.delegateSubmitting.value ? 'Creating...' : 'Create Token' }}
            </Button>
          </DialogFooter>
        </template>

        <template v-else>
          <!-- 阶段 2: 结果展示 -->
          <DialogHeader>
            <DialogTitle>{{ t('agent.delegateToken') }}</DialogTitle>
            <DialogDescription>
              Token valid until {{ new Date(drawer.delegateResult.value.expiresAt).toLocaleString() }}
            </DialogDescription>
          </DialogHeader>

          <div class="space-y-4 py-2">
            <div class="bg-muted rounded-lg p-4">
              <code class="text-xs break-all font-mono select-all">{{ drawer.delegateResult.value.token }}</code>
            </div>
            <Button class="w-full" variant="outline" @click="copyToken">
              <Copy class="size-4 mr-1" /> Copy Token
            </Button>
            <p class="text-xs text-amber-600 dark:text-amber-400 text-center">
              {{ t('agent.delegateCopyWarning') }}
            </p>
          </div>

          <DialogFooter>
            <Button class="w-full" @click="handleClose">Close</Button>
          </DialogFooter>
        </template>
      </DialogContent>
    </Dialog>
  </div>
</template>
