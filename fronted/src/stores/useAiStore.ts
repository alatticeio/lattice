import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { nanoid } from '@/lib/nanoid'

export interface ToolCall {
  tool: string
  input: Record<string, unknown>
}

export interface Message {
  id: string
  role: 'user' | 'assistant'
  content: string
  toolCalls: ToolCall[]
  isStreaming: boolean
  error?: string
}

export interface Conversation {
  id: string
  title: string
  messages: Message[]
  workspaceId: string
  createdAt: number
  updatedAt: number
}

const STORAGE_KEY = 'wf_ai_conversations'
const MAX_CONVERSATIONS = 50

function load(): Conversation[] {
  try {
    return JSON.parse(localStorage.getItem(STORAGE_KEY) || '[]')
  } catch {
    return []
  }
}

function save(list: Conversation[]) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(list.slice(0, MAX_CONVERSATIONS)))
}

export const useAiStore = defineStore('ai', () => {
  const conversations = ref<Conversation[]>(load())
  const activeId = ref<string | null>(conversations.value[0]?.id ?? null)

  const active = computed(() =>
    conversations.value.find(c => c.id === activeId.value) ?? null,
  )

  function newConversation(workspaceId: string): Conversation {
    const c: Conversation = {
      id: nanoid(),
      title: '新对话',
      messages: [],
      workspaceId,
      createdAt: Date.now(),
      updatedAt: Date.now(),
    }
    conversations.value.unshift(c)
    activeId.value = c.id
    save(conversations.value)
    return c
  }

  function selectConversation(id: string) {
    activeId.value = id
  }

  function deleteConversation(id: string) {
    const idx = conversations.value.findIndex(c => c.id === id)
    if (idx === -1) return
    conversations.value.splice(idx, 1)
    if (activeId.value === id) {
      activeId.value = conversations.value[0]?.id ?? null
    }
    save(conversations.value)
  }

  function addUserMessage(conversationId: string, content: string): Message {
    const msg: Message = {
      id: nanoid(),
      role: 'user',
      content,
      toolCalls: [],
      isStreaming: false,
    }
    const conv = conversations.value.find(c => c.id === conversationId)
    if (!conv) throw new Error('conversation not found')
    conv.messages.push(msg)
    // Set title from first user message
    if (conv.messages.filter(m => m.role === 'user').length === 1) {
      conv.title = content.slice(0, 40) + (content.length > 40 ? '…' : '')
    }
    conv.updatedAt = Date.now()
    save(conversations.value)
    return msg
  }

  function startAssistantMessage(conversationId: string): Message {
    const msg: Message = {
      id: nanoid(),
      role: 'assistant',
      content: '',
      toolCalls: [],
      isStreaming: true,
    }
    const conv = conversations.value.find(c => c.id === conversationId)
    if (!conv) throw new Error('conversation not found')
    conv.messages.push(msg)
    return msg
  }

  function appendToken(messageId: string, conversationId: string, token: string) {
    const conv = conversations.value.find(c => c.id === conversationId)
    const msg = conv?.messages.find(m => m.id === messageId)
    if (msg) msg.content += token
  }

  function addToolCall(messageId: string, conversationId: string, toolCall: ToolCall) {
    const conv = conversations.value.find(c => c.id === conversationId)
    const msg = conv?.messages.find(m => m.id === messageId)
    if (msg) msg.toolCalls.push(toolCall)
  }

  function finalizeMessage(messageId: string, conversationId: string, error?: string) {
    const conv = conversations.value.find(c => c.id === conversationId)
    const msg = conv?.messages.find(m => m.id === messageId)
    if (msg) {
      msg.isStreaming = false
      if (error) msg.error = error
    }
    if (conv) conv.updatedAt = Date.now()
    save(conversations.value)
  }

  function clearAll() {
    conversations.value = []
    activeId.value = null
    localStorage.removeItem(STORAGE_KEY)
  }

  return {
    conversations,
    activeId,
    active,
    newConversation,
    selectConversation,
    deleteConversation,
    addUserMessage,
    startAssistantMessage,
    appendToken,
    addToolCall,
    finalizeMessage,
    clearAll,
  }
})
