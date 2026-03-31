import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import AccountTableFilters from '../AccountTableFilters.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

describe('AccountTableFilters', () => {
  it('renders plan type and privacy mode options', async () => {
    const wrapper = mount(AccountTableFilters, {
      props: {
        searchQuery: '',
        filters: {
          platform: '',
          type: '',
          status: '',
          plan_type: '',
          group: '',
          privacy_mode: ''
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
    expect(selects).toHaveLength(6)

    const planOptions = JSON.parse(selects[3].attributes('data-options'))
    expect(planOptions).toEqual([
      { value: '', label: 'admin.accounts.allPlanTypes' },
      { value: 'free', label: 'Free' },
      { value: 'plus', label: 'Plus' },
      { value: 'pro', label: 'Pro' },
      { value: 'team', label: 'Team' }
    ])

    const privacyOptions = JSON.parse(selects[4].attributes('data-options'))
    expect(privacyOptions).toEqual([
      { value: '', label: 'admin.accounts.allPrivacyModes' },
      { value: '__unset__', label: 'admin.accounts.privacyUnset' },
      { value: 'training_off', label: 'Privacy' },
      { value: 'training_set_cf_blocked', label: 'CF' },
      { value: 'training_set_failed', label: 'Fail' }
    ])
  })
})
