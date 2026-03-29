/**
 * Admin Copilot Analytics API endpoints
 * Covers:
 *  - User dimension: daily stats, hourly timeline, hierarchical request list
 *  - Account dimension: overview with live quota, quota trend, hourly stats,
 *    quota refresh, budget alert upsert
 */

import { apiClient } from '../client'

// ─────────────────────────────────────────────
// 用户维度类型
// ─────────────────────────────────────────────

export interface CopilotUserStatEntry {
  user_id: number
  username: string
  premium_requests: number
  agent_requests: number
  total_requests: number
  models: string[]
  last_request_at: string | null
}

export interface CopilotUserStatsSummary {
  total_premium_requests: number
  total_agent_requests: number
  active_users: number
}

export interface CopilotUserStatsResult {
  date: string
  total_premium_requests: number
  total_agent_requests: number
  active_users: number
  users: CopilotUserStatEntry[]
}

export interface CopilotHourlyBucket {
  hour: number
  premium_count: number
  agent_count: number
}

export interface CopilotUserTimelineResult {
  user_id: number
  date: string
  hourly: CopilotHourlyBucket[]
}

export interface CopilotRequestItem {
  request_id: string
  model: string
  initiator: 'user' | 'agent'
  created_at: string
  duration_ms: number | null
  sub_requests?: CopilotRequestItem[]
}

export interface CopilotUserRequestsResult {
  total: number
  items: CopilotRequestItem[]
}

// ─────────────────────────────────────────────
// 账户维度类型
// ─────────────────────────────────────────────

export interface CopilotAccountQuotaSnapshot {
  entitlement: number
  remaining: number
  github_total_used: number
  overage: number
  unlimited: boolean
  external_used: number
  cached_at: string | null
}

export interface CopilotAccountBudgetAlertInfo {
  monthly_budget: number
  alert_threshold: number
  enabled: boolean
}

export type CopilotAlertStatus = 'ok' | 'warning' | 'critical'

export interface CopilotAccountOverviewEntry {
  account_id: number
  name: string
  plan_type: string
  seat_count: number
  monthly_cost: number
  cost_per_premium_request: number
  system_today_premium_requests: number
  system_month_premium_requests: number
  quota_snapshot: CopilotAccountQuotaSnapshot | null
  budget_alert: CopilotAccountBudgetAlertInfo | null
  alert_status: CopilotAlertStatus
}

export interface CopilotAccountsOverviewResult {
  total_accounts: number
  estimated_monthly_cost: number
  today_premium_requests: number
  alert_count: number
  accounts: CopilotAccountOverviewEntry[]
}

export interface CopilotQuotaSnapshotTrendEntry {
  id: number
  account_id: number
  snapshot_date: string
  plan_type: string | null
  premium_entitlement: number
  premium_remaining: number
  premium_used: number
  premium_overage: number
  unlimited: boolean
  created_at: string
}

export interface CopilotAccountQuotaTrendResult {
  account_id: number
  trend: CopilotQuotaSnapshotTrendEntry[]
}

export interface CopilotAccountHourlyStatsResult {
  account_id: number
  date: string
  hourly: CopilotHourlyBucket[]
}

export interface BudgetAlertUpsertRequest {
  monthly_budget: number
  alert_threshold: number
  enabled: boolean
}

// ─────────────────────────────────────────────
// API 调用函数
// ─────────────────────────────────────────────

const BASE = '/admin/copilot'

// 用户维度

export async function getCopilotUserStats(params: {
  date?: string
  user_id?: number
}): Promise<CopilotUserStatsResult> {
  const { data } = await apiClient.get(`${BASE}/users/stats`, { params })
  return data
}

export async function getCopilotUserTimeline(
  userId: number,
  params: { date?: string },
): Promise<CopilotUserTimelineResult> {
  const { data } = await apiClient.get(`${BASE}/users/${userId}/timeline`, { params })
  return data
}

export async function getCopilotUserRequests(
  userId: number,
  params: { date?: string; page?: number; page_size?: number },
): Promise<CopilotUserRequestsResult> {
  const { data } = await apiClient.get(`${BASE}/users/${userId}/requests`, { params })
  return data
}

// 账户维度

export interface CopilotAccountDailyEntry {
  account_id: number
  date: string
  count: number
}

export interface CopilotAccountDailyAccountInfo {
  account_id: number
  name: string
}

export interface CopilotAccountsDailyStatsResult {
  accounts: CopilotAccountDailyAccountInfo[]
  days: CopilotAccountDailyEntry[]
}

export async function getCopilotAccountsDailyStats(
  params: { days?: number } = {},
): Promise<CopilotAccountsDailyStatsResult> {
  const { data } = await apiClient.get(`${BASE}/accounts/daily-stats`, { params })
  return data
}

export async function getCopilotAccountsOverview(): Promise<CopilotAccountsOverviewResult> {
  const { data } = await apiClient.get(`${BASE}/accounts/overview`)
  return data
}

export async function getCopilotAccountQuotaTrend(
  accountId: number,
  params: { days?: number } = {},
): Promise<CopilotAccountQuotaTrendResult> {
  const { data } = await apiClient.get(`${BASE}/accounts/${accountId}/quota-trend`, { params })
  return data
}

export async function getCopilotAccountHourlyStats(
  accountId: number,
  params: { date?: string } = {},
): Promise<CopilotAccountHourlyStatsResult> {
  const { data } = await apiClient.get(`${BASE}/accounts/${accountId}/hourly-stats`, { params })
  return data
}

export async function refreshCopilotAccountQuota(accountId: number): Promise<unknown> {
  const { data } = await apiClient.post(`${BASE}/accounts/${accountId}/quota-refresh`)
  return data
}

export async function upsertCopilotBudgetAlert(
  accountId: number,
  body: BudgetAlertUpsertRequest,
): Promise<unknown> {
  const { data } = await apiClient.put(`${BASE}/accounts/${accountId}/budget`, body)
  return data
}
