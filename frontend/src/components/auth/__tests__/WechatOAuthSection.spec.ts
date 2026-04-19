import { mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import WechatOAuthSection from '../WechatOAuthSection.vue'

const { getOAuthStartUrlMock, getWechatOAuthAvailabilityHintKeyMock } = vi.hoisted(() => ({
  getOAuthStartUrlMock: vi.fn(),
  getWechatOAuthAvailabilityHintKeyMock: vi.fn()
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({
    query: {}
  })
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

vi.mock('@/api/auth', () => ({
  getOAuthStartUrl: getOAuthStartUrlMock,
  getWechatOAuthAvailabilityHintKey: getWechatOAuthAvailabilityHintKeyMock
}))

describe('WechatOAuthSection', () => {
  const defaultUserAgent = window.navigator.userAgent

  beforeEach(() => {
    Object.defineProperty(window.navigator, 'userAgent', {
      configurable: true,
      value: defaultUserAgent
    })
    getOAuthStartUrlMock.mockReset()
    getWechatOAuthAvailabilityHintKeyMock.mockReset()
    getOAuthStartUrlMock.mockReturnValue('/auth/start')
    getWechatOAuthAvailabilityHintKeyMock.mockReturnValue(null)
  })

  afterEach(() => {
    Object.defineProperty(window.navigator, 'userAgent', {
      configurable: true,
      value: defaultUserAgent
    })
  })

  it('shows an unavailable hint when no WeChat login mode is enabled', () => {
    getOAuthStartUrlMock.mockReturnValue(null)
    getWechatOAuthAvailabilityHintKeyMock.mockReturnValue('auth.wechat.disabledUnavailable')

    const wrapper = mount(WechatOAuthSection, {
      props: {
        openEnabled: false,
        mpEnabled: false
      }
    })

    expect(wrapper.text()).toContain('auth.wechat.disabledUnavailable')
    expect(wrapper.get('button').attributes('disabled')).toBeDefined()
    expect(getWechatOAuthAvailabilityHintKeyMock).toHaveBeenCalledWith({
      wechatOpenEnabled: false,
      wechatOpenConfigured: true,
      wechatMpEnabled: false,
      wechatMpConfigured: true,
      userAgent: defaultUserAgent
    })
  })

  it('shows the in-WeChat availability hint when only mp login is enabled outside WeChat', () => {
    getOAuthStartUrlMock.mockReturnValue(null)
    getWechatOAuthAvailabilityHintKeyMock.mockReturnValue('auth.wechat.disabledNeedWechatEnv')

    const wrapper = mount(WechatOAuthSection, {
      props: {
        openEnabled: false,
        mpEnabled: true
      }
    })

    expect(wrapper.text()).toContain('auth.wechat.disabledNeedWechatEnv')
    expect(wrapper.get('button').attributes('disabled')).toBeDefined()
  })

  it('prefers open login outside WeChat when both modes are enabled', async () => {
    const wrapper = mount(WechatOAuthSection, {
      props: {
        openEnabled: true,
        mpEnabled: true
      }
    })

    await wrapper.get('button').trigger('click')

    expect(getOAuthStartUrlMock).toHaveBeenCalledWith('wechat', '/dashboard', 'login', {
      wechatOpenEnabled: true,
      wechatOpenConfigured: true,
      wechatMpEnabled: true,
      wechatMpConfigured: true,
      userAgent: defaultUserAgent
    })
  })

  it('shows the external-browser availability hint when only open login is enabled inside WeChat', () => {
    Object.defineProperty(window.navigator, 'userAgent', {
      configurable: true,
      value: 'Mozilla/5.0 MicroMessenger'
    })
    getOAuthStartUrlMock.mockReturnValue(null)
    getWechatOAuthAvailabilityHintKeyMock.mockReturnValue('auth.wechat.disabledNeedExternalBrowser')

    const wrapper = mount(WechatOAuthSection, {
      props: {
        openEnabled: true,
        mpEnabled: false
      }
    })

    expect(wrapper.text()).toContain('auth.wechat.disabledNeedExternalBrowser')
    expect(wrapper.get('button').attributes('disabled')).toBeDefined()
  })

  it('prefers mp login inside WeChat when both modes are enabled', async () => {
    Object.defineProperty(window.navigator, 'userAgent', {
      configurable: true,
      value: 'Mozilla/5.0 MicroMessenger'
    })

    const wrapper = mount(WechatOAuthSection, {
      props: {
        openEnabled: true,
        mpEnabled: true
      }
    })

    await wrapper.get('button').trigger('click')

    expect(getOAuthStartUrlMock).toHaveBeenCalledWith('wechat', '/dashboard', 'login', {
      wechatOpenEnabled: true,
      wechatOpenConfigured: true,
      wechatMpEnabled: true,
      wechatMpConfigured: true,
      userAgent: 'Mozilla/5.0 MicroMessenger'
    })
  })

  it('does not start a broken flow when only mp login is enabled outside WeChat', async () => {
    getOAuthStartUrlMock.mockReturnValue(null)
    getWechatOAuthAvailabilityHintKeyMock.mockReturnValue('auth.wechat.disabledNeedWechatEnv')

    const wrapper = mount(WechatOAuthSection, {
      props: {
        openEnabled: false,
        mpEnabled: true
      }
    })

    await wrapper.get('button').trigger('click')

    expect(wrapper.get('button').attributes('disabled')).toBeDefined()
    expect(getOAuthStartUrlMock).toHaveBeenCalledWith('wechat', '/dashboard', 'login', {
      wechatOpenEnabled: false,
      wechatOpenConfigured: true,
      wechatMpEnabled: true,
      wechatMpConfigured: true,
      userAgent: defaultUserAgent
    })
  })
})
