/**
 * Recharge Store
 * Manages recharge configuration state and menu visibility
 */

import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { rechargeAPI, type RechargeConfig } from '@/api'

const CACHE_ENABLED_KEY = 'recharge_enabled_cached'

export const useRechargeStore = defineStore('recharge', () => {
  // ==================== State ====================

  // 从缓存读取初始值，避免 UI 闪烁
  const readCachedEnabled = (): boolean => {
    try {
      const raw = localStorage.getItem(CACHE_ENABLED_KEY)
      return raw === 'true'
    } catch {
      return false
    }
  }

  const writeCachedEnabled = (value: boolean) => {
    try {
      localStorage.setItem(CACHE_ENABLED_KEY, value ? 'true' : 'false')
    } catch {
      // ignore localStorage failures
    }
  }

  const config = ref<RechargeConfig | null>(null)
  const loading = ref(false)
  const loaded = ref(false)
  const error = ref<string | null>(null)

  // 使用缓存的初始值
  const cachedEnabled = ref(readCachedEnabled())

  // ==================== Computed ====================

  // 计算属性：是否启用充值
  const isEnabled = computed(() => {
    if (config.value !== null) {
      return config.value.enabled
    }
    // 未加载时使用缓存值
    return cachedEnabled.value
  })

  // 配置详情
  const minAmount = computed(() => config.value?.min_amount ?? 1)
  const maxAmount = computed(() => config.value?.max_amount ?? 1000)
  const defaultAmounts = computed(() => config.value?.default_amounts ?? [10, 50, 100, 200, 500])
  const exchangeRate = computed(() => config.value?.exchange_rate ?? 1)

  // ==================== Actions ====================

  /**
   * 获取充值配置
   */
  async function fetchConfig(force = false): Promise<void> {
    if (loaded.value && !force) return
    if (loading.value) return

    loading.value = true
    error.value = null

    try {
      const data = await rechargeAPI.getConfig()
      config.value = data
      cachedEnabled.value = data.enabled
      writeCachedEnabled(data.enabled)
      loaded.value = true
    } catch (err) {
      console.error('[rechargeStore] Failed to fetch config:', err)
      error.value = err instanceof Error ? err.message : 'Unknown error'
      // 加载失败时保持缓存值，不改变 enabled 状态
      loaded.value = true
    } finally {
      loading.value = false
    }
  }

  /**
   * 重置状态（用于登出等场景）
   */
  function reset() {
    config.value = null
    loaded.value = false
    error.value = null
  }

  return {
    config,
    loading,
    loaded,
    error,
    isEnabled,
    minAmount,
    maxAmount,
    defaultAmounts,
    exchangeRate,
    fetchConfig,
    reset
  }
})
