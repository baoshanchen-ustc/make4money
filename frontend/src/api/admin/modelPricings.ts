/**
 * Admin Model Pricing API endpoints
 * Manages fallback billing prices for models
 */

import { apiClient } from '../client'

export interface ModelPricingEntry {
  id: number
  model_key: string
  display_name: string
  input_price_per_million: number
  output_price_per_million: number
  input_price_per_million_priority: number
  output_price_per_million_priority: number
  cache_read_price_per_million: number
  cache_read_price_per_million_priority: number
  cache_creation_price_per_million: number
  enabled: boolean
  note: string
  created_at: string
  updated_at: string
}

export interface UpsertModelPricingRequest {
  model_key: string
  display_name?: string
  input_price_per_million: number
  output_price_per_million: number
  input_price_per_million_priority: number
  output_price_per_million_priority: number
  cache_read_price_per_million: number
  cache_read_price_per_million_priority: number
  cache_creation_price_per_million: number
  enabled: boolean
  note?: string
}

export async function list(): Promise<ModelPricingEntry[]> {
  const { data } = await apiClient.get<ModelPricingEntry[]>('/admin/model-pricings')
  return data
}

export async function create(req: UpsertModelPricingRequest): Promise<ModelPricingEntry> {
  const { data } = await apiClient.post<ModelPricingEntry>('/admin/model-pricings', req)
  return data
}

export async function update(id: number, req: UpsertModelPricingRequest): Promise<ModelPricingEntry> {
  const { data } = await apiClient.put<ModelPricingEntry>(`/admin/model-pricings/${id}`, req)
  return data
}

export async function remove(id: number): Promise<void> {
  await apiClient.delete(`/admin/model-pricings/${id}`)
}

// ── Compare API ───────────────────────────────────────────────────────────────

/** 单层价格数据（per-million USD），null 表示该层无数据 */
export interface PriceTier {
  input_per_million: number
  output_per_million: number
  cache_read_per_million: number
  cache_creation_per_million: number
  input_priority_per_million: number
  output_priority_per_million: number
}

/** Compare API 返回的单行：数据库条目 + LiteLLM 对比 */
export interface ModelPricingCompareItem extends ModelPricingEntry {
  litellm: PriceTier | null
}

// ── Lookup API ────────────────────────────────────────────────────────────────

export type ActiveSource = 'litellm' | 'database' | 'fallback' | 'none'

/** Lookup API 返回的三层价格对比 */
export interface ModelPricingLookup {
  model: string
  litellm: PriceTier | null
  database: PriceTier | null
  fallback: PriceTier | null
  active_source: ActiveSource
}

/** 获取数据库所有条目并附带 LiteLLM 价格快照（用于对比列） */
export async function compare(): Promise<ModelPricingCompareItem[]> {
  const { data } = await apiClient.get<ModelPricingCompareItem[]>('/admin/model-pricings/compare')
  return data
}

/** 查询任意模型在三层的价格对比 */
export async function lookup(model: string): Promise<ModelPricingLookup> {
  const { data } = await apiClient.get<ModelPricingLookup>('/admin/model-pricings/lookup', {
    params: { model }
  })
  return data
}

export const modelPricingsAPI = { list, create, update, remove, compare, lookup }
export default modelPricingsAPI
