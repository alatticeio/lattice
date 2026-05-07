import { describe, it, expect } from 'vitest'

describe('workspace API', () => {
  it('imported module resolves', async () => {
    const mod = await import('./workspace')
    expect(mod).toBeDefined()
    expect(typeof mod.listWs).toBe('function')
  })
})
