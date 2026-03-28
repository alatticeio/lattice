import request from '@/api/request';

// --- 网络管理 (Namespace) ---
export const getNetworks = () => request.get('/networks');
export const createNetwork = (name) => request.post('/networks', { name });

// --- 节点管理 (Peers) ---
export const getPeers = (networkId) =>
    request.get(`/networks/${networkId}/peers`);


