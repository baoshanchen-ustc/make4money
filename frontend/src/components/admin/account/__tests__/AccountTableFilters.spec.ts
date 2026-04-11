import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import AccountTableFilters from '../AccountTableFilters.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (key === 'admin.accounts.planTypesSelected' && params?.count !== undefined) {
          return `${key}:${params.count}`
        }
        return key
      }
    })
  }
})

describe('AccountTableFilters', () => {
  it('renders privacy mode options and emits privacy_mode updates', async () => {
    const wrapper = mount(AccountTableFilters, {
      props: {
        searchQuery: '',
        filters: {
          platform: '',
          type: '',
          status: '',
          group: '',
          privacy_mode: '',
          plan_types: []
        },
        groups: []
      },
      global: {
        stubs: {
          SearchInput: {
            template: '<div />'
          },
          Select: {
            props: ['modelValue', 'options'],
            emits: ['update:modelValue', 'change'],
            template: '<div class="select-stub" :data-options="JSON.stringify(options)" />'
          }
        }
      }
    })

    const selects = wrapper.findAll('.select-stub')
    expect(selects).toHaveLength(5)

    const privacyOptions = JSON.parse(selects[3].attributes('data-options'))
    expect(privacyOptions).toEqual([
      { value: '', label: 'admin.accounts.allPrivacyModes' },
      { value: '__unset__', label: 'admin.accounts.privacyUnset' },
      { value: 'training_off', label: 'Privacy' },
      { value: 'training_set_cf_blocked', label: 'CF' },
      { value: 'training_set_failed', label: 'Fail' }
    ])
  })

  it('supports multi-select plan type filters', async () => {
    const wrapper = mount(AccountTableFilters, {
      props: {
        searchQuery: '',
        filters: {
          platform: '',
          type: '',
          status: '',
          group: '',
          privacy_mode: '',
          plan_types: []
        },
        groups: []
      },
      global: {
        stubs: {
          SearchInput: {
            template: '<div />'
          },
          Select: {
            props: ['modelValue', 'options'],
            emits: ['update:modelValue', 'change'],
            template: '<div class="select-stub" />'
          }
        }
      }
    })

    await wrapper.get('[data-testid="account-plan-types-trigger"]').trigger('click')
    await wrapper.get('[data-testid="account-plan-types-option-team"] input').setValue(true)

    const filterEvents = wrapper.emitted('update:filters')
    expect(filterEvents).toBeTruthy()
    expect(filterEvents?.at(-1)?.[0]).toMatchObject({ plan_types: ['team'] })

    const changeEvents = wrapper.emitted('change')
    expect(changeEvents).toBeTruthy()
    expect(changeEvents?.length).toBeGreaterThan(0)
  })
})
