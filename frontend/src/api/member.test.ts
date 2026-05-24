import { describe, it, expect } from 'vitest'

describe('member API', () => {
  it('imported module resolves without crash', async () => {
    const mod = await import('./member')
    expect(mod).toBeDefined()
    expect(typeof mod.listMembers).toBe('function')
    expect(typeof mod.addMemberToWorkspace).toBe('function')
    expect(typeof mod.removeMember).toBe('function')
  })
})
