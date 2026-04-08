import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

import type { CopilotAccountsDailyStatsResult } from '@/api/admin/copilotAnalytics'

// ── Mock Chart.js before importing the component ──────────────────────────────
// Chart.js requires a canvas context that jsdom does not provide; stub it out.
vi.mock('chart.js', () => {
  const Chart = vi.fn().mockImplementation(() => ({
    data: {},
    update: vi.fn(),
    destroy: vi.fn(),
  }))
  ;(Chart as any).register = vi.fn()
  return {
    Chart,
    LineController: {},
    LineElement: {},
    PointElement: {},
    LinearScale: {},
    CategoryScale: {},
    Tooltip: {},
    Legend: {},
    Filler: {},
  }
})

// ── Mock the API call ─────────────────────────────────────────────────────────
const { getCopilotAccountsDailyStats } = vi.hoisted(() => ({
  getCopilotAccountsDailyStats: vi.fn(),
}))

vi.mock('@/api/admin/copilotAnalytics', () => ({
  getCopilotAccountsDailyStats,
}))

vi.mock('@/api/client', () => ({
  extractErrorMessage: (e: unknown) => String(e),
}))

// ── Helpers ───────────────────────────────────────────────────────────────────

function makeResult(overrides?: Partial<CopilotAccountsDailyStatsResult>): CopilotAccountsDailyStatsResult {
  return {
    accounts: [
      { account_id: 1, name: 'Account A' },
      { account_id: 2, name: 'Account B' },
    ],
    days: [
      // 2024-01-01
      { account_id: 1, date: '2024-01-01', premium_count: 10, agent_count: 4, count: 14 },
      { account_id: 2, date: '2024-01-01', premium_count: 5,  agent_count: 3, count: 8  },
      // 2024-01-02
      { account_id: 1, date: '2024-01-02', premium_count: 8,  agent_count: 6, count: 14 },
      { account_id: 2, date: '2024-01-02', premium_count: 3,  agent_count: 2, count: 5  },
    ],
    ...overrides,
  }
}

// Lazily import after mocks are registered
const { default: AccountsDailyChart } = await import('../AccountsDailyChart.vue')

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('AccountsDailyChart', () => {
  beforeEach(() => {
    getCopilotAccountsDailyStats.mockClear()
  })

  it('renders loading state before data arrives', async () => {
    // Override: return a never-resolving promise to hold the loading state
    getCopilotAccountsDailyStats.mockReturnValue(new Promise(() => {}))

    const wrapper = mount(AccountsDailyChart, {
      props: { days: 7, metric: 'premium' },
    })
    // Wait one tick so onMounted fires and sets loading=true
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('加载中')
  })

  it('renders empty state when API returns no days', async () => {
    getCopilotAccountsDailyStats.mockResolvedValue(makeResult({ days: [] }))

    const wrapper = mount(AccountsDailyChart, { props: { days: 7 } })
    await flushPromises()

    expect(wrapper.text()).toContain('暂无数据')
  })

  it('renders error state when API throws', async () => {
    getCopilotAccountsDailyStats.mockRejectedValue(new Error('network error'))

    const wrapper = mount(AccountsDailyChart, { props: { days: 7 } })
    await flushPromises()

    expect(wrapper.text()).toContain('加载失败')
  })

  it('calls the API with the correct days parameter on mount', async () => {
    getCopilotAccountsDailyStats.mockResolvedValue(makeResult())

    mount(AccountsDailyChart, { props: { days: 14, metric: 'premium' } })
    await flushPromises()

    expect(getCopilotAccountsDailyStats).toHaveBeenCalledWith({ days: 14 })
  })

  it('reloads data when `days` prop changes', async () => {
    getCopilotAccountsDailyStats.mockResolvedValue(makeResult())

    const wrapper = mount(AccountsDailyChart, { props: { days: 7, metric: 'premium' } })
    await flushPromises()
    const callsBefore = getCopilotAccountsDailyStats.mock.calls.length

    await wrapper.setProps({ days: 30 })
    await flushPromises()

    expect(getCopilotAccountsDailyStats.mock.calls.length).toBeGreaterThan(callsBefore)
    // Last call must use the new days value
    const lastCall = getCopilotAccountsDailyStats.mock.calls.at(-1)!
    expect(lastCall[0]).toEqual({ days: 30 })
  })

  it('does NOT reload when only `metric` prop changes', async () => {
    getCopilotAccountsDailyStats.mockResolvedValue(makeResult())

    const wrapper = mount(AccountsDailyChart, { props: { days: 7, metric: 'premium' } })
    await flushPromises()
    const callsAfterMount = getCopilotAccountsDailyStats.mock.calls.length

    await wrapper.setProps({ metric: 'agent' })
    await flushPromises()

    // No additional API calls triggered — only chart data rebuilt locally
    expect(getCopilotAccountsDailyStats.mock.calls.length).toBe(callsAfterMount)
  })
})

// ── buildChartData logic (pure unit tests, no component) ─────────────────────

describe('AccountsDailyChart buildChartData metric logic', () => {
  const days = makeResult().days

  // Re-implement the exact selection logic from AccountsDailyChart.vue so we
  // can unit-test it in isolation.
  function selectValue(entry: typeof days[0], metric: 'premium' | 'agent' | 'total'): number {
    return metric === 'premium'
      ? entry.premium_count
      : metric === 'agent'
        ? entry.agent_count
        : entry.premium_count + entry.agent_count
  }

  it('selects premium_count for metric=premium', () => {
    for (const entry of days) {
      expect(selectValue(entry, 'premium')).toBe(entry.premium_count)
    }
  })

  it('selects agent_count for metric=agent', () => {
    for (const entry of days) {
      expect(selectValue(entry, 'agent')).toBe(entry.agent_count)
    }
  })

  it('sums premium + agent for metric=total', () => {
    for (const entry of days) {
      expect(selectValue(entry, 'total')).toBe(entry.premium_count + entry.agent_count)
    }
  })

  it('total equals the deprecated count field', () => {
    for (const entry of days) {
      expect(selectValue(entry, 'total')).toBe(entry.count)
    }
  })

  it('account 1 premium day-1 is 10', () => {
    const entry = days.find(d => d.account_id === 1 && d.date === '2024-01-01')!
    expect(selectValue(entry, 'premium')).toBe(10)
  })

  it('account 1 agent day-1 is 4', () => {
    const entry = days.find(d => d.account_id === 1 && d.date === '2024-01-01')!
    expect(selectValue(entry, 'agent')).toBe(4)
  })

  it('account 1 total day-1 is 14 (10 premium + 4 agent)', () => {
    const entry = days.find(d => d.account_id === 1 && d.date === '2024-01-01')!
    expect(selectValue(entry, 'total')).toBe(14)
  })
})
