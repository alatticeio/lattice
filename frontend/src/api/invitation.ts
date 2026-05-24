import request from '@/api/request'

function wsID(): string {
  return localStorage.getItem('active_ws_id') || ''
}

export interface InvitationVo {
  id: string
  workspaceId: string
  inviterId: string
  email: string
  role: 'admin' | 'editor' | 'member' | 'viewer'
  token: string
  status: 'pending' | 'accepted' | 'expired' | 'revoked'
  expiresAt: string
  createdAt: string
}

export interface InvitePreviewVo {
  email: string
  workspaceId: string
  workspaceName: string
  inviterName: string
  inviterEmail: string
  role: string
  expiresAt: string
  status: string
}

export const listInvitations = (params?: any) =>
  request.get(`/workspaces/${wsID()}/invitations`, params)

export const createInvitation = (data: { email: string; role: string }) =>
  request.post(`/workspaces/${wsID()}/invitations`, data)

export const revokeInvitation = (invID: string) =>
  request.delete(`/workspaces/${wsID()}/invitations/${invID}`)

// Public — no auth required
export const previewInvitation = (token: string) =>
  request.get(`/invite/${token}`, {})

export const acceptInvitation = (token: string) =>
  request.post(`/invite/${token}/accept`, {})

export const registerAndAccept = (token: string, data: { username: string; password: string }) =>
  request.post(`/invite/${token}/register`, data)
