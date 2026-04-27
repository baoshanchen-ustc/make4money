import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import DataTable from '../DataTable.vue'
import type { Column } from '../types'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

const columns: Column[] = [
  { key: 'name', label: 'Name', sortable: true, width: 120, minWidth: 80, maxWidth: 240, resizable: true },
  { key: 'status', label: 'Status', sortable: true, width: 100, minWidth: 80, maxWidth: 160, resizable: true }
]

const data = [
  { id: 1, name: 'alpha', status: 'active' },
  { id: 2, name: 'beta', status: 'inactive' }
]

const mountTable = () => mount(DataTable, {
  attachTo: document.body,
  props: {
    columns,
    data,
    serverSideSort: true,
    columnWidthStorageKey: 'test-column-widths',
    estimateRowHeight: 56
  },
  global: {
    stubs: {
      Icon: true
    }
  }
})

describe('DataTable column widths', () => {
  beforeEach(() => {
    localStorage.clear()
    document.body.innerHTML = ''
    window.matchMedia = vi.fn().mockImplementation((query: string) => ({
      matches: true,
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn()
    }))
  })

  it('does not sort when the resize handle is clicked', async () => {
    const wrapper = mountTable()

    await wrapper.get('.column-resize-handle').trigger('click')

    expect(wrapper.emitted('sort')).toBeUndefined()
  })

  it('persists resized widths and exposes reset for the owning table', async () => {
    const wrapper = mountTable()
    const handle = wrapper.get('.column-resize-handle')

    await handle.trigger('mousedown', { clientX: 100 })
    document.dispatchEvent(new MouseEvent('mousemove', { clientX: 180, bubbles: true }))
    document.dispatchEvent(new MouseEvent('mouseup', { bubbles: true }))

    expect(JSON.parse(localStorage.getItem('test-column-widths') ?? '{}')).toEqual({ name: 200 })

    ;(wrapper.vm as unknown as { resetColumnWidths: () => void }).resetColumnWidths()

    expect(localStorage.getItem('test-column-widths')).toBeNull()
  })
})
