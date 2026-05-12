import request from '@/api/request'

export interface IntentPlanRequest {
  workspaceId: string
  intent: string
  dryRun?: boolean
}

export interface CRDChange {
  action: string
  resource: string
  before: string  // YAML of current state (empty if new)
  after: string   // YAML of desired state (empty if deleting)
}

export interface IntentPlanView {
  id: string
  summary: string
  changes: CRDChange[]
  riskLevel: string
  expiresAt: string // ISO 8601
}

export interface IntentApplyResult {
  workflowIds: string[]
  message: string
}

export interface IntentHistoryItem {
  id: string
  intent: string
  summary: string
  riskLevel: string
  appliedAt: string | null
  appliedBy: string
}

export async function planNetworkChange(data: IntentPlanRequest): Promise<IntentPlanView> {
  const res: any = await request.post('/ai/intent/plan', data)
  return res.data
}

export async function applyNetworkChange(planId: string): Promise<IntentApplyResult> {
  const res: any = await request.post('/ai/intent/apply', { planId })
  return res.data
}

export async function getIntentHistory(workspaceId: string): Promise<IntentHistoryItem[]> {
  const res: any = await request.get('/ai/intent/history', { params: { workspaceId } })
  return res.data ?? []
}
