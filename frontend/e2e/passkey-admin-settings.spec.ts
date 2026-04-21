import { expect, test } from '@playwright/test'
import { passkeyTestEnv } from './helpers/test-app'

type MockSettings = {
  registration_enabled: boolean
  email_verify_enabled: boolean
  registration_email_suffix_whitelist: string[]
  promo_code_enabled: boolean
  password_reset_enabled: boolean
  frontend_url: string
  invitation_code_enabled: boolean
  totp_enabled: boolean
  totp_encryption_key_configured: boolean
  passkey_enabled: boolean
  passkey_rp_id: string
  passkey_rp_name: string
  passkey_allowed_origins: string[]
  passkey_config_valid: boolean
  passkey_config_error: string
  default_balance: number
  default_concurrency: number
  default_subscriptions: Array<{ group_id: number; validity_days: number }>
  site_name: string
  site_logo: string
  site_subtitle: string
  api_base_url: string
  contact_info: string
  doc_url: string
  home_content: string
  hide_ccs_import_button: boolean
  table_default_page_size: number
  table_page_size_options: number[]
  backend_mode_enabled: boolean
  custom_menu_items: Array<never>
  custom_endpoints: Array<never>
  smtp_host: string
  smtp_port: number
  smtp_username: string
  smtp_password_configured: boolean
  smtp_from_email: string
  smtp_from_name: string
  smtp_use_tls: boolean
  turnstile_enabled: boolean
  turnstile_site_key: string
  turnstile_secret_key_configured: boolean
  linuxdo_connect_enabled: boolean
  linuxdo_connect_client_id: string
  linuxdo_connect_client_secret_configured: boolean
  linuxdo_connect_redirect_url: string
  oidc_connect_enabled: boolean
  oidc_connect_provider_name: string
  oidc_connect_client_id: string
  oidc_connect_client_secret_configured: boolean
  oidc_connect_issuer_url: string
  oidc_connect_discovery_url: string
  oidc_connect_authorize_url: string
  oidc_connect_token_url: string
  oidc_connect_userinfo_url: string
  oidc_connect_jwks_url: string
  oidc_connect_scopes: string
  oidc_connect_redirect_url: string
  oidc_connect_frontend_redirect_url: string
  oidc_connect_token_auth_method: string
  oidc_connect_use_pkce: boolean
  oidc_connect_validate_id_token: boolean
  oidc_connect_allowed_signing_algs: string
  oidc_connect_clock_skew_seconds: number
  oidc_connect_require_email_verified: boolean
  oidc_connect_userinfo_email_path: string
  oidc_connect_userinfo_id_path: string
  oidc_connect_userinfo_username_path: string
  enable_model_fallback: boolean
  fallback_model_anthropic: string
  fallback_model_openai: string
  fallback_model_gemini: string
  fallback_model_antigravity: string
  enable_identity_patch: boolean
  identity_patch_prompt: string
  ops_monitoring_enabled: boolean
  ops_realtime_monitoring_enabled: boolean
  ops_query_mode_default: string
  ops_metrics_interval_seconds: number
  min_claude_code_version: string
  max_claude_code_version: string
  allow_ungrouped_key_scheduling: boolean
  enable_fingerprint_unification: boolean
  enable_metadata_passthrough: boolean
  enable_cch_signing: boolean
  payment_enabled: boolean
  payment_min_amount: number
  payment_max_amount: number
  payment_daily_limit: number
  payment_order_timeout_minutes: number
  payment_max_pending_orders: number
  payment_enabled_types: string[]
  payment_balance_disabled: boolean
  payment_load_balance_strategy: string
  payment_product_name_prefix: string
  payment_product_name_suffix: string
  payment_help_image_url: string
  payment_help_text: string
  payment_cancel_rate_limit_enabled: boolean
  payment_cancel_rate_limit_max: number
  payment_cancel_rate_limit_window: number
  payment_cancel_rate_limit_unit: string
  payment_cancel_rate_limit_window_mode: string
}

const adminUser = {
  id: 1,
  username: passkeyTestEnv.admin.username,
  email: passkeyTestEnv.admin.email,
  role: 'admin' as const,
  balance: 0,
  concurrency: 5,
  status: 'active' as const,
  allowed_groups: null,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z'
}

