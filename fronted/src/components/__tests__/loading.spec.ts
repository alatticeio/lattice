import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import LoadingSpinner from '@/components/loading.vue'

describe('LoadingSpinner', () => {
  it('renders without crashing', () => {
    const wrapper = mount(LoadingSpinner)
    expect(wrapper.exists()).toBe(true)
  })

  it('renders a loading indicator', () => {
    const wrapper = mount(LoadingSpinner)
    expect(wrapper.find('[role="status"]').exists() || wrapper.classes().length > 0).toBe(true)
  })
})
