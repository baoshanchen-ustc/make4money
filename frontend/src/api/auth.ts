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
  TotpLogin2FARequest,
  OAuthProvider,
  OAuthTokenPairResponse,
  OAuthCallbackResult,
  PendingAuthSessionCallbackPayload,
  PendingAuthSessionSummary,
  OAuthBindingStartIntent
} from '@/types'

/**
 * Login response type - can be either full auth or 2FA required
 */
export type LoginResponse = AuthResponse | TotpLoginResponse

export interface OAuthBindLoginRequest {
  pendingAuthToken: string
  email: string
  password: string
  turnstileToken?: string
  adoptDisplayName?: boolean
  adoptAvatar?: boolean
}

export interface OAuthCreateAccountRequest {
  pendingAuthToken: string
  email?: string
  password?: string
  verifyCode?: string
  invitationCode?: string
  adoptDisplayName?: boolean
  adoptAvatar?: boolean
}

export type OAuthBindLoginResponse = OAuthTokenPairResponse | TotpLoginResponse

export interface PendingAuthAdoptionDecision {
  adoptDisplayName: boolean
  adoptAvatar: boolean
}

type PendingAuthSessionWithDecision = PendingAuthSessionSummary & {
  adoption_decision?: {
    adopt_display_name?: boolean
    adopt_avatar?: boolean
  } | null
}

/**
 * Type guard to check if login response requires 2FA
 */
export function isTotp2FARequired(
  response: LoginResponse | OAuthBindLoginResponse
): response is TotpLoginResponse {
  return 'requires_2fa' in response && response.requires_2fa === true
}

export function isPendingAuthSessionCallbackPayload(
  payload: OAuthCallbackResult | Record<string, unknown> | null | undefined
): payload is PendingAuthSessionCallbackPayload {
  const candidate = payload as { auth_result?: unknown; pending_auth_token?: unknown } | null | undefined
  return candidate?.auth_result === 'pending_session' && typeof candidate?.pending_auth_token === 'string'
}

export function normalizePendingAuthSessionSummary(
  payload: PendingAuthSessionCallbackPayload
): PendingAuthSessionSummary {
  return {
    token: payload.pending_auth_token,
    provider: payload.provider,
    intent: payload.intent,
    auth_result: payload.auth_result,
    redirect: payload.redirect,
    adoption_required: payload.adoption_required,
    suggested_display_name: payload.suggested_display_name,
    suggested_avatar_url: payload.suggested_avatar_url
  }
}

export function getPendingAuthSessionAdoptionDecision(
  session: PendingAuthSessionSummary | null | undefined
): PendingAuthAdoptionDecision | null {
  const rawDecision = (session as PendingAuthSessionWithDecision | null | undefined)?.adoption_decision
  if (!rawDecision || typeof rawDecision !== 'object') {
    return null
  }

  const adoptDisplayName = rawDecision.adopt_display_name
  const adoptAvatar = rawDecision.adopt_avatar

  if (typeof adoptDisplayName !== 'boolean' && typeof adoptAvatar !== 'boolean') {
    return null
  }

  return {
    adoptDisplayName: adoptDisplayName === true,
    adoptAvatar: adoptAvatar === true
  }
}

export function withPendingAuthSessionAdoptionDecision(
  session: PendingAuthSessionSummary,
  decision: PendingAuthAdoptionDecision
): PendingAuthSessionSummary {
  return {
    ...session,
    adoption_decision: {
      adopt_display_name: decision.adoptDisplayName,
      adopt_avatar: decision.adoptAvatar
    }
  } as PendingAuthSessionSummary
}

export function inheritPendingAuthSessionAdoptionDecision(
  session: PendingAuthSessionSummary,
  source: PendingAuthSessionSummary | null | undefined
): PendingAuthSessionSummary {
  const decision = getPendingAuthSessionAdoptionDecision(source)
  if (!decision) {
    return session
  }

  return withPendingAuthSessionAdoptionDecision(session, decision)
}

