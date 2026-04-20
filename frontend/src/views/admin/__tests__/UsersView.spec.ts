import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import UsersView from '../UsersView.vue'

const { push, listUsers, listEnabledDefinitions, getAllGroups, getBatchUsersUsage, getBatchUserAttributes } = vi.hoisted(() => {
  vi.stubGlobal('localStorage', {
    getItem: vi.fn(() => null),
    setItem: vi.fn(),
    removeItem: vi.fn(),
  })

  return {
    push: vi.fn(),
    listUsers: vi.fn(),
    listEnabledDefinitions: vi.fn(),
    getAllGroups: vi.fn(),
    getBatchUsersUsage: vi.fn(),
    getBatchUserAttributes: vi.fn(),
  }
})

vi.mock('vue-router', () => ({
  useRouter: () => ({
    push,
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showWarning: vi.fn(),
    showSuccess: vi.fn(),
    showInfo: vi.fn(),
  }),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      list: listUsers,
      toggleStatus: vi.fn(),
      delete: vi.fn(),
    },
    groups: {
      getAll: getAllGroups,
    },
    dashboard: {
      getBatchUsersUsage,
    },
    userAttributes: {
      listEnabledDefinitions,
      getBatchUserAttributes,
    },
  },
}))

vi.mock('@/utils/format', () => ({
  formatDateTime: (value: string) => value,
}))

const AppLayoutStub = { template: '<div><slot /></div>' }
const TablePageLayoutStub = {
  template: `
    <div>
      <slot name="filters" />
      <slot name="table" />
      <slot name="pagination" />
    </div>
  `,
}
const DataTableStub = {
  props: ['data'],
  template: `
    <div>
      <div v-for="row in data" :key="row.id" class="row">
        <slot name="cell-email" :value="row.email" :row="row" />
      </div>
    </div>
  `,
}

describe('admin UsersView', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    push.mockReset()
    listUsers.mockReset()
    listEnabledDefinitions.mockReset()
    getAllGroups.mockReset()
    getBatchUsersUsage.mockReset()
    getBatchUserAttributes.mockReset()

    listEnabledDefinitions.mockResolvedValue([])
    listUsers.mockResolvedValue({
      items: [
        {
          id: 168,
          email: 'user@example.com',
          username: 'user',
          notes: '',
          role: 'user',
          balance: 0,
          concurrency: 1,
          status: 'active',
          allowed_groups: [],
          balance_notify_enabled: false,
          balance_notify_threshold: null,
          balance_notify_extra_emails: [],
          subscriptions: [],
          created_at: '2026-04-19T00:00:00Z',
          updated_at: '2026-04-19T00:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    getAllGroups.mockResolvedValue([])
    getBatchUsersUsage.mockResolvedValue({ stats: {} })
    getBatchUserAttributes.mockResolvedValue({ attributes: {} })
    vi.setSystemTime(new Date('2026-04-19T10:30:00Z'))
  })

  afterEach(() => {
    vi.runOnlyPendingTimers()
    vi.useRealTimers()
  })

  it('clicking user email navigates to admin usage with expected query', async () => {
    const wrapper = mount(UsersView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          Icon: true,
          Teleport: true,
        },
      },
    })

    await flushPromises()
    await vi.advanceTimersByTimeAsync(60)
    await flushPromises()

    await wrapper.get('button[type="button"]').trigger('click')

    expect(push).toHaveBeenCalledWith({
      path: '/admin/usage',
      query: {
        user_id: '168',
        start_date: '2026-04-18',
        end_date: '2026-04-19',
      },
    })

    wrapper.unmount()
  })
})
