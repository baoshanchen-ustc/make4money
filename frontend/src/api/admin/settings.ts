/**
 * Admin Settings API endpoints
 * Handles system settings management for administrators
 */

import { apiClient } from '../client'
import type { GalleryData } from '@/types'

/**
 * System settings interface
 */
export interface SystemSettings {
  // Registration settings
  registration_enabled: boolean
  email_verify_enabled: boolean
  promo_code_enabled: boolean
  password_reset_enabled: boolean
  invitation_code_enabled: boolean
  totp_enabled: boolean // TOTP 双因素认证
  totp_encryption_key_configured: boolean // TOTP 加密密钥是否已配置
  // Default settings
  default_balance: number
  default_concurrency: number
  // OEM settings
  site_name: string
  site_logo: string
  site_logo_dark: string
  site_subtitle: string
  api_base_url: string
  contact_info: string
  contact_qrcode_wechat: string
  contact_qrcode_group: string
  doc_url: string
  home_content: string
  hide_ccs_import_button: boolean
  purchase_subscription_enabled: boolean
  purchase_subscription_url: string
  // SMTP settings
  smtp_host: string
  smtp_port: number
  smtp_username: string
  smtp_password_configured: boolean
  smtp_from_email: string
  smtp_from_name: string
  smtp_use_tls: boolean
  // Cloudflare Turnstile settings
  turnstile_enabled: boolean
  turnstile_site_key: string
  turnstile_secret_key_configured: boolean

  // LinuxDo Connect OAuth settings
  linuxdo_connect_enabled: boolean
  linuxdo_connect_client_id: string
  linuxdo_connect_client_secret_configured: boolean
  linuxdo_connect_redirect_url: string

  // WeChat public account verification code login settings
  wechat_auth_enabled: boolean
  wechat_account_type: 'subscription' | 'unverified_official' | string
  wechat_server_address: string
  wechat_server_token_configured: boolean
  wechat_account_qrcode_url: string
  wechat_account_qrcode_data: string
  wechat_app_id: string
  wechat_app_secret_configured: boolean
  force_email_bind: boolean

  // Model fallback configuration
  enable_model_fallback: boolean
  fallback_model_anthropic: string
  fallback_model_openai: string
  fallback_model_gemini: string
  fallback_model_antigravity: string

  // Identity patch configuration (Claude -> Gemini)
  enable_identity_patch: boolean
  identity_patch_prompt: string

  // Ops Monitoring (vNext)
  ops_monitoring_enabled: boolean
  ops_realtime_monitoring_enabled: boolean
  ops_query_mode_default: 'auto' | 'raw' | 'preagg' | string
  ops_metrics_interval_seconds: number

  // Usage Report Settings
  usage_report_global_enabled: boolean
  usage_report_target_scope: 'all' | 'active_today' | 'opted_in' | string
  usage_report_global_schedule: string

  // Homepage & Install Guide
  install_guide_videos: string
  home_testimonials: string

  // Account expiry reminder
  account_expiry_reminder_email: string
  account_expiry_reminder_advance_days: number

  // Balance lot expiry
  balance_lot_expiry_days: number
  balance_expiry_reminder_enabled: boolean
  balance_expiry_reminder_advance_days: number
}

