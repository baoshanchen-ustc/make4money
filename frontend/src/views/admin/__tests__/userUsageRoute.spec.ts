import { describe, expect, it } from 'vitest'

import { buildAdminUserUsageRoute, formatAdminUsageDate } from '../utils/userUsageRoute'

describe('admin user usage route', () => {
  it('formats dates as YYYY-MM-DD', () => {
    expect(formatAdminUsageDate(new Date('2026-04-19T15:16:17Z'))).toBe('2026-04-19')
  })

  it('builds usage route with previous day and current day', () => {
    const route = buildAdminUserUsageRoute(168, new Date('2026-04-19T10:30:00Z'))

    expect(route).toEqual({
      path: '/admin/usage',
      query: {
        user_id: '168',
        start_date: '2026-04-18',
        end_date: '2026-04-19'
      }
    })
  })
})
