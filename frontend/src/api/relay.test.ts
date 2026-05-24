import { describe, it, expect } from 'vitest'

describe('relay API', () => {
  it('imported module resolves', async () => {
    const mod = await import('./relay')
    expect(mod).toBeDefined()
  })
})
