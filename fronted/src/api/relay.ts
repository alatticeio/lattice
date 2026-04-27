import request from './request'

export interface RelayServer {
  id: string
  name: string
  description?: string
  region?: string
  tcpUrl: string
  quicUrl?: string
  enabled: boolean
  status?: 'healthy' | 'degraded' | 'offline' | 'unknown'
  latencyMs?: number
  connectedPeers?: number
  workspaces?: string[]   // workspace slugs that use this relay
  peerLabels?: string[]   // labels applied to peers using this relay, e.g. ["relay=asia-hk"]
  createdBy?: string
  createdAt: string
  updatedBy?: string
  updatedAt?: string
}

export interface CreateRelayParams {
  name: string
  displayName?: string
  description?: string
  region?: string
  tcpUrl: string
  quicUrl?: string
  enabled?: boolean
  workspaces?: string[]
  peerLabels?: string[]
}

export interface UpdateRelayParams extends Partial<CreateRelayParams> {}

export interface ListRelayParams {
  page?: number
  pageSize?: number
  keyword?: string
}

export function listRelays(params?: ListRelayParams) {
  return request.get('/settings/relays', { params })
}

export function createRelay(data: CreateRelayParams) {
  return request.post('/settings/relays', data)
}

export function updateRelay(id: string, data: UpdateRelayParams) {
  return request.put(`/settings/relays/${id}`, data)
}

export function deleteRelay(id: string) {
  return request.delete(`/settings/relays/${id}`)
}

export function testRelay(id: string) {
  return request.post(`/settings/relays/${id}/test`)
}