function derivePasskeyConfig(
  frontendURL: string,
  siteName: string
): Pick<MockSettings, 'passkey_rp_id' | 'passkey_rp_name' | 'passkey_allowed_origins' | 'passkey_config_valid' | 'passkey_config_error'> {
  const trimmed = frontendURL.trim()
  if (!trimmed) {
    return {
      passkey_rp_id: '',
      passkey_rp_name: siteName,
      passkey_allowed_origins: [],
      passkey_config_valid: false,
      passkey_config_error: 'Passkey is enabled but Frontend URL could not be used to derive a relying party ID. Set Frontend URL to your site origin.'
    }
  }

  try {
    const parsed = new URL(trimmed)
    if (!['http:', 'https:'].includes(parsed.protocol) || !parsed.hostname) {
      throw new Error('invalid frontend url')
    }

    const isLoopbackHost = parsed.hostname === 'localhost' || parsed.hostname === '127.0.0.1'
    if (parsed.protocol === 'http:' && !isLoopbackHost) {
      return {
        passkey_rp_id: parsed.hostname.toLowerCase(),
        passkey_rp_name: siteName,
        passkey_allowed_origins: [`${parsed.protocol}//${parsed.host.toLowerCase()}`],
        passkey_config_valid: false,
        passkey_config_error: `Passkey origin "${parsed.protocol}//${parsed.host.toLowerCase()}" is invalid. Check Frontend URL and use https://, or http://localhost/127.0.0.1 only for local development.`
      }
    }

    return {
      passkey_rp_id: parsed.hostname.toLowerCase(),
      passkey_rp_name: siteName,
      passkey_allowed_origins: [`${parsed.protocol}//${parsed.host.toLowerCase()}`],
      passkey_config_valid: true,
      passkey_config_error: ''
    }
  } catch {
    return {
      passkey_rp_id: '',
      passkey_rp_name: siteName,
      passkey_allowed_origins: [],
      passkey_config_valid: false,
      passkey_config_error: 'Passkey is enabled but Frontend URL could not be used to derive a relying party ID. Set Frontend URL to your site origin.'
    }
  }
}

