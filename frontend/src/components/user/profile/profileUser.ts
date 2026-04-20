import { getOAuthStartUrl, getWechatOAuthAvailabilityHintKey } from '@/api/auth'
import { useAppStore } from '@/stores/app'
import type { PublicSettings, UserAccountBinding, UserExternalIdentity } from '@/types'

export type UserAccountBindingProvider = 'email' | 'linuxdo' | 'wechat' | 'oidc'

type BindingRecord = UserAccountBinding & Record<string, unknown>
type BindingCollection =
  | Partial<Record<UserAccountBindingProvider, UserAccountBinding | null>>
  | UserAccountBinding[]
  | null

type UserAvatarRecord = {
  url?: string | null
}

type UserWithBindings = {
  email?: string | null
  username?: string | null
  avatar?: UserAvatarRecord | null
  avatar_url?: string | null
  avatar_thumbnail_url?: string | null
  profile?: Record<string, unknown> | null
  account_bindings?: BindingCollection
  external_identities?: UserExternalIdentity[] | null
  wechat?: string | null
  wechat_openid?: string | null
  wechat_unionid?: string | null
  wechat_nickname?: string | null
  wechat_bound?: boolean | null
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
  const profile = toRecord(user?.profile)
  return normalizeAvatarUrl(readFirstString(
    user?.avatar_url,
    user?.avatar_thumbnail_url,
    user?.avatar?.url,
    readRecordString(profile, 'avatar_url', 'avatarUrl'),
    readExternalIdentityAvatarUrl(user?.external_identities)
  ))
}

const BIND_REDIRECT = '/profile?oauth_intent=bind'

const PROVIDER_ORDER: UserAccountBindingProvider[] = ['email', 'linuxdo', 'wechat', 'oidc']

function toRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return null
  }
  return value as Record<string, unknown>
}

function toBindingRecord(value: unknown): BindingRecord | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return null
  }

  return value as BindingRecord
}

function readFirstString(...values: Array<string | null | undefined>): string {
  for (const value of values) {
    if (typeof value !== 'string') {
      continue
    }

    const trimmed = value.trim()
    if (trimmed) {
      return trimmed
    }
  }

  return ''
}

function readRecordString(source: Record<string, unknown> | null, ...keys: string[]): string {
  for (const key of keys) {
    const trimmed = readFirstString(source?.[key] as string | null | undefined)
    if (trimmed) {
      return trimmed
    }
  }

  return ''
}

function normalizeAvatarUrl(value: string): string | null {
  if (!value) {
    return null
  }

  if (/^(data:|blob:|https?:\/\/)/i.test(value)) {
    return value
  }

  try {
    return new URL(value, window.location.origin).toString()
  } catch {
    return value
  }
}

function readExternalIdentityAvatarUrl(
  identities: UserExternalIdentity[] | null | undefined
): string {
  if (!Array.isArray(identities)) {
    return ''
  }

  for (const item of identities) {
    const record = toRecord(item)
    const metadata = toRecord(record?.metadata)
    const avatarUrl = readRecordString(record, 'avatar_url', 'avatarUrl')
      || readRecordString(metadata, 'avatar_url', 'avatarUrl')
    if (avatarUrl) {
      return avatarUrl
    }
  }

  return ''
}

function matchesProvider(record: BindingRecord | null, provider: UserAccountBindingProvider): boolean {
  const normalizedProvider = readString(record, 'provider').toLowerCase()
  if (normalizedProvider === provider) {
    return true
  }

  const providerKey = readString(record, 'provider_key').toLowerCase()
  return (
    providerKey === provider
    || providerKey.startsWith(`${provider}-`)
    || providerKey.startsWith(`${provider}_`)
  )
}

function getBindingRecords(
  bindings: BindingCollection | undefined,
  provider: UserAccountBindingProvider
): BindingRecord[] {
  if (!bindings) {
    return []
  }

  if (Array.isArray(bindings)) {
    const matches: BindingRecord[] = []
    for (const item of bindings) {
      const record = toBindingRecord(item)
      if (record && matchesProvider(record, provider)) {
        matches.push(record)
      }
    }
    return matches
  }

  const matches: BindingRecord[] = []
  const exactMatch = toBindingRecord(bindings[provider])
  if (exactMatch) {
    matches.push(exactMatch)
  }

  for (const value of Object.values(bindings)) {
    const record = toBindingRecord(value)
    if (!record || record === exactMatch || !matchesProvider(record, provider)) {
      continue
    }
    matches.push(record)
  }

  return matches
}

