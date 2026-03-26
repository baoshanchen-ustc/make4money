import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AccountBulkActionsBar from '../AccountBulkActionsBar.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

describe('AccountBulkActionsBar', () => {
  it('emits test-connection when bulk test button is clicked', async () => {
    const wrapper = mount(AccountBulkActionsBar, {
      props: {
        selectedIds: [1, 2]
      }
    })

    const button = wrapper.findAll('button').find((node) => node.text().includes('admin.accounts.bulkActions.testConnection'))
    expect(button).toBeTruthy()

    await button!.trigger('click')
    expect(wrapper.emitted('test-connection')).toBeTruthy()
  })
})
