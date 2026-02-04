/**
 * Authentication API endpoints
 * Handles user login, registration, and logout operations
 */

import { apiClient } from './client'
import type {
  LoginRequest,
  RegisterRequest,
  AuthResponse,
  CurrentUserResponse,
  SendVerifyCodeRequest,
  SendVerifyCodeResponse,
  PublicSettings,
  TotpLoginResponse,
  TotpLogin2FARequest
} from '@/types'

/**
 * Login response type - can be either full auth or 2FA required
 */
export type LoginResponse = AuthResponse | TotpLoginResponse

/**
 * Type guard to check if login response requires 2FA
 */
export function isTotp2FARequired(response: LoginResponse): response is TotpLoginResponse {
  return 'requires_2fa' in response && response.requires_2fa === true
}

/**
 * Store authentication token in localStorage
 */
export function setAuthToken(token: string): void {
  localStorage.setItem('auth_token', token)
}

/**
 * Get authentication token from localStorage
 */
export function getAuthToken(): string | null {
  return localStorage.getItem('auth_token')
}

/**
 * Clear authentication token from localStorage
 */
export function clearAuthToken(): void {
  localStorage.removeItem('auth_token')
  localStorage.removeItem('auth_user')
}

/**
 * User login
 * @param credentials - Email and password
 * @returns Authentication response with token and user data, or 2FA required response
 */
export async function login(credentials: LoginRequest): Promise<LoginResponse> {
  const { data } = await apiClient.post<LoginResponse>('/auth/login', credentials)

  // Only store token if 2FA is not required
  if (!isTotp2FARequired(data)) {
    setAuthToken(data.access_token)
    localStorage.setItem('auth_user', JSON.stringify(data.user))
  }

  return data
}

/**
 * Complete login with 2FA code
 * @param request - Temp token and TOTP code
 * @returns Authentication response with token and user data
 */
export async function login2FA(request: TotpLogin2FARequest): Promise<AuthResponse> {
  const { data } = await apiClient.post<AuthResponse>('/auth/login/2fa', request)

  // Store token and user data
  setAuthToken(data.access_token)
  localStorage.setItem('auth_user', JSON.stringify(data.user))

  return data
}

/**
 * User registration
 * @param userData - Registration data (username, email, password)
 * @returns Authentication response with token and user data
 */
export async function register(userData: RegisterRequest): Promise<AuthResponse> {
  const { data } = await apiClient.post<AuthResponse>('/auth/register', userData)

  // Store token and user data
  setAuthToken(data.access_token)
  localStorage.setItem('auth_user', JSON.stringify(data.user))

  return data
}

/**
 * Get current authenticated user
 * @returns User profile data
 */
export async function getCurrentUser() {
  return apiClient.get<CurrentUserResponse>('/auth/me')
}

/**
 * User logout
 * Clears authentication token and user data from localStorage
 */
export function logout(): void {
  clearAuthToken()
  // Optionally redirect to login page
  // window.location.href = '/login';
}

/**
 * Check if user is authenticated
 * @returns True if user has valid token
 */
export function isAuthenticated(): boolean {
  return getAuthToken() !== null
}

/**
 * Get public settings (no auth required)
 * @returns Public settings including registration and Turnstile config
 */
export async function getPublicSettings(): Promise<PublicSettings> {
  const { data } = await apiClient.get<PublicSettings>('/settings/public')
  return data
}

/**
 * Send verification code to email
 * @param request - Email and optional Turnstile token
 * @returns Response with countdown seconds
 */
export async function sendVerifyCode(
  request: SendVerifyCodeRequest
): Promise<SendVerifyCodeResponse> {
  const { data } = await apiClient.post<SendVerifyCodeResponse>('/auth/send-verify-code', request)
  return data
}

/**
 * Validate promo code response
 */
export interface ValidatePromoCodeResponse {
  valid: boolean
  bonus_amount?: number
  error_code?: string
  message?: string
}

/**
 * Validate promo code (public endpoint, no auth required)
 * @param code - Promo code to validate
 * @returns Validation result with bonus amount if valid
 */
export async function validatePromoCode(code: string): Promise<ValidatePromoCodeResponse> {
  const { data } = await apiClient.post<ValidatePromoCodeResponse>('/auth/validate-promo-code', { code })
  return data
}

/**
 * WeChat public account verification code login
 * @param code - Verification code from WeChat public account
 * @returns Authentication response with token and user data
 */
export async function wechatAuth(code: string): Promise<AuthResponse> {
  const { data } = await apiClient.get<AuthResponse>('/auth/oauth/wechat', {
    params: { code }
  })

  // Store token and user data
  setAuthToken(data.access_token)
  localStorage.setItem('auth_user', JSON.stringify(data.user))

  return data
}

/**
 * WeChat scan login init response
 */
export interface WeChatScanInitResponse {
  scene_id: string
  qr_code_url: string
  expire_seconds: number
}

/**
 * WeChat scan login poll response
 */
