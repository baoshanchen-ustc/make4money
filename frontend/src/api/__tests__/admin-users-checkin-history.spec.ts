import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { AdminUserCheckInHistoryResponse } from '@/types'

const getMock = vi.fn()

vi.mock('@/api/client', () => ({
  apiClient: {
    get: (...args: unknown[]) => getMock(...args)
  }
}))

describe('admin users check-in history api', () => {
  beforeEach(() => {
    getMock.mockReset()
  })

  it('loads user check-in history from admin endpoint', async () => {
    const payload: AdminUserCheckInHistoryResponse = {
      items: [
        {
          id: 11,
          check_in_date: '2026-04-08',
          checked_in_at: '2026-04-08T08:00:00Z',
          reward_type: 'balance',
          reward_amount: 1.5
        }
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
      total_reward: 1.5,
      total_checkins: 1,
      last_check_in_at: '2026-04-08T08:00:00Z'
    }
    getMock.mockResolvedValue({ data: payload })

    const { usersAPI } = await import('@/api/admin/users')
    const result = await usersAPI.getUserCheckInHistory(42)

    expect(getMock).toHaveBeenCalledWith('/admin/users/42/check-in-history')
    expect(result).toEqual(payload)
  })
})
