import { describe, expect, it } from 'vitest'

import type { ProviderInstance } from '@/types/payment'
import {
  buildWechatPaymentMpSyncConfig,
  getEnabledProviderKeysForPaymentTypes,
  getUserFacingPaymentTypesForProviderInstance,
  providerInstanceSupportsEnabledPaymentTypes,
  shouldDisableProviderAfterPaymentTypeRemoved,
} from '@/components/payment/providerConfig'

function createProvider(overrides: Partial<ProviderInstance>): ProviderInstance {
  return {
    id: 1,
    provider_key: 'easypay',
    name: 'Provider',
    config: {},
    supported_types: ['alipay', 'wxpay'],
    enabled: true,
    payment_mode: '',
    refund_enabled: false,
    allow_user_refund: false,
    limits: '',
    sort_order: 0,
    ...overrides,
  }
}

describe('provider source mapping', () => {
  it('maps enabled payment methods to the correct provider keys', () => {
    expect(getEnabledProviderKeysForPaymentTypes(['alipay'])).toEqual(['easypay', 'alipay'])
    expect(getEnabledProviderKeysForPaymentTypes(['wxpay'])).toEqual(['easypay', 'wxpay'])
    expect(getEnabledProviderKeysForPaymentTypes(['alipay', 'wxpay', 'stripe'])).toEqual([
      'easypay',
      'alipay',
      'wxpay',
      'stripe',
    ])
  })

  it('normalizes legacy enabled payment methods before mapping provider keys', () => {
    expect(getEnabledProviderKeysForPaymentTypes(['alipay_direct', 'wxpay_direct', 'card', 'link'])).toEqual([
      'easypay',
      'alipay',
      'wxpay',
      'stripe',
    ])
  })

  it('treats easypay as available only when one of its enabled user-facing methods remains', () => {
    const easypay = createProvider({
      provider_key: 'easypay',
      supported_types: ['alipay', 'wxpay'],
    })

    expect(providerInstanceSupportsEnabledPaymentTypes(easypay, ['alipay'])).toBe(true)
    expect(providerInstanceSupportsEnabledPaymentTypes(easypay, ['wxpay'])).toBe(true)
    expect(providerInstanceSupportsEnabledPaymentTypes(easypay, ['stripe'])).toBe(false)
    expect(providerInstanceSupportsEnabledPaymentTypes(easypay, ['wxpay_direct'])).toBe(true)
  })

  it('normalizes provider instance supported types to visible capability labels', () => {
    const easypay = createProvider({
      provider_key: 'easypay',
      supported_types: ['alipay_direct'],
    })

    expect(getUserFacingPaymentTypesForProviderInstance(easypay)).toEqual(['alipay'])
  })

  it('only auto-disables a provider when none of its enabled methods remain', () => {
    const alipayOnlyEasyPay = createProvider({
      provider_key: 'easypay',
      supported_types: ['alipay'],
    })
    const dualEasyPay = createProvider({
      provider_key: 'easypay',
      supported_types: ['alipay', 'wxpay'],
    })
    const stripe = createProvider({
      provider_key: 'stripe',
      supported_types: ['card', 'alipay', 'wxpay', 'link'],
    })

    expect(shouldDisableProviderAfterPaymentTypeRemoved(alipayOnlyEasyPay, 'alipay', ['wxpay'])).toBe(true)
    expect(shouldDisableProviderAfterPaymentTypeRemoved(dualEasyPay, 'alipay', ['wxpay'])).toBe(false)
    expect(shouldDisableProviderAfterPaymentTypeRemoved(stripe, 'stripe', [])).toBe(true)
  })

  it('builds wxpay mp sync payload from login mp config only', () => {
    expect(buildWechatPaymentMpSyncConfig({
      wechat_login_mp_app_id: ' wx-login-app ',
      wechat_login_mp_app_secret: ' wx-login-secret ',
      wechat_login_open_app_id: 'ignored-open-app',
      wechat_login_open_app_secret: 'ignored-open-secret',
      mchId: 'ignored-merchant',
    })).toEqual({
      mpAppId: 'wx-login-app',
      mpAppSecret: 'wx-login-secret',
    })
  })
})