export function sanitizeAuthRedirectPath(
  path: string | null | undefined,
  fallback = '/dashboard'
): string {
  if (!path) return fallback
  if (!path.startsWith('/')) return fallback
  if (path.startsWith('//')) return fallback
  if (path.includes('://')) return fallback
  if (path.includes('\n') || path.includes('\r')) return fallback
  return path
}

function persistTokenPair(response: OAuthTokenPairResponse): void {
  setAuthToken(response.access_token)
  if (response.refresh_token) {
    setRefreshToken(response.refresh_token)
  }
  if (response.expires_in) {
    setTokenExpiresAt(response.expires_in)
  }
}

function getOAuthProviderBasePath(provider: OAuthProvider): string {
  return `/auth/oauth/${provider}`
}

function withPendingAuthToken(token: string): Record<'pending_auth_token' | 'pending_oauth_token', string> {
  return {
    pending_auth_token: token,
    pending_oauth_token: token
  }
}

function withPendingAuthPayload(
  token: string,
  decision?: PendingAuthAdoptionDecision | null
): Record<string, unknown> {
  const payload: Record<string, unknown> = {
    ...withPendingAuthToken(token)
  }

  if (decision) {
    payload.adopt_display_name = decision.adoptDisplayName
    payload.adopt_avatar = decision.adoptAvatar
  }

  return payload
}

/**
 * Store authentication token in localStorage
 */
export function setAuthToken(token: string): void {
  localStorage.setItem('auth_token', token)
}

/**
 * Store refresh token in localStorage
 */
export function setRefreshToken(token: string): void {
  localStorage.setItem('refresh_token', token)
}

/**
 * Store token expiration timestamp in localStorage
 * Converts expires_in (seconds) to absolute timestamp (milliseconds)
 */
export function setTokenExpiresAt(expiresIn: number): void {
  const expiresAt = Date.now() + expiresIn * 1000
  localStorage.setItem('token_expires_at', String(expiresAt))
}

/**
 * Get authentication token from localStorage
 */
export function getAuthToken(): string | null {
  return localStorage.getItem('auth_token')
}

/**
 * Get refresh token from localStorage
 */
export function getRefreshToken(): string | null {
  return localStorage.getItem('refresh_token')
}

/**
 * Get token expiration timestamp from localStorage
 */
export function getTokenExpiresAt(): number | null {
  const value = localStorage.getItem('token_expires_at')
  return value ? parseInt(value, 10) : null
}

/**
 * Clear authentication token from localStorage
 */
