import request from '@/api/request'

function wsNS(): string {
  return localStorage.getItem('active_ws_id') || ''
}

// ── Types (matches backend CRD spec) ────────────────────────────

export interface MCPTool {
  name: string
  description?: string
  riskLevel?: 'low' | 'medium' | 'high' | 'critical'
}

export interface MCPServerSpec {
  peerName?: string
  endpoint: string
  tools?: MCPTool[]
}

export interface MCPServerStatus {
  phase?: 'Pending' | 'Ready' | 'Degraded'
  mode?: 'internal' | 'external'
  peerAddress?: string
  lastSyncedAt?: string
}

export interface MCPServer {
  name: string
  namespace?: string
  spec: MCPServerSpec
  status?: MCPServerStatus
  createdAt?: string
}

export interface MCPServerInput {
  name: string
  spec: MCPServerSpec
}

// ── API functions ────────────────────────────────────────────────

export const listMCPServers = (): Promise<{ data: MCPServer[] }> =>
  request.get('/mcp-servers', { namespace: wsNS() })

export const getMCPServer = (name: string): Promise<{ data: MCPServer }> =>
  request.get(`/mcp-servers/${name}`, { namespace: wsNS() })

export const createMCPServer = (input: MCPServerInput): Promise<{ data: MCPServer }> =>
  request.post(`/mcp-servers?namespace=${encodeURIComponent(wsNS())}`, input)

export const updateMCPServer = (name: string, spec: MCPServerSpec): Promise<{ data: MCPServer }> =>
  request.put(`/mcp-servers/${name}?namespace=${encodeURIComponent(wsNS())}`, { spec })

export const deleteMCPServer = (name: string): Promise<void> =>
  request.delete(`/mcp-servers/${name}`, { params: { namespace: wsNS() } })

export const discoverMCPTools = (endpoint: string): Promise<{ data: MCPTool[] }> =>
  request.post('/mcp-servers/discover-tools', { endpoint })
