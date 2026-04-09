import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/api/client', () => ({
  apiClient: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}))

import { apiClient } from '@/api/client'
import checkInAPI from '@/api/checkIn'
import usersAPI from '@/api/admin/users'

describe('Check-in API contract', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  it('GET /api/v1/check-in/status wrapper calls /check-in/status and returns status payload', async () => {
    const payload = {
      enabled: true,
      reward_type: 'balance',
      reward_amount: 1.25,
      timezone: 'Asia/Shanghai',
      history_visible: true,
      checked_in_today: false,
      current_streak: 7,
      total_checkins: 20,
      streak_broken: false,
      check_in_date: '2026-04-09',
      last_check_in_date: '2026-04-08',
      last_check_in_at: '2026-04-08T01:02:03Z',
      next_available_at: '2026-04-10T00:00:00Z',
    }
    ;(apiClient.get as any).mockResolvedValueOnce({ data: payload })

    const result = await checkInAPI.getStatus()

    expect(result).toEqual(payload)
    expect(apiClient.get).toHaveBeenCalledTimes(1)
    expect((apiClient.get as any).mock.calls[0][0]).toBe('/check-in/status')
  })

  it('POST /api/v1/check-in wrapper calls /check-in and preserves duplicate-day success fields', async () => {
    const payload = {
      checked_in: false,
      already_checked_in: true,
      check_in_date: '2026-04-09',
      checked_in_at: '2026-04-09T03:00:00Z',
      current_streak: 9,
      total_checkins: 40,
      streak_broken: false,
      reward: {
        type: 'balance',
        amount: 1.25,
        new_balance: 99.5,
      },
    }
    ;(apiClient.post as any).mockResolvedValueOnce({ data: payload })

    const result = await checkInAPI.checkIn()

    expect(result).toEqual(payload)
    expect(apiClient.post).toHaveBeenCalledTimes(1)
    expect((apiClient.post as any).mock.calls[0][0]).toBe('/check-in')
    expect(result.already_checked_in).toBe(true)
  })

  it('GET /api/v1/check-in/history wrapper calls /check-in/history and returns item array', async () => {
    const payload = [
      {
        id: 1,
        check_in_date: '2026-04-09',
        checked_in_at: '2026-04-09T03:00:00Z',
        reward_type: 'balance',
        reward_amount: 1.25,
      },
    ]
    ;(apiClient.get as any).mockResolvedValueOnce({ data: payload })

    const result = await checkInAPI.getHistory()

    expect(result).toEqual(payload)
    expect(apiClient.get).toHaveBeenCalledTimes(1)
    expect((apiClient.get as any).mock.calls[0][0]).toBe('/check-in/history')
  })

  it('GET /api/v1/admin/users/:id/check-in-history wrapper calls the admin user history endpoint', async () => {
    const payload = {
      items: [
        {
          id: 1,
          check_in_date: '2026-04-09',
          checked_in_at: '2026-04-09T03:00:00Z',
          reward_type: 'balance',
          reward_amount: 1.25,
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
      total_reward: 1.25,
      total_checkins: 1,
      last_check_in_at: '2026-04-09T03:00:00Z',
    }
    ;(apiClient.get as any).mockResolvedValueOnce({ data: payload })

    const result = await usersAPI.getUserCheckInHistory(7)

    expect(result).toEqual(payload)
    expect(apiClient.get).toHaveBeenCalledTimes(1)
    expect((apiClient.get as any).mock.calls[0][0]).toBe('/admin/users/7/check-in-history')
  })
})
