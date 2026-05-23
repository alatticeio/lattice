import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import AlertDialog from '@/components/AlertDialog.vue'

// Stub for AlertDialog sub-components from reka-ui / shadcn
const AlertDialogStubs = {
  AlertDialog: {
    props: ['open', 'onUpdate:open'],
    template: '<div v-if="open" data-testid="alert-dialog"><slot /></div>',
  },
  AlertDialogTrigger: { template: '<div><slot /></div>' },
  AlertDialogContent: { template: '<div><slot /></div>' },
  AlertDialogHeader: { template: '<div><slot /></div>' },
  AlertDialogFooter: { template: '<div><slot /></div>' },
  AlertDialogTitle: { template: '<h2><slot /></h2>' },
  AlertDialogDescription: { template: '<p><slot /></p>' },
  AlertDialogCancel: { template: '<button data-testid="cancel"><slot /></button>' },
  AlertDialogAction: { template: '<button data-testid="confirm"><slot /></button>' },
}

describe('AlertDialog', () => {
  it('renders with open prop', () => {
    const wrapper = mount(AlertDialog, {
      props: {
        open: true,
        title: 'Confirm',
        description: 'Are you sure?',
      },
      global: {
        stubs: AlertDialogStubs,
      },
    })
    expect(wrapper.exists()).toBe(true)
  })

  it('does not render when closed', () => {
    const wrapper = mount(AlertDialog, {
      props: {
        open: false,
        title: 'Confirm',
        description: 'Are you sure?',
      },
      global: {
        stubs: AlertDialogStubs,
      },
    })
    expect(wrapper.find('[data-testid="alert-dialog"]').exists()).toBe(false)
  })

  it('emits confirm event on confirm button click', async () => {
    const wrapper = mount(AlertDialog, {
      props: {
        open: true,
        title: 'Confirm',
        description: 'Are you sure?',
      },
      global: {
        stubs: AlertDialogStubs,
      },
    })
    await wrapper.find('[data-testid="confirm"]').trigger('click')
    expect(wrapper.emitted('confirm')).toBeTruthy()
  })

  it('emits cancel event on cancel button click', async () => {
    const wrapper = mount(AlertDialog, {
      props: {
        open: true,
        title: 'Confirm',
        description: 'Are you sure?',
      },
      global: {
        stubs: AlertDialogStubs,
      },
    })
    await wrapper.find('[data-testid="cancel"]').trigger('click')
    expect(wrapper.emitted('cancel')).toBeTruthy()
  })

  it('uses default text when no props provided', () => {
    const wrapper = mount(AlertDialog, {
      props: { open: true },
      global: {
        stubs: AlertDialogStubs,
      },
    })
    expect(wrapper.find('h2').text()).toBe('确认执行此操作吗？')
    expect(wrapper.find('p').text()).toBe('此操作执行后可能无法撤销，请谨慎操作。')
  })
})
