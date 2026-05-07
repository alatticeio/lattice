import { describe, it, expect } from 'vitest'

describe('platform API', () => {
  it('imported module resolves', async () => {
    const mod = await import('./platform')
    expect(mod).toBeDefined()
  })
})
