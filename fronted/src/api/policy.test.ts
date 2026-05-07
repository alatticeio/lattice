import { describe, it, expect } from 'vitest'

describe('policy API', () => {
  it('imported module resolves', async () => {
    const mod = await import('./policy')
    expect(mod).toBeDefined()
    expect(typeof mod.listPolicy).toBe('function')
  })
})