function getExternalIdentityRecords(
  identities: UserExternalIdentity[] | null | undefined,
  provider: UserAccountBindingProvider
): BindingRecord[] {
  if (!Array.isArray(identities)) {
    return []
  }

  const matches: BindingRecord[] = []
  for (const item of identities) {
    const record = toBindingRecord(item)
    if (record && matchesProvider(record, provider)) {
      matches.push(record)
    }
  }

  return matches
}

function readString(source: BindingRecord | null, ...keys: string[]): string {
  for (const key of keys) {
    const trimmed = readFirstString(source?.[key] as string | null | undefined)
    if (trimmed) {
      return trimmed
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

function getLegacyWechatBindingRecords(user: UserWithBindings): BindingRecord[] {
  const subject = readFirstString(user.wechat_unionid, user.wechat_openid, user.wechat)
  const displayName = readFirstString(user.wechat_nickname)
  const bound = user.wechat_bound === true || Boolean(subject || displayName)

  if (!bound) {
    return []
  }

  return [{
    provider: 'wechat',
    provider_key: 'wechat',
    bound,
    provider_subject: subject,
    display_name: displayName
  }]
}

function readChannelValue(source: BindingRecord | null): string {
  const channels = source?.channels
  if (!Array.isArray(channels)) {
    return ''
  }

  for (const channel of channels) {
    const record = toBindingRecord(channel)
    const value = readString(record, 'display_name', 'subject', 'app_id', 'channel')
    if (value) {
      return value
    }
  }

  return ''
}

function hasResolvedBindingIdentity(source: BindingRecord | null): boolean {
  return Boolean(
    readString(
      source,
      'value',
      'identifier',
      'provider_subject',
      'provider_user_id',
      'provider_union_id',
      'provider_username',
      'subject',
      'display_name',
      'email'
    )
    || readChannelValue(source)
  )
}

function isBindingBound(source: BindingRecord | null): boolean {
  return readBoolean(source, 'bound') || hasResolvedBindingIdentity(source)
}

function pickPrimaryBindingRecord(bindings: BindingRecord[]): BindingRecord | null {
  return (
    bindings.find((binding) => isBindingBound(binding) && hasResolvedBindingIdentity(binding))
    || bindings.find((binding) => isBindingBound(binding))
    || bindings.find((binding) => hasResolvedBindingIdentity(binding))
    || bindings[0]
    || null
  )
}

function resolveBindingValue(
  user: UserWithBindings,
  provider: UserAccountBindingProvider,
  bindings: BindingRecord[]
): string {
  const binding = pickPrimaryBindingRecord(bindings)

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
    'provider_user_id',
    'provider_union_id',
    'provider_username',
    'subject',
    'display_name',
    'provider_name',
    'provider_label',
    'issuer'
  ) || readChannelValue(binding)
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
  const bindingRecords = [
    ...getBindingRecords(user.account_bindings, provider),
    ...getExternalIdentityRecords(user.external_identities, provider),
    ...(provider === 'wechat' ? getLegacyWechatBindingRecords(user) : [])
  ]
  const binding = pickPrimaryBindingRecord(bindingRecords)
  const bound =
    provider === 'email'
      ? binding
        ? readBoolean(binding, 'bound')
        : hasLegacyUsableEmailLogin(user)
      : bindingRecords.some((record) => isBindingBound(record))

  const connectUrl = bound ? null : buildConnectUrl(provider, settings)
  const connectDisabled = !bound
    && provider !== 'email'
    && isProviderEnabled(provider, settings)
    && connectUrl === null

  return {
    provider,
    bound,
    value: resolveBindingValue(user, provider, bindingRecords),
    connectUrl,
    connectDisabled,
    availabilityHintKey: bound ? null : resolveAvailabilityHintKey(provider, settings, connectUrl),
    canDisconnect: bindingRecords.some((record) => readBoolean(record, 'can_disconnect'))
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

export function getUserInitials(user: UserWithBindings | null | undefined): string {
  const source = (user?.username || user?.email || 'U').trim()
  if (!source) {
    return 'U'
  }

  const parts = source.split(/[\s@._-]+/).filter(Boolean)
  if (parts.length >= 2) {
    return `${parts[0][0] || ''}${parts[1][0] || ''}`.toUpperCase()
  }

  return source.slice(0, 2).toUpperCase()
}

export function getUserDisplayName(user: UserWithBindings | null | undefined): string {
  if (!user) {
    return ''
  }

  return user.username?.trim() || user.email?.split('@')[0] || user.email || ''
}
