import { describe, expect, it, beforeEach, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import PaymentResultView from '../PaymentResultView.vue'
import type { PaymentOrder } from '@/types/payment'

const {
  verifyOrderPublic,
  verifyOrder,
  pollOrderStatus,
  push,
  routeState,
} = vi.hoisted(() => ({
  verifyOrderPublic: vi.fn(),
  verifyOrder: vi.fn(),
  pollOrderStatus: vi.fn(),
  push: vi.fn(),
  routeState: {
    query: {} as Record<string, unknown>,
  },
}))

const messages: Record<string, string> = {
  'payment.result.success': 'Payment successful',
  'payment.result.failed': 'Payment failed',
  'payment.result.backToRecharge': 'Back to recharge',
  'payment.result.viewOrders': 'View orders',
  'payment.orders.orderId': 'Order ID',
  'payment.orders.orderNo': 'Order No',
  'payment.orders.baseAmount': 'Base amount',
  'payment.orders.fee': 'Fee',
  'payment.orders.payAmount': 'Pay amount',
  'payment.orders.creditedAmount': 'Credited amount',
  'payment.orders.paymentMethod': 'Payment method',
  'payment.orders.status': 'Status',
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

vi.mock('vue-router', () => ({
  useRoute: () => routeState,
  useRouter: () => ({ push }),
}))

vi.mock('@/api/payment', () => ({
  paymentAPI: {
    verifyOrderPublic,
    verifyOrder,
  },
}))

vi.mock('@/stores/payment', () => ({
  usePaymentStore: () => ({
    pollOrderStatus,
  }),
}))

function makeOrder(overrides: Partial<PaymentOrder> = {}): PaymentOrder {
  return {
    id: 42,
    user_id: 9,
    amount: 100,
    pay_amount: 100,
    fee_rate: 0,
    payment_type: 'alipay',
    out_trade_no: 'trade-42',
    status: 'PAID',
    order_type: 'balance',
    created_at: '2026-04-19T00:00:00Z',
    expires_at: '2026-04-19T00:30:00Z',
    refund_amount: 0,
    ...overrides,
  }
}

describe('PaymentResultView', () => {
  beforeEach(() => {
    verifyOrderPublic.mockReset()
    verifyOrder.mockReset()
    pollOrderStatus.mockReset()
    push.mockReset()
    routeState.query = {}
  })

  it('uses public verification first and does not fall back when it succeeds', async () => {
    routeState.query = {
      out_trade_no: 'provider-return-42',
      status: 'success',
    }
    verifyOrderPublic.mockResolvedValue({ data: makeOrder() })

    const wrapper = mount(PaymentResultView, {
      global: {
        stubs: {
          OrderStatusBadge: { template: '<div data-test="order-status-badge" />' },
        },
      },
    })

    await flushPromises()

    expect(verifyOrderPublic).toHaveBeenCalledWith('provider-return-42')
    expect(verifyOrder).not.toHaveBeenCalled()
    expect(pollOrderStatus).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('#42')
  })

  it('handles public verification payloads that omit fee_rate without rendering NaN', async () => {
    routeState.query = {
      out_trade_no: 'public-minimal-shape',
      status: 'success',
    }
    verifyOrderPublic.mockResolvedValue({
      data: {
        id: 84,
        out_trade_no: 'public-minimal-shape',
        amount: 100,
        pay_amount: 100,
        payment_type: 'alipay',
        order_type: 'balance',
        status: 'PAID',
      } as PaymentOrder,
    })

    const wrapper = mount(PaymentResultView, {
      global: {
        stubs: {
          OrderStatusBadge: { template: '<div data-test="order-status-badge" />' },
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('#84')
    expect(wrapper.text()).toContain('100.00')
    expect(wrapper.text()).not.toContain('NaN')
  })

  it('shows fallback return info without parsing numeric order ids from out_trade_no', async () => {
    routeState.query = {
      out_trade_no: 'merchant_order_987',
      money: '88.66',
      type: 'easypay',
      trade_status: 'TRADE_SUCCESS',
      status: 'success',
    }
    verifyOrderPublic.mockRejectedValue(new Error('public verify failed'))
    verifyOrder.mockRejectedValue(new Error('auth verify failed'))

    const wrapper = mount(PaymentResultView, {
      global: {
        stubs: {
          OrderStatusBadge: { template: '<div data-test="order-status-badge" />' },
        },
      },
    })

    await flushPromises()

    expect(verifyOrderPublic).toHaveBeenCalledWith('merchant_order_987')
    expect(verifyOrder).toHaveBeenCalledWith('merchant_order_987')
    expect(pollOrderStatus).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('merchant_order_987')
    expect(wrapper.text()).toContain('88.66')
    expect(wrapper.text()).toContain('Alipay')
  })

  it('normalizes stripe and wechat provider return types to visible payment labels', async () => {
    routeState.query = {
      out_trade_no: 'stripe-return-42',
      money: '66.00',
      type: 'wechat_pay',
      trade_status: 'TRADE_SUCCESS',
      status: 'success',
    }
    verifyOrderPublic.mockResolvedValue({
      data: makeOrder({
        payment_type: 'card',
      }),
    })

    const wrapper = mount(PaymentResultView, {
      global: {
        stubs: {
          OrderStatusBadge: { template: '<div data-test="order-status-badge" />' },
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('Stripe')
    expect(wrapper.text()).not.toContain('card')
  })

  it('normalizes wechat pay fallback return types to the visible label', async () => {
    routeState.query = {
      out_trade_no: 'wxpay-return-42',
      money: '66.00',
      type: 'wechat_pay',
      trade_status: 'TRADE_SUCCESS',
      status: 'success',
    }
    verifyOrderPublic.mockRejectedValue(new Error('public verify failed'))
    verifyOrder.mockRejectedValue(new Error('auth verify failed'))

    const wrapper = mount(PaymentResultView, {
      global: {
        stubs: {
          OrderStatusBadge: { template: '<div data-test="order-status-badge" />' },
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('WeChat Pay')
    expect(wrapper.text()).not.toContain('wechat_pay')
  })
})
