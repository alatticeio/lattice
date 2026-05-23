import request from '@/api/request'

function wsID(): string {
  return localStorage.getItem('active_ws_id') || ''
}

// ── Types ────────────────────────────────────────────────────────

export interface SandboxAgent {
  name: string
  sandboxId: string
  namespace: string
  mode: 'gvisor' | 'cgroup'
  status: 'online' | 'offline'
  vpnIP: string
  publicKey: string
  allowedTools: string[]
  trafficRx: number
  trafficTx: number
  createdAt: string
}

export interface EnrollmentToken {
  token?: string
  maskedToken: string
  expiresAt: string
  createdAt: string
  status: 'active' | 'expired' | 'revoked'
  allowedTools: string[]
}

export interface CreateTokenInput {
  allowedTools: string[]
  ttlSeconds: number
}

export interface TrafficAuditEvent {
  id: string
  timestamp: string
  sandboxName: string
  srcIP: string
  dstIP: string
  dstPort: number
  protocol: 'tcp' | 'udp'
  verdict: 'allow' | 'drop'
  detail?: string
}

export interface TrafficAuditParams {
  keyword?: string
  verdict?: 'allow' | 'drop' | ''
  from?: string
  to?: string
  page?: number
  pageSize?: number
}

// ── API functions ────────────────────────────────────────────────

export const listSandboxes = (): Promise<SandboxAgent[]> =>
  request.get(`/agent-isolation/agents?workspace=${wsID()}`)

export const revokeSandbox = (name: string): Promise<void> =>
  request.delete(`/agent-isolation/agents/${name}?workspace=${wsID()}`)

export const listTokens = (): Promise<EnrollmentToken[]> =>
  request.get(`/agent-isolation/enrollment-tokens?workspace=${wsID()}`)

export const createToken = (input: CreateTokenInput): Promise<EnrollmentToken> =>
  request.post('/agent-isolation/enrollment-tokens', { ...input, namespace: wsID() })

export const revokeToken = (token: string): Promise<void> =>
  request.delete(`/agent-isolation/enrollment-tokens/${token}?workspace=${wsID()}`)

export const listTrafficAudit = (params: TrafficAuditParams = {}): Promise<TrafficAuditEvent[]> =>
  request.get(`/workspaces/${wsID()}/audit-logs`, { ...params, type: 'traffic' })

// ── ToolSpan / FlowEvent types (agent-platform-integrated) ────────

export interface ToolSpan {
  traceId: string
  agentId: string
  parentId?: string
  namespace: string
  tool: string
  status: 'ok' | 'error' | 'blocked'
  errorMsg?: string
  durationMs: number
  startedAt: string
}

export interface FlowEvent {
  traceId: string
  agentId: string
  direction: 'egress' | 'ingress'
  dstIp: string
  dstPort: number
  bytes: number
  ts: string
}

export interface TraceListParams {
  agentId?: string
  from?: string
  to?: string
  limit?: number
}

export interface DelegateInput {
  agentName: string
  allowedTools: string[]
  ttlSeconds: number
  namespace: string
  parentAgentId: string
}

export interface DelegateResult {
  token: string
  expiresAt: string
}

// ── ToolSpan / FlowEvent API functions ────────────────────────────

export const listTraces = (agentId: string, params?: { from?: string; to?: string; limit?: number }): Promise<ToolSpan[]> =>
  request.get('/agent-isolation/audit/traces', { agentId, ...params })

export const getTrace = (traceId: string): Promise<ToolSpan> =>
  request.get(`/agent-isolation/audit/traces/${traceId}`)

export const listFlowEvents = (traceId: string): Promise<FlowEvent[]> =>
  request.get('/agent-isolation/flow-events', { traceId })

export const createDelegateToken = (input: DelegateInput): Promise<DelegateResult> =>
  request.post('/agent-isolation/enrollment-tokens', input)
