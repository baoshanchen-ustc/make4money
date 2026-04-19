/**
 * Shared constants and types for payment provider management.
 */

import type { PaymentProviderKey, ProviderInstance } from '@/types/payment'

// --- Types ---

export interface ConfigFieldDef {
  key: string
  label?: string
  labelKey?: string
  hintKey?: string
  section?: 'default' | 'merchant' | 'wechatMp'
  sensitive: boolean
  optional?: boolean
  defaultValue?: string
}

export interface TypeOption {
  value: string
  label: string
}

/** Callback URL paths for a provider. */
export interface CallbackPaths {
  notifyUrl?: string
  returnUrl?: string
}

type ProviderSourceInstance = Pick<ProviderInstance, 'provider_key' | 'supported_types'>

export interface WechatLoginMpSettings {
  wechat_login_mp_app_id?: string
  wechat_login_mp_app_secret?: string
}

// --- Constants ---

/** Maps provider key → available payment types. */
export const PROVIDER_SUPPORTED_TYPES: Record<string, string[]> = {
  easypay: ['alipay', 'wxpay'],
  alipay: ['alipay'],
  wxpay: ['wxpay'],
  stripe: ['card', 'alipay', 'wxpay', 'link'],
}

/** User-facing capability tags per provider. */
export const PROVIDER_CAPABILITY_TYPES: Record<string, string[]> = {
  easypay: ['alipay', 'wxpay'],
  alipay: ['alipay'],
  wxpay: ['wxpay'],
  stripe: ['stripe'],
}

/** Available payment modes for EasyPay providers. */
export const EASYPAY_PAYMENT_MODES = ['qrcode', 'popup'] as const

/** Fixed display order for user-facing payment methods */
export const METHOD_ORDER = ['alipay', 'alipay_direct', 'wxpay', 'wxpay_direct', 'stripe'] as const

/** Payment mode constants */
export const PAYMENT_MODE_QRCODE = 'qrcode'
export const PAYMENT_MODE_POPUP = 'popup'

/** Window features for payment popup windows */
export const POPUP_WINDOW_FEATURES = 'width=1000,height=750,left=100,top=80,scrollbars=yes,resizable=yes'

/** Wider popup for Stripe redirect methods (Alipay checkout page needs ~1200px) */
export const STRIPE_POPUP_WINDOW_FEATURES = 'width=1250,height=780,left=80,top=60,scrollbars=yes,resizable=yes'

/** Webhook paths for each provider (relative to origin). */
export const WEBHOOK_PATHS: Record<string, string> = {
  easypay: '/api/v1/payment/webhook/easypay',
  alipay: '/api/v1/payment/webhook/alipay',
  wxpay: '/api/v1/payment/webhook/wxpay',
  stripe: '/api/v1/payment/webhook/stripe',
}

export const RETURN_PATH = '/payment/result'

/** Fixed callback paths per provider — displayed as read-only after base URL. */
export const PROVIDER_CALLBACK_PATHS: Record<string, CallbackPaths> = {
  easypay: { notifyUrl: WEBHOOK_PATHS.easypay, returnUrl: RETURN_PATH },
  alipay: { notifyUrl: WEBHOOK_PATHS.alipay, returnUrl: RETURN_PATH },
  wxpay: { notifyUrl: WEBHOOK_PATHS.wxpay },
  // stripe: no callback URL config needed (webhook is separate)
}

