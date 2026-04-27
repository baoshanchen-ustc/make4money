import { defineComponent, nextTick } from 'vue'
import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AccountTableFilters from '../AccountTableFilters.vue'

const messages: Record<string, string> = {
  'admin.accounts.searchAccounts': 'Search accounts...',
  'admin.accounts.filters.title': 'Filters',
  'admin.accounts.filters.account': 'Account',
  'admin.accounts.filters.status': 'Status',
  'admin.accounts.filters.plan': 'Plan',
  'admin.accounts.filters.platform': 'Platform',
  'admin.accounts.filters.type': 'Type',
  'admin.accounts.filters.privacy': 'Privacy',
  'admin.accounts.filters.group': 'Group',
  'admin.accounts.filters.clearAll': 'Clear all',
  'admin.accounts.filters.allPlans': 'All Plans',
  'admin.accounts.filters.unrecognizedPlan': 'Unrecognized',
  'admin.accounts.allPlatforms': 'All Platforms',
  'admin.accounts.allTypes': 'All Types',
  'admin.accounts.allStatus': 'All Status',
  'admin.accounts.allGroups': 'All Groups',
  'admin.accounts.allPrivacyModes': 'All Privacy States',
  'admin.accounts.privacyUnset': 'Unset',
  'admin.accounts.ungroupedGroup': 'Ungrouped',
  'admin.accounts.oauthType': 'OAuth',
  'admin.accounts.setupToken': 'Setup Token',
  'admin.accounts.apiKey': 'API Key',
  'admin.accounts.status.active': 'Active',
  'admin.accounts.status.inactive': 'Inactive',
  'admin.accounts.status.error': 'Error',
  'admin.accounts.status.rateLimited': 'Rate Limited',
  'admin.accounts.status.tempUnschedulable': 'Temp Unschedulable',
  'admin.accounts.status.unschedulable': 'Unschedulable'
}

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => messages[key] ?? key
  })
}))

const SelectStub = defineComponent({
  name: 'Select',
  props: {
    modelValue: [String, Number, Boolean],
    options: {
      type: Array,
      default: () => []
    }
  },
  emits: ['update:modelValue', 'change'],
  methods: {
    onChange(event: Event) {
      this.$emit('update:modelValue', (event.target as HTMLSelectElement).value)
      this.$emit('change')
    }
  },
  template: `
    <select
      data-test="filter-select"
      :value="modelValue"
      @change="onChange"
    >
      <option v-for="option in options" :key="String(option.value)" :value="option.value">
        {{ option.label }}
      </option>
    </select>
  `
})

const SearchInputStub = defineComponent({
  name: 'SearchInput',
  props: {
    modelValue: String,
    placeholder: String
  },
  emits: ['update:modelValue', 'search'],
  template: '<input data-test="search" :placeholder="placeholder" :value="modelValue" />'
})

const mountFilters = () => mount(AccountTableFilters, {
  attachTo: document.body,
  props: {
    searchQuery: '',
    filters: {
      platform: 'openai',
      type: '',
      status: '',
      privacy_mode: '',
      group: '',
      plan_type: '__unset__'
    },
    groups: []
  },
  global: {
    stubs: {
      Select: SelectStub,
      SearchInput: SearchInputStub,
      Icon: true
    }
  }
})

describe('AccountTableFilters', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
  })

  it('uses localized labels for active tags and menu text', async () => {
    const wrapper = mountFilters()

    expect(wrapper.text()).toContain('Platform: OpenAI')
    expect(wrapper.text()).toContain('Plan: Unrecognized')
    expect(wrapper.text()).not.toContain('平台:')
    expect(wrapper.text()).not.toContain('未识别')

    await wrapper.get('button[aria-haspopup="menu"]').trigger('click')

    expect(wrapper.text()).toContain('Account')
    expect(wrapper.text()).toContain('All Plans')
    expect(wrapper.text()).toContain('Clear all')
  })

  it('exposes menu state and closes with Escape or outside click', async () => {
    const wrapper = mountFilters()
    const trigger = wrapper.get('button[aria-haspopup="menu"]')

    expect(trigger.attributes('aria-expanded')).toBe('false')
    await trigger.trigger('click')
    expect(trigger.attributes('aria-expanded')).toBe('true')

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await nextTick()
    expect(trigger.attributes('aria-expanded')).toBe('false')

    await trigger.trigger('click')
    expect(trigger.attributes('aria-expanded')).toBe('true')
    document.body.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await nextTick()
    expect(trigger.attributes('aria-expanded')).toBe('false')
  })
})
