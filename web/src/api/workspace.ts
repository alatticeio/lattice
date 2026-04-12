import request from '@/api/request';

export interface Workspace {
    id: string;
    slug: string;
    namespace?: string;
    displayName: string;
    nodeCount: number;
    tokenCount: number;
    maxNodeCount: number;
    status: 'active' | 'inactive';
    createdAt: string;
}

// --- Workspace空间管理 ---
export const add    = (data?: any)              => request.post('/workspaces/add', data);
export interface ListWsParams {
    page?: number
    pageSize?: number
    search?: string
    status?: string
}
export const listWs = (params?: ListWsParams) => request.get('/workspaces/list', params);
export const updateWs = (id: string, data?: any) => request.put(`/workspaces/${id}`, data);
export const deleteWs = (id: string)            => request.delete(`/workspaces/${id}`);

