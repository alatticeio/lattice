import request from './request'

export const api = {
    nodes: {
        list: () => request.get('/api/nodes'),
    },
    tokens: {
        list: () => request.get('/api/tokens'),
    },
    policies: {
        list: () => request.get('/api/policies'),
    },
    dns: {
        list: () => request.get('/api/dns/records'),
    },
    me: {
        get: () => request.get('/api/me'),
    },
}