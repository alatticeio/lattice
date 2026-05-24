import { describe, it, expect } from 'vitest'

describe('invitation API', () => {
  it('imported module resolves', async () => {
    const mod = await import('./invitation')
    expect(mod).toBeDefined()
  })
})
