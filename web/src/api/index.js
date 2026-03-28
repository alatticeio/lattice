import { request } from './http'

export const api = {
    nodes: {
        list: () => request('/api/nodes'),
    },
    tokens: {
        list: () => request('/api/tokens'),
    },
    policies: {
        list: () => request('/api/policies'),
    },
    dns: {
        list: () => request('/api/dns/records'),
    },
    me: {
        get: () => request('/api/me'),
    },
}