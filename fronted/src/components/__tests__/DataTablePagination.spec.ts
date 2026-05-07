import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import DataTablePagination from '@/components/DataTablePagination.vue'
import type { Table } from '@tanstack/vue-table'
// eslint-disable-next-line @typescript-eslint/no-explicit-any

function createMockTable(overrides: Record<string, unknown> = {}) {
  return {
    getState: () => ({
      pagination: { pageIndex: 0, pageSize: 10 },
    }),
    getPageCount: () => 3,
    getCanPreviousPage: () => false,
    getCanNextPage: () => true,
    previousPage: () => {},
    nextPage: () => {},
    setPageIndex: () => {},
    getFilteredRowModel: () => ({
      rows: [{ id: '1' }, { id: '2' }, { id: '3' }],
    }),
    ...overrides,
  } as unknown as Table<any>
}

describe('DataTablePagination', () => {
  it('renders without crashing', () => {
    const wrapper = mount(DataTablePagination, {
      props: {
        table: createMockTable(),
      },
    })
    expect(wrapper.exists()).toBe(true)
  })

  it('displays the correct page count', () => {
    const wrapper = mount(DataTablePagination, {
      props: {
        table: createMockTable({ getPageCount: () => 5 }),
      },
    })
    expect(wrapper.text()).toContain('5')
  })

  it('disables previous page button on first page', () => {
    const wrapper = mount(DataTablePagination, {
      props: {
        table: createMockTable({ getCanPreviousPage: () => false }),
      },
    })
    const prevButton = wrapper.findAll('button').at(0)
    expect(prevButton?.attributes('disabled')).toBeDefined()
  })
})
