import request from '@/api/request'

function wsNS(): string {
  return localStorage.getItem('active_ws_id') || ''
}

// ── Types (matches backend model) ──────────────────────────────

export interface AgentIdentity {
  id: string
  tenant_id: string
  name: string
  peer_ref: string
  allowed_tools?: string // JSON array
  allowed_namespaces?: string // JSON array
  sandbox: string
  audit_level: string
  enforcement_mode: string
  expires_at?: string
  parent_ref?: string
  spawnable_roles?: string // JSON array
  phase: string
  peer_ip?: string
  last_seen_at?: string
  description?: string
  created_at?: string
  updated_at?: string
}

export interface AgentIdentityInput {
  name: string
  peer_ref: string
  allowed_tools?: string[]
  allowed_namespaces?: string[]
  sandbox?: string
  audit_level?: string
  enforcement_mode?: string
  parent_ref?: string
  spawnable_roles?: string[]
  description?: string
}

// ── API functions ────────────────────────────────────────────────

export const listAgentIdentities = (): Promise<{ data: AgentIdentity[] }> =>
  request.get('/agent-identities', { namespace: wsNS() })

export const getAgentIdentity = (id: string): Promise<{ data: AgentIdentity }> =>
  request.get(`/agent-identities/${id}`)

export const createAgentIdentity = (input: AgentIdentityInput): Promise<{ data: AgentIdentity }> =>
  request.post(`/agent-identities?namespace=${encodeURIComponent(wsNS())}`, input)

export const updateAgentIdentity = (id: string, input: AgentIdentityInput): Promise<{ data: AgentIdentity }> =>
  request.put(`/agent-identities/${id}`, input)

export const deleteAgentIdentity = (id: string): Promise<void> =>
  request.delete(`/agent-identities/${id}`)
