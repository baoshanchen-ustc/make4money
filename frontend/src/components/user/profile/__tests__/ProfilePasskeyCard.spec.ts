import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import ProfilePasskeyCard from '../ProfilePasskeyCard.vue'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import * as webauthn from '@simplewebauthn/browser'

vi.mock('@/stores/auth', () => ({
  useAuthStore: vi.fn()
}))

vi.mock('@/stores/app', () => ({
  useAppStore: vi.fn()
}))

vi.mock('@simplewebauthn/browser', () => ({
  startRegistration: vi.fn()
}))

const i18n = createI18n({
  legacy: false,
  locale: 'en',
  messages: {
    en: {
      profile: {
        passkey: {
          title: 'Passkeys',
          description: 'Sign in securely',
          featureDisabled: 'Feature Unavailable',
          recentAuthRequired: 'Recent Authentication Required',
          noPasskeys: 'No Passkeys',
          addPasskey: 'Add Passkey',
          renameTitle: 'Rename Passkey',
          revokeTitle: 'Revoke Passkey'
        }
      },
      common: {
        rename: 'Rename',
        revoke: 'Revoke',
        save: 'Save',
        cancel: 'Cancel'
      }
    }
  }
})

describe('ProfilePasskeyCard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  const mountComponent = (initialState = {}) => {
    const mockAuthStore = {
      passkeyStatus: {
        feature_enabled: true,
        can_manage: true,
        has_passkeys: false,
        active_count: 0,
        password_fallback_available: true,
        ...(initialState as any).passkeyStatus
      },
      passkeys: (initialState as any).passkeys || [],
      refreshPasskeyManagement: vi.fn().mockResolvedValue({}),
      beginPasskeyEnrollment: vi.fn(),
      finishPasskeyEnrollment: vi.fn()
    }
    vi.mocked(useAuthStore).mockReturnValue(mockAuthStore as any)

    const mockAppStore = {
      showSuccess: vi.fn(),
      showError: vi.fn()
    }
    vi.mocked(useAppStore).mockReturnValue(mockAppStore as any)

    return mount(ProfilePasskeyCard, {
      global: {
        plugins: [i18n]
      }
    })
  }

  it('shows feature disabled state', async () => {
    const wrapper = mountComponent({
      passkeyStatus: { feature_enabled: false }
    })
    
    await wrapper.vm.$nextTick()
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('profile.passkey.featureDisabled')
    expect(wrapper.find('[data-testid="passkey-enroll-button"]').exists()).toBe(false)
  })

  it('shows recent auth required state', async () => {
    const wrapper = mountComponent({
      passkeyStatus: { feature_enabled: true, can_manage: false }
    })
    
    await wrapper.vm.$nextTick()
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('profile.passkey.recentAuthRequired')
    const enrollBtn = wrapper.find('[data-testid="passkey-enroll-button"]')
    expect(enrollBtn.exists()).toBe(true)
    expect(enrollBtn.attributes('disabled')).toBeDefined()
  })

  it('shows empty state with enroll button', async () => {
    const wrapper = mountComponent()
    
    await wrapper.vm.$nextTick()
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('profile.passkey.noPasskeys')
    expect(wrapper.find('[data-testid="passkey-enroll-button"]').exists()).toBe(true)
  })

  it('handles undefined passkeys gracefully', async () => {
    const wrapper = mountComponent({
      passkeys: undefined
    })
    
    await wrapper.vm.$nextTick()
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('profile.passkey.noPasskeys')
    expect(wrapper.find('[data-testid="passkey-enroll-button"]').exists()).toBe(true)
  })

  it('shows list of passkeys', async () => {
    const wrapper = mountComponent({
      passkeys: [
        {
          credential_id: 'cred-1',
          friendly_name: 'My iPhone',
          created_at: 1600000000,
          backup_eligible: true,
          synced: true
        }
      ]
    })
    
    await wrapper.vm.$nextTick()
    await wrapper.vm.$nextTick()

    const rows = wrapper.findAll('[data-testid="passkey-row"]')
    expect(rows.length).toBe(1)
    expect(rows[0].text()).toContain('My iPhone')
  })

  it('shows list of passkeys even when recent auth is required', async () => {
    const wrapper = mountComponent({
      passkeyStatus: { feature_enabled: true, can_manage: false },
      passkeys: [
        {
          credential_id: 'cred-1',
          friendly_name: 'My iPhone',
          created_at: 1600000000,
          backup_eligible: true,
          synced: true
        }
      ]
    })
    
    await wrapper.vm.$nextTick()
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('profile.passkey.recentAuthRequired')
    const rows = wrapper.findAll('[data-testid="passkey-row"]')
    expect(rows.length).toBe(1)
    expect(rows[0].text()).toContain('My iPhone')
    
    const renameBtn = rows[0].find('button.btn-outline-secondary')
    const revokeBtn = rows[0].find('[data-testid="passkey-revoke-button"]')
    expect(renameBtn.attributes('disabled')).toBeDefined()
    expect(revokeBtn.attributes('disabled')).toBeDefined()
  })

  it('handles enrollment flow', async () => {
    const wrapper = mountComponent()
    const authStore = useAuthStore()
    const appStore = useAppStore()

    vi.mocked(authStore.beginPasskeyEnrollment).mockResolvedValue({
      flow_id: 'flow-1',
      options: { publicKey: { challenge: 'test' } } as any
    })
    vi.mocked(authStore.finishPasskeyEnrollment).mockResolvedValue({
      credential: {
        credential_id: 'new-cred',
        friendly_name: 'New Passkey',
        created_at: 1600000000,
        backup_eligible: true,
        synced: true
      }
    })
    vi.mocked(webauthn.startRegistration).mockResolvedValue({ id: 'test-id' } as any)

    await wrapper.vm.$nextTick()
    await wrapper.vm.$nextTick()

    const enrollBtn = wrapper.find('[data-testid="passkey-enroll-button"]')
    await enrollBtn.trigger('click')

    expect(authStore.beginPasskeyEnrollment).toHaveBeenCalled()
    expect(webauthn.startRegistration).toHaveBeenCalledWith({
      optionsJSON: { challenge: 'test' }
    })
    expect(authStore.finishPasskeyEnrollment).toHaveBeenCalled()
    expect(appStore.showSuccess).toHaveBeenCalled()
  })

  it('handles enrollment cancellation gracefully', async () => {
    const wrapper = mountComponent()
    const authStore = useAuthStore()
    const appStore = useAppStore()

    vi.mocked(authStore.beginPasskeyEnrollment).mockResolvedValue({
      flow_id: 'flow-1',
      options: { publicKey: { challenge: 'test' } } as any
    })
    
    const cancelError = new Error('Cancelled')
    cancelError.name = 'NotAllowedError'
    vi.mocked(webauthn.startRegistration).mockRejectedValue(cancelError)

    await wrapper.vm.$nextTick()
    await wrapper.vm.$nextTick()

    const enrollBtn = wrapper.find('[data-testid="passkey-enroll-button"]')
    await enrollBtn.trigger('click')

    expect(authStore.beginPasskeyEnrollment).toHaveBeenCalled()
    expect(webauthn.startRegistration).toHaveBeenCalled()
    expect(authStore.finishPasskeyEnrollment).not.toHaveBeenCalled()
    expect(appStore.showError).not.toHaveBeenCalled()
  })

  it('handles recent auth required error during enrollment', async () => {
    const wrapper = mountComponent()
    const authStore = useAuthStore()
    const appStore = useAppStore()

    const recentAuthError = new Error('Recent auth required') as any
    recentAuthError.response = { data: { code: 'RECENT_AUTH_REQUIRED' } }
    vi.mocked(authStore.beginPasskeyEnrollment).mockRejectedValue(recentAuthError)

    await wrapper.vm.$nextTick()
    await wrapper.vm.$nextTick()

    const enrollBtn = wrapper.find('[data-testid="passkey-enroll-button"]')
    await enrollBtn.trigger('click')

    expect(authStore.beginPasskeyEnrollment).toHaveBeenCalled()
    expect(webauthn.startRegistration).not.toHaveBeenCalled()
    expect(appStore.showError).toHaveBeenCalledWith('profile.passkey.recentAuthRequiredHint')
  })
})