export function clearAuthToken(): void {
  localStorage.removeItem('auth_token')
  localStorage.removeItem('refresh_token')
  localStorage.removeItem('auth_user')
  localStorage.removeItem('token_expires_at')
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
    persistTokenPair(data)
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
  persistTokenPair(data)
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
  persistTokenPair(data)
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
 * Optionally revokes the refresh token on the server
 */
export async function logout(): Promise<void> {
  const refreshToken = getRefreshToken()

  // Try to revoke the refresh token on the server
  if (refreshToken) {
    try {
      await apiClient.post('/auth/logout', { refresh_token: refreshToken })
    } catch {
      // Ignore errors - we still want to clear local state
    }
  }

  clearAuthToken()
}

/**
 * Refresh token response
 */
export interface RefreshTokenResponse {
  access_token: string
  refresh_token: string
  expires_in: number
  token_type: string
}

/**
 * Refresh the access token using the refresh token
 * @returns New token pair
 */
export async function refreshToken(): Promise<RefreshTokenResponse> {
  const currentRefreshToken = getRefreshToken()
  if (!currentRefreshToken) {
    throw new Error('No refresh token available')
  }

  const { data } = await apiClient.post<RefreshTokenResponse>('/auth/refresh', {
    refresh_token: currentRefreshToken
  })

  // Update tokens in localStorage
  setAuthToken(data.access_token)
  setRefreshToken(data.refresh_token)
  setTokenExpiresAt(data.expires_in)

  return data
}

/**
 * Revoke all sessions for the current user
 * @returns Response with message
 */
export async function revokeAllSessions(): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>('/auth/revoke-all-sessions')
  return data
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
 * Complete LinuxDo OAuth registration by supplying an invitation code
 * @param pendingOAuthToken - Short-lived JWT from the OAuth callback
 * @param invitationCode - Invitation code entered by the user
 * @returns Token pair on success
 */
export async function completeLinuxDoOAuthRegistration(
  pendingAuthToken: string,
  invitationCode: string
): Promise<OAuthTokenPairResponse> {
  return completeOAuthRegistration('linuxdo', pendingAuthToken, invitationCode)
}

/**
 * Complete OIDC OAuth registration by supplying an invitation code
 * @param pendingOAuthToken - Short-lived JWT from the OAuth callback
 * @param invitationCode - Invitation code entered by the user
 * @returns Token pair on success
 */
export async function completeOIDCOAuthRegistration(
  pendingAuthToken: string,
  invitationCode: string
): Promise<OAuthTokenPairResponse> {
  return completeOAuthRegistration('oidc', pendingAuthToken, invitationCode)
}

export async function completeOAuthRegistration(
  provider: OAuthProvider,
  pendingAuthToken: string,
  invitationCode: string
): Promise<OAuthTokenPairResponse> {
  const { data } = await apiClient.post<OAuthTokenPairResponse>(
    `${getOAuthProviderBasePath(provider)}/complete-registration`,
    {
      ...withPendingAuthToken(pendingAuthToken),
      invitation_code: invitationCode
    }
  )
  return data
}

export async function bindOAuthLogin(
  provider: OAuthProvider,
  request: OAuthBindLoginRequest
): Promise<OAuthBindLoginResponse> {
  const decision =
    typeof request.adoptDisplayName === 'boolean' || typeof request.adoptAvatar === 'boolean'
      ? {
          adoptDisplayName: request.adoptDisplayName === true,
          adoptAvatar: request.adoptAvatar === true
        }
      : null

  const { data } = await apiClient.post<OAuthBindLoginResponse>(
    `${getOAuthProviderBasePath(provider)}/bind-login`,
    {
      ...withPendingAuthPayload(request.pendingAuthToken, decision),
      email: request.email,
      password: request.password,
      turnstile_token: request.turnstileToken
    }
  )
  return data
}

export async function createOAuthAccount(
  provider: OAuthProvider,
  request: OAuthCreateAccountRequest
): Promise<OAuthTokenPairResponse> {
  if (
    (provider === 'linuxdo' || provider === 'oidc')
    && request.invitationCode
    && !request.email
    && !request.password
    && !request.verifyCode
  ) {
    return completeOAuthRegistration(provider, request.pendingAuthToken, request.invitationCode)
  }

  const decision =
    typeof request.adoptDisplayName === 'boolean' || typeof request.adoptAvatar === 'boolean'
      ? {
          adoptDisplayName: request.adoptDisplayName === true,
          adoptAvatar: request.adoptAvatar === true
        }
      : null

  const { data } = await apiClient.post<OAuthTokenPairResponse>(
    `${getOAuthProviderBasePath(provider)}/create-account`,
    {
      ...withPendingAuthPayload(request.pendingAuthToken, decision),
      email: request.email,
      password: request.password,
      verify_code: request.verifyCode,
      invitation_code: request.invitationCode
    }
  )
  return data
}

export function parseOAuthCallbackResult(params: URLSearchParams): OAuthCallbackResult | null {
  if (params.get('auth_result') === 'pending_session') {
    const pendingAuthToken = params.get('pending_auth_token')
    const provider = params.get('provider')
    const intent = params.get('intent')

    if (!pendingAuthToken || !provider || !intent) {
      return null
    }

    return {
      auth_result: 'pending_session',
      pending_auth_token: pendingAuthToken,
      provider: provider as OAuthProvider,
      intent: intent as PendingAuthSessionSummary['intent'],
      redirect: params.get('redirect') || undefined,
      adoption_required: params.get('adoption_required') === 'true',
      suggested_display_name: params.get('suggested_display_name'),
      suggested_avatar_url: params.get('suggested_avatar_url')
    }
  }

  const accessToken = params.get('access_token')
  if (!accessToken) {
    return null
  }

  const expiresIn = params.get('expires_in')
  return {
    access_token: accessToken,
    refresh_token: params.get('refresh_token') || undefined,
    expires_in: expiresIn ? parseInt(expiresIn, 10) : undefined,
    token_type: params.get('token_type') || 'Bearer'
  }
}

export function persistOAuthTokenPair(response: OAuthTokenPairResponse): void {
  persistTokenPair(response)
}

export function clearPendingAuthSessionStorage(): void {
  sessionStorage.removeItem('pending_auth_session')
}

export function persistPendingAuthSession(session: PendingAuthSessionSummary): void {
  sessionStorage.setItem('pending_auth_session', JSON.stringify(session))
}

export function getPersistedPendingAuthSession(): PendingAuthSessionSummary | null {
  const raw = sessionStorage.getItem('pending_auth_session')
  if (!raw) {
    return null
  }

  try {
    return JSON.parse(raw) as PendingAuthSessionSummary
  } catch {
    sessionStorage.removeItem('pending_auth_session')
    return null
  }
}

export function consumePendingAuthSession(): PendingAuthSessionSummary | null {
  const session = getPersistedPendingAuthSession()
  clearPendingAuthSessionStorage()
  return session
}

type OAuthStartUrlOptions = {
  wechatOpenEnabled?: boolean
  wechatOpenConfigured?: boolean
  wechatMpEnabled?: boolean
  wechatMpConfigured?: boolean
  wechatMode?: 'open' | 'mp' | null
  userAgent?: string
}

function isWechatBrowser(userAgent?: string): boolean {
  const resolvedUserAgent =
    userAgent ?? (typeof navigator === 'undefined' ? '' : navigator.userAgent.toLowerCase())
  return /micromessenger/.test(resolvedUserAgent)
}

function resolveWechatOAuthMode(options: OAuthStartUrlOptions | undefined): 'open' | 'mp' | null {
  if (options?.wechatMode) {
    return options.wechatMode
  }

  const openEnabled = options?.wechatOpenEnabled === true
  const openConfigured = options?.wechatOpenConfigured !== false
  const mpEnabled = options?.wechatMpEnabled === true
  const mpConfigured = options?.wechatMpConfigured !== false
  const openAvailable = openEnabled && openConfigured
  const mpAvailable = mpEnabled && mpConfigured
  const inWechatBrowser = isWechatBrowser(options?.userAgent)

  if (inWechatBrowser) {
    if (mpAvailable) {
      return 'mp'
    }
    if (openAvailable) {
      return 'open'
    }
    return null
  }

  if (openAvailable) {
    return 'open'
  }
  if (mpAvailable) {
    return 'mp'
  }
  return null
}

function isWechatOAuthAvailable(options: OAuthStartUrlOptions | undefined): boolean {
  const mode = resolveWechatOAuthMode(options)
  if (!mode) {
    return false
  }

  if (options?.wechatMode) {
    return true
  }

  const openEnabled = options?.wechatOpenEnabled === true && options?.wechatOpenConfigured !== false
  const mpEnabled = options?.wechatMpEnabled === true && options?.wechatMpConfigured !== false
  const inWechatBrowser = isWechatBrowser(options?.userAgent)

  if (!inWechatBrowser && !openEnabled && mpEnabled) {
    return false
  }

  if (inWechatBrowser && !mpEnabled && openEnabled) {
    return false
  }

  return true
}

export function getWechatOAuthAvailabilityHintKey(options: OAuthStartUrlOptions | undefined): string | null {
  const openEnabled = options?.wechatOpenEnabled === true && options?.wechatOpenConfigured !== false
  const mpEnabled = options?.wechatMpEnabled === true && options?.wechatMpConfigured !== false
  const inWechatBrowser = isWechatBrowser(options?.userAgent)

  if (!openEnabled && !mpEnabled) {
    return 'auth.wechat.disabledUnavailable'
  }

  if (!inWechatBrowser && !openEnabled && mpEnabled) {
    return 'auth.wechat.disabledNeedWechatEnv'
  }

  if (inWechatBrowser && !mpEnabled && openEnabled) {
    return 'auth.wechat.disabledNeedExternalBrowser'
  }

  return null
}

export function getOAuthStartUrl(
  provider: OAuthProvider,
  redirectTo: string,
  intent: OAuthBindingStartIntent | 'login' = 'login',
  options?: OAuthStartUrlOptions
): string | null {
  const apiBase = (import.meta.env.VITE_API_BASE_URL as string | undefined) || '/api/v1'
  const normalized = apiBase.replace(/\/$/, '')
  const searchParams = new URLSearchParams({
    redirect: sanitizeAuthRedirectPath(redirectTo),
    intent
  })

  if (provider === 'wechat') {
    if (!isWechatOAuthAvailable(options)) {
      return null
    }

    const mode = resolveWechatOAuthMode(options)
    if (!mode) {
      return null
    }
    searchParams.set('mode', mode)
  }

  return `${normalized}/auth/oauth/${provider}/start?${searchParams.toString()}`
}

export function prepareOAuthBindAccessTokenCookie(): void {
  if (typeof document === 'undefined' || typeof window === 'undefined') {
    return
  }

  const token = getAuthToken()
  if (!token) {
    return
  }

  const secure = window.location.protocol === 'https:' ? '; Secure' : ''
  const apiBase = (import.meta.env.VITE_API_BASE_URL as string | undefined) || '/api/v1'
  const normalized = apiBase.replace(/\/$/, '')
  document.cookie =
    `oauth_bind_access_token=${encodeURIComponent(token)}; Path=${normalized}/auth/oauth; Max-Age=600; SameSite=Lax${secure}`
}

export async function completePendingAuthSession(
  provider: OAuthProvider,
  pendingAuthToken: string,
  payload: Record<string, unknown>
): Promise<OAuthTokenPairResponse> {
  const { data } = await apiClient.post<OAuthTokenPairResponse>(
    `${getOAuthProviderBasePath(provider)}/complete`,
    {
      ...withPendingAuthToken(pendingAuthToken),
      ...payload
    }
  )
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
  setRefreshToken,
  setTokenExpiresAt,
  getAuthToken,
  getRefreshToken,
  getTokenExpiresAt,
  clearAuthToken,
  getPublicSettings,
  sendVerifyCode,
  validatePromoCode,
  validateInvitationCode,
  forgotPassword,
  resetPassword,
  refreshToken,
  revokeAllSessions,
  isPendingAuthSessionCallbackPayload,
  normalizePendingAuthSessionSummary,
  getPendingAuthSessionAdoptionDecision,
  withPendingAuthSessionAdoptionDecision,
  inheritPendingAuthSessionAdoptionDecision,
  parseOAuthCallbackResult,
  persistOAuthTokenPair,
  persistPendingAuthSession,
  getPersistedPendingAuthSession,
  consumePendingAuthSession,
  clearPendingAuthSessionStorage,
  getOAuthStartUrl,
  bindOAuthLogin,
  createOAuthAccount,
  completePendingAuthSession,
  completeOAuthRegistration,
  completeLinuxDoOAuthRegistration,
  completeOIDCOAuthRegistration
}

export default authAPI
