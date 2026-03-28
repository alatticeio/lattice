import request from '@/api/request';
import type { WorkspaceResponse } from '@/types/monitor';

/**
 * 获取当前空间下的节点快照数据
 * @param params 这里的 data 通常作为 URL 查询参数 (Query Params)
 * @returns 返回 Promise，其解析值为 WorkspaceResponse 结构
 */
export const getSnapshot = () =>
    request.get<WorkspaceResponse>("/monitor/ws-snapshot");