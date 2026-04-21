import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import PasskeyRenameDialog from '../PasskeyRenameDialog.vue'
import { useAppStore } from '@/stores/app'
import { authAPI } from '@/api'

vi.mock('@/stores/app', () => ({
  useAppStore: vi.fn()
}))

vi.mock('@/api', () => ({
  authAPI: {
    renamePasskey: vi.fn()
  }
}))

const i18n = createI18n({
  legacy: false,
  locale: 'en',
  messages: {
    en: {
      profile: {
        passkey: {
          renameTitle: 'Rename Passkey',
          friendlyName: 'Friendly Name',
          friendlyNamePlaceholder: 'Enter a friendly name',
          renameSuccess: 'Passkey renamed successfully',
          renameFailed: 'Failed to rename passkey',
          recentAuthRequiredHint: 'Please sign out and sign in again to manage your passkeys'
        }
      },
      common: {
        save: 'Save',
        cancel: 'Cancel'
      }
    }
  }
})

describe('PasskeyRenameDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  const mountComponent = () => {
    const mockAppStore = {
      showSuccess: vi.fn(),
      showError: vi.fn()
    }
    vi.mocked(useAppStore).mockReturnValue(mockAppStore as any)

    return mount(PasskeyRenameDialog, {
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
    vi.mocked(authAPI.renamePasskey).mockRejectedValue(recentAuthError)

    const input = wrapper.find('[data-testid="passkey-rename-input"]')
    await input.setValue('New Name')

    const form = wrapper.find('form')
    await form.trigger('submit.prevent')

    expect(authAPI.renamePasskey).toHaveBeenCalledWith('cred-1', 'New Name')
    
    await wrapper.vm.$nextTick()
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('profile.passkey.recentAuthRequiredHint')
  })
})
