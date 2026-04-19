import { getOAuthStartUrl, getWechatOAuthAvailabilityHintKey } from '@/api/auth'
import { useAppStore } from '@/stores/app'
import type { PublicSettings, UserAccountBinding } from '@/types'

export type UserAccountBindingProvider = 'email' | 'linuxdo' | 'wechat' | 'oidc'

type BindingRecord = UserAccountBinding & Record<string, unknown>
type BindingCollection =
  | Partial<Record<UserAccountBindingProvider, UserAccountBinding | null>>
  | UserAccountBinding[]
  | null

type UserWithBindings = {
  email?: string | null
  avatar_url?: string | null
  account_bindings?: BindingCollection
}

type PublicAuthVisibilitySettings = Pick<
  PublicSettings,
  | 'linuxdo_oauth_enabled'
  | 'oidc_oauth_enabled'
  | 'wechat_login_open_enabled'
  | 'wechat_login_open_configured'
  | 'wechat_login_mp_enabled'
  | 'wechat_login_mp_configured'
>

export interface ResolvedUserBinding {
  provider: UserAccountBindingProvider
  bound: boolean
  value: string
  connectUrl: string | null
  connectDisabled: boolean
  availabilityHintKey: string | null
  canDisconnect: boolean
}

export function resolveUserAvatarUrl(user: UserWithBindings | null | undefined): string | null {
  const avatarUrl = user?.avatar_url?.trim()
  return avatarUrl || null
}

const BIND_REDIRECT = '/profile?oauth_intent=bind'

const PROVIDER_ORDER: UserAccountBindingProvider[] = ['email', 'linuxdo', 'wechat', 'oidc']

function toBindingRecord(value: unknown): BindingRecord | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return null
  }

  return value as BindingRecord
}

function getBindingRecord(
  bindings: BindingCollection | undefined,
  provider: UserAccountBindingProvider
): BindingRecord | null {
  if (!bindings) {
    return null
  }

  if (Array.isArray(bindings)) {
    for (const item of bindings) {
      const record = toBindingRecord(item)
      if (!record) {
        continue
      }
      if (readString(record, 'provider') === provider) {
        return record
      }
    }
    return null
  }

  return toBindingRecord(bindings[provider])
}

function readString(source: BindingRecord | null, ...keys: string[]): string {
  for (const key of keys) {
    const value = source?.[key]
    if (typeof value === 'string') {
      const trimmed = value.trim()
      if (trimmed) {
        return trimmed
      }
    }
  }

  return ''
}

function readBoolean(source: BindingRecord | null, key: string, fallback = false): boolean {
  const value = source?.[key]
  return typeof value === 'boolean' ? value : fallback
}

function isSyntheticEmail(email: string | null | undefined): boolean {
  const normalized = email?.trim().toLowerCase() ?? ''
  return (
    normalized.endsWith('@wechat-connect.invalid')
    || normalized.endsWith('@linuxdo-connect.invalid')
    || normalized.endsWith('@oidc-connect.invalid')
  )
}

function hasLegacyUsableEmailLogin(user: UserWithBindings): boolean {
  const email = user.email?.trim()
  return Boolean(email) && !isSyntheticEmail(email)
}

function resolveBindingValue(
  user: UserWithBindings,
  provider: UserAccountBindingProvider,
  binding: BindingRecord | null
): string {
  if (provider === 'email') {
    const bound = binding ? readBoolean(binding, 'bound', false) : hasLegacyUsableEmailLogin(user)
    if (!bound) {
      return ''
    }
    return readString(binding, 'value', 'identifier', 'provider_subject', 'display_name', 'email')
      || (hasLegacyUsableEmailLogin(user) ? user.email?.trim() ?? '' : '')
  }

  return readString(
    binding,
    'value',
    'identifier',
    'provider_subject',
    'subject',
    'display_name',
    'provider_name',
    'provider_label',
    'issuer'
  )
}

