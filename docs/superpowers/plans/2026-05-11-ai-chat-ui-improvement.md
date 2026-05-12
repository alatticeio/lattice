# AI Chat UI 优化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 优化 AI Chat 页面的消息布局、Markdown 渲染质量、交互细节和空状态页。

**Architecture:** 用 `marked` + `highlight.js` 替换手写正则渲染器；MessageBubble 改为 ChatGPT 风格（用户气泡右、AI 正文左）；ChatWindow 增加顶部标题栏；SuggestedPrompts 改为分组列表；Message 接口增加 `createdAt` 时间戳字段。

**Tech Stack:** Vue 3.5, Pinia, Vitest + @vue/test-utils, `marked` ^15, `highlight.js` ^11, Tailwind CSS 4

---

## File Map

| 文件 | 改动 |
|------|------|
| `fronted/package.json` | 新增 `marked`、`highlight.js` |
| `fronted/src/main.ts` | 引入 highlight.js 主题 CSS |
| `fronted/src/stores/useAiStore.ts` | Message 接口增加 `createdAt: number` |
| `fronted/src/components/ai/ChatInput.vue` | 修复 nextTick 导入 bug |
| `fronted/src/components/ai/ChatWindow.vue` | 增加顶部标题栏 |
| `fronted/src/components/ai/MessageBubble.vue` | 重写：新布局 + marked + 复制 + 时间戳 |
| `fronted/src/components/ai/SuggestedPrompts.vue` | 重写：分组列表布局 |
| `fronted/src/components/ai/__tests__/useAiStore.spec.ts` | 新建：Message createdAt 测试 |

---

## Task 1: 安装依赖

**Files:**
- Modify: `fronted/package.json`

- [ ] **Step 1: 安装 marked 和 highlight.js**

```bash
cd fronted && pnpm add marked highlight.js
```

Expected output: 类似 `+ marked 15.x.x` `+ highlight.js 11.x.x`

- [ ] **Step 2: 验证安装**

```bash
cd fronted && node -e "require('marked'); require('highlight.js'); console.log('OK')"
```

Expected: `OK`

- [ ] **Step 3: 在 main.ts 引入 highlight.js 主题 CSS**

在 `fronted/src/main.ts` 第 5 行（`import './style.css'` 之后）新增：

```typescript
import './style.css'
import 'highlight.js/styles/github-dark-dimmed.min.css'
```

- [ ] **Step 4: 提交**

```bash
cd fronted && git add package.json pnpm-lock.yaml src/main.ts && git commit -s -m "feat(ai-chat): install marked and highlight.js"
```

---

## Task 2: 给 Message 增加 createdAt 字段

**Files:**
- Modify: `fronted/src/stores/useAiStore.ts`
- Create: `fronted/src/components/ai/__tests__/useAiStore.spec.ts`

- [ ] **Step 1: 写失败测试**

新建 `fronted/src/components/ai/__tests__/useAiStore.spec.ts`：

```typescript
import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAiStore } from '@/stores/useAiStore'

describe('useAiStore - Message timestamps', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
  })

  it('addUserMessage 设置 createdAt 时间戳', () => {
    const store = useAiStore()
    const conv = store.newConversation('ws-1')
    const before = Date.now()
    const msg = store.addUserMessage(conv.id, 'hello')
    const after = Date.now()
    expect(msg.createdAt).toBeGreaterThanOrEqual(before)
    expect(msg.createdAt).toBeLessThanOrEqual(after)
  })

  it('startAssistantMessage 设置 createdAt 时间戳', () => {
    const store = useAiStore()
    const conv = store.newConversation('ws-1')
    store.addUserMessage(conv.id, 'hello')
    const before = Date.now()
    const msg = store.startAssistantMessage(conv.id)
    const after = Date.now()
    expect(msg.createdAt).toBeGreaterThanOrEqual(before)
    expect(msg.createdAt).toBeLessThanOrEqual(after)
  })
})
```

- [ ] **Step 2: 运行确认测试失败**

```bash
cd fronted && pnpm test -- src/components/ai/__tests__/useAiStore.spec.ts
```

Expected: FAIL — `msg.createdAt` is `undefined`

- [ ] **Step 3: 在 Message 接口增加 createdAt，并在两个函数中填充**

修改 `fronted/src/stores/useAiStore.ts`：

