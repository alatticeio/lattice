import { getToken } from '@/utils/auth'

const BASE = (import.meta.env.VITE_API_BASE as string) || '/api/v1'

export interface DebugStreamEvent {
  type: 'token' | 'tool_use' | 'preview' | 'error' | 'done'
  content?: string
  tool?: string
  input?: Record<string, unknown>
  error?: string
}

export async function streamDebug(
  workspaceId: string,
  question: string,
  from?: string,
  to?: string,
  onEvent?: (event: DebugStreamEvent) => void,
  signal?: AbortSignal,
): Promise<void> {
  const token = getToken()
  const res = await fetch(`${BASE}/ai/debug`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify({ workspaceId, question, from, to }),
    signal,
  })

  if (!res.ok) {
    const text = await res.text()
    throw new Error(`AI debug failed (${res.status}): ${text}`)
  }

  const reader = res.body!.getReader()
  const decoder = new TextDecoder()
  let buf = ''

  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buf += decoder.decode(value, { stream: true })
    const parts = buf.split('\n\n')
    buf = parts.pop() ?? ''
    for (const part of parts) {
      for (const line of part.split('\n')) {
        if (!line.startsWith('data: ')) continue
        const data = line.slice(6).trim()
        if (!data) continue
        try {
          const event: DebugStreamEvent = JSON.parse(data)
          onEvent?.(event)
        } catch {
          // ignore malformed lines
        }
      }
    }
  }
}
