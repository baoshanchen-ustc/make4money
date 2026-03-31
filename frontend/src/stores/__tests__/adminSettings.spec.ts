import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useAdminSettingsStore } from '@/stores/adminSettings'

const mockGetSettings = vi.fn()
const mockGetExtremePerformanceSettings = vi.fn()

vi.mock('@/api', () => ({
  adminAPI: {
    settings: {
      getSettings: (...args: any[]) => mockGetSettings(...args),
      getExtremePerformanceSettings: (...args: any[]) => mockGetExtremePerformanceSettings(...args)
    }
  }
}))

describe('useAdminSettingsStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    vi.clearAllMocks()
  })

  it('fetch caches extreme performance settings for admin pages', async () => {
    mockGetSettings.mockResolvedValue({
      ops_monitoring_enabled: true,
      ops_realtime_monitoring_enabled: true,
      ops_query_mode_default: 'auto',
      custom_menu_items: []
    })
    mockGetExtremePerformanceSettings.mockResolvedValue({
      enabled: true,
      admin: {
        disable_auto_usage_fetch: true,
        disable_auto_today_stats_fetch: false,
        allow_manual_usage_fetch: true
      },
      pool: {
        platform_limits: { openai: 2000, gemini: 300, anthropic: 100 },
        refill_trigger_gap: 50,
        refill_batch_size: 100,
        selection_order: 'imported_first'
      },
      account_policy: {
        delete_on_any_upstream_401: true,
        cooldown_on_429_minutes: 5,
        cooldown_on_5xx_minutes: 1,
        remove_from_hot_pool_on_overload: true,
        remove_from_hot_pool_on_temp_unschedulable: true
      }
    })

    const store = useAdminSettingsStore()
    await store.fetch(true)

    expect(store.extremeModeEnabled).toBe(true)
    expect(store.disableAutoUsageFetch).toBe(true)
    expect(store.disableAutoTodayStatsFetch).toBe(false)
    expect(store.allowManualUsageFetch).toBe(true)
    expect(localStorage.getItem('extreme_mode_enabled_cached')).toBe('true')
    expect(localStorage.getItem('extreme_disable_auto_today_stats_fetch_cached')).toBe('false')
  })

  it('falls back to cached extreme performance values when fetch fails', async () => {
    localStorage.setItem('extreme_mode_enabled_cached', 'true')
    localStorage.setItem('extreme_disable_auto_usage_fetch_cached', 'false')
    localStorage.setItem('extreme_disable_auto_today_stats_fetch_cached', 'true')
    localStorage.setItem('extreme_allow_manual_usage_fetch_cached', 'false')

    mockGetSettings.mockRejectedValue(new Error('network'))
    mockGetExtremePerformanceSettings.mockRejectedValue(new Error('network'))

    const store = useAdminSettingsStore()
    await store.fetch(true)

    expect(store.extremeModeEnabled).toBe(true)
    expect(store.disableAutoUsageFetch).toBe(false)
    expect(store.disableAutoTodayStatsFetch).toBe(true)
    expect(store.allowManualUsageFetch).toBe(false)
  })
})
