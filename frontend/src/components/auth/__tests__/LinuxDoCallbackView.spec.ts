import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import LinuxDoCallbackView from '@/views/auth/LinuxDoCallbackView.vue'
import ThirdPartyAuthCallbackFlow from '@/components/auth/ThirdPartyAuthCallbackFlow.vue'

const {
  replaceMock,
  appStoreMock,
  setTokenMock,
  setPendingAuthSessionMock,
  clearPendingAuthSessionMock,
  setCurrentUserMock,
  refreshUserMock,
  bindAccountMock,
  setAccountBindingAdoptionDecisionMock,
  createOAuthAccountMock,
  persistOAuthTokenPairMock
} = vi.hoisted(() => ({
  replaceMock: vi.fn(),
  appStoreMock: {
    showError: vi.fn(),
    showSuccess: vi.fn(),
    thirdPartyFirstLoginRequireEmail: false,
    cachedPublicSettings: {
      invitation_code_enabled: false
    },
    fetchPublicSettings: vi.fn(),
    siteName: 'Sub2API'
  },
  setTokenMock: vi.fn(),
  setPendingAuthSessionMock: vi.fn(),
  clearPendingAuthSessionMock: vi.fn(),
  setCurrentUserMock: vi.fn(),
  refreshUserMock: vi.fn(),
  bindAccountMock: vi.fn(),
  setAccountBindingAdoptionDecisionMock: vi.fn(),
  createOAuthAccountMock: vi.fn(),
  persistOAuthTokenPairMock: vi.fn()
}))

const authStore = {
  token: null as string | null,
  pendingAuthSession: null as Record<string, unknown> | null,
  setToken: setTokenMock,
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
  return {
    ...actual,
    createOAuthAccount: createOAuthAccountMock,
    persistOAuthTokenPair: persistOAuthTokenPairMock
  }
})

vi.mock('@/api/user', () => ({
  userAPI: {
    bindAccount: bindAccountMock,
    setAccountBindingAdoptionDecision: setAccountBindingAdoptionDecisionMock
  }
}))

function mountView() {
  return mount(LinuxDoCallbackView, {
    global: {
      stubs: {
        AuthLayout: {
          template: '<div><slot /></div>'
        }
      }
    }
  })
}