```typescript
export interface Message {
  id: string
  role: 'user' | 'assistant'
  content: string
  toolCalls: ToolCall[]
  isStreaming: boolean
  error?: string
  createdAt: number   // 新增
}
```

在 `addUserMessage` 函数中（第 81-87 行的 msg 对象）：

```typescript
  function addUserMessage(conversationId: string, content: string): Message {
    const msg: Message = {
      id: nanoid(),
      role: 'user',
      content,
      toolCalls: [],
      isStreaming: false,
      createdAt: Date.now(),   // 新增
    }
```

在 `startAssistantMessage` 函数中（第 101-107 行的 msg 对象）：

```typescript
  function startAssistantMessage(conversationId: string): Message {
    const msg: Message = {
      id: nanoid(),
      role: 'assistant',
      content: '',
      toolCalls: [],
      isStreaming: true,
      createdAt: Date.now(),   // 新增
    }
```

- [ ] **Step 4: 运行确认测试通过**

```bash
cd fronted && pnpm test -- src/components/ai/__tests__/useAiStore.spec.ts
```

Expected: PASS (2 tests)

- [ ] **Step 5: 提交**

```bash
cd fronted && git add src/stores/useAiStore.ts src/components/ai/__tests__/useAiStore.spec.ts && git commit -s -m "feat(ai-chat): add createdAt timestamp to Message"
```

---

## Task 3: 修复 ChatInput.vue nextTick 导入 bug

**Files:**
- Modify: `fronted/src/components/ai/ChatInput.vue`

当前代码有两个 script 块，`nextTick` 在第二个 `<script lang="ts">` 中导入。需要合并到 `<script setup>` 中。

- [ ] **Step 1: 替换 ChatInput.vue 全部内容**

将 `fronted/src/components/ai/ChatInput.vue` 改为：

```vue
<script setup lang="ts">
import { ref, computed, nextTick } from 'vue'
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
```

- [ ] **Step 2: 确认项目可以编译**

```bash
cd fronted && pnpm build 2>&1 | tail -5
```

Expected: 无 TypeScript 错误

- [ ] **Step 3: 提交**

```bash
cd fronted && git add src/components/ai/ChatInput.vue && git commit -s -m "fix(ai-chat): merge nextTick import into script setup"
```

---

## Task 4: ChatWindow 增加顶部标题栏

**Files:**
- Modify: `fronted/src/components/ai/ChatWindow.vue`

- [ ] **Step 1: 替换 ChatWindow.vue 全部内容**

```vue
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
    <!-- Title bar -->
    <div
      v-if="activeConv"
      class="flex h-12 shrink-0 items-center border-b border-border px-6"
    >
      <span class="text-sm font-medium text-foreground truncate">{{ activeConv.title }}</span>
    </div>

    <!-- Message area -->
    <div ref="scrollEl" class="flex-1 overflow-y-auto">
      <template v-if="activeConv && activeConv.messages.length > 0">
        <div class="py-4">
          <MessageBubble
            v-for="msg in activeConv.messages"
            :key="msg.id"
            :message="msg"
          />
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
```

- [ ] **Step 2: 确认编译**

```bash
cd fronted && pnpm build 2>&1 | tail -5
```

Expected: 无错误

- [ ] **Step 3: 提交**

```bash
cd fronted && git add src/components/ai/ChatWindow.vue && git commit -s -m "feat(ai-chat): add title bar to chat window"
```

---

## Task 5: 重写 MessageBubble.vue（布局 + marked + 复制 + 时间戳）

**Files:**
- Modify: `fronted/src/components/ai/MessageBubble.vue`

这是改动最大的一个任务。新布局：
- 用户消息：右侧气泡，统一在 `max-w-3xl` 容器中右对齐
- AI 消息：无背景正文，`max-w-3xl` 容器中左对齐，顶部显示 "Lattice AI · 时间"
- Markdown 由 `marked` 解析，代码块由 `highlight.js` 高亮，带复制按钮
- hover AI 消息时右上角出现"复制"按钮
- hover 时在消息旁显示时间戳

- [ ] **Step 1: 替换 MessageBubble.vue 全部内容**

```vue
<script setup lang="ts">
import { computed, ref } from 'vue'
import { Copy, Check } from 'lucide-vue-next'
import { marked } from 'marked'
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
const renderedContent = computed(() => marked.parse(props.message.content) as string)

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
  const original = btn.textContent
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
```

- [ ] **Step 2: 确认编译无报错**