function getPublicAuthVisibilitySettings(): PublicAuthVisibilitySettings | null {
  try {
    return (useAppStore().cachedPublicSettings as PublicAuthVisibilitySettings | null) ?? null
  } catch {
    return null
  }
}

function isProviderEnabled(
  provider: UserAccountBindingProvider,
  settings: PublicAuthVisibilitySettings | null
): boolean {
  if (provider === 'email') {
    return true
  }

  if (provider === 'linuxdo') {
    return settings?.linuxdo_oauth_enabled === true
  }

  if (provider === 'oidc') {
    return settings?.oidc_oauth_enabled === true
  }

  return (
    (settings?.wechat_login_open_enabled === true && settings?.wechat_login_open_configured !== false)
    || (settings?.wechat_login_mp_enabled === true && settings?.wechat_login_mp_configured !== false)
  )
}

function buildConnectUrl(
  provider: UserAccountBindingProvider,
  settings: PublicAuthVisibilitySettings | null
): string | null {
  if (provider === 'email') {
    return null
  }

  if (!isProviderEnabled(provider, settings)) {
    return null
  }

  if (provider === 'wechat') {
    return getOAuthStartUrl(provider, BIND_REDIRECT, 'bind', {
      wechatOpenEnabled: settings?.wechat_login_open_enabled === true,
      wechatOpenConfigured: settings?.wechat_login_open_configured !== false,
      wechatMpEnabled: settings?.wechat_login_mp_enabled === true,
      wechatMpConfigured: settings?.wechat_login_mp_configured !== false
    })
  }

  return getOAuthStartUrl(provider, BIND_REDIRECT, 'bind')
}

function resolveAvailabilityHintKey(
  provider: UserAccountBindingProvider,
  settings: PublicAuthVisibilitySettings | null,
  connectUrl: string | null
): string | null {
  if (provider !== 'wechat' || connectUrl) {
    return null
  }

  return getWechatOAuthAvailabilityHintKey({
    wechatOpenEnabled: settings?.wechat_login_open_enabled === true,
    wechatOpenConfigured: settings?.wechat_login_open_configured !== false,
    wechatMpEnabled: settings?.wechat_login_mp_enabled === true,
    wechatMpConfigured: settings?.wechat_login_mp_configured !== false
  })
}

function resolveUserBindingInternal(
  user: UserWithBindings,
  provider: UserAccountBindingProvider,
  settings: PublicAuthVisibilitySettings | null
): ResolvedUserBinding {
  const binding = getBindingRecord(user.account_bindings, provider)
  const bound =
    provider === 'email'
      ? binding
        ? readBoolean(binding, 'bound')
        : hasLegacyUsableEmailLogin(user)
      : readBoolean(binding, 'bound')

  const connectUrl = bound ? null : buildConnectUrl(provider, settings)
  const connectDisabled = !bound
    && provider !== 'email'
    && isProviderEnabled(provider, settings)
    && connectUrl === null

  return {
    provider,
    bound,
    value: resolveBindingValue(user, provider, binding),
    connectUrl,
    connectDisabled,
    availabilityHintKey: bound ? null : resolveAvailabilityHintKey(provider, settings, connectUrl),
    canDisconnect: readBoolean(binding, 'can_disconnect')
  }
}

export function resolveUserBinding(
  user: UserWithBindings,
  provider: UserAccountBindingProvider
): ResolvedUserBinding {
  return resolveUserBindingInternal(user, provider, getPublicAuthVisibilitySettings())
}

export function resolveUserBindings(user: UserWithBindings): ResolvedUserBinding[] {
  const settings = getPublicAuthVisibilitySettings()

  return PROVIDER_ORDER
    .map((provider) => resolveUserBindingInternal(user, provider, settings))
    .filter((binding) => binding.provider === 'email' || binding.bound || isProviderEnabled(binding.provider, settings))
}
