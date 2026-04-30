<script setup lang="ts">
import { ref, computed } from 'vue'
import { ArrowUp, Square } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'

const props = defineProps<{
  loading: boolean
  disabled?: boolean
}>()

const emit = defineEmits<{
  send: [message: string]
  stop: []
}>()

const input = ref('')

const canSend = computed(() => input.value.trim().length > 0 && !props.loading)

function handleSend() {
  if (!canSend.value) return
  const msg = input.value.trim()
  input.value = ''
  // Reset height
  nextTick(() => {
    const el = document.querySelector('.chat-textarea') as HTMLTextAreaElement
    if (el) el.style.height = 'auto'
  })
  emit('send', msg)
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault()
    handleSend()
  }
}

function autoResize(e: Event) {
  const el = e.target as HTMLTextAreaElement
  el.style.height = 'auto'
  el.style.height = Math.min(el.scrollHeight, 200) + 'px'
}
</script>

<script lang="ts">
import { nextTick } from 'vue'
</script>

<template>
  <div class="border-t border-border bg-background/95 px-4 py-4 backdrop-blur">
    <div class="mx-auto max-w-3xl">
      <div
        class="relative rounded-2xl border border-border bg-card shadow-sm transition-shadow focus-within:shadow-md focus-within:border-primary/40"
      >
        <textarea
          v-model="input"
          :disabled="disabled || loading"
          rows="1"
          placeholder="给 Lattice AI 发消息… (Enter 发送，Shift+Enter 换行)"
          class="chat-textarea w-full resize-none bg-transparent px-4 py-3.5 pr-14 text-sm placeholder:text-muted-foreground/60 focus:outline-none disabled:opacity-50 max-h-[200px] leading-relaxed"
          @keydown="handleKeydown"
          @input="autoResize"
        />

        <!-- Send / Stop -->
        <div class="absolute bottom-2.5 right-2.5">
          <Button
            v-if="!loading"
            :disabled="!canSend"
            size="icon"
            class="size-8 rounded-xl"
            @click="handleSend"
          >
            <ArrowUp class="size-4" />
          </Button>
          <Button
            v-else
            size="icon"
            variant="outline"
            class="size-8 rounded-xl"
            @click="emit('stop')"
          >
            <Square class="size-3.5 fill-current" />
          </Button>
        </div>
      </div>

      <p class="mt-2 text-center text-xs text-muted-foreground/60">
        AI 可能会出错，请对重要操作进行二次确认
      </p>
    </div>
  </div>
</template>
