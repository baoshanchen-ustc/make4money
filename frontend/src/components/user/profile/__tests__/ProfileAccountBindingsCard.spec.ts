import { mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import ProfileAccountBindingsCard from '@/components/user/profile/ProfileAccountBindingsCard.vue'

const { appStoreMock } = vi.hoisted(() => ({
  appStoreMock: {
    cachedPublicSettings: null as null | Record<string, unknown>,
    showSuccess: vi.fn(),
    showError: vi.fn()
  }
}))

const { userApiMock } = vi.hoisted(() => ({
  userApiMock: {
    bindAccount: vi.fn(),
    unbindAccount: vi.fn(async () => null)
  }
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

vi.mock('@/stores', () => ({
  useAppStore: () => appStoreMock,
  useAuthStore: () => ({
    refreshUser: vi.fn(),
    pendingAuthSession: null,
    clearPendingAuthSession: vi.fn()
  })
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStoreMock
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    refreshUser: vi.fn(),
    pendingAuthSession: null,
    clearPendingAuthSession: vi.fn()
  })
}))

vi.mock('@/api/auth', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/auth')>()
  return {
    ...actual,
    sendVerifyCode: vi.fn(async () => ({ countdown: 60, message: 'ok' }))
  }
})

vi.mock('@/api/user', () => ({
  userAPI: userApiMock
}))

function mountCard(user: Record<string, unknown>) {
  return mount(ProfileAccountBindingsCard, {
    props: {
      user
    }
  })
}

describe('ProfileAccountBindingsCard regressions', () => {
  const defaultUserAgent = window.navigator.userAgent

  beforeEach(() => {
    appStoreMock.cachedPublicSettings = null
    appStoreMock.showSuccess.mockReset()
    appStoreMock.showError.mockReset()
    userApiMock.bindAccount.mockReset()
    userApiMock.unbindAccount.mockReset()
    userApiMock.unbindAccount.mockResolvedValue(null)
    Object.defineProperty(window.navigator, 'userAgent', {
      configurable: true,
      value: defaultUserAgent
    })
  })

  afterEach(() => {
    Object.defineProperty(window.navigator, 'userAgent', {
      configurable: true,
      value: defaultUserAgent
    })
  })

  it('hides disabled third-party providers from the profile binding list', () => {
    appStoreMock.cachedPublicSettings = {
      linuxdo_oauth_enabled: false,
      oidc_oauth_enabled: false,
      wechat_login_open_enabled: false,
      wechat_login_mp_enabled: false
    }

    const wrapper = mountCard({
      email: 'user@example.com',
      account_bindings: {
        email: {
          provider: 'email',
          bound: true,
          display_name: 'user@example.com'
        },
        linuxdo: {
          provider: 'linuxdo',
          bound: false
        },
        wechat: {
          provider: 'wechat',
          bound: false
        },
        oidc: {
          provider: 'oidc',
          bound: false
        }
      }
    })

    expect(wrapper.text()).toContain('profile.bindings.providers.email')
    expect(wrapper.text()).not.toContain('profile.bindings.providers.linuxdo')
    expect(wrapper.text()).not.toContain('profile.bindings.providers.wechat')
    expect(wrapper.text()).not.toContain('profile.bindings.providers.oidc')
  })

  it('keeps the WeChat availability hint consistent with mp-only availability outside WeChat', () => {
    appStoreMock.cachedPublicSettings = {
      wechat_login_open_enabled: false,
      wechat_login_mp_enabled: true
    }

    const wrapper = mountCard({
      email: 'user@example.com',
      account_bindings: {
        email: {
          provider: 'email',
          bound: true,
          display_name: 'user@example.com'
        },
        wechat: {
          provider: 'wechat',
          bound: false
        }
      }
    })

    expect(wrapper.text()).toContain('auth.wechat.disabledNeedWechatEnv')
    expect(wrapper.find('a[href*="/auth/oauth/wechat/start"]').exists()).toBe(false)
    expect(wrapper.find('button[disabled]').text()).toContain('profile.bindings.actions.connect')
  })

  it('shows email verification controls instead of exposing a synthetic email as bound', () => {
    const wrapper = mountCard({
      email: 'wechat-union-abc@wechat-connect.invalid',
      account_bindings: {
        email: {
          provider: 'email',
          bound: false,
          display_name: ''
        }
      }
    })

    expect(wrapper.text()).not.toContain('wechat-union-abc@wechat-connect.invalid')
    expect(wrapper.find('input[type="email"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('profile.balanceNotify.sendCode')
    expect(wrapper.text()).toContain('profile.balanceNotify.verify')
  })

  it('shows a disconnect action for removable third-party bindings and calls the unbind API', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)

    const wrapper = mountCard({
      email: 'owner@example.com',
      account_bindings: {
        email: {
          provider: 'email',
          bound: true,
          display_name: 'owner@example.com'
        },
        wechat: {
          provider: 'wechat',
          bound: true,
          provider_subject: 'union-1',
          can_disconnect: true
        }
      }
    })

    await wrapper.find('button').trigger('click')

    expect(confirmSpy).toHaveBeenCalledWith('profile.bindings.disconnectConfirm')
    expect(userApiMock.unbindAccount).toHaveBeenCalledWith('wechat')
    expect(appStoreMock.showSuccess).toHaveBeenCalledWith('profile.bindings.disconnectSuccess')
    confirmSpy.mockRestore()
  })
})