function createSettings(frontendURL: string): MockSettings {
  const siteName = 'Sub2API Passkey E2E'

  return {
    registration_enabled: true,
    email_verify_enabled: false,
    registration_email_suffix_whitelist: [],
    promo_code_enabled: true,
    password_reset_enabled: false,
    frontend_url: frontendURL,
    invitation_code_enabled: false,
    totp_enabled: false,
    totp_encryption_key_configured: true,
    passkey_enabled: true,
    ...derivePasskeyConfig(frontendURL, siteName),
    default_balance: 0,
    default_concurrency: 5,
    default_subscriptions: [],
    site_name: siteName,
    site_logo: '',
    site_subtitle: 'Subscription to API Conversion Platform',
    api_base_url: '',
    contact_info: '',
    doc_url: '',
    home_content: '',
    hide_ccs_import_button: false,
    table_default_page_size: 20,
    table_page_size_options: [10, 20, 50, 100],
    backend_mode_enabled: false,
    custom_menu_items: [],
    custom_endpoints: [],
    smtp_host: '',
    smtp_port: 587,
    smtp_username: '',
    smtp_password_configured: false,
    smtp_from_email: '',
    smtp_from_name: '',
    smtp_use_tls: false,
    turnstile_enabled: false,
    turnstile_site_key: '',
    turnstile_secret_key_configured: false,
    linuxdo_connect_enabled: false,
    linuxdo_connect_client_id: '',
    linuxdo_connect_client_secret_configured: false,
    linuxdo_connect_redirect_url: '',
    oidc_connect_enabled: false,
    oidc_connect_provider_name: 'OIDC',
    oidc_connect_client_id: '',
    oidc_connect_client_secret_configured: false,
    oidc_connect_issuer_url: '',
    oidc_connect_discovery_url: '',
    oidc_connect_authorize_url: '',
    oidc_connect_token_url: '',
    oidc_connect_userinfo_url: '',
    oidc_connect_jwks_url: '',
    oidc_connect_scopes: 'openid email profile',
    oidc_connect_redirect_url: '',
    oidc_connect_frontend_redirect_url: '/auth/oidc/callback',
    oidc_connect_token_auth_method: 'client_secret_post',
    oidc_connect_use_pkce: false,
    oidc_connect_validate_id_token: true,
    oidc_connect_allowed_signing_algs: 'RS256,ES256,PS256',
    oidc_connect_clock_skew_seconds: 120,
    oidc_connect_require_email_verified: false,
    oidc_connect_userinfo_email_path: '',
    oidc_connect_userinfo_id_path: '',
    oidc_connect_userinfo_username_path: '',
    enable_model_fallback: false,
    fallback_model_anthropic: '',
    fallback_model_openai: '',
    fallback_model_gemini: '',
    fallback_model_antigravity: '',
    enable_identity_patch: false,
    identity_patch_prompt: '',
    ops_monitoring_enabled: false,
    ops_realtime_monitoring_enabled: true,
    ops_query_mode_default: 'auto',
    ops_metrics_interval_seconds: 60,
    min_claude_code_version: '',
    max_claude_code_version: '',
    allow_ungrouped_key_scheduling: false,
    enable_fingerprint_unification: true,
    enable_metadata_passthrough: false,
    enable_cch_signing: false,
    payment_enabled: false,
    payment_min_amount: 0,
    payment_max_amount: 0,
    payment_daily_limit: 0,
    payment_order_timeout_minutes: 0,
    payment_max_pending_orders: 0,
    payment_enabled_types: [],
    payment_balance_disabled: false,
    payment_load_balance_strategy: '',
    payment_product_name_prefix: '',
    payment_product_name_suffix: '',
    payment_help_image_url: '',
    payment_help_text: '',
    payment_cancel_rate_limit_enabled: false,
    payment_cancel_rate_limit_max: 0,
    payment_cancel_rate_limit_window: 0,
    payment_cancel_rate_limit_unit: '',
    payment_cancel_rate_limit_window_mode: ''
  }
}

