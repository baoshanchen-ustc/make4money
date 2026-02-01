/**
 * Admin Recharge API endpoints
 * Handles recharge order management for administrators (including refunds)
 */

import { apiClient } from '../client'

/**
 * Admin recharge order type
 */
export interface AdminRechargeOrder {
  id: number
  order_no: string
  user_id: number
  amount: number
  status: 'pending' | 'paid' | 'failed' | 'expired' | 'cancelled' | 'refunded'
  payment_method: string
  payment_channel: string
  transaction_id?: string
  expire_at: string
  paid_at?: string
  refunded_at?: string
  refund_reason?: string
  notes?: string
  created_at: string
  updated_at: string
}

/**
 * Admin recharge order list item (summary)
 */
export interface AdminRechargeOrderListItem {
  id: number
  order_no: string
  user_id: number
  amount: number
  status: string
  payment_method: string
  created_at: string
  paid_at?: string
}

/**
 * Admin recharge orders list response (matches backend AdminListOrdersResponse)
 */
export interface AdminRechargeOrdersListResponse {
  orders: AdminRechargeOrderListItem[]
  total: number
  page: number
  page_size: number
}

/**
 * Refund order request
 */
export interface RefundOrderRequest {
  reason: string
}

/**
 * Refund order response
 */
export interface RefundOrderResponse {
  order_no: string
  status: string
  refund_status: string
  message: string
}

/**
 * List recharge orders with pagination
 * @param page - Page number (default: 1)
 * @param pageSize - Items per page (default: 20)
 * @param filters - Optional filters (status, user_id)
 * @returns Paginated list of recharge orders
 */
export async function listOrders(
  page: number = 1,
  pageSize: number = 20,
  filters?: {
    status?: string
    user_id?: number
  }
): Promise<AdminRechargeOrdersListResponse> {
  const { data } = await apiClient.get<AdminRechargeOrdersListResponse>(
    '/admin/recharge/orders',
    {
      params: {
        page,
        page_size: pageSize,
        status: filters?.status,
        user_id: filters?.user_id
      }
    }
  )
  return data
}

/**
 * Get recharge order details
 * @param orderNo - Order number
 * @returns Order details
 */
export async function getOrder(orderNo: string): Promise<AdminRechargeOrder> {
  const { data } = await apiClient.get<AdminRechargeOrder>(
    `/admin/recharge/orders/${orderNo}`
  )
  return data
}

/**
 * Refund an order
 * @param orderNo - Order number
 * @param request - Refund request with reason
 * @returns Refund result
 */
export async function refundOrder(
  orderNo: string,
  request: RefundOrderRequest
): Promise<RefundOrderResponse> {
  const { data } = await apiClient.post<RefundOrderResponse>(
    `/admin/recharge/orders/${orderNo}/refund`,
    request
  )
  return data
}

export const rechargeAPI = {
  listOrders,
  getOrder,
  refundOrder
}

export default rechargeAPI
