import { http, HttpResponse } from 'msw'

const API_BASE = '/api/v1'

export const handlers = [
  // Auth
  http.post(`${API_BASE}/login`, () => {
    return HttpResponse.json({ code: 200, data: { token: 'mock-token', user: { id: '1', name: 'test' } } })
  }),

  // Dashboard
  http.get(`${API_BASE}/dashboard/overview`, () => {
    return HttpResponse.json({
      code: 200,
      data: {
        totalNodes: 4,
        activeTunnels: 12,
        policyCount: 8,
        activeAlerts: 2,
      },
    })
  }),

  // Nodes
  http.get(`${API_BASE}/nodes`, () => {
    return HttpResponse.json({
      code: 200,
      data: [
        { id: '1', name: 'node-1', status: 'online', ip: '10.0.0.1', version: '0.2.0' },
        { id: '2', name: 'node-2', status: 'online', ip: '10.0.0.2', version: '0.2.0' },
        { id: '3', name: 'node-3', status: 'offline', ip: '10.0.0.3', version: '0.1.9' },
      ],
    })
  }),

  // Tokens
  http.get(`${API_BASE}/token/list`, () => {
    return HttpResponse.json({
      code: 200,
      data: [{ id: '1', token: 'wf_xxxx', network: 'default', expiresAt: '2026-12-31' }],
    })
  }),

  http.post(`${API_BASE}/token/generate`, () => {
    return HttpResponse.json({
      code: 200,
      data: { id: '2', token: 'wf_new_token', network: 'default', expiresAt: '2027-12-31' },
    })
  }),

  http.delete(`${API_BASE}/token/:token`, () => {
    return HttpResponse.json({
      code: 200,
      data: null,
    })
  }),

  // Policies
  http.get(`${API_BASE}/policies`, () => {
    return HttpResponse.json({
      code: 200,
      data: [
        { id: '1', name: 'default-deny', type: 'deny', priority: 100 },
        { id: '2', name: 'allow-web', type: 'allow', priority: 50, port: 443 },
      ],
    })
  }),
]