/** Per-provider config fields (excludes notifyUrl/returnUrl which are handled separately). */
export const PROVIDER_CONFIG_FIELDS: Record<string, ConfigFieldDef[]> = {
  easypay: [
    { key: 'pid', labelKey: 'admin.settings.payment.field_pid', hintKey: 'admin.settings.payment.fieldHint_easypay_pid', sensitive: false },
    { key: 'pkey', labelKey: 'admin.settings.payment.field_pkey', hintKey: 'admin.settings.payment.fieldHint_easypay_pkey', sensitive: true },
    { key: 'apiBase', labelKey: 'admin.settings.payment.field_apiBase', hintKey: 'admin.settings.payment.fieldHint_easypay_apiBase', sensitive: false },
    { key: 'cidAlipay', labelKey: 'admin.settings.payment.field_cidAlipay', hintKey: 'admin.settings.payment.fieldHint_easypay_cidAlipay', sensitive: false, optional: true },
    { key: 'cidWxpay', labelKey: 'admin.settings.payment.field_cidWxpay', hintKey: 'admin.settings.payment.fieldHint_easypay_cidWxpay', sensitive: false, optional: true },
  ],
  alipay: [
    { key: 'appId', labelKey: 'admin.settings.payment.field_appId', hintKey: 'admin.settings.payment.fieldHint_alipay_appId', sensitive: false },
    { key: 'privateKey', labelKey: 'admin.settings.payment.field_privateKey', hintKey: 'admin.settings.payment.fieldHint_alipay_privateKey', sensitive: true },
    { key: 'publicKey', labelKey: 'admin.settings.payment.field_publicKey', hintKey: 'admin.settings.payment.fieldHint_alipay_publicKey', sensitive: true },
  ],
  wxpay: [
    { key: 'appId', labelKey: 'admin.settings.payment.field_appId', hintKey: 'admin.settings.payment.fieldHint_wxpay_appId', section: 'merchant', sensitive: false },
    { key: 'mchId', labelKey: 'admin.settings.payment.field_mchId', hintKey: 'admin.settings.payment.fieldHint_wxpay_mchId', section: 'merchant', sensitive: false },
    { key: 'privateKey', labelKey: 'admin.settings.payment.field_privateKey', hintKey: 'admin.settings.payment.fieldHint_wxpay_privateKey', section: 'merchant', sensitive: true },
    { key: 'apiV3Key', labelKey: 'admin.settings.payment.field_apiV3Key', hintKey: 'admin.settings.payment.fieldHint_wxpay_apiV3Key', section: 'merchant', sensitive: true },
    { key: 'publicKey', labelKey: 'admin.settings.payment.field_publicKey', hintKey: 'admin.settings.payment.fieldHint_wxpay_publicKey', section: 'merchant', sensitive: true },
    { key: 'publicKeyId', labelKey: 'admin.settings.payment.field_publicKeyId', hintKey: 'admin.settings.payment.fieldHint_wxpay_publicKeyId', section: 'merchant', sensitive: false, optional: true },
    { key: 'certSerial', labelKey: 'admin.settings.payment.field_certSerial', hintKey: 'admin.settings.payment.fieldHint_wxpay_certSerial', section: 'merchant', sensitive: false, optional: true },
    { key: 'mpAppId', labelKey: 'admin.settings.payment.field_mpAppId', hintKey: 'admin.settings.payment.fieldHint_wxpay_mpAppId', section: 'wechatMp', sensitive: false, optional: true },
    { key: 'mpAppSecret', labelKey: 'admin.settings.payment.field_mpAppSecret', hintKey: 'admin.settings.payment.fieldHint_wxpay_mpAppSecret', section: 'wechatMp', sensitive: true, optional: true },
  ],
  stripe: [
    { key: 'secretKey', labelKey: 'admin.settings.payment.field_secretKey', hintKey: 'admin.settings.payment.fieldHint_stripe_secretKey', sensitive: true },
    { key: 'publishableKey', labelKey: 'admin.settings.payment.field_publishableKey', hintKey: 'admin.settings.payment.fieldHint_stripe_publishableKey', sensitive: false },
    { key: 'webhookSecret', labelKey: 'admin.settings.payment.field_webhookSecret', hintKey: 'admin.settings.payment.fieldHint_stripe_webhookSecret', sensitive: true },
  ],
}

// --- Helpers ---

export function normalizeVisiblePaymentType(type: string): string {
  const lower = type.trim().toLowerCase()
  if (lower === 'stripe' || lower.includes('stripe') || lower === 'card' || lower === 'link') return 'stripe'
  if (lower.includes('wxpay') || lower.includes('wechat')) return 'wxpay'
  if (lower.includes('alipay') || lower === 'easypay') return 'alipay'
  return lower
}

