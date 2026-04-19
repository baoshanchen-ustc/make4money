import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import PaymentMethodChart from '../PaymentMethodChart.vue'

const messages: Record<string, string> = {
  'payment.admin.paymentDistribution': 'Payment Distribution',
  'payment.admin.noData': 'No data',
  'payment.methods.alipay': 'Alipay',
  'payment.methods.wxpay': 'WeChat Pay',
  'payment.methods.stripe': 'Stripe',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, fallback?: string) => messages[key] ?? fallback ?? key,
    }),
  }
})

describe('PaymentMethodChart', () => {
  it('renders normalized payment methods in capability order', () => {
    const wrapper = mount(PaymentMethodChart, {
      props: {
        methods: [
          { type: 'stripe', amount: 7.5, count: 2 },
          { type: 'alipay', amount: 15, count: 3 },
          { type: 'wxpay', amount: 20, count: 1 },
        ],
      },
    })

    const rows = wrapper.findAll('.space-y-1')
    expect(rows).toHaveLength(3)
    expect(rows[0].text()).toContain('Alipay')
    expect(rows[0].text()).toContain('¥15.00')
    expect(rows[0].text()).toContain('(3)')
    expect(rows[1].text()).toContain('WeChat Pay')
    expect(rows[1].text()).toContain('¥20.00')
    expect(rows[2].text()).toContain('Stripe')
    expect(rows[2].text()).toContain('¥7.50')
    expect(wrapper.text()).not.toContain('alipay_direct')
    expect(wrapper.text()).not.toContain('card')
  })

  it('renders the empty state when no payment methods exist', () => {
    const wrapper = mount(PaymentMethodChart, {
      props: {
        methods: [],
      },
    })

    expect(wrapper.text()).toContain('No data')
  })
})
