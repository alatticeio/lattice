import request from '@/api/request';

// --- Token 管理 ---
// 获取指定网络的入网 Token 及其安装指令
export const listTokens = (data: object) => request.get(`/token/list`, data);

export const create = (data: object) => request.post("/token/generate", data)

export const rmToken = (token: string) => request.delete(`/token/${token}`)