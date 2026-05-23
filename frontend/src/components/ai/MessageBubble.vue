<script setup lang="ts">
import { computed, ref } from 'vue'
import { Copy, Check } from 'lucide-vue-next'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import hljs from 'highlight.js/lib/core'
import bash from 'highlight.js/lib/languages/bash'
import json from 'highlight.js/lib/languages/json'
import yaml from 'highlight.js/lib/languages/yaml'
import golang from 'highlight.js/lib/languages/go'
import ToolCallCard from './ToolCallCard.vue'
import type { Message } from '@/stores/useAiStore'

hljs.registerLanguage('bash', bash)
hljs.registerLanguage('shell', bash)
hljs.registerLanguage('json', json)
hljs.registerLanguage('yaml', yaml)
hljs.registerLanguage('go', golang)

marked.use({
  renderer: {
    code({ text, lang }: { text: string; lang?: string }) {
      const language = lang && hljs.getLanguage(lang) ? lang : 'plaintext'
      let highlighted: string
      try {
        highlighted = hljs.highlight(text, { language }).value
      } catch {
        highlighted = text
          .replace(/&/g, '&amp;')
          .replace(/</g, '&lt;')
          .replace(/>/g, '&gt;')
      }
      const encoded = encodeURIComponent(text)
      return `<div class="code-block my-3 rounded-lg overflow-hidden border border-white/10">
        <div class="code-block-header flex items-center justify-between px-4 py-1.5 bg-zinc-800 border-b border-white/10">
          <span class="text-[11px] text-zinc-400 font-mono">${language}</span>
          <button class="code-copy-btn text-[11px] text-zinc-400 hover:text-white border border-zinc-600 rounded px-2 py-0.5 transition-colors" data-code="${encoded}">复制</button>
        </div>
        <pre class="overflow-x-auto bg-zinc-950 m-0"><code class="hljs language-${language} text-xs font-mono !p-4 block leading-relaxed">${highlighted}</code></pre>
      </div>`
    },
  },
})

const props = defineProps<{ message: Message }>()

const isUser = computed(() => props.message.role === 'user')
const renderedContent = computed(() =>
  DOMPurify.sanitize(marked.parse(props.message.content, { async: false }), { ALLOW_DATA_ATTR: true })
)

const formattedTime = computed(() => {
  if (!props.message.createdAt) return ''
  return new Date(props.message.createdAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
})

const messageCopied = ref(false)
function copyMessage() {
  navigator.clipboard.writeText(props.message.content)
  messageCopied.value = true
  setTimeout(() => { messageCopied.value = false }, 2000)
}

function handleCodeCopy(e: MouseEvent) {
  const btn = (e.target as HTMLElement).closest('.code-copy-btn') as HTMLElement | null
  if (!btn) return
  const code = decodeURIComponent(btn.getAttribute('data-code') ?? '')
  navigator.clipboard.writeText(code)
  const original = btn.textContent ?? '复制'
  btn.textContent = '已复制'
  setTimeout(() => { btn.textContent = original }, 2000)
}
</script>

<template>
  <!-- User message -->
  <div v-if="isUser" class="flex justify-center px-4 py-2 group">
    <div class="w-full max-w-3xl flex justify-end">
      <div class="flex flex-col items-end">
        <div
          class="rounded-2xl rounded-tr-sm bg-primary px-4 py-3 text-sm leading-relaxed text-primary-foreground shadow-sm max-w-[75%]"
        >
          <span class="whitespace-pre-wrap">{{ message.content }}</span>
        </div>
        <span
          class="mt-1 text-[11px] text-muted-foreground/50 opacity-0 group-hover:opacity-100 transition-opacity"
        >{{ formattedTime }}</span>
      </div>
    </div>
  </div>

  <!-- Assistant message -->
  <div v-else class="flex justify-center px-4 py-4 group">
    <div class="w-full max-w-3xl">
      <!-- Header: label + copy button -->
      <div class="flex items-center justify-between mb-2">
        <span class="text-[11px] font-medium text-muted-foreground">
          Lattice AI
          <span class="opacity-0 group-hover:opacity-100 transition-opacity">· {{ formattedTime }}</span>
        </span>
        <button
          v-if="message.content && !message.isStreaming"
          class="opacity-0 group-hover:opacity-100 transition-opacity flex items-center gap-1 text-[11px] text-muted-foreground hover:text-foreground"
          @click="copyMessage"
        >
          <Check v-if="messageCopied" class="size-3 text-green-500" />
          <Copy v-else class="size-3" />
          <span>{{ messageCopied ? '已复制' : '复制' }}</span>
        </button>
      </div>

      <!-- Tool calls -->
      <ToolCallCard
        v-for="(tc, i) in message.toolCalls"
        :key="i"
        :tool-call="tc"
        :streaming="message.isStreaming && i === message.toolCalls.length - 1"
        class="mb-2"
      />

      <!-- Text content -->
      <div
        v-if="message.content || message.isStreaming"
        class="text-sm leading-relaxed text-foreground"
        @click.capture="handleCodeCopy"
      >
        <div v-html="renderedContent" />
        <!-- Streaming cursor -->
        <span
          v-if="message.isStreaming && message.content"
          class="inline-block ml-0.5 h-[1em] w-0.5 bg-foreground/60 align-text-bottom animate-pulse"
        />
        <!-- Loading dots -->
        <span
          v-if="message.isStreaming && !message.content && !message.toolCalls.length"
          class="flex items-center gap-1 text-muted-foreground text-xs"
        >
          <span class="size-1.5 rounded-full bg-muted-foreground/60 animate-bounce" style="animation-delay:0ms" />
          <span class="size-1.5 rounded-full bg-muted-foreground/60 animate-bounce" style="animation-delay:150ms" />
          <span class="size-1.5 rounded-full bg-muted-foreground/60 animate-bounce" style="animation-delay:300ms" />
        </span>
      </div>

      <!-- Error -->
      <div
        v-if="message.error"
        class="flex items-start gap-2 rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2.5 text-sm text-destructive"
      >
        <span class="font-medium">出错了：</span>{{ message.error }}
      </div>
    </div>
  </div>
</template>
