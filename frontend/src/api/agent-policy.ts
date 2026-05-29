import request from '@/api/request'

// ── Types (matches backend CRD spec) ────────────────────────────

export interface AgentToolPermission {
  mcpServer: string
  tools: string[]
}

export interface AgentPolicySpec {
  agentSelector: { matchLabels: Record<string, string> }
  allowedTools?: AgentToolPermission[]
  defaultDeny?: boolean
}

export interface AgentPolicy {
  name: string
  namespace?: string
  spec: AgentPolicySpec
  createdAt?: string
}

export interface AgentPolicyInput {
  name: string
  spec: AgentPolicySpec
}

// ── API functions ────────────────────────────────────────────────

export const listAgentPolicies = (namespace?: string): Promise<{ data: AgentPolicy[] }> =>
  request.get('/agent-policies', namespace ? { namespace } : {})

export const getAgentPolicy = (name: string): Promise<{ data: AgentPolicy }> =>
  request.get(`/agent-policies/${name}`)

export const createAgentPolicy = (input: AgentPolicyInput): Promise<{ data: AgentPolicy }> =>
  request.post('/agent-policies', input)

export const updateAgentPolicy = (name: string, spec: AgentPolicySpec): Promise<{ data: AgentPolicy }> =>
  request.put(`/agent-policies/${name}`, { spec })

export const deleteAgentPolicy = (name: string): Promise<void> =>
  request.delete(`/agent-policies/${name}`)
