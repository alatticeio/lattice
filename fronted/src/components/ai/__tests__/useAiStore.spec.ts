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
