import { describe, it, expect } from 'vitest'

describe('monitor API', () => {
  it('imported module resolves', async () => {
    const mod = await import('./monitor')
    expect(mod).toBeDefined()
  })
})
