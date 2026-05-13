import request from '@/api/request'

function wsID(): string {
  return localStorage.getItem('active_ws_id') || ''
}

// ── Types ────────────────────────────────────────────────────────

export interface UserMCPTool {
  name: string
  description: string
  mcpServerURL: string
  workspaceId: string
  visibility: 'workspace' | 'private' | 'public'
  ownerId: string
  createdAt: string
}

export interface RegisterMCPToolInput {
  name: string
  description: string
  mcpServerURL: string
  visibility: 'workspace' | 'private' | 'public'
}

// ── API functions ────────────────────────────────────────────────

export const listUserMCPTools = (): Promise<UserMCPTool[]> =>
  request.get('/tools/user/list', { workspace: wsID() })

export const registerMCPTool = (input: RegisterMCPToolInput): Promise<UserMCPTool> =>
  request.post('/tools/user/register', { ...input, workspaceId: wsID() })

export const deleteMCPTool = (name: string): Promise<void> =>
  request.delete(`/tools/user/${name}`, { params: { workspace: wsID() } })