function normalizeVisiblePaymentTypes(types: string[]): string[] {
  const seen = new Set<string>()
  return types
    .map(normalizeVisiblePaymentType)
    .filter((type) => {
      if (!['alipay', 'wxpay', 'stripe'].includes(type) || seen.has(type)) return false
      seen.add(type)
      return true
    })
}

export function getProviderCapabilityTypes(providerKey: string): string[] {
  return PROVIDER_CAPABILITY_TYPES[providerKey] || []
}

export function getEnabledProviderKeysForPaymentTypes(enabledPaymentTypes: string[]): PaymentProviderKey[] {
  const enabledSet = new Set(normalizeVisiblePaymentTypes(enabledPaymentTypes))
  return (Object.keys(PROVIDER_CAPABILITY_TYPES) as PaymentProviderKey[])
    .filter(providerKey => getProviderCapabilityTypes(providerKey).some(type => enabledSet.has(type)))
}

export function providerKeySupportsEnabledPaymentTypes(providerKey: string, enabledPaymentTypes: string[]): boolean {
  const enabledSet = new Set(normalizeVisiblePaymentTypes(enabledPaymentTypes))
  return getProviderCapabilityTypes(providerKey).some(type => enabledSet.has(type))
}

export function getUserFacingPaymentTypesForProviderInstance(provider: ProviderSourceInstance): string[] {
  const capabilityTypes = getProviderCapabilityTypes(provider.provider_key)
  if (!capabilityTypes.length) return []
  if (provider.provider_key === 'stripe') return capabilityTypes

  const selectedTypes = normalizeVisiblePaymentTypes(provider.supported_types)
    .filter(type => capabilityTypes.includes(type))
  return selectedTypes.length ? selectedTypes : capabilityTypes
}

export function providerInstanceSupportsEnabledPaymentTypes(
  provider: ProviderSourceInstance,
  enabledPaymentTypes: string[],
): boolean {
  const enabledSet = new Set(normalizeVisiblePaymentTypes(enabledPaymentTypes))
  return getUserFacingPaymentTypesForProviderInstance(provider).some(type => enabledSet.has(type))
}

export function shouldDisableProviderAfterPaymentTypeRemoved(
  provider: ProviderSourceInstance,
  removedPaymentType: string,
  remainingEnabledPaymentTypes: string[],
): boolean {
  return providerInstanceSupportsEnabledPaymentTypes(provider, [removedPaymentType])
    && !providerInstanceSupportsEnabledPaymentTypes(provider, remainingEnabledPaymentTypes)
}

/** Resolve type label for display. */
export function resolveTypeLabel(
  typeVal: string,
  _providerKey: string,
  allTypes: TypeOption[],
  _redirectLabel: string,
): TypeOption {
  return allTypes.find(pt => pt.value === typeVal) || { value: typeVal, label: typeVal }
}

/** Get available type options for a provider key. */
export function getAvailableTypes(
  providerKey: string,
  allTypes: TypeOption[],
  redirectLabel: string,
): TypeOption[] {
  const types = PROVIDER_SUPPORTED_TYPES[providerKey] || []
  return types.map(t => resolveTypeLabel(t, providerKey, allTypes, redirectLabel))
}

/** Extract base URL from a full callback URL by removing the known path suffix. */
export function extractBaseUrl(fullUrl: string, path: string): string {
  if (!fullUrl) return ''
  if (fullUrl.endsWith(path)) return fullUrl.slice(0, -path.length)
  // Fallback: try to extract origin
  try { return new URL(fullUrl).origin } catch { return fullUrl }
}

export function buildWechatPaymentMpSyncConfig(settings: WechatLoginMpSettings): Record<string, string> {
  return {
    mpAppId: (settings.wechat_login_mp_app_id || '').trim(),
    mpAppSecret: (settings.wechat_login_mp_app_secret || '').trim(),
  }
}
