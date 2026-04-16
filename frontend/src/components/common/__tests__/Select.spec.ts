import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { ref } from 'vue'

import Select from '../Select.vue'

const messages: Record<string, string> = {
  'common.selectOption': 'Select option',
  'common.searchPlaceholder': 'Search...',
  'common.noOptionsFound': 'No options found'
}

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => messages[key] ?? key,
    locale: ref('en')
  })
}))

describe('Select', () => {
  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('emits search when opened and when the query changes', async () => {
    const wrapper = mount(Select, {
      attachTo: document.body,
      props: {
        modelValue: null,
        searchable: true,
        options: [
          { value: 1, label: 'Alpha' },
          { value: 2, label: 'Beta' }
        ]
      },
      global: {
        stubs: {
          Icon: true,
          Teleport: true
        }
      }
    })

    await wrapper.get('button[aria-haspopup="true"]').trigger('click')
    await flushPromises()

    expect(wrapper.emitted('search')).toEqual([['']])

    const searchInput = wrapper.find('.select-search-input')
    expect(searchInput.exists()).toBe(true)

    await searchInput.setValue('abc')
    await flushPromises()

    expect(wrapper.emitted('search')).toEqual([[''], ['abc']])
  })

  it('does not emit search when searchable is disabled', async () => {
    const wrapper = mount(Select, {
      attachTo: document.body,
      props: {
        modelValue: null,
        options: [
          { value: 1, label: 'Alpha' },
          { value: 2, label: 'Beta' }
        ]
      },
      global: {
        stubs: {
          Icon: true,
          Teleport: true
        }
      }
    })

    await wrapper.get('button[aria-haspopup="true"]').trigger('click')
    await flushPromises()

    expect(wrapper.emitted('search')).toBeUndefined()
  })

  it('keeps change and update:modelValue behavior unchanged', async () => {
    const wrapper = mount(Select, {
      attachTo: document.body,
      props: {
        modelValue: null,
        options: [
          { value: 1, label: 'Alpha' },
          { value: 2, label: 'Beta' }
        ]
      },
      global: {
        stubs: {
          Icon: true,
          Teleport: true
        }
      }
    })

    await wrapper.get('button[aria-haspopup="true"]').trigger('click')
    await flushPromises()

    const options = wrapper.findAll('.select-option')
    expect(options).toHaveLength(2)

    await options[1].trigger('click')
    await flushPromises()

    expect(wrapper.emitted('update:modelValue')).toEqual([[2]])
    expect(wrapper.emitted('change')).toEqual([[2, { value: 2, label: 'Beta' }]])
  })
})
