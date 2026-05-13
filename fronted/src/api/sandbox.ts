import request from '@/api/request'

function wsID(): string {
  return localStorage.getItem('active_ws_id') || ''
}

// ── Types ────────────────────────────────────────────────────────

export interface SandboxAgent {
  name: string
  sandboxId: string
  mode: 'gvisor' | 'cgroup'
  status: 'online' | 'offline'
  vpnIP: string
  publicKey: string
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