```bash
cd fronted && pnpm build 2>&1 | grep -E "error|Error" | head -20
```

Expected: 无输出（或只有警告）

- [ ] **Step 3: 提交**

```bash
cd fronted && git add src/components/ai/MessageBubble.vue && git commit -s -m "feat(ai-chat): rewrite MessageBubble with new layout, marked, copy and timestamps"
```

---

## Task 6: 重写 SuggestedPrompts.vue（分组列表）

**Files:**
- Modify: `fronted/src/components/ai/SuggestedPrompts.vue`

- [ ] **Step 1: 替换 SuggestedPrompts.vue 全部内容**

```vue
<script setup lang="ts">
import { Network, ShieldCheck, Search, Zap, Terminal, GitBranch } from 'lucide-vue-next'

const emit = defineEmits<{ select: [prompt: string] }>()

const groups = [
  {
    label: '常见问题',
    items: [
      { icon: Search,  text: '现在哪些 Peer 离线了？' },
      { icon: ShieldCheck, text: '分析当前工作区的安全策略' },
    ],
  },
  {
    label: '网络管理',
    items: [
      { icon: Network, text: '列出当前所有网络和它们的 CIDR' },
      { icon: Zap,     text: '为什么两个 Peer 之间无法通信？' },
    ],
  },
  {
    label: '运维诊断',
    items: [
      { icon: Terminal,   text: '查看最近的连接失败事件' },
      { icon: GitBranch,  text: '当前有哪些活跃的中继节点？' },
    ],
  },
]
</script>

<template>
  <div class="flex h-full flex-col items-center justify-center px-6">
    <!-- Heading -->
    <div class="mb-8 text-center">
      <div class="mx-auto mb-4 flex size-12 items-center justify-center rounded-2xl bg-primary/10 ring-1 ring-primary/20">
        <svg class="size-6 text-primary" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
          <path stroke-linecap="round" stroke-linejoin="round"
            d="M8.625 12a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0H8.25m4.125 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0H12m4.125 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm0 0h-.375M21 12c0 4.556-4.03 8.25-9 8.25a9.764 9.764 0 0 1-2.555-.337A5.972 5.972 0 0 1 5.41 20.97a5.969 5.969 0 0 1-.474-.065 4.48 4.48 0 0 0 .978-2.025c.09-.457-.133-.901-.467-1.226C3.93 16.178 3 14.189 3 12c0-4.556 4.03-8.25 9-8.25s9 3.694 9 8.25Z" />
        </svg>
      </div>
      <h2 class="text-2xl font-semibold tracking-tight">Lattice AI</h2>
      <p class="mt-2 text-sm text-muted-foreground">用自然语言管理 WireGuard 网络</p>
    </div>

    <!-- Grouped prompts -->
    <div class="w-full max-w-lg space-y-5">
      <div v-for="group in groups" :key="group.label">
        <p class="mb-2 text-[11px] font-medium uppercase tracking-wider text-muted-foreground/60">
          {{ group.label }}
        </p>
        <div class="flex flex-col gap-1.5">
          <button
            v-for="item in group.items"
            :key="item.text"
            class="flex items-center gap-3 rounded-lg border border-border bg-card px-4 py-2.5 text-left text-sm transition-all hover:border-primary/40 hover:bg-primary/5 hover:shadow-sm"
            @click="emit('select', item.text)"
          >
            <component :is="item.icon" class="size-3.5 shrink-0 text-muted-foreground" />
            <span class="text-foreground/80">{{ item.text }}</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
```

- [ ] **Step 2: 确认编译**

```bash
cd fronted && pnpm build 2>&1 | grep -E "error|Error" | head -20
```

Expected: 无输出

- [ ] **Step 3: 运行所有测试确认无回归**

```bash
cd fronted && pnpm test
```

Expected: 所有测试通过，包括 Task 2 新增的 useAiStore 测试

- [ ] **Step 4: 提交**

```bash
cd fronted && git add src/components/ai/SuggestedPrompts.vue && git commit -s -m "feat(ai-chat): rewrite SuggestedPrompts with grouped list layout"
```

---

## 验收检查

全部任务完成后，运行以下命令验收：

```bash
cd fronted && pnpm test && pnpm build
```

Expected:
- 所有 Vitest 测试通过
- TypeScript 编译无错误
- `internal/web/dist/` 产出新的构建产物（如需嵌入 Go 二进制：`make build-ui`）
