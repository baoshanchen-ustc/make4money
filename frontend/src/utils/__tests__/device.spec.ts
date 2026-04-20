import { afterEach, describe, expect, it, vi } from 'vitest'

import { isMobileDevice } from '@/utils/device'

function mockNavigator(overrides: Partial<Navigator>) {
  Object.defineProperty(window, 'navigator', {
    value: {
      userAgent: 'Mozilla/5.0',
      platform: 'Win32',
      maxTouchPoints: 0,
      ...overrides
    },
    configurable: true
  })
}

describe('isMobileDevice', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('uses userAgentData.mobile when available', () => {
    mockNavigator({
      userAgentData: { mobile: true } as Navigator['userAgentData']
    } as Partial<Navigator>)

    expect(isMobileDevice()).toBe(true)
  })

  it('treats iPadOS desktop-class user agents as mobile when touch is available', () => {
    mockNavigator({
      userAgent: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)',
      platform: 'MacIntel',
      maxTouchPoints: 5
    })

    expect(isMobileDevice()).toBe(true)
  })

  it('falls back to coarse-pointer mobile heuristics for embedded browsers', () => {
    mockNavigator({
      userAgent: 'Mozilla/5.0 CustomWebView/1.0',
      maxTouchPoints: 2
    })
    Object.defineProperty(window, 'innerWidth', { value: 390, configurable: true })
    Object.defineProperty(window, 'innerHeight', { value: 844, configurable: true })
    vi.stubGlobal('matchMedia', vi.fn().mockReturnValue({ matches: true }))

    expect(isMobileDevice()).toBe(true)
  })

  it('keeps desktop browsers as non-mobile', () => {
    mockNavigator({
      userAgent: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 Chrome/124.0 Safari/537.36',
      platform: 'Linux x86_64',
      maxTouchPoints: 0
    })
    Object.defineProperty(window, 'innerWidth', { value: 1440, configurable: true })
    Object.defineProperty(window, 'innerHeight', { value: 900, configurable: true })
    vi.stubGlobal('matchMedia', vi.fn().mockReturnValue({ matches: false }))

    expect(isMobileDevice()).toBe(false)
  })
})
