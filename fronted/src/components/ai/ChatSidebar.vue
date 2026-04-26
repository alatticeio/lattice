<script setup lang="ts">
import { computed } from 'vue'
import { Plus, Trash2, MessageSquare } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { useAiStore } from '@/stores/useAiStore'
import { useWorkspaceStore } from '@/stores/workspace'

const aiStore = useAiStore()
const workspaceStore = useWorkspaceStore()

const conversations = computed(() => aiStore.conversations)
const activeId = computed(() => aiStore.activeId)

function formatTime(ts: number): string {
  const d = new Date(ts)
  const now = new Date()
  const diffMs = now.getTime() - d.getTime()
  const diffDays = Math.floor(diffMs / 86400000)
  if (diffDays === 0) return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  if (diffDays === 1) return '昨天'
  if (diffDays < 7) return `${diffDays} 天前`
  return d.toLocaleDateString()
}

function newChat() {
  const workspaceId = workspaceStore.currentWorkspace?.id ?? ''
  aiStore.newConversation(workspaceId)
}
</script>

<template>
  <div class="flex h-full w-60 shrink-0 flex-col border-r border-border bg-background">
    <!-- Header -->
    <div class="flex h-14 items-center justify-between px-4">
      <span class="text-sm font-semibold text-foreground">对话历史</span>
      <Button
        variant="ghost"
        size="icon"
        class="size-7 text-muted-foreground hover:text-foreground"
        title="新对话"
        @click="newChat"
      >
        <Plus class="size-4" />
      </Button>
    </div>

    <!-- List -->
    <div class="flex-1 overflow-y-auto px-2 pb-4">
      <!-- Empty -->
      <div
        v-if="conversations.length === 0"
        class="flex flex-col items-center justify-center py-12 text-center"
      >
        <MessageSquare class="mb-2 size-8 text-muted-foreground/40" />
        <p class="text-xs text-muted-foreground">暂无对话</p>
        <p class="mt-0.5 text-xs text-muted-foreground/60">点击 + 开始新对话</p>
      </div>

      <div
        v-for="conv in conversations"
        :key="conv.id"
        class="group relative mb-0.5 flex cursor-pointer items-start rounded-lg px-3 py-2.5 transition-colors"
        :class="conv.id === activeId
          ? 'bg-primary/10 text-foreground'
          : 'text-muted-foreground hover:bg-muted hover:text-foreground'"
        @click="aiStore.selectConversation(conv.id)"
      >
        <div class="min-w-0 flex-1">
          <p class="truncate text-[13px] font-medium leading-tight" :class="conv.id === activeId ? 'text-foreground' : ''">
            {{ conv.title }}
          </p>
          <p class="mt-0.5 text-[11px] opacity-60">{{ formatTime(conv.updatedAt) }}</p>
        </div>

        <button
          class="ml-1 shrink-0 rounded p-0.5 opacity-0 transition-opacity group-hover:opacity-100 hover:text-destructive"
          @click.stop="aiStore.deleteConversation(conv.id)"
        >
          <Trash2 class="size-3" />
        </button>
      </div>
    </div>
  </div>
</template>
