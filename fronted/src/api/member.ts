import request from '@/api/request'

function wsID(): string {
  return localStorage.getItem('active_ws_id') || ''
}

export interface MemberVo {
  userId: string
  name: string
  email: string
  avatar: string
  role: 'admin' | 'editor' | 'member' | 'viewer'
  provider: string
  status: string
  joinedAt?: string
}

export const listMembers = (params?: any) =>
  request.get(`/workspaces/${wsID()}/members`, params)

export const addMemberToWorkspace = (targetWsID: string, userID: string, role: string) =>
  request.post(`/workspaces/${targetWsID}/members/${userID}`, { role })

export const updateMemberRole = (userID: string, role: string) =>
  request.put(`/workspaces/${wsID()}/members/${userID}`, { role })

export const removeMember = (userID: string) =>
  request.delete(`/workspaces/${wsID()}/members/${userID}`)

export const getUserWorkspaces = (userID: string) =>
  request.get(`/users/${userID}/workspaces`, {})

export const removeMemberFromWorkspace = (targetWsID: string, userID: string) =>
  request.delete(`/workspaces/${targetWsID}/members/${userID}`)

export const updateMemberRoleInWorkspace = (targetWsID: string, userID: string, role: string) =>
  request.put(`/workspaces/${targetWsID}/members/${userID}`, { role })
