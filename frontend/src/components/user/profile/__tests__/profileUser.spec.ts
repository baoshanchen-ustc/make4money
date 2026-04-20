import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { useAppStore } from '@/stores/app'
import {
  getUserDisplayName,
  getUserInitials,
  resolveUserAvatarUrl,
  resolveUserBinding
} from '@/components/user/profile/profileUser'

describe('profileUser avatar helpers', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('returns the stored avatar url when present', () => {
    expect(resolveUserAvatarUrl({
      avatar_url: 'data:image/png;base64,QUJD'
    } as any)).toBe('data:image/png;base64,QUJD')
  })

  it('falls back to the structured avatar url when avatar_url is empty', () => {
    expect(resolveUserAvatarUrl({
      avatar_url: '   ',
      avatar: {
        url: 'https://cdn.example.com/avatar.png'
      }
    } as any)).toBe('https://cdn.example.com/avatar.png')
  })

  it('returns null when the user has no avatar url', () => {
    expect(resolveUserAvatarUrl({
      avatar_url: '   '
    } as any)).toBeNull()
  })

  it('falls back to external identity avatars when profile avatar fields are empty', () => {
    expect(resolveUserAvatarUrl({
      avatar_url: '',
      external_identities: [
        {
          provider: 'wechat',
          avatar_url: 'https://cdn.example.com/wechat-avatar.png'
        }
      ]
    } as any)).toBe('https://cdn.example.com/wechat-avatar.png')
  })

  it('builds display names and initials from username or email', () => {
    expect(getUserDisplayName({
      username: 'alice',
      email: 'alice@example.com'
    } as any)).toBe('alice')
    expect(getUserInitials({
      username: '',
      email: 'alice.smith@example.com'
    } as any)).toBe('AS')
  })

  it('does not expose a wechat connect url when only mp login is enabled outside wechat', () => {
    const store = useAppStore()
    store.cachedPublicSettings = {
      linuxdo_oauth_enabled: true,
      oidc_oauth_enabled: true,
      wechat_login_open_enabled: false,
      wechat_login_mp_enabled: true
    } as any

    const binding = resolveUserBinding({
      account_bindings: {
        wechat: {
          provider: 'wechat',
          bound: false
        }
      }
    } as any, 'wechat')

    expect(binding.connectUrl).toBeNull()
  })

  it('does not treat a synthetic email as a bound local login fallback', () => {
    const binding = resolveUserBinding({
      email: 'wechat-union-abc@wechat-connect.invalid'
    } as any, 'email')

    expect(binding.bound).toBe(false)
    expect(binding.value).toBe('')
  })

  it('treats later WeChat channel records as a bound binding', () => {
    const binding = resolveUserBinding({
      account_bindings: [
        {
          provider: 'wechat',
          bound: false
        },
        {
          provider: 'wechat',
          provider_key: 'wechat-main',
          bound: true,
          provider_subject: 'union-1'
        }
      ]
    } as any, 'wechat')

    expect(binding.bound).toBe(true)
    expect(binding.value).toBe('union-1')
  })

  it('falls back to legacy wechat binding fields when account bindings are missing', () => {
    const binding = resolveUserBinding({
      wechat_bound: true,
      wechat_unionid: 'union-legacy-1',
      wechat_nickname: 'Legacy WeChat'
    } as any, 'wechat')

    expect(binding.bound).toBe(true)
    expect(binding.value).toBe('union-legacy-1')
  })

  it('falls back to external wechat identities when account bindings are missing', () => {
    const binding = resolveUserBinding({
      external_identities: [
        {
          provider: 'wechat',
          provider_key: 'wechat_mp',
          provider_subject: 'openid-1'
        }
      ]
    } as any, 'wechat')

    expect(binding.bound).toBe(true)
    expect(binding.value).toBe('openid-1')
  })

  it('treats provider user ids from external identities as a bound wechat identity', () => {
    const binding = resolveUserBinding({
      external_identities: [
        {
          provider: 'wechat',
          provider_union_id: 'union-bridge-1'
        }
      ]
    } as any, 'wechat')

    expect(binding.bound).toBe(true)
    expect(binding.value).toBe('union-bridge-1')
  })
})
