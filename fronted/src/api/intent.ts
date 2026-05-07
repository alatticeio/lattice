import request from '@/api/request'

export interface IntentPlanRequest {
  workspaceId: string
  intent: string
  dryRun?: boolean
}

export interface CRDChange {
  action: string
  resource: string
}

export interface IntentPlanView {
  id: string
  summary: string
  changes: CRDChange[]
  riskLevel: string
}

export interface IntentApplyResult {
  workflowIds: string[]
  message: string
}

export async function planNetworkChange(data: IntentPlanRequest): Promise<IntentPlanView> {
  const res: any = await request.post('/ai/intent/plan', data)
  return res.data
}

export async function applyNetworkChange(planId: string): Promise<IntentApplyResult> {
  const res: any = await request.post('/ai/intent/apply', { planId })
  return res.data
}
