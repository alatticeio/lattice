import { describe, it, expect } from 'vitest'
import { listTokens, create, rmToken } from './token'

describe('token API', () => {
  it('listTokens returns token array', async () => {
    const result: any = await listTokens({ network: 'default' })
    expect(result.code).toBe(200)
    expect(Array.isArray(result.data)).toBe(true)
    expect(result.data[0].token).toMatch(/^wf_/)
  })

  it('create returns a response', async () => {
    const result: any = await create({ network: 'default' })
    expect(result).toBeDefined()
  })

  it('rmToken resolves sucessfully', async () => {
    await expect(rmToken('1')).resolves.toBeDefined()
  })
})
