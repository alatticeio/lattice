import request from '@/api/request'
import type { DashboardResponse, WorkspaceDashboardResponse } from '@/types/monitor'

export const getGlobalDashboard = () =>
    request.get<{ code: number; data: DashboardResponse }>('/dashboard/overview')

export const getWorkspaceDashboard = (wsID: string) =>
    request.get<{ code: number; data: WorkspaceDashboardResponse }>(`/workspaces/${wsID}/dashboard`)