export interface WeChatScanPollResponse {
  status: 'waiting' | 'confirmed'
  access_token?: string
  token_type?: string
  user?: {
    id: string
    email: string
    username: string
    role: string
    created_at: string
  }
}

/**
 * Initialize WeChat scan login session (subscription account only)
 * @returns Scene ID and QR code URL for scanning
 */
export async function wechatScanInit(): Promise<WeChatScanInitResponse> {
  const { data } = await apiClient.get<WeChatScanInitResponse>('/auth/wechat/scan/init')
  return data
}

/**
 * Poll WeChat scan login status
 * @param sceneId - Scene ID from wechatScanInit
 * @returns Status and auth data if confirmed
 */
export async function wechatScanPoll(sceneId: string): Promise<WeChatScanPollResponse> {
  const { data } = await apiClient.get<WeChatScanPollResponse>('/auth/wechat/scan/poll', {
    params: { scene_id: sceneId }
  })

  // If confirmed, store token and user data
  if (data.status === 'confirmed' && data.access_token && data.user) {
    setAuthToken(data.access_token)
    localStorage.setItem('auth_user', JSON.stringify(data.user))
  }

  return data
}

/**
 * Validate invitation code response
 */
export interface ValidateInvitationCodeResponse {
  valid: boolean
  error_code?: string
}

/**
 * Validate invitation code (public endpoint, no auth required)
 * @param code - Invitation code to validate
 * @returns Validation result
 */
export async function validateInvitationCode(code: string): Promise<ValidateInvitationCodeResponse> {
  const { data } = await apiClient.post<ValidateInvitationCodeResponse>('/auth/validate-invitation-code', { code })
  return data
}

/**
 * Forgot password request
 */
export interface ForgotPasswordRequest {
  email: string
  turnstile_token?: string
}

/**
 * Forgot password response
 */
export interface ForgotPasswordResponse {
  message: string
}

/**
 * Request password reset link
 * @param request - Email and optional Turnstile token
 * @returns Response with message
 */
export async function forgotPassword(request: ForgotPasswordRequest): Promise<ForgotPasswordResponse> {
  const { data } = await apiClient.post<ForgotPasswordResponse>('/auth/forgot-password', request)
  return data
}

/**
 * WeChat account binding for logged-in users
 * @param code - Verification code from WeChat public account
 * @returns Binding result with wechat_id
 */
export async function wechatBind(code: string): Promise<{ wechat_id: string; message: string }> {
  const { data } = await apiClient.get<{ wechat_id: string; message: string }>('/auth/oauth/wechat/bind', {
    params: { code }
  })
  return data
}

/**
 * WeChat account unbinding for logged-in users
 * @returns Unbind result with message
 */
export async function wechatUnbind(): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>('/auth/oauth/wechat/bind')
  return data
}

/**
 * Reset password request
 */
export interface ResetPasswordRequest {
  email: string
  token: string
  new_password: string
}

/**
 * Reset password response
 */
export interface ResetPasswordResponse {
  message: string
}

/**
 * Reset password with token
 * @param request - Email, token, and new password
 * @returns Response with message
 */
export async function resetPassword(request: ResetPasswordRequest): Promise<ResetPasswordResponse> {
  const { data } = await apiClient.post<ResetPasswordResponse>('/auth/reset-password', request)
  return data
}

/**
 * Email binding request for WeChat users
 */
export interface BindEmailRequest {
  email: string
  verify_code: string
}

/**
 * Email binding response
 */
export interface BindEmailResponse {
  email: string
  message: string
}

/**
 * Bind email for logged-in user (used by WeChat login users to bind a real email)
 * @param request - Email and verification code
 * @returns Binding result with email
 */
export async function bindEmail(request: BindEmailRequest): Promise<BindEmailResponse> {
  const { data } = await apiClient.post<BindEmailResponse>('/auth/bind-email', request)
  return data
}

/**
 * Send bind email verification code request
 */
export interface SendBindEmailCodeRequest {
  email: string
  turnstile_token?: string
}

/**
 * Send verification code for binding email (logged-in users only)
 * @param request - Email and optional Turnstile token
 * @returns Response with countdown seconds
 */
export async function sendBindEmailCode(
  request: SendBindEmailCodeRequest
): Promise<SendVerifyCodeResponse> {
  const { data } = await apiClient.post<SendVerifyCodeResponse>('/auth/send-bind-email-code', request)
  return data
}

export const authAPI = {
  login,
  login2FA,
  isTotp2FARequired,
  register,
  getCurrentUser,
  logout,
  isAuthenticated,
  setAuthToken,
  getAuthToken,
  clearAuthToken,
  getPublicSettings,
  sendVerifyCode,
  validatePromoCode,
  validateInvitationCode,
  wechatAuth,
  wechatScanInit,
  wechatScanPoll,
  wechatBind,
  wechatUnbind,
  forgotPassword,
  resetPassword,
  bindEmail,
  sendBindEmailCode
}

export default authAPI
