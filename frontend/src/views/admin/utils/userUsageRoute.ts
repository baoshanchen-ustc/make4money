export type AdminUsageRoute = {
  path: '/admin/usage'
  query: {
    user_id: string
    start_date: string
    end_date: string
  }
}

export const formatAdminUsageDate = (date: Date): string => {
  const year = date.getFullYear()
  const month = `${date.getMonth() + 1}`.padStart(2, '0')
  const day = `${date.getDate()}`.padStart(2, '0')
  return `${year}-${month}-${day}`
}

export const buildAdminUserUsageRoute = (userId: number, now: Date = new Date()): AdminUsageRoute => {
  const endDate = new Date(now)
  const startDate = new Date(now)
  startDate.setDate(startDate.getDate() - 1)

  return {
    path: '/admin/usage',
    query: {
      user_id: String(userId),
      start_date: formatAdminUsageDate(startDate),
      end_date: formatAdminUsageDate(endDate)
    }
  }
}