export interface UpdateSettingsRequest {
  registration_enabled?: boolean
  email_verify_enabled?: boolean
  promo_code_enabled?: boolean
  password_reset_enabled?: boolean
  invitation_code_enabled?: boolean
  totp_enabled?: boolean // TOTP 双因素认证
  default_balance?: number
  default_concurrency?: number
  site_name?: string
  site_logo?: string
  site_logo_dark?: string
  site_subtitle?: string
  api_base_url?: string
  contact_info?: string
  contact_qrcode_wechat?: string
  contact_qrcode_group?: string
  doc_url?: string
  home_content?: string
  hide_ccs_import_button?: boolean
  purchase_subscription_enabled?: boolean
  purchase_subscription_url?: string
  smtp_host?: string
  smtp_port?: number
  smtp_username?: string
  smtp_password?: string
  smtp_from_email?: string
  smtp_from_name?: string
  smtp_use_tls?: boolean
  turnstile_enabled?: boolean
  turnstile_site_key?: string
  turnstile_secret_key?: string
  linuxdo_connect_enabled?: boolean
  linuxdo_connect_client_id?: string
  linuxdo_connect_client_secret?: string
  linuxdo_connect_redirect_url?: string
  force_email_bind?: boolean
  wechat_auth_enabled?: boolean
  wechat_account_type?: 'subscription' | 'unverified_official' | string
  wechat_server_address?: string
  wechat_server_token?: string
  wechat_account_qrcode_url?: string
  wechat_account_qrcode_data?: string
  wechat_app_id?: string
  wechat_app_secret?: string
  enable_model_fallback?: boolean
  fallback_model_anthropic?: string
  fallback_model_openai?: string
  fallback_model_gemini?: string
  fallback_model_antigravity?: string
  enable_identity_patch?: boolean
  identity_patch_prompt?: string
  ops_monitoring_enabled?: boolean
  ops_realtime_monitoring_enabled?: boolean
  ops_query_mode_default?: 'auto' | 'raw' | 'preagg' | string
  ops_metrics_interval_seconds?: number
  // Usage Report Settings
  usage_report_global_enabled?: boolean
  usage_report_target_scope?: 'all' | 'active_today' | 'opted_in' | string
  usage_report_global_schedule?: string

  // Homepage & Install Guide
  install_guide_videos?: string
  home_testimonials?: string

  // Account expiry reminder
  account_expiry_reminder_email?: string
  account_expiry_reminder_advance_days?: number

  // Balance lot expiry
  balance_lot_expiry_days?: number
  balance_expiry_reminder_enabled?: boolean
  balance_expiry_reminder_advance_days?: number
}

/**
 * Get all system settings
 * @returns System settings
 */
export async function getSettings(): Promise<SystemSettings> {
  const { data } = await apiClient.get<SystemSettings>('/admin/settings')
  return data
}

/**
 * Update system settings
 * @param settings - Partial settings to update
 * @returns Updated settings
 */
export async function updateSettings(settings: UpdateSettingsRequest): Promise<SystemSettings> {
  const { data } = await apiClient.put<SystemSettings>('/admin/settings', settings)
  return data
}

/**
 * Test SMTP connection request
 */
export interface TestSmtpRequest {
  smtp_host: string
  smtp_port: number
  smtp_username: string
  smtp_password: string
  smtp_use_tls: boolean
}

/**
 * Test SMTP connection with provided config
 * @param config - SMTP configuration to test
 * @returns Test result message
 */
export async function testSmtpConnection(config: TestSmtpRequest): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>('/admin/settings/test-smtp', config)
  return data
}

/**
 * Send test email request
 */
export interface SendTestEmailRequest {
  email: string
  smtp_host: string
  smtp_port: number
  smtp_username: string
  smtp_password: string
  smtp_from_email: string
  smtp_from_name: string
  smtp_use_tls: boolean
}

/**
 * Send test email with provided SMTP config
 * @param request - Email address and SMTP config
 * @returns Test result message
 */
export async function sendTestEmail(request: SendTestEmailRequest): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>(
    '/admin/settings/send-test-email',
    request
  )
  return data
}

/**
 * Admin API Key status response
 */
export interface AdminApiKeyStatus {
  exists: boolean
  masked_key: string
}

/**
 * Get admin API key status
 * @returns Status indicating if key exists and masked version
 */
export async function getAdminApiKey(): Promise<AdminApiKeyStatus> {
  const { data } = await apiClient.get<AdminApiKeyStatus>('/admin/settings/admin-api-key')
  return data
}

/**
 * Regenerate admin API key
 * @returns The new full API key (only shown once)
 */
export async function regenerateAdminApiKey(): Promise<{ key: string }> {
  const { data } = await apiClient.post<{ key: string }>('/admin/settings/admin-api-key/regenerate')
  return data
}

/**
 * Delete admin API key
 * @returns Success message
 */
export async function deleteAdminApiKey(): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>('/admin/settings/admin-api-key')
  return data
}

/**
 * Stream timeout settings interface
 */
