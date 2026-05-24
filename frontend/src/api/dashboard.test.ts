import { describe, it, expect } from 'vitest'

describe('dashboard API', () => {
  it('imported module resolves', async () => {
    const mod = await import('./dashboard')
    expect(mod).toBeDefined()
    expect(typeof mod.getGlobalDashboard).toBe('function')
    expect(typeof mod.getWorkspaceDashboard).toBe('function')
  })
})
