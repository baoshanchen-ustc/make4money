/**
 * Subscription Plan API Client
 * Handles purchasable subscription plans and subscription orders
 */

import axios from 'axios'
import { apiClient } from './client'
import {
  RateLimitExceededError,
  CaptchaRequiredError,
  type RateLimitErrorResponse,
  type CaptchaRequiredResponse,
} from './recharge'

// ==================== Types ====================

/**
 * Purchasable subscription plan
 */
export interface SubscriptionPlan {
  id: number
  name: string
  description: string | null
  purchasable_description: string | null
  platform: string
  price_cny: number
  daily_limit_usd: number | null
  weekly_limit_usd: number | null
  monthly_limit_usd: number | null
  display_order: number
}

/**
 * Create subscription order request
 */
export interface CreateSubscriptionOrderRequest {
  group_id: number
  payment_method: string
  payment_channel?: string // native/jsapi
  captcha_token?: string  // Turnstile token
}

/**
 * Subscription order
 */
export interface SubscriptionOrder {
  id: number
  order_no: string
  user_id: number
  group_id: number
  amount: number
  validity_days: number
  payment_method: string
  payment_channel: string
  status: 'pending' | 'paying' | 'paid' | 'failed' | 'expired' | 'cancelled'
  qrcode_url?: string
  prepay_id?: string
  wechat_transaction_id?: string
  paid_at?: string
  created_at: string
  expire_at: string
  group_name?: string
}

// JSAPI payment params
export interface JSAPIPaymentParams {
  appId: string
  timeStamp: string
  nonceStr: string
  package: string
  signType: string
  paySign: string
}

// Initiate payment request
export interface InitiatePaymentRequest {
  openid?: string // Required for JSAPI
}

// Initiate payment response
export interface InitiatePaymentResponse {
  order_no: string
  payment_channel: string
  qrcode_url?: string
  prepay_id?: string
  jsapi_params?: JSAPIPaymentParams
}

// Order list request params
export interface ListOrdersRequest {
  page?: number
  page_size?: number
  status?: string
  start_time?: string
  end_time?: string
}

// Order list item
export interface OrderListItem {
  order_no: string
  group_id: number
  group_name: string
  amount: number
  validity_days: number
  status: string
  created_at: string
  paid_at?: string
}

// Order list response
export interface ListOrdersResponse {
  orders: OrderListItem[]
  total: number
  page: number
  page_size: number
}

// Sync order status response
export interface SyncOrderStatusResponse {
  order_no: string
  status: 'pending' | 'paying' | 'paid' | 'failed' | 'expired' | 'cancelled'
  wechat_status: string
  synced_at: string
}

// ==================== API Functions ====================

export const subscriptionPlanAPI = {
  /**
   * Get purchasable subscription plans (public, no auth required)
   */
  async listPlans(): Promise<SubscriptionPlan[]> {
    const response = await apiClient.get<SubscriptionPlan[]>('/subscription-plans')
    return response.data
  },

  /**
   * Create subscription order
   * @throws {RateLimitExceededError} When rate limited
   * @throws {CaptchaRequiredError} When captcha required
   */
  async createOrder(data: CreateSubscriptionOrderRequest): Promise<SubscriptionOrder> {
    try {
      const response = await apiClient.post<SubscriptionOrder>('/subscription-orders', data)
      return response.data
    } catch (error) {
      if (axios.isAxiosError(error)) {
        // Handle 428 Precondition Required - captcha required
        if (error.response?.status === 428) {
          const captchaData = error.response.data as CaptchaRequiredResponse
          throw new CaptchaRequiredError(captchaData.message || '请完成验证码验证')
        }
        // Handle 429 Too Many Requests - rate limited
        if (error.response?.status === 429) {
          const rateLimitData = error.response.data as RateLimitErrorResponse
          const limitType = rateLimitData.limit_type || 'minute'
          throw new RateLimitExceededError(
            rateLimitData.message || '操作过于频繁，请稍后重试',
            limitType,
            rateLimitData.retry_after,
            rateLimitData.reset_time
          )
        }
      }
      throw error
    }
  },

  /**
   * Get order details
   */
  async getOrder(orderNo: string): Promise<SubscriptionOrder> {
    const response = await apiClient.get<SubscriptionOrder>(`/subscription-orders/${orderNo}`)
    return response.data
  },

  /**
   * Initiate payment
   */
  async initiatePayment(orderNo: string, data?: InitiatePaymentRequest): Promise<InitiatePaymentResponse> {
    const response = await apiClient.post<InitiatePaymentResponse>(`/subscription-orders/${orderNo}/pay`, data || {})
    return response.data
  },

  /**
   * List subscription orders
   */
  async listOrders(params?: ListOrdersRequest): Promise<ListOrdersResponse> {
    const response = await apiClient.get<ListOrdersResponse>('/subscription-orders', { params })
    return response.data
  },

  /**
   * Cancel order
   */
  async cancelOrder(orderNo: string): Promise<void> {
    await apiClient.post(`/subscription-orders/${orderNo}/cancel`)
  },

  /**
   * Sync order status with WeChat
   */
  async syncOrderStatus(orderNo: string): Promise<SyncOrderStatusResponse> {
    const response = await apiClient.post<SyncOrderStatusResponse>(`/subscription-orders/${orderNo}/sync`)
    return response.data
  },
}

export default subscriptionPlanAPI
