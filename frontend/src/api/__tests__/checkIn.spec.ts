import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { CheckInActionResult, CheckInHistoryItem, CheckInStatus } from '@/types'

const getMock = vi.fn()
const postMock = vi.fn()

vi.mock('@/api/client', () => ({
  apiClient: {
    get: (...args: unknown[]) => getMock(...args),
    post: (...args: unknown[]) => postMock(...args)
  }
}))

describe('checkInAPI', () => {
  beforeEach(() => {
    getMock.mockReset()
    postMock.mockReset()
  })

  it('loads check-in status', async () => {
    const payload: CheckInStatus = {
      enabled: true,
      reward_type: 'balance',
      reward_amount: 1.5,
      timezone: 'Asia/Shanghai',
      history_visible: true,
      checked_in_today: false,
      current_streak: 3,
      total_checkins: 12,
      streak_broken: false,
      check_in_date: '2026-04-09'
    }
    getMock.mockResolvedValue({ data: payload })

    const { checkInAPI } = await import('@/api/checkIn')
    const result = await checkInAPI.getStatus()

    expect(getMock).toHaveBeenCalledWith('/check-in/status')
    expect(result).toEqual(payload)
  })

  it('submits daily check-in', async () => {
    const payload: CheckInActionResult = {
      checked_in: true,
      already_checked_in: false,
      check_in_date: '2026-04-09',
      checked_in_at: '2026-04-09T08:00:00Z',
      current_streak: 4,
      total_checkins: 13,
      streak_broken: false,
      reward: {
        type: 'balance',
        amount: 1.5,
        new_balance: 12.5
      }
    }
    postMock.mockResolvedValue({ data: payload })

    const { checkInAPI } = await import('@/api/checkIn')
    const result = await checkInAPI.checkIn()

    expect(postMock).toHaveBeenCalledWith('/check-in')
    expect(result).toEqual(payload)
  })

  it('loads check-in history list', async () => {
    const payload: CheckInHistoryItem[] = [
      {
        id: 1,
        check_in_date: '2026-04-09',
        checked_in_at: '2026-04-09T08:00:00Z',
        reward_type: 'balance',
        reward_amount: 1.5
      }
    ]
    getMock.mockResolvedValue({ data: payload })

    const { checkInAPI } = await import('@/api/checkIn')
    const result = await checkInAPI.getHistory()

    expect(getMock).toHaveBeenCalledWith('/check-in/history')
    expect(result).toEqual(payload)
  })
})
