/**
 * Admin User View API endpoints
 * Proxy endpoints to view any user's dashboard data as admin
 */

import { apiClient } from '../client'
import type { UserDashboardStats, TrendParams, TrendResponse, ModelStatsResponse } from '@/api/usage'

// Re-export types for convenience
export type { UserDashboardStats, TrendParams, TrendResponse, ModelStatsResponse }

/**
 * Get dashboard stats for a specific user (admin only)
 * @param userId - User ID to view
 * @returns Dashboard stats for the user
 */
export async function getAdminUserDashboardStats(userId: number | string): Promise<UserDashboardStats> {
  const { data } = await apiClient.get<UserDashboardStats>(`/admin/users/${userId}/dashboard/stats`)
  return data
}

/**
 * Get dashboard trend data for a specific user (admin only)
 * @param userId - User ID to view
 * @param params - Query parameters for filtering
 * @returns Usage trend data for the user
 */
export async function getAdminUserDashboardTrend(
  userId: number | string,
  params?: TrendParams
): Promise<TrendResponse> {
  const { data } = await apiClient.get<TrendResponse>(`/admin/users/${userId}/dashboard/trend`, { params })
  return data
}

/**
 * Get model usage statistics for a specific user (admin only)
 * @param userId - User ID to view
 * @param params - Query parameters for filtering
 * @returns Model usage stats for the user
 */
export async function getAdminUserDashboardModels(
  userId: number | string,
  params?: { start_date?: string; end_date?: string }
): Promise<ModelStatsResponse> {
  const { data } = await apiClient.get<ModelStatsResponse>(`/admin/users/${userId}/dashboard/models`, { params })
  return data
}

export const adminUserViewAPI = {
  getDashboardStats: getAdminUserDashboardStats,
  getDashboardTrend: getAdminUserDashboardTrend,
  getDashboardModels: getAdminUserDashboardModels,
}

export default adminUserViewAPI
