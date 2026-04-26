import { getToken } from '@/utils/auth'

const BASE = (import.meta.env.VITE_API_BASE as string) || '/api/v1'

export interface ChatMessage {
  role: 'user' | 'assistant'
  content: string
}

export interface StreamEvent {
  type: 'token' | 'tool_use' | 'preview' | 'error' | 'done'
  content?: string
  tool?: string
  input?: Record<string, unknown>
  error?: string
}

export interface AuditFinding {
  severity: 'high' | 'medium' | 'low'
  rule: string
  resource: string
  description: string
  suggestion: string
}

export interface AuditReport {
  score: number
  generatedAt: string
  findings: AuditFinding[]
}

/**
 * Stream a chat message via SSE (fetch + ReadableStream).
 * Calls `onEvent` for each parsed SSE event until the stream ends or errors.
 */
export async function streamChat(
  workspaceId: string,
  message: string,
  history: ChatMessage[],
  onEvent: (event: StreamEvent) => void,
  signal?: AbortSignal,
): Promise<void> {
  const token = getToken()
  const res = await fetch(`${BASE}/ai/chat`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify({ message, workspaceId, history }),
    signal,
  })

  if (!res.ok) {
    const text = await res.text()
    throw new Error(`AI chat failed (${res.status}): ${text}`)
  }

  const reader = res.body!.getReader()
  const decoder = new TextDecoder()
  let buf = ''

  while (true) {
    const { done, value } = await reader.read()
    if (done) break

    buf += decoder.decode(value, { stream: true })

    // SSE lines are separated by \n\n
    const parts = buf.split('\n\n')
    buf = parts.pop() ?? ''

    for (const part of parts) {
      for (const line of part.split('\n')) {
        if (!line.startsWith('data: ')) continue
        const data = line.slice(6).trim()
        if (!data) continue
        try {
          const event: StreamEvent = JSON.parse(data)
          onEvent(event)
        } catch {
          // ignore malformed lines
        }
      }
    }
  }
}

export async function fetchAuditReport(workspaceId: string): Promise<AuditReport> {
  const token = getToken()
  const res = await fetch(`${BASE}/ai/audit?workspaceId=${encodeURIComponent(workspaceId)}`, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(`Audit failed (${res.status}): ${text}`)
  }
  const json = await res.json()
  return json.data as AuditReport
}
