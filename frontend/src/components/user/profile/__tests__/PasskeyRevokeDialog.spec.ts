import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import PasskeyRevokeDialog from '../PasskeyRevokeDialog.vue'
import { useAppStore } from '@/stores/app'
import { authAPI } from '@/api'

vi.mock('@/stores/app', () => ({
  useAppStore: vi.fn()
}))

vi.mock('@/api', () => ({
  authAPI: {
    revokePasskey: vi.fn()
  }
}))

const i18n = createI18n({
  legacy: false,
  locale: 'en',
  messages: {
    en: {
      profile: {
        passkey: {
          revokeTitle: 'Revoke Passkey',
          revokeWarning: 'Are you sure you want to revoke {name}?',
          confirmRevoke: 'Revoke',
          revokeSuccess: 'Passkey revoked successfully',
          revokeFailed: 'Failed to revoke passkey',
          recentAuthRequiredHint: 'Please sign out and sign in again to manage your passkeys'
        }
      },
      common: {
        cancel: 'Cancel'
      }
    }
  }
})

describe('PasskeyRevokeDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  const mountComponent = () => {
    const mockAppStore = {
      showSuccess: vi.fn(),
      showError: vi.fn()
    }
    vi.mocked(useAppStore).mockReturnValue(mockAppStore as any)

    return mount(PasskeyRevokeDialog, {
      global: {
        plugins: [i18n]
      },
      props: {
        passkey: {
          credential_id: 'cred-1',
          friendly_name: 'My iPhone',
          created_at: 1600000000,
          backup_eligible: true,
          synced: true
        }
      }
    })
  }

  it('handles recent auth required error', async () => {
    const wrapper = mountComponent()
    
    const recentAuthError = new Error('Recent auth required') as any
    recentAuthError.response = { data: { code: 'RECENT_AUTH_REQUIRED' } }
    vi.mocked(authAPI.revokePasskey).mockRejectedValue(recentAuthError)

    const revokeBtn = wrapper.find('[data-testid="passkey-revoke-confirm-button"]')
    await revokeBtn.trigger('click')

    expect(authAPI.revokePasskey).toHaveBeenCalledWith('cred-1')
    
    await wrapper.vm.$nextTick()
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('profile.passkey.recentAuthRequiredHint')
  })
})
