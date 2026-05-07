import { describe, it, expect } from 'vitest'

describe('alert API', () => {
  it('imported module resolves', async () => {
    const mod = await import('./alert')
    expect(mod).toBeDefined()
    expect(typeof mod.listAlertRules).toBe('function')
  })
})
