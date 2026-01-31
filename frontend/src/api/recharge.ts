/**
 * Recharge API Client
 * Handles balance recharge configuration and operations
 */

import { apiClient } from './client'

// ==================== Types ====================

export interface RechargeConfig {
  enabled: boolean
  min_amount: number
  max_amount: number
  default_amounts: number[]
}

// ==================== API Functions ====================

export const rechargeAPI = {
  /**
   * 获取充值配置（公开接口，无需认证）
   */
  async getConfig(): Promise<RechargeConfig> {
    const response = await apiClient.get<RechargeConfig>('/recharge/config')
    return response.data
  }
}

export default rechargeAPI