describe('LinuxDoCallbackView pending auth flow', () => {
  beforeEach(() => {
    replaceMock.mockReset()
    appStoreMock.showError.mockReset()
    appStoreMock.showSuccess.mockReset()
    appStoreMock.thirdPartyFirstLoginRequireEmail = false
    appStoreMock.cachedPublicSettings = {
      invitation_code_enabled: false
    }
    appStoreMock.fetchPublicSettings.mockReset()
    appStoreMock.fetchPublicSettings.mockImplementation(async () => {
      return {
        ...(appStoreMock.cachedPublicSettings ?? {}),
        third_party_first_login_require_email: appStoreMock.thirdPartyFirstLoginRequireEmail
      }
    })
    setTokenMock.mockReset()
    setPendingAuthSessionMock.mockReset()
    clearPendingAuthSessionMock.mockReset()
    setCurrentUserMock.mockReset()
    refreshUserMock.mockReset()
    bindAccountMock.mockReset()
    setAccountBindingAdoptionDecisionMock.mockReset()
    createOAuthAccountMock.mockReset()
    persistOAuthTokenPairMock.mockReset()
    authStore.token = null
    authStore.pendingAuthSession = null
    setPendingAuthSessionMock.mockImplementation((session) => {
      authStore.pendingAuthSession = session as Record<string, unknown> | null
    })
    clearPendingAuthSessionMock.mockImplementation(() => {
      authStore.pendingAuthSession = null
    })
  })

  it('keeps the adoption decision attached when routing a pending login session to create-account', async () => {
    appStoreMock.thirdPartyFirstLoginRequireEmail = true

    const wrapper = mountView()
    const flow = wrapper.findComponent(ThirdPartyAuthCallbackFlow)

    const summary = {
      authResult: 'pending_session' as const,
      pendingAuthToken: 'pending-token',
      provider: 'linuxdo' as const,
      intent: 'login' as const,
      redirect: '/welcome',
      adoptionRequired: true,
      suggestedDisplayName: 'LinuxDo User',
      suggestedAvatarUrl: 'https://example.com/avatar.png'
    }

    flow.vm.$emit('adoption-decision', {
      adoptDisplayName: true,
      adoptAvatar: false,
      context: summary
    })
    await flushPromises()

    flow.vm.$emit('create-account', summary)
    await flushPromises()

    expect(setPendingAuthSessionMock).toHaveBeenLastCalledWith(
      expect.objectContaining({
        token: 'pending-token',
        provider: 'linuxdo',
        intent: 'login',
        adoption_decision: {
          adopt_display_name: true,
          adopt_avatar: false
        }
      })
    )
    expect(replaceMock).toHaveBeenCalledWith({
      name: 'Register',
      query: { redirect: '/welcome' }
    })
  })

  it('creates the oauth account directly when email binding is not required', async () => {
    createOAuthAccountMock.mockResolvedValue({
      access_token: 'direct-access',
      refresh_token: 'direct-refresh',
      expires_in: 3600,
      token_type: 'Bearer'
    })
    setTokenMock.mockResolvedValue(undefined)

    const wrapper = mountView()
    const flow = wrapper.findComponent(ThirdPartyAuthCallbackFlow)

    const summary = {
      authResult: 'pending_session' as const,
      pendingAuthToken: 'pending-direct-token',
      provider: 'linuxdo' as const,
      intent: 'login' as const,
      redirect: '/welcome',
      adoptionRequired: true,
      suggestedDisplayName: 'LinuxDo User',
      suggestedAvatarUrl: 'https://example.com/avatar.png'
    }

    flow.vm.$emit('adoption-decision', {
      adoptDisplayName: true,
      adoptAvatar: false,
      context: summary
    })
    await flushPromises()

    flow.vm.$emit('create-account', summary)
    await flushPromises()

    expect(createOAuthAccountMock).toHaveBeenCalledWith('linuxdo', {
      pendingAuthToken: 'pending-direct-token',
      adoptDisplayName: true,
      adoptAvatar: false
    })
    expect(persistOAuthTokenPairMock).toHaveBeenCalled()
    expect(setTokenMock).toHaveBeenCalledWith('direct-access')
    expect(clearPendingAuthSessionMock).toHaveBeenCalled()
    expect(replaceMock).toHaveBeenCalledWith('/welcome')
  })

  it('submits the adoption decision before completing bind_current_user', async () => {
    authStore.token = 'active-token'
    bindAccountMock.mockResolvedValue({ id: 7, email: 'owner@example.com' })
    refreshUserMock.mockResolvedValue({
      id: 7,
      email: 'owner@example.com',
      account_bindings: {
        linuxdo: {
          provider: 'linuxdo',
          bound: true,
          provider_subject: 'linuxdo-user-1'
        }
      }
    })
    setAccountBindingAdoptionDecisionMock.mockResolvedValue(undefined)

    const wrapper = mountView()
    const flow = wrapper.findComponent(ThirdPartyAuthCallbackFlow)

    const summary = {
      authResult: 'pending_session' as const,
      pendingAuthToken: 'pending-bind-token',
      provider: 'linuxdo' as const,
      intent: 'bind_current_user' as const,
      redirect: '/profile',
      adoptionRequired: true,
      suggestedDisplayName: 'LinuxDo User',
      suggestedAvatarUrl: 'https://example.com/avatar.png'
    }

    flow.vm.$emit('adoption-decision', {
      adoptDisplayName: true,
      adoptAvatar: true,
      context: summary
    })
    await flushPromises()

    expect(setAccountBindingAdoptionDecisionMock).toHaveBeenCalledWith(
      'linuxdo',
      'pending-bind-token',
      true,
      true
    )
    expect(bindAccountMock).toHaveBeenCalledWith('linuxdo', 'pending-bind-token')
    expect(refreshUserMock).toHaveBeenCalledTimes(1)
    expect(setCurrentUserMock).toHaveBeenLastCalledWith(expect.objectContaining({
      id: 7,
      account_bindings: {
        linuxdo: expect.objectContaining({
          bound: true
        })
      }
    }))
    expect(clearPendingAuthSessionMock).toHaveBeenCalled()
  })

  it('auto-completes bind_current_user callbacks for an authenticated user', async () => {
    authStore.token = 'active-token'
    bindAccountMock.mockResolvedValue({ id: 7, email: 'owner@example.com' })
    refreshUserMock.mockResolvedValue({
      id: 7,
      email: 'owner@example.com',
      account_bindings: {
        linuxdo: {
          provider: 'linuxdo',
          bound: true,
          provider_subject: 'linuxdo-user-1'
        }
      }
    })

    const wrapper = mountView()
    const flow = wrapper.findComponent(ThirdPartyAuthCallbackFlow)

    const summary = {
      authResult: 'pending_session' as const,
      pendingAuthToken: 'pending-auto-bind-token',
      provider: 'linuxdo' as const,
      intent: 'bind_current_user' as const,
      redirect: '/profile',
      adoptionRequired: false,
      suggestedDisplayName: null,
      suggestedAvatarUrl: null
    }

    flow.vm.$emit('pending-session', summary)
    await flushPromises()

    expect(bindAccountMock).toHaveBeenCalledWith('linuxdo', 'pending-auto-bind-token')
    expect(refreshUserMock).toHaveBeenCalledTimes(1)
    expect(setCurrentUserMock).toHaveBeenLastCalledWith(expect.objectContaining({
      id: 7,
      account_bindings: {
        linuxdo: expect.objectContaining({
          bound: true
        })
      }
    }))
    expect(clearPendingAuthSessionMock).toHaveBeenCalled()
    expect(replaceMock).toHaveBeenCalledWith('/profile')
  })
})
