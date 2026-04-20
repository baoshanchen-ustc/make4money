import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import ThirdPartyAuthCallbackFlow from '@/components/auth/ThirdPartyAuthCallbackFlow.vue'
import WechatCallbackView from '@/views/auth/WechatCallbackView.vue'

const {
  replaceMock,
  appStoreMock,
  setPendingAuthSessionMock,
  clearPendingAuthSessionMock,
  setCurrentUserMock,
  refreshUserMock,
  bindAccountMock
} = vi.hoisted(() => ({
  replaceMock: vi.fn(),
  appStoreMock: {
    showError: vi.fn(),
    showSuccess: vi.fn()
  },
  setPendingAuthSessionMock: vi.fn(),
  clearPendingAuthSessionMock: vi.fn(),
  setCurrentUserMock: vi.fn(),
  refreshUserMock: vi.fn(),
  bindAccountMock: vi.fn()
}))

const authStore = {
  token: null as string | null,
  pendingAuthSession: null as Record<string, unknown> | null,
  setPendingAuthSession: setPendingAuthSessionMock,
  clearPendingAuthSession: clearPendingAuthSessionMock,
  setCurrentUser: setCurrentUserMock,
  refreshUser: refreshUserMock
}

vi.mock('vue-router', () => ({
  useRouter: () => ({
    replace: replaceMock
  })
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
  useAuthStore: () => authStore,
  useAppStore: () => appStoreMock
}))

vi.mock('@/api/auth', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/auth')>()
  return actual
})

vi.mock('@/api/user', () => ({
  userAPI: {
    bindAccount: bindAccountMock
  }
}))

function mountView() {
  return mount(WechatCallbackView, {
    global: {
      stubs: {
        AuthLayout: {
          template: '<div><slot /></div>'
        }
      }
    }
  })
}

describe('WechatCallbackView bind_current_user refresh', () => {
  beforeEach(() => {
    replaceMock.mockReset()
    appStoreMock.showError.mockReset()
    appStoreMock.showSuccess.mockReset()
    setPendingAuthSessionMock.mockReset()
    clearPendingAuthSessionMock.mockReset()
    setCurrentUserMock.mockReset()
    refreshUserMock.mockReset()
    bindAccountMock.mockReset()
    authStore.token = 'active-token'
    authStore.pendingAuthSession = null
  })

  it('refreshes the authenticated user after binding wechat', async () => {
    bindAccountMock.mockResolvedValue({ id: 7, email: 'owner@example.com' })
    refreshUserMock.mockResolvedValue({
      id: 7,
      email: 'owner@example.com',
      account_bindings: {
        wechat: {
          provider: 'wechat',
          bound: true,
          provider_subject: 'union-1'
        }
      }
    })

    const wrapper = mountView()
    const flow = wrapper.findComponent(ThirdPartyAuthCallbackFlow)

    flow.vm.$emit('pending-session', {
      authResult: 'pending_session',
      pendingAuthToken: 'pending-wechat-token',
      provider: 'wechat',
      intent: 'bind_current_user',
      redirect: '/profile',
      adoptionRequired: false,
      suggestedDisplayName: null,
      suggestedAvatarUrl: null
    })
    await flushPromises()

    expect(bindAccountMock).toHaveBeenCalledWith('wechat', 'pending-wechat-token')
    expect(refreshUserMock).toHaveBeenCalledTimes(1)
    expect(setCurrentUserMock).toHaveBeenLastCalledWith(expect.objectContaining({
      id: 7,
      account_bindings: {
        wechat: expect.objectContaining({
          bound: true
        })
      }
    }))
    expect(clearPendingAuthSessionMock).toHaveBeenCalled()
    expect(replaceMock).toHaveBeenCalledWith('/profile')
  })
})
