import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import TotpLoginModal from '../TotpLoginModal.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

describe('TotpLoginModal', () => {
  it('does not render when show is false', () => {
    const wrapper = mount(TotpLoginModal, {
      props: {
        show: false,
        tempToken: 'temp-token'
      }
    })

    expect(wrapper.find('button').exists()).toBe(false)
    expect(wrapper.text()).toBe('')
  })

  it('emits both cancel and close when cancelled', async () => {
    const wrapper = mount(TotpLoginModal, {
      props: {
        show: true,
        tempToken: 'temp-token'
      }
    })

    await wrapper.find('button').trigger('click')

    expect(wrapper.emitted('cancel')).toHaveLength(1)
    expect(wrapper.emitted('close')).toHaveLength(1)
  })
})
