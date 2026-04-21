import type { APIRequestContext, APIResponse } from '@playwright/test'
import { passkeyTestEnv } from './test-app'

interface ApiEnvelope<T> {
  code: number
  message: string
  data: T
}

interface AuthResponse {
  access_token: string
  refresh_token?: string
  user: {
    id: number
    email: string
    username: string
  }
}

interface PasskeyListResponse {
  items: Array<{
    credential_id: string
  }>
}

class SeedSessionExpiredError extends Error {}

let cachedAdminSession: AuthResponse | null = null
let cachedUserSession: AuthResponse | null = null

async function unwrapApiResponse<T>(response: APIResponse, label: string): Promise<T> {
  const rawBody = await response.text()

  let payload: ApiEnvelope<T>
  try {
    payload = JSON.parse(rawBody) as ApiEnvelope<T>
  } catch {
    throw new Error(`${label} returned non-JSON response (${response.status()}): ${rawBody}`)
  }

  if (!response.ok() || payload.code !== 0) {
    throw new Error(`${label} failed (${response.status()}): ${payload.message}`)
  }

  return payload.data
}

async function unwrapAuthedApiResponse<T>(response: APIResponse, label: string): Promise<T> {
  if (response.status() === 401) {
    throw new SeedSessionExpiredError(`${label} session expired`)
  }

  return unwrapApiResponse<T>(response, label)
}

async function tryPasswordLogin(
  request: APIRequestContext,
  email: string,
  password: string
): Promise<AuthResponse | null> {
  const response = await request.post(`${passkeyTestEnv.backendBaseURL}/api/v1/auth/login`, {
    data: { email, password }
  })

  if (!response.ok()) {
    return null
  }

  return unwrapApiResponse<AuthResponse>(response, `login ${email}`)
}

async function requirePasswordLogin(
  request: APIRequestContext,
  email: string,
  password: string
): Promise<AuthResponse> {
  const response = await request.post(`${passkeyTestEnv.backendBaseURL}/api/v1/auth/login`, {
    data: { email, password }
  })

  return unwrapApiResponse<AuthResponse>(response, `login ${email}`)
}

async function ensurePasskeySettings(request: APIRequestContext, accessToken: string): Promise<void> {
  const response = await request.put(`${passkeyTestEnv.backendBaseURL}/api/v1/admin/settings`, {
    headers: {
      Authorization: `Bearer ${accessToken}`
    },
    data: {
      registration_enabled: true,
      email_verify_enabled: false,
      registration_email_suffix_whitelist: [],
      promo_code_enabled: false,
      password_reset_enabled: false,
      frontend_url: passkeyTestEnv.frontendBaseURL,
      invitation_code_enabled: false,
      totp_enabled: false,
      passkey_enabled: true,
      smtp_host: '',
      smtp_port: 587,
      smtp_username: '',
      smtp_from_email: '',
      smtp_from_name: '',
      smtp_use_tls: false,
      turnstile_enabled: false,
      turnstile_site_key: '',
      linuxdo_connect_enabled: false,
      linuxdo_connect_client_id: '',
      linuxdo_connect_redirect_url: '',
      site_name: 'Sub2API Passkey E2E',
      site_logo: '',
      site_subtitle: '',
      api_base_url: '',
      contact_info: '',
      doc_url: '',
      home_content: '',
      hide_ccs_import_button: false,
      purchase_subscription_enabled: false,
      purchase_subscription_url: '',
      sora_client_enabled: false,
      custom_menu_items: [],
      custom_endpoints: [],
      default_concurrency: 5,
      default_balance: 0,
      default_subscriptions: [],
      enable_model_fallback: false,
      fallback_model_anthropic: '',
      fallback_model_openai: '',
      fallback_model_gemini: '',
      fallback_model_antigravity: '',
      enable_identity_patch: false,
      identity_patch_prompt: '',
      min_claude_code_version: '',
      max_claude_code_version: '',
      allow_ungrouped_key_scheduling: false,
      backend_mode_enabled: false,
      enable_fingerprint_unification: false,
      enable_metadata_passthrough: false
    }
  })

  await unwrapAuthedApiResponse<Record<string, unknown>>(response, 'update passkey test settings')
}

async function ensureSeedUser(request: APIRequestContext): Promise<AuthResponse> {
  if (cachedUserSession) {
    return cachedUserSession
  }

  const existingSession = await tryPasswordLogin(
    request,
    passkeyTestEnv.user.email,
    passkeyTestEnv.user.password
  )

  if (existingSession) {
    cachedUserSession = existingSession
    return existingSession
  }

  const registerResponse = await request.post(`${passkeyTestEnv.backendBaseURL}/api/v1/auth/register`, {
    data: {
      email: passkeyTestEnv.user.email,
      password: passkeyTestEnv.user.password,
      username: passkeyTestEnv.user.username
    }
  })

  await unwrapApiResponse<AuthResponse>(registerResponse, `register ${passkeyTestEnv.user.email}`)

  cachedUserSession = await requirePasswordLogin(
    request,
    passkeyTestEnv.user.email,
    passkeyTestEnv.user.password
  )

  return cachedUserSession
}

async function clearSeedUserPasskeys(request: APIRequestContext, accessToken: string): Promise<void> {
  const listResponse = await request.get(`${passkeyTestEnv.backendBaseURL}/api/v1/user/passkeys`, {
    headers: {
      Authorization: `Bearer ${accessToken}`
    }
  })

  const list = await unwrapAuthedApiResponse<PasskeyListResponse>(listResponse, 'list passkeys')

  for (const item of list.items) {
    const revokeResponse = await request.delete(
      `${passkeyTestEnv.backendBaseURL}/api/v1/user/passkeys/${encodeURIComponent(item.credential_id)}`,
      {
        headers: {
          Authorization: `Bearer ${accessToken}`
        }
      }
    )

    await unwrapAuthedApiResponse<Record<string, unknown>>(
      revokeResponse,
      `revoke passkey ${item.credential_id}`
    )
  }
}

async function ensureAdminSession(request: APIRequestContext): Promise<AuthResponse> {
  if (cachedAdminSession) {
    return cachedAdminSession
  }

  cachedAdminSession = await requirePasswordLogin(
    request,
    passkeyTestEnv.admin.email,
    passkeyTestEnv.admin.password
  )

  return cachedAdminSession
}

export async function seedPasskeyTestUser(request: APIRequestContext): Promise<void> {
  let adminSession = await ensureAdminSession(request)

  try {
    await ensurePasskeySettings(request, adminSession.access_token)
  } catch (error) {
    if (!(error instanceof SeedSessionExpiredError)) {
      throw error
    }

    cachedAdminSession = null
    adminSession = await ensureAdminSession(request)
    await ensurePasskeySettings(request, adminSession.access_token)
  }

  let userSession = await ensureSeedUser(request)

  try {
    await clearSeedUserPasskeys(request, userSession.access_token)
  } catch (error) {
    if (!(error instanceof SeedSessionExpiredError)) {
      throw error
    }

    cachedUserSession = null
    userSession = await ensureSeedUser(request)
    await clearSeedUserPasskeys(request, userSession.access_token)
  }
}
