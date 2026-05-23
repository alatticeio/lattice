import request from '@/api/request'

function wsID(): string {
  return localStorage.getItem('active_ws_id') || ''
}

export interface WorkflowRequestVo {
  id: string
  createdAt: string
  workspaceId: string
  requestedBy: string
  requestedByName: string
  requestedByEmail: string
  resourceType: string
  resourceName: string
  action: string
  status: 'pending' | 'approved' | 'rejected' | 'executed' | 'failed'
  reviewedBy?: string
  reviewedByName?: string
  reviewedAt?: string
  reviewNote?: string
  executedAt?: string
  errorMessage?: string
}

export interface WorkflowListParams {
  resourceType?: string
  action?: string
  status?: string
  page?: number
  pageSize?: number
}

export const listWorkflowRequests = (params?: WorkflowListParams) =>
  request.get(`/workspaces/${wsID()}/workflow-requests`, params ?? {})

export const getWorkflowRequest = (id: string) =>
  request.get(`/workspaces/${wsID()}/workflow-requests/${id}`, {})

export const approveWorkflowRequest = (id: string, note?: string) =>
  request.post(`/workspaces/${wsID()}/workflow-requests/${id}/approve`, { note })

export const rejectWorkflowRequest = (id: string, note?: string) =>
  request.post(`/workspaces/${wsID()}/workflow-requests/${id}/reject`, { note })
