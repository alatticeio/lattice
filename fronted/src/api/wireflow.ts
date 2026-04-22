import request from '@/api/request';

// --- 网络管理 (Namespace) ---
export const getNetworks = () => request.get('/networks');
export const createNetwork = (name: string) => request.post('/networks', { name });

// --- 节点管理 (Peers) ---
export const getPeers = (networkId: string) =>
    request.get(`/networks/${networkId}/peers`);


