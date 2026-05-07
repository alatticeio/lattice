import request from '@/api/request'

export interface NetworkSnapshot {
  id: string
  workspaceId: string
  namespace: string
  capturedAt: string
  triggerType: string
  triggerBy: string
  peers: string
  policies: string
  networks: string
  presence: string
}

export interface ListSnapshotsParams {
  from?: string
  to?: string
  triggerType?: string
}

export async function listSnapshots(workspaceId: string, params?: ListSnapshotsParams): Promise<NetworkSnapshot[]> {
  const res: any = await request.get(`/workspaces/${workspaceId}/snapshots`, params)
  return res.data
}

export async function getSnapshot(workspaceId: string, snapshotId: string): Promise<NetworkSnapshot> {
  const res: any = await request.get(`/workspaces/${workspaceId}/snapshots/${snapshotId}`)
  return res.data
}

export interface DiffResult {
  from: NetworkSnapshot
  to: NetworkSnapshot
  diffNotes: string
}

export async function diffSnapshots(workspaceId: string, fromId: string, toId: string): Promise<DiffResult> {
  const res: any = await request.get(`/workspaces/${workspaceId}/snapshots/diff`, { from: fromId, to: toId })
  return res.data
}
