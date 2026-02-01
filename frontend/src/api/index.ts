/**
 * API Client for Sub2API Backend
 * Central export point for all API modules
 */

// Re-export the HTTP client
export { apiClient } from './client'

// Auth API
export { authAPI, isTotp2FARequired, type LoginResponse } from './auth'

// User APIs
export { keysAPI } from './keys'
export { usageAPI } from './usage'
export { userAPI } from './user'
export { redeemAPI, type RedeemHistoryItem } from './redeem'
export { userGroupsAPI } from './groups'
export { totpAPI } from './totp'
export { usageReportAPI } from './usageReport'
export { rechargeAPI, isRateLimitError, isCaptchaRequiredError, RateLimitExceededError, CaptchaRequiredError, type RechargeConfig, type OrderListItem } from './recharge'

// Admin APIs
export { adminAPI } from './admin'

// Default export
export { default } from './client'
