<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { useAiStore } from '@/stores/useAiStore'
import { streamChat } from '@/api/ai'
import { useWorkspaceStore } from '@/stores/workspace'
import MessageBubble from './MessageBubble.vue'
import ChatInput from './ChatInput.vue'
import SuggestedPrompts from './SuggestedPrompts.vue'

const aiStore = useAiStore()
const workspaceStore = useWorkspaceStore()

const scrollEl = ref<HTMLElement | null>(null)
const abortController = ref<AbortController | null>(null)

const activeConv = computed(() => aiStore.active)
const loading = computed(() => activeConv.value?.messages.some(m => m.isStreaming) ?? false)

function scrollToBottom() {
  nextTick(() => {
    if (scrollEl.value) {
      scrollEl.value.scrollTop = scrollEl.value.scrollHeight
    }
  })
}

watch(
  () => activeConv.value?.messages.length,
  () => scrollToBottom(),
)

async function handleSend(text: string) {
  const workspaceId = workspaceStore.currentWorkspace?.id ?? ''
  let convId = activeConv.value?.id

  if (!convId) {
    const c = aiStore.newConversation(workspaceId)
    convId = c.id
  }

  aiStore.addUserMessage(convId, text)
  scrollToBottom()

  const assistantMsg = aiStore.startAssistantMessage(convId)
  abortController.value = new AbortController()

  const history = (activeConv.value?.messages ?? [])
    .slice(0, -1)
    .filter(m => !m.isStreaming)
    .map(m => ({ role: m.role as 'user' | 'assistant', content: m.content }))

  try {
    await streamChat(
      workspaceId,
      text,
      history,
      (event) => {
        if (event.type === 'token' && event.content) {
          aiStore.appendToken(assistantMsg.id, convId!, event.content)
          scrollToBottom()
        } else if (event.type === 'tool_use' && event.tool) {
          aiStore.addToolCall(assistantMsg.id, convId!, {
            tool: event.tool,
            input: event.input ?? {},
          })
          scrollToBottom()
        } else if (event.type === 'error') {
          aiStore.finalizeMessage(assistantMsg.id, convId!, event.error)
        }
      },
      abortController.value.signal,
    )
    aiStore.finalizeMessage(assistantMsg.id, convId!)
  } catch (err: unknown) {
    if (err instanceof Error && err.name === 'AbortError') {
      aiStore.finalizeMessage(assistantMsg.id, convId!)
    } else {
      const msg = err instanceof Error ? err.message : String(err)
      aiStore.finalizeMessage(assistantMsg.id, convId!, msg)
    }
  } finally {
    abortController.value = null
  }
}

function handleStop() {
  abortController.value?.abort()
}
</script>

<template>
  <div class="flex h-full flex-col bg-background">
    <!-- Message area -->
    <div ref="scrollEl" class="flex-1 overflow-y-auto">
      <template v-if="activeConv && activeConv.messages.length > 0">
        <div class="py-4">
          <MessageBubble
            v-for="msg in activeConv.messages"
            :key="msg.id"
            :message="msg"
          />
          <!-- Bottom padding so last message isn't flush against input -->
          <div class="h-4" />
        </div>
      </template>
      <SuggestedPrompts v-else @select="handleSend" />
    </div>

    <!-- Input -->
    <ChatInput
      :loading="loading"
      @send="handleSend"
      @stop="handleStop"
    />
  </div>
</template>
