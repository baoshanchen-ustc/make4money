/**
 * Unit tests for the resolveDefaultUserId helper extracted from CopilotUsersView.
 *
 * Covers the watcher logic that determines which user should be pre-selected
 * in the "按用户" trend tab.
 */
import { describe, it, expect } from 'vitest'
import { resolveDefaultUserId } from '../resolveDefaultUserId'

// Minimal shape the function cares about
type User = { userId: number; premiumRequests: number }

describe('resolveDefaultUserId', () => {
  it('空列表时返回 null', () => {
    expect(resolveDefaultUserId([], null)).toBeNull()
    expect(resolveDefaultUserId([], 1)).toBeNull()
  })

  it('当前选中 id 在新列表中时保持不变', () => {
    const users: User[] = [
      { userId: 1, premiumRequests: 10 },
      { userId: 2, premiumRequests: 5 },
    ]
    expect(resolveDefaultUserId(users, 2)).toBe(2)
  })

  it('当前选中 id 不在新列表中时，选 Premium 最多的用户', () => {
    const users: User[] = [
      { userId: 1, premiumRequests: 3 },
      { userId: 2, premiumRequests: 20 },
      { userId: 3, premiumRequests: 7 },
    ]
    // id=99 已离开列表，应重置为 Premium 最多的 id=2
    expect(resolveDefaultUserId(users, 99)).toBe(2)
  })

  it('纯 Agent 流量（所有用户 Premium=0）时，降级选列表第一个用户', () => {
    const users: User[] = [
      { userId: 10, premiumRequests: 0 },
      { userId: 20, premiumRequests: 0 },
    ]
    expect(resolveDefaultUserId(users, null)).toBe(10)
  })

  it('纯 Agent 流量且当前选中不在列表中时，降级选列表第一个', () => {
    const users: User[] = [
      { userId: 10, premiumRequests: 0 },
      { userId: 20, premiumRequests: 0 },
    ]
    expect(resolveDefaultUserId(users, 99)).toBe(10)
  })

  it('初始 currentId=null 时选 Premium 最多的用户', () => {
    const users: User[] = [
      { userId: 1, premiumRequests: 5 },
      { userId: 2, premiumRequests: 100 },
      { userId: 3, premiumRequests: 30 },
    ]
    expect(resolveDefaultUserId(users, null)).toBe(2)
  })

  it('日期范围缩短导致用户离开列表后自动重选最优用户', () => {
    // 模拟：原来选的 id=5 在 60 天有数据，切换 7 天后消失
    const usersAfterRangeChange: User[] = [
      { userId: 1, premiumRequests: 8 },
      { userId: 2, premiumRequests: 15 },
    ]
    expect(resolveDefaultUserId(usersAfterRangeChange, 5)).toBe(2)
  })
})