test('shows a passkey config warning derived from frontend URL and captures a screenshot', async ({ page }, testInfo) => {
  let settingsState = createSettings(passkeyTestEnv.frontendBaseURL)

  await page.addInitScript((user) => {
    window.localStorage.setItem('auth_token', 'mock-admin-token')
    window.localStorage.setItem('refresh_token', 'mock-refresh-token')
    window.localStorage.setItem('token_expires_at', String(Date.now() + 60 * 60 * 1000))
    window.localStorage.setItem('auth_user', JSON.stringify(user))
    window.localStorage.setItem(`onboarding_tour_${user.id}_${user.role}_v4_interactive`, 'true')
  }, adminUser)

  await page.route('**/api/v1/**', async (route) => {
    const url = new URL(route.request().url())
    const path = url.pathname
    const method = route.request().method()

    const fulfill = async (data: unknown) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ code: 0, message: 'success', data })
      })
    }

    if (path === '/api/v1/auth/me' && method === 'GET') {
      await fulfill({ ...adminUser, run_mode: 'standard' })
      return
    }

    if (path === '/api/v1/settings/public' && method === 'GET') {
      await fulfill({
        registration_enabled: true,
        email_verify_enabled: false,
        registration_email_suffix_whitelist: [],
        promo_code_enabled: true,
        password_reset_enabled: false,
        invitation_code_enabled: false,
        passkey_enabled: true,
        turnstile_enabled: false,
        turnstile_site_key: '',
        site_name: 'Sub2API Passkey E2E',
        site_logo: '',
        site_subtitle: 'Subscription to API Conversion Platform',
        api_base_url: '',
        contact_info: '',
        doc_url: '',
        home_content: '',
        hide_ccs_import_button: false,
        payment_enabled: false,
        table_default_page_size: 20,
        table_page_size_options: [10, 20, 50, 100],
        custom_menu_items: [],
        custom_endpoints: [],
        linuxdo_oauth_enabled: false,
        oidc_oauth_enabled: false,
        oidc_oauth_provider_name: 'OIDC',
        backend_mode_enabled: false,
        version: 'e2e'
      })
      return
    }

    if (path === '/api/v1/admin/settings' && method === 'GET') {
      await fulfill(settingsState)
      return
    }

    if (path === '/api/v1/admin/settings' && method === 'PUT') {
      const payload = route.request().postDataJSON() as Partial<MockSettings>
      const frontendURL = typeof payload.frontend_url === 'string' ? payload.frontend_url : settingsState.frontend_url
      const passkeyEnabled = typeof payload.passkey_enabled === 'boolean' ? payload.passkey_enabled : settingsState.passkey_enabled
      const siteName = typeof payload.site_name === 'string' ? payload.site_name : settingsState.site_name
      settingsState = {
        ...settingsState,
        ...payload,
        frontend_url: frontendURL,
        site_name: siteName,
        passkey_enabled: passkeyEnabled,
        ...derivePasskeyConfig(frontendURL, siteName),
      }
      if (!passkeyEnabled) {
        settingsState.passkey_config_valid = true
        settingsState.passkey_config_error = ''
      }
      await fulfill(settingsState)
      return
    }

    if (path === '/api/v1/admin/groups/all' && method === 'GET') {
      await fulfill([])
      return
    }

    if (path === '/api/v1/admin/settings/admin-api-key' && method === 'GET') {
      await fulfill({ exists: false, masked_key: '' })
      return
    }

    if (path === '/api/v1/admin/settings/overload-cooldown' && method === 'GET') {
      await fulfill({ enabled: true, cooldown_minutes: 10 })
      return
    }

    if (path === '/api/v1/admin/settings/stream-timeout' && method === 'GET') {
      await fulfill({
        enabled: false,
        action: 'temp_unsched',
        temp_unsched_minutes: 5,
        threshold_count: 3,
        threshold_window_minutes: 10
      })
      return
    }

    if (path === '/api/v1/admin/settings/rectifier' && method === 'GET') {
      await fulfill({
        enabled: true,
        thinking_signature_enabled: true,
        thinking_budget_enabled: true,
        apikey_signature_enabled: false,
        apikey_signature_patterns: []
      })
      return
    }

    if (path === '/api/v1/admin/settings/beta-policy' && method === 'GET') {
      await fulfill({ rules: [] })
      return
    }

    if (path === '/api/v1/admin/payment/providers' && method === 'GET') {
      await fulfill([])
      return
    }

    if (path === '/api/v1/admin/payment/config' && method === 'GET') {
      await fulfill({ enabled: false })
      return
    }

    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ code: 0, message: 'success', data: {} })
    })
  })

  await page.goto('/admin/settings')
  await page.waitForLoadState('networkidle')
  await page.getByTestId('settings-tab-security').click()

  const frontendUrlInput = page.getByTestId('frontend-url-input')
  const saveButton = page.getByTestId('settings-save-button')

  await expect(frontendUrlInput).toBeVisible()
  await frontendUrlInput.fill('not-a-valid-url')
  await saveButton.evaluate((button: HTMLButtonElement) => button.click())

  const warning = page.getByTestId('passkey-config-warning')
  await expect(warning).toBeVisible()
  await expect(warning).toContainText('Frontend URL')
  await warning.scrollIntoViewIfNeeded()

  await page.evaluate(() => {
    document.querySelectorAll('.driver-overlay, .driver-popover, [aria-live="polite"]').forEach((element) => {
      element.remove()
    })
  })

  const screenshotPath = testInfo.outputPath('passkey-config-warning-full-page.png')
  const viewport = page.viewportSize() ?? { width: 1280, height: 720 }
  const documentHeight = await page.evaluate(() => {
    return Math.max(
      document.documentElement.scrollHeight,
      document.body.scrollHeight,
      document.documentElement.offsetHeight,
      document.body.offsetHeight
    )
  })
  await page.setViewportSize({
    width: viewport.width,
    height: Math.min(Math.max(documentHeight, viewport.height), 6000)
  })
  await page.evaluate(() => window.scrollTo(0, 0))
  await page.screenshot({ path: screenshotPath })
  await testInfo.attach('passkey-config-warning', {
    path: screenshotPath,
    contentType: 'image/png'
  })
})