export interface StreamTimeoutSettings {
  enabled: boolean
  action: 'temp_unsched' | 'error' | 'none'
  temp_unsched_minutes: number
  threshold_count: number
  threshold_window_minutes: number
}

/**
 * Get stream timeout settings
 * @returns Stream timeout settings
 */
export async function getStreamTimeoutSettings(): Promise<StreamTimeoutSettings> {
  const { data } = await apiClient.get<StreamTimeoutSettings>('/admin/settings/stream-timeout')
  return data
}

/**
 * Update stream timeout settings
 * @param settings - Stream timeout settings to update
 * @returns Updated settings
 */
export async function updateStreamTimeoutSettings(
  settings: StreamTimeoutSettings
): Promise<StreamTimeoutSettings> {
  const { data } = await apiClient.put<StreamTimeoutSettings>(
    '/admin/settings/stream-timeout',
    settings
  )
  return data
}

/**
 * Generate WeChat QR code request
 */
export interface GenerateWeChatQRCodeRequest {
  app_id?: string
  app_secret?: string
  scene_str?: string
}

/**
 * Generate WeChat QR code response
 */
export interface GenerateWeChatQRCodeResponse {
  qrcode_url: string
  ticket: string
}

/**
 * Generate WeChat permanent QR code
 * @param request - AppID, AppSecret and scene string
 * @returns QR code URL and ticket
 */
export async function generateWeChatQRCode(
  request: GenerateWeChatQRCodeRequest
): Promise<GenerateWeChatQRCodeResponse> {
  const { data } = await apiClient.post<GenerateWeChatQRCodeResponse>(
    '/admin/settings/wechat/generate-qrcode',
    request
  )
  return data
}

/**
 * WeChat Pay configuration status (read-only, masked)
 */
export interface WeChatPayStatus {
  enabled: boolean
  app_id_masked: string
  mch_id_masked: string
  notify_url: string
  configured: boolean
}

/**
 * Get WeChat Pay configuration status (read-only, masked)
 * @returns WeChat Pay status with masked sensitive fields
 */
export async function getWeChatPayStatus(): Promise<WeChatPayStatus> {
  const { data } = await apiClient.get<WeChatPayStatus>('/admin/settings/payment/wechat/status')
  return data
}

/**
 * Recharge settings interface
 */
export interface RechargeSettings {
  min_amount: number
  max_amount: number
  default_amounts: number[]
  order_expire_minutes: number
  exchange_rate: number // 人民币兑额度汇率
}

/**
 * Get recharge settings
 * @returns Recharge settings
 */
export async function getRechargeSettings(): Promise<RechargeSettings> {
  const { data } = await apiClient.get<RechargeSettings>('/admin/settings/recharge')
  return data
}

/**
 * Update recharge settings
 * @param settings - Recharge settings to update
 * @returns Updated settings
 */
export async function updateRechargeSettings(settings: RechargeSettings): Promise<RechargeSettings> {
  const { data } = await apiClient.put<RechargeSettings>('/admin/settings/recharge', settings)
  return data
}

export type { GalleryData }

/**
 * Get home gallery settings
 * @returns Gallery data or null if not configured
 */
export async function getHomeGallery(): Promise<GalleryData | null> {
  const { data } = await apiClient.get<GalleryData | null>('/admin/settings/gallery')
  return data
}

/**
 * Update home gallery settings
 * @param galleryData - Full gallery data object
 * @returns Success message
 */
export async function updateHomeGallery(galleryData: GalleryData): Promise<{ message: string }> {
  const { data } = await apiClient.put<{ message: string }>('/admin/settings/gallery', galleryData)
  return data
}

export const settingsAPI = {
  getSettings,
  updateSettings,
  testSmtpConnection,
  sendTestEmail,
  getAdminApiKey,
  regenerateAdminApiKey,
  deleteAdminApiKey,
  getStreamTimeoutSettings,
  updateStreamTimeoutSettings,
  generateWeChatQRCode,
  getWeChatPayStatus,
  getRechargeSettings,
  updateRechargeSettings,
  getHomeGallery,
  updateHomeGallery
}

export default settingsAPI
