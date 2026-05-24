import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import DashboardPage from '../index.vue'

declare global {
  // eslint-disable-next-line no-var
  var definePage: (...args: unknown[]) => void
}

// Hoisted global mock for definePage (normally auto-imported by unplugin-auto-import)
vi.hoisted(() => {
  globalThis.definePage = vi.fn()
})

// Mock vue-i18n
vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
}))

// Mock vue-sonner to prevent side-effects from error toasts in stores
vi.mock('vue-sonner', () => ({
  toast: {
    error: vi.fn(),
    success: vi.fn(),
    info: vi.fn(),
  },
}))

describe('Dashboard Page', () => {
  it('mounts without crashing', async () => {
    setActivePinia(createPinia())
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/', component: DashboardPage },
      ],
    })

    const wrapper = mount(DashboardPage, {
      global: {
        plugins: [router],
      },
    })

    expect(wrapper.exists()).toBe(true)
  })
})
