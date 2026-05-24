import request from '@/api/request'

function wsID(): string {
  return localStorage.getItem('active_ws_id') || ''
}

export interface AuditLogVo {
  id: string
  createdAt: string
  userId: string
  userName: string
  userEmail: string
  userIP: string
  workspaceId: string
  action: string
  resource: string
  resourceId: string
  resourceName: string
  scope: string
  status: 'success' | 'failed'
  statusCode: number
  detail?: string
}

export interface AuditListParams {
  action?: string
  resource?: string
  status?: string
  keyword?: string
  from?: string
  to?: string
  page?: number
  pageSize?: number
}

export const listAuditLogs = (params?: AuditListParams) =>
  request.get(`/workspaces/${wsID()}/audit-logs`, params ?? {})
