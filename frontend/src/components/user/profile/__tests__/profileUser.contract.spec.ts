import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { useAppStore } from '@/stores/app'
import { resolveUserBinding, resolveUserBindings } from '@/components/user/profile/profileUser'

describe('profileUser binding contract', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  afterEach(() => {
    vi.unstubAllEnvs()
  })

  it('renders oidc as a managed profile binding', () => {
    const store = useAppStore()
    store.cachedPublicSettings = {
      linuxdo_oauth_enabled: true,
      oidc_oauth_enabled: true,
      wechat_login_open_enabled: true,
      wechat_login_mp_enabled: true
    } as any

    const binding = resolveUserBinding({
      account_bindings: {
        oidc: {
          provider: 'oidc',
          bound: true,
          display_name: 'Acme SSO',
          provider_subject: 'oidc-user-42'
        }
      }
    } as any, 'oidc' as any)

    expect(binding.bound).toBe(true)
    expect(binding.value).toBe('oidc-user-42')
  })

  it('uses the env-aware oauth helper for oidc bind urls', () => {
    vi.stubEnv('VITE_API_BASE_URL', 'https://gateway.example.com/custom-api')

    const store = useAppStore()
    store.cachedPublicSettings = {
      linuxdo_oauth_enabled: true,
      oidc_oauth_enabled: true,
      wechat_login_open_enabled: true,
      wechat_login_mp_enabled: true
    } as any

    const binding = resolveUserBinding({
      account_bindings: {
        oidc: {
          provider: 'oidc',
          bound: false
        }
      }
    } as any, 'oidc' as any)

    expect(binding.connectUrl).toBe(
      'https://gateway.example.com/custom-api/auth/oauth/oidc/start?redirect=%2Fprofile%3Foauth_intent%3Dbind&intent=bind'
    )
  })

  it('hides unbound third-party providers when public settings disable them', () => {
    const store = useAppStore()
    store.cachedPublicSettings = {
      linuxdo_oauth_enabled: false,
      oidc_oauth_enabled: false,
      wechat_login_open_enabled: false,
      wechat_login_mp_enabled: false
    } as any

    const bindings = resolveUserBindings({
      account_bindings: {
        linuxdo: { provider: 'linuxdo', bound: false },
        wechat: { provider: 'wechat', bound: false },
        oidc: { provider: 'oidc', bound: false }
      }
    } as any)

    expect(bindings.map(binding => binding.provider)).toEqual(['email'])
  })

  it('keeps bound providers visible even after the provider is disabled', () => {
    const store = useAppStore()
    store.cachedPublicSettings = {
      linuxdo_oauth_enabled: false,
      oidc_oauth_enabled: false,
      wechat_login_open_enabled: false,
      wechat_login_mp_enabled: false
    } as any

    const bindings = resolveUserBindings({
      account_bindings: {
        oidc: {
          provider: 'oidc',
          bound: true,
          display_name: 'Acme SSO'
        }
      }
    } as any)

    expect(bindings.map(binding => binding.provider)).toEqual(['email', 'oidc'])
    expect(bindings[1].connectUrl).toBeNull()
  })

  it('keeps wechat visible with a disabled action when only mp login is enabled outside wechat', () => {
    const store = useAppStore()
    store.cachedPublicSettings = {
      linuxdo_oauth_enabled: false,
      oidc_oauth_enabled: false,
      wechat_login_open_enabled: false,
      wechat_login_mp_enabled: true
    } as any

    const bindings = resolveUserBindings({
      account_bindings: {
        wechat: {
          provider: 'wechat',
          bound: false
        }
      }
    } as any)

    expect(bindings.map(binding => binding.provider)).toEqual(['email', 'wechat'])
    expect(bindings[1].connectUrl).toBeNull()
    expect(bindings[1].connectDisabled).toBe(true)
    expect(bindings[1].availabilityHintKey).toBe('auth.wechat.disabledNeedWechatEnv')
  })

  it('treats WeChat channel-specific binding records as one bound provider', () => {
    const store = useAppStore()
    store.cachedPublicSettings = {
      linuxdo_oauth_enabled: false,
      oidc_oauth_enabled: false,
      wechat_login_open_enabled: true,
      wechat_login_mp_enabled: true
    } as any

    const binding = resolveUserBinding({
      account_bindings: {
        wechat_open: {
          provider: 'wechat',
          provider_key: 'wechat-main',
          bound: true,
          channels: [
            {
              channel: 'open',
              subject: 'open-user-1'
            }
          ]
        }
      }
    } as any, 'wechat')

    expect(binding.bound).toBe(true)
    expect(binding.value).toBe('open-user-1')
  })

  it('prefers legacy WeChat compatibility fields when the structured binding is stale', () => {
    const store = useAppStore()
    store.cachedPublicSettings = {
      linuxdo_oauth_enabled: false,
      oidc_oauth_enabled: false,
      wechat_login_open_enabled: true,
      wechat_login_mp_enabled: true
    } as any

    const binding = resolveUserBinding({
      account_bindings: {
        wechat: {
          provider: 'wechat',
          bound: false
        }
      },
      wechat_bound: true,
      wechat_unionid: 'union-compat-1'
    } as any, 'wechat')

    expect(binding.bound).toBe(true)
    expect(binding.value).toBe('union-compat-1')
  })
})
