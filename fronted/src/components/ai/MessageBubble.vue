<script setup lang="ts">
import { computed } from 'vue'
import { Bot } from 'lucide-vue-next'
import ToolCallCard from './ToolCallCard.vue'
import type { Message } from '@/stores/useAiStore'

const props = defineProps<{ message: Message }>()

const isUser = computed(() => props.message.role === 'user')

/** Minimal markdown → HTML */
function renderMarkdown(text: string): string {
  if (!text) return ''

  let html = text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')

  // Fenced code blocks
  html = html.replace(/```(\w*)\n?([\s\S]*?)```/g, (_, lang, code) =>
    `<pre class="my-3 rounded-lg bg-zinc-950/5 dark:bg-white/5 border border-border p-4 text-xs overflow-x-auto leading-relaxed"><code class="language-${lang || 'text'} font-mono">${code.trimEnd()}</code></pre>`)

  // Headings
  html = html.replace(/^### (.+)$/gm, '<h3 class="mt-4 mb-1.5 text-sm font-semibold">$1</h3>')
  html = html.replace(/^## (.+)$/gm, '<h2 class="mt-5 mb-2 text-base font-semibold">$1</h2>')
  html = html.replace(/^# (.+)$/gm, '<h1 class="mt-5 mb-2 text-lg font-bold">$1</h1>')

  // Bold + italic
  html = html.replace(/\*\*\*(.+?)\*\*\*/g, '<strong><em>$1</em></strong>')
  html = html.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
  html = html.replace(/\*(.+?)\*/g, '<em>$1</em>')

  // Inline code
  html = html.replace(/`([^`]+)`/g, '<code class="rounded-md bg-muted px-1.5 py-0.5 text-xs font-mono">$1</code>')

  // Unordered lists
  html = html.replace(/^[-•] (.+)$/gm, '<li class="ml-5 list-disc leading-relaxed">$1</li>')
  html = html.replace(/(<li[\s\S]*?<\/li>\n?)+/g, m => `<ul class="my-2 space-y-1">${m}</ul>`)

  // Ordered lists
  html = html.replace(/^\d+\. (.+)$/gm, '<li class="ml-5 list-decimal leading-relaxed">$1</li>')

  // Paragraphs
  html = html.replace(/\n\n+/g, '</p><p class="mt-3">')
  html = `<p>${html}</p>`

  // Single newlines
  html = html.replace(/\n(?!<)/g, '<br>')

  return html
}
</script>

<template>
  <!-- User message -->
  <div v-if="isUser" class="flex justify-end px-4 py-2 group">
    <div class="max-w-[70%]">
      <div class="rounded-2xl rounded-tr-sm bg-primary px-4 py-3 text-sm leading-relaxed text-primary-foreground shadow-sm">
        <span class="whitespace-pre-wrap">{{ message.content }}</span>
      </div>
    </div>
  </div>

  <!-- Assistant message -->
  <div v-else class="px-4 py-4 group">
    <div class="mx-auto max-w-3xl flex gap-4">
      <!-- Avatar -->
      <div class="mt-0.5 flex size-7 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary ring-1 ring-primary/20">
        <Bot class="size-3.5" />
      </div>

      <!-- Content -->
      <div class="min-w-0 flex-1 space-y-2">
        <!-- Tool calls -->
        <ToolCallCard
          v-for="(tc, i) in message.toolCalls"
          :key="i"
          :tool-call="tc"
          :streaming="message.isStreaming && i === message.toolCalls.length - 1"
        />

        <!-- Text -->
        <div
          v-if="message.content || message.isStreaming"
          class="text-sm leading-relaxed text-foreground"
        >
          <div v-html="renderMarkdown(message.content)" />
          <span
            v-if="message.isStreaming && message.content"
            class="inline-block ml-0.5 h-[1em] w-0.5 bg-foreground/60 align-text-bottom animate-pulse"
          />
          <!-- Empty streaming indicator -->
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
  </div>
</template>
