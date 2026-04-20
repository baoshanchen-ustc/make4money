import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import AdminOrderTable from '../AdminOrderTable.vue'
import type { PaymentOrder } from '@/types/payment'

const messages: Record<string, string> = {
  'payment.methods.alipay': 'Alipay',
  'payment.methods.wxpay': 'WeChat Pay',
  'payment.methods.stripe': 'Stripe',
  'payment.orders.orderId': 'Order ID',
  'payment.admin.colUser': 'User',
  'payment.orders.payAmount': 'Pay Amount',
  'payment.orders.paymentMethod': 'Payment Method',
  'payment.orders.status': 'Status',
  'payment.orders.orderType': 'Order Type',
  'payment.orders.createdAt': 'Created At',
  'payment.orders.actions': 'Actions',
  'payment.admin.allStatuses': 'All statuses',
  'payment.admin.allPaymentTypes': 'All payment types',
  'payment.admin.allOrderTypes': 'All order types',
  'payment.admin.balanceOrder': 'Balance',
  'payment.admin.subscriptionOrder': 'Subscription',
  'payment.status.pending': 'Pending',
  'payment.status.paid': 'Paid',
  'payment.status.completed': 'Completed',
  'payment.status.expired': 'Expired',
  'payment.status.cancelled': 'Cancelled',
  'payment.status.failed': 'Failed',
  'payment.status.refunded': 'Refunded',
  'payment.status.refund_requested': 'Refund requested',
  'payment.status.refund_failed': 'Refund failed',
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

const DataTableStub = {
  props: ['data'],
  template: `
    <table>
      <tbody>
        <tr v-for="row in data" :key="row.id">
          <td class="payment-type-cell">
            <slot name="cell-payment_type" :row="row" :value="row.payment_type" />
          </td>
        </tr>
      </tbody>
    </table>
  `,
}

const baseOrder: PaymentOrder = {
  id: 1,
  user_id: 1,
  user_email: 'user@example.com',
  amount: 10,
  pay_amount: 10,
  fee_rate: 0,
  payment_type: 'alipay',
  out_trade_no: 'out-trade-no',
  status: 'PENDING',
  order_type: 'balance',
  created_at: '2025-01-02T03:04:05Z',
  expires_at: '2025-01-02T03:34:05Z',
  refund_amount: 0,
}

describe('AdminOrderTable', () => {
  it('shows normalized labels and preserves raw payment types as secondary text', () => {
    const wrapper = mount(AdminOrderTable, {
      props: {
        orders: [
          { ...baseOrder, id: 1, payment_type: 'alipay_direct' },
          { ...baseOrder, id: 2, payment_type: 'card' },
          { ...baseOrder, id: 3, payment_type: 'wechat_h5' },
        ],
        loading: false,
        page: 1,
        pageSize: 20,
        total: 0,
      },
      global: {
        stubs: {
          DataTable: DataTableStub,
          Pagination: true,
          Select: true,
          Icon: true,
        },
      },
    })

    const cells = wrapper.findAll('.payment-type-cell')
    expect(cells).toHaveLength(3)
    expect(cells[0].text()).toContain('Alipay')
    expect(cells[0].text()).toContain('alipay_direct')
    expect(cells[1].text()).toContain('Stripe')
    expect(cells[1].text()).toContain('card')
    expect(cells[2].text()).toContain('WeChat Pay')
    expect(cells[2].text()).toContain('wechat_h5')
  })
})
