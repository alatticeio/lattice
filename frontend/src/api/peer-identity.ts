import request from '@/api/request'

function wsNS(): string {
  return localStorage.getItem('active_ws_id') || ''
}

// ── Types (matches backend model) ──────────────────────────────

export interface PeerIdentity {
  id: string
  network_id: string
  name: string
  peer_ref: string
  previous_peer_ref?: string
  grace_period_seconds: number
  resolved_peer_ip?: string
  previous_peer_ip?: string
  grace_period_expires_at?: string
  description?: string
  created_at?: string
  updated_at?: string
}

export interface PeerIdentityInput {
  name: string
  peer_ref: string
  previous_peer_ref?: string
  grace_period_seconds?: number
  description?: string
}

// ── API functions ────────────────────────────────────────────────

export const listPeerIdentities = (): Promise<{ data: PeerIdentity[] }> =>
  request.get('/peer-identities', { namespace: wsNS() })

export const getPeerIdentity = (id: string): Promise<{ data: PeerIdentity }> =>
  request.get(`/peer-identities/${id}`)

export const createPeerIdentity = (input: PeerIdentityInput): Promise<{ data: PeerIdentity }> =>
  request.post(`/peer-identities?namespace=${encodeURIComponent(wsNS())}`, input)

export const updatePeerIdentity = (id: string, input: PeerIdentityInput): Promise<{ data: PeerIdentity }> =>
  request.put(`/peer-identities/${id}`, input)

export const deletePeerIdentity = (id: string): Promise<void> =>
  request.delete(`/peer-identities/${id}`)
