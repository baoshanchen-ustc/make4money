/**
 * Admin Copilot Platform Config API
 * GET  /api/v1/admin/copilot/platform-config         — 获取所有 plan_type 的配置
 * PUT  /api/v1/admin/copilot/platform-config/:plan_type — 更新指定 plan_type 的配置
 */

import { apiClient } from '../client'

export type CopilotPlanType =
  | 'individual_free'
  | 'individual_pro'
  | 'individual_pro_plus'
  | 'business'
  | 'enterprise'

export interface CopilotPlatformConfigEntry {
  plan_type: CopilotPlanType
  max_output_tokens: number | null
  max_body_kb: number | null
  model_mapping: Record<string, string>
  model_whitelist: string[]
}

export interface UpdateCopilotPlatformConfigRequest {
  max_output_tokens: number | null
  max_body_kb: number | null
  model_mapping: Record<string, string>
  model_whitelist: string[]
}

export const COPILOT_PLAN_TYPES: CopilotPlanType[] = [
  'individual_free',
  'individual_pro',
  'individual_pro_plus',
  'business',
  'enterprise',
]

/**
 * 获取全部 5 个 plan_type 的平台配置。
 * apiClient 拦截器已自动解包 { code, message, data }，
 * 直接解构 { data } 即可得到数组。
 */
export async function listCopilotPlatformConfigs(): Promise<CopilotPlatformConfigEntry[]> {
  const { data } = await apiClient.get<CopilotPlatformConfigEntry[]>(
    '/admin/copilot/platform-config'
  )
  return data
}

/**
 * 更新指定 plan_type 的平台配置。
 */
export async function updateCopilotPlatformConfig(
  planType: CopilotPlanType,
  payload: UpdateCopilotPlatformConfigRequest
): Promise<CopilotPlatformConfigEntry> {
  const { data } = await apiClient.put<CopilotPlatformConfigEntry>(
    `/admin/copilot/platform-config/${planType}`,
    payload
  )
  return data
}
