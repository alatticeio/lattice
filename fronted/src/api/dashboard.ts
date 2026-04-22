import request from '@/api/request'
import type { DashboardResponse } from '@/types/monitor'

export const getGlobalDashboard = () =>
    request.get<{ code: number; data: DashboardResponse }>('/dashboard/overview')
