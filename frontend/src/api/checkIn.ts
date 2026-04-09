/**
 * Daily check-in API endpoints
 * Handles user check-in status, action, and history
 */

import { apiClient } from './client'
import type { CheckInStatus, CheckInActionResult, CheckInHistoryItem } from '@/types'

/**
 * Get current user's daily check-in status
 */
export async function getStatus(): Promise<CheckInStatus> {
  const { data } = await apiClient.get<CheckInStatus>('/check-in/status')
  return data
}

/**
 * Perform daily check-in
 */
export async function checkIn(): Promise<CheckInActionResult> {
  const { data } = await apiClient.post<CheckInActionResult>('/check-in')
  return data
}

/**
 * Get current user's check-in history
 */
export async function getHistory(): Promise<CheckInHistoryItem[]> {
  const { data } = await apiClient.get<CheckInHistoryItem[]>('/check-in/history')
  return data
}

export const checkInAPI = {
  getStatus,
  checkIn,
  getHistory
}

export default checkInAPI
