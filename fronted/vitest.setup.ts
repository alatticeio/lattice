import { server } from '@/__mocks__/server'
import { beforeAll, afterAll, afterEach } from 'vitest'

beforeAll(() => server.listen({ onUnhandledRequest: 'warn' }))
afterEach(() => server.resetHandlers())
afterAll(() => server.close())
