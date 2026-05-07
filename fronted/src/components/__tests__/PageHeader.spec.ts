import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import PageHeader from '@/components/PageHeader.vue'

// Mock vue-i18n composable
vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => {
      const map: Record<string, string> = {
        'common.breadcrumb.home': 'Home',
      }
      return map[key] || key
    },
    te: () => false,
  }),
}))

// Mock vue-router composable
vi.mock('vue-router', () => ({
  useRoute: () => ({
    path: '/',
  }),
}))

// Stub for Breadcrumb sub-components
const BreadcrumbStubs = {
  Breadcrumb: { template: '<div><slot /></div>' },
  BreadcrumbList: { template: '<div><slot /></div>' },
  BreadcrumbItem: { template: '<div><slot /></div>' },
  BreadcrumbLink: { template: '<a><slot /></a>' },
  BreadcrumbPage: { template: '<span><slot /></span>' },
  BreadcrumbSeparator: { template: '<span>/</span>' },
}

describe('PageHeader', () => {
  it('renders title prop', () => {
    const wrapper = mount(PageHeader, {
      props: { title: 'Test Page' },
      global: {
        stubs: BreadcrumbStubs,
      },
    })
    expect(wrapper.text()).toContain('Test Page')
  })

  it('renders description when provided', () => {
    const wrapper = mount(PageHeader, {
      props: {
        title: 'Test Page',
        description: 'A description for the page',
      },
      global: {
        stubs: BreadcrumbStubs,
      },
    })
    expect(wrapper.text()).toContain('A description for the page')
  })

  it('does not render description when not provided', () => {
    const wrapper = mount(PageHeader, {
      props: { title: 'Test Page' },
      global: {
        stubs: BreadcrumbStubs,
      },
    })
    expect(wrapper.text()).not.toContain('undefined')
  })
})
