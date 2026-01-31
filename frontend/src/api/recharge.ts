/**
 * Recharge API Client
 * Handles balance recharge configuration and operations
 */

import { apiClient } from './client'

// ==================== Types ====================

export interface RechargeConfig {
  enabled: boolean
  min_amount: number
  max_amount: number
  default_amounts: number[]
}

export interface CreateOrderRequest {
  amount: number
  payment_method: string
  payment_channel?: string // 支付渠道：native/jsapi
}

export interface RechargeOrder {
  id: number
  order_no: string
  amount: number
  status: string
  payment_method: string
  payment_channel: string
  qrcode_url?: string // Native 支付二维码 URL
  prepay_id?: string // JSAPI 预支付 ID
  paid_at?: string // 支付时间
  created_at: string
  expire_at: string
}

// JSAPI 支付调起参数
export interface JSAPIPaymentParams {
  appId: string     // 公众号/小程序 AppID
  timeStamp: string // 时间戳（秒级）
  nonceStr: string  // 随机字符串
  package: string   // 订单详情扩展字符串，格式为 prepay_id=xxx
  signType: string  // 签名类型，固定为 RSA
  paySign: string   // 签名值
}

// 发起支付请求
export interface InitiatePaymentRequest {
  openid?: string // 用户 OpenID（JSAPI 支付必填）
}

// 发起支付响应
export interface InitiatePaymentResponse {
  order_no: string
  payment_channel: string
  qrcode_url?: string        // Native 支付二维码 URL
  prepay_id?: string         // JSAPI 预支付 ID
  jsapi_params?: JSAPIPaymentParams // JSAPI 支付调起参数
}

// 订单列表请求参数
export interface ListOrdersRequest {
  page?: number
  page_size?: number
  status?: string
  start_time?: string // RFC3339 或 YYYY-MM-DD 格式
  end_time?: string   // RFC3339 或 YYYY-MM-DD 格式
}

// 订单列表项
export interface OrderListItem {
  order_no: string
  amount: number
  status: string
  created_at: string
  paid_at?: string
}

// 订单列表响应
export interface ListOrdersResponse {
  orders: OrderListItem[]
  total: number
  page: number
  page_size: number
}

// ==================== API Functions ====================

export const rechargeAPI = {
  /**
   * 获取充值配置（公开接口，无需认证）
   */
  async getConfig(): Promise<RechargeConfig> {
    const response = await apiClient.get<RechargeConfig>('/recharge/config')
    return response.data
  },

  /**
   * 创建充值订单
   */
  async createOrder(data: CreateOrderRequest): Promise<RechargeOrder> {
    const response = await apiClient.post<RechargeOrder>('/recharge/orders', data)
    return response.data
  },

  /**
   * 获取订单详情
   */
  async getOrder(orderNo: string): Promise<RechargeOrder> {
    const response = await apiClient.get<RechargeOrder>(`/recharge/orders/${orderNo}`)
    return response.data
  },

  /**
   * 发起支付
   * 调用微信支付创建预支付订单，返回支付参数
   */
  async initiatePayment(orderNo: string, data?: InitiatePaymentRequest): Promise<InitiatePaymentResponse> {
    const response = await apiClient.post<InitiatePaymentResponse>(`/recharge/orders/${orderNo}/pay`, data || {})
    return response.data
  },

  /**
   * 获取充值记录列表
   */
  async listOrders(params?: ListOrdersRequest): Promise<ListOrdersResponse> {
    const response = await apiClient.get<ListOrdersResponse>('/recharge/orders', { params })
    return response.data
  }
}

export default rechargeAPI
