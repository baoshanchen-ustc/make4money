/**
 * Referral API endpoints
 * Handles user referral/invitation system
 */

import { apiClient } from './client'
import type { ReferralInfo, ReferralInvitee, BasePaginationResponse, ReferralStats } from '@/types'

/**
 * Get current user's referral info (code, link, stats)
 */
export async function getMyReferralInfo(): Promise<ReferralInfo> {
  const { data } = await apiClient.get<ReferralInfo>('/referral')
  return data
}

/**
 * Get paginated list of users invited by current user
 */
export async function listMyInvitees(
  page = 1,
  pageSize = 20
): Promise<BasePaginationResponse<ReferralInvitee>> {
  const { data } = await apiClient.get<BasePaginationResponse<ReferralInvitee>>(
    '/referral/invitees',
    { params: { page, page_size: pageSize } }
  )
  return data
}

/**
 * Admin: Get platform-wide referral statistics
 */
export async function getAdminReferralStats(): Promise<ReferralStats> {
  const { data } = await apiClient.get<ReferralStats>('/admin/referral/stats')
  return data
}

export const referralAPI = {
  getMyReferralInfo,
  listMyInvitees,
  getAdminReferralStats
}

export default referralAPI
