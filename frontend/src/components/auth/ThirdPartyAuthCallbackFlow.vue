<template>
  <AuthLayout>
    <div class="space-y-6">
      <div class="text-center">
        <h2 class="text-2xl font-bold text-gray-900 dark:text-white">
          {{ callbackTitle }}
        </h2>
        <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">
          {{ descriptionText }}
        </p>
      </div>

      <div
        v-if="mode === 'choose'"
        class="rounded-xl border border-primary-200 bg-primary-50 p-4 dark:border-primary-800/50 dark:bg-primary-900/20"
      >
        <p class="text-sm text-primary-700 dark:text-primary-300">
          {{ t('auth.thirdParty.unboundAccount', { providerName: resolvedProviderName }) }}
        </p>
      </div>

      <transition name="fade">
        <div v-if="mode === 'choose'" class="grid gap-3" :class="bindingIntent ? 'sm:grid-cols-1' : 'sm:grid-cols-2'">
          <button type="button" class="btn btn-primary w-full" @click="mode = 'bind'">
            {{ t('auth.thirdParty.bindExistingAccount') }}
          </button>
          <button v-if="!bindingIntent" type="button" class="btn btn-secondary w-full" @click="mode = 'create'">
            {{ t('auth.thirdParty.createNewAccount') }}
          </button>
        </div>
      </transition>

      <transition name="fade">
        <form v-if="mode === 'bind'" class="space-y-5" @submit.prevent="handleBindExistingAccount">
          <div>
            <label for="bind-email" class="input-label">
              {{ t('auth.emailLabel') }}
            </label>
            <input
              id="bind-email"
              v-model="bindForm.email"
              type="email"
              autocomplete="email"
              :disabled="isBinding"
              class="input w-full"
              :class="{ 'input-error': bindErrors.email }"
              :placeholder="t('auth.emailPlaceholder')"
            />
            <p v-if="bindErrors.email" class="input-error-text">
              {{ bindErrors.email }}
            </p>
          </div>

          <div>
            <label for="bind-password" class="input-label">
              {{ t('auth.passwordLabel') }}
            </label>
            <div class="relative">
              <input
                id="bind-password"
                v-model="bindForm.password"
                :type="showPassword ? 'text' : 'password'"
                autocomplete="current-password"
                :disabled="isBinding"
                class="input w-full pr-11"
                :class="{ 'input-error': bindErrors.password }"
                :placeholder="t('auth.passwordPlaceholder')"
              />
              <button
                type="button"
                class="absolute inset-y-0 right-0 flex items-center pr-3.5 text-gray-400 transition-colors hover:text-gray-600 dark:hover:text-dark-300"
                @click="showPassword = !showPassword"
              >
                <Icon v-if="showPassword" name="eyeOff" size="md" />
                <Icon v-else name="eye" size="md" />
              </button>
            </div>
            <p v-if="bindErrors.password" class="input-error-text">
              {{ bindErrors.password }}
            </p>
          </div>

          <div v-if="turnstileEnabled && turnstileSiteKey">
            <TurnstileWidget
              ref="turnstileRef"
              :site-key="turnstileSiteKey"
              @verify="onTurnstileVerify"
              @expire="onTurnstileExpire"
              @error="onTurnstileError"
            />
            <p v-if="bindErrors.turnstile" class="input-error-text mt-2 text-center">
              {{ bindErrors.turnstile }}
            </p>
          </div>

          <p v-if="actionError" class="input-error-text">
            {{ actionError }}
          </p>

          <div class="grid gap-3 sm:grid-cols-2">
            <button type="submit" class="btn btn-primary w-full" :disabled="isBinding">
              {{ isBinding ? t('auth.thirdParty.binding') : t('auth.thirdParty.bindSubmit') }}
            </button>
            <button type="button" class="btn btn-secondary w-full" :disabled="isBinding" @click="goBackToChoices">
              {{ t('auth.thirdParty.chooseAnotherAction') }}
            </button>
          </div>
        </form>
      </transition>

      <transition name="fade">
        <form v-if="mode === 'create'" class="space-y-5" @submit.prevent="handleCreateAccount">
          <template v-if="supportsVerifiedEmailCreate">
            <div>
              <label for="create-email" class="input-label">
                {{ t('auth.emailLabel') }}
              </label>
              <input
                id="create-email"
                v-model="createForm.email"
                type="email"
                autocomplete="email"
                :disabled="isCreating || isSendingCreateCode"
                class="input w-full"
                :class="{ 'input-error': createErrors.email }"
                :placeholder="t('auth.emailPlaceholder')"
              />
              <p v-if="createErrors.email" class="input-error-text">
                {{ createErrors.email }}
              </p>
            </div>

            <div>
              <label for="create-verify-code" class="input-label">
                {{ t('auth.verificationCode') }}
              </label>
              <div class="flex gap-3">
                <input
                  id="create-verify-code"
                  v-model="createForm.verifyCode"
                  type="text"
                  inputmode="numeric"
                  autocomplete="one-time-code"
                  maxlength="6"
                  :disabled="isCreating"
                  class="input min-w-0 flex-1"
                  :class="{ 'input-error': createErrors.verifyCode }"
                  placeholder="000000"
                />
                <button
                  type="button"
                  class="btn btn-secondary shrink-0"
                  :disabled="isCreating || isSendingCreateCode || createCodeCountdown > 0"
                  @click="handleSendCreateVerifyCode"
                >
                  {{
                    isSendingCreateCode
                      ? t('auth.sendingCode')
                      : createCodeCountdown > 0
                        ? t('auth.resendCountdown', { countdown: createCodeCountdown })
                        : t('auth.resendCode')
                  }}
                </button>
              </div>
              <p v-if="createErrors.verifyCode" class="input-error-text">
                {{ createErrors.verifyCode }}
              </p>
              <p v-else class="input-hint">
                {{ t('auth.verificationCodeHint') }}
              </p>
              <p v-if="createCodeSent" class="mt-2 text-sm text-green-600 dark:text-green-400">
                {{ t('auth.codeSentSuccess') }}
              </p>
            </div>

            <div v-if="turnstileEnabled && turnstileSiteKey && showCreateTurnstile">
              <TurnstileWidget
                ref="turnstileRef"
                :site-key="turnstileSiteKey"
                @verify="onTurnstileVerify"
                @expire="onTurnstileExpire"
                @error="onTurnstileError"
              />
              <p v-if="createErrors.turnstile" class="input-error-text mt-2 text-center">
                {{ createErrors.turnstile }}
              </p>
            </div>
          </template>

          <div
            v-if="requiresInvitation || invitationCodeEnabled"
            class="rounded-xl border border-amber-200 bg-amber-50 p-4 dark:border-amber-800/50 dark:bg-amber-900/20"
          >
            <p class="text-sm text-amber-700 dark:text-amber-300">
              {{ t('auth.thirdParty.invitationRequired', { providerName: resolvedProviderName }) }}
            </p>
          </div>

          <div v-if="requiresInvitation || invitationCodeEnabled">
            <label for="oauth-invitation" class="input-label">
              {{ t('auth.invitationCodeLabel') }}
            </label>
            <input
              id="oauth-invitation"
              v-model="createForm.invitationCode"
              type="text"
              class="input w-full"
              :disabled="isCreating"
              :placeholder="t('auth.invitationCodePlaceholder')"
            />
          </div>

          <p v-if="actionError" class="input-error-text">
            {{ actionError }}
          </p>

          <div class="grid gap-3 sm:grid-cols-2">
            <button type="submit" class="btn btn-primary w-full" :disabled="isCreating">
              {{ isCreating ? t('auth.thirdParty.creating') : t('auth.thirdParty.createSubmit') }}
            </button>
            <button
              v-if="canReturnToChoices"
              type="button"
              class="btn btn-secondary w-full"
              :disabled="isCreating"
              @click="goBackToChoices"
            >
              {{ t('auth.thirdParty.chooseAnotherAction') }}
            </button>
          </div>
        </form>
      </transition>

      <div v-if="isProcessing" class="flex items-center justify-center py-10">
        <div class="h-8 w-8 animate-spin rounded-full border-4 border-primary-500 border-t-transparent"></div>
      </div>

      <transition name="fade">
        <div
          v-if="errorMessage"
          class="rounded-xl border border-red-200 bg-red-50 p-4 dark:border-red-800/50 dark:bg-red-900/20"
        >
          <div class="flex items-start gap-3">
            <div class="flex-shrink-0">
              <Icon name="exclamationCircle" size="md" class="text-red-500" />
            </div>
            <div class="space-y-2">
              <p class="text-sm text-red-700 dark:text-red-400">
                {{ errorMessage }}
              </p>
              <router-link to="/login" class="btn btn-primary">
                {{ backToLoginText }}
              </router-link>
            </div>
          </div>
        </div>
      </transition>
    </div>

    <TotpLoginModal
      v-if="show2FAModal"
      ref="totpModalRef"
      :temp-token="totpTempToken"
      :user-email-masked="totpUserEmailMasked"
      @verify="handle2FAVerify"
      @cancel="handle2FACancel"
    />
  </AuthLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { AuthLayout } from '@/components/layout'
import Icon from '@/components/icons/Icon.vue'
import TurnstileWidget from '@/components/TurnstileWidget.vue'
import TotpLoginModal from '@/components/auth/TotpLoginModal.vue'
import {
  bindOAuthLogin,
  createOAuthAccount,
  getPublicSettings,
  sendVerifyCode,
  setRefreshToken,
  setTokenExpiresAt,
  type OAuthProvider,
  type OAuthBindLoginResponse,
  type OAuthTokenPairResponse
} from '@/api/auth'
import { useAuthStore, useAppStore } from '@/stores'

const props = defineProps<{
  provider: OAuthProvider
}>()

type FlowMode = 'idle' | 'choose' | 'bind' | 'create'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()

const invitationRequiredErrors = new Set(['invitation_required', 'oauth_invitation_required'])
const bindingRequiredErrors = new Set([
  'account_binding_required',
  'binding_required',
  'oauth_account_not_bound',
  'oauth_binding_required',
  'oauth_not_bound',
  'unbound_oauth_account'
])

const isProcessing = ref(true)
const isBinding = ref(false)
const isCreating = ref(false)
const errorMessage = ref('')
const actionError = ref('')
const mode = ref<FlowMode>('idle')
const pendingOAuthToken = ref('')
const redirectTo = ref('/dashboard')
const bindingIntent = ref(false)
const showPassword = ref(false)
const requiresInvitation = ref(false)
const invitationCodeEnabled = ref(false)
const turnstileEnabled = ref(false)
const turnstileSiteKey = ref('')
const oidcProviderName = ref('OIDC')
const wechatPaymentCallbackPath = '/auth/wechat/payment/callback'
const isSendingCreateCode = ref(false)
const createCodeSent = ref(false)
const createCodeCountdown = ref(0)
const showCreateTurnstile = ref(false)
const show2FAModal = ref(false)
const totpTempToken = ref('')
const totpUserEmailMasked = ref('')
const totpPendingOAuthToken = ref('')
let createCodeCountdownTimer: ReturnType<typeof setInterval> | null = null

const bindForm = reactive({
  email: '',
  password: ''
})

const createForm = reactive({
  email: '',
  verifyCode: '',
  invitationCode: ''
})

const bindErrors = reactive({
  email: '',
  password: '',
  turnstile: ''
})

const createErrors = reactive({
  email: '',
  verifyCode: '',
  turnstile: ''
})

const turnstileRef = ref<InstanceType<typeof TurnstileWidget> | null>(null)
const totpModalRef = ref<InstanceType<typeof TotpLoginModal> | null>(null)
const turnstileToken = ref('')
const canReturnToChoices = ref(false)
const supportsVerifiedEmailCreate = computed(() => props.provider === 'linuxdo' || props.provider === 'wechat')

const resolvedProviderName = computed(() => {
  if (props.provider === 'linuxdo') return 'Linux.do'
  if (props.provider === 'wechat') return 'WeChat'
  return oidcProviderName.value || 'OIDC'
})

const callbackTitle = computed(() => {
  if (props.provider === 'linuxdo') return t('auth.linuxdo.callbackTitle')
  if (props.provider === 'wechat') return t('auth.wechat.callbackTitle')
  return t('auth.oidc.callbackTitle', { providerName: resolvedProviderName.value })
})

const descriptionText = computed(() => {
  if (isProcessing.value) {
    if (props.provider === 'linuxdo') return t('auth.linuxdo.callbackProcessing')
    if (props.provider === 'wechat') return t('auth.wechat.callbackProcessing')
    return t('auth.oidc.callbackProcessing', { providerName: resolvedProviderName.value })
  }

  if (mode.value === 'choose') {
    return t('auth.thirdParty.chooseActionHint', { providerName: resolvedProviderName.value })
  }

  if (mode.value === 'bind') {
    return t('auth.thirdParty.bindHint', { providerName: resolvedProviderName.value })
  }

  if (mode.value === 'create') {
    return t('auth.thirdParty.createHint', { providerName: resolvedProviderName.value })
  }

  if (props.provider === 'linuxdo') return t('auth.linuxdo.callbackHint')
  if (props.provider === 'wechat') return t('auth.wechat.callbackHint')
  return t('auth.oidc.callbackHint')
})

const backToLoginText = computed(() => {
  if (props.provider === 'linuxdo') return t('auth.linuxdo.backToLogin')
  if (props.provider === 'wechat') return t('auth.wechat.backToLogin')
  return t('auth.oidc.backToLogin')
})

function readQueryParam(key: string): string {
  const value = route.query[key]
  if (Array.isArray(value)) {
    return typeof value[0] === 'string' ? value[0] : ''
  }
  return typeof value === 'string' ? value : ''
}

function parseFragmentParams(): URLSearchParams {
  const raw = typeof window !== 'undefined' ? window.location.hash : ''
  const hash = raw.startsWith('#') ? raw.slice(1) : raw
  return new URLSearchParams(hash)
}

function readParam(params: URLSearchParams, key: string): string {
  return params.get(key) || readQueryParam(key)
}

function readPendingOAuthToken(params: URLSearchParams): string {
  return readParam(params, 'pending_oauth_token') || readParam(params, 'pending_login_token')
}

function sanitizeRedirectPath(path: string | null | undefined): string {
  if (!path) return '/dashboard'
  if (!path.startsWith('/')) return '/dashboard'
  if (path.startsWith('//')) return '/dashboard'
  if (path.includes('://')) return '/dashboard'
  if (path.includes('\n') || path.includes('\r')) return '/dashboard'
  return path
}

function hasBindingIntent(path: string | null | undefined): boolean {
  if (!path) return false

  try {
    const parsed = new URL(path, window.location.origin)
    return parsed.searchParams.get('oauth_intent') === 'bind'
  } catch {
    return path.includes('oauth_intent=bind')
  }
}

function stripBindingIntent(path: string): string {
  try {
    const parsed = new URL(path, window.location.origin)
    parsed.searchParams.delete('oauth_intent')
    const query = parsed.searchParams.toString()
    return `${parsed.pathname}${query ? `?${query}` : ''}${parsed.hash}`
  } catch {
    return path
      .replace(/([?&])oauth_intent=bind&?/g, '$1')
      .replace(/[?&]$/, '')
  }
}

function extractErrorMessage(error: unknown, fallback: string): string {
  const err = error as {
    message?: string
    response?: {
      data?: {
        detail?: string
        error?: string
        error_description?: string
        message?: string
      }
    }
  }

  return (
    err.response?.data?.message ||
    err.response?.data?.detail ||
    err.response?.data?.error_description ||
    err.response?.data?.error ||
    err.message ||
    fallback
  )
}

function resetTurnstile(): void {
  if (turnstileRef.value) {
    turnstileRef.value.reset()
  }
  turnstileToken.value = ''
  createErrors.turnstile = ''
}

function onTurnstileVerify(token: string): void {
  turnstileToken.value = token
  bindErrors.turnstile = ''
}

function onTurnstileExpire(): void {
  turnstileToken.value = ''
  bindErrors.turnstile = t('auth.turnstileExpired')
}

function onTurnstileError(): void {
  turnstileToken.value = ''
  bindErrors.turnstile = t('auth.turnstileFailed')
}

function validateBindForm(): boolean {
  bindErrors.email = ''
  bindErrors.password = ''
  bindErrors.turnstile = ''

  let isValid = true

  if (!bindForm.email.trim()) {
    bindErrors.email = t('auth.emailRequired')
    isValid = false
  } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(bindForm.email)) {
    bindErrors.email = t('auth.invalidEmail')
    isValid = false
  }

  if (!bindForm.password) {
    bindErrors.password = t('auth.passwordRequired')
    isValid = false
  }

  if (turnstileEnabled.value && !turnstileToken.value) {
    bindErrors.turnstile = t('auth.completeVerification')
    isValid = false
  }

  return isValid
}

function isOAuthBind2FAResponse(response: OAuthBindLoginResponse): response is OAuthBindLoginResponse & {
  requires_2fa: true
  temp_token?: string
  pending_oauth_token?: string
  user_email_masked?: string
} {
  return 'requires_2fa' in response && response.requires_2fa === true
}

function startCreateCodeCountdown(seconds: number): void {
  createCodeCountdown.value = seconds
  if (createCodeCountdownTimer) {
    clearInterval(createCodeCountdownTimer)
  }
  createCodeCountdownTimer = setInterval(() => {
    if (createCodeCountdown.value > 0) {
      createCodeCountdown.value -= 1
      return
    }
    if (createCodeCountdownTimer) {
      clearInterval(createCodeCountdownTimer)
      createCodeCountdownTimer = null
    }
  }, 1000)
}

function validateCreateForm(): boolean {
  createErrors.email = ''
  createErrors.verifyCode = ''

  let isValid = true

  if (supportsVerifiedEmailCreate.value) {
    if (!createForm.email.trim()) {
      createErrors.email = t('auth.emailRequired')
      isValid = false
    } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(createForm.email)) {
      createErrors.email = t('auth.invalidEmail')
      isValid = false
    }

    if (!createForm.verifyCode.trim()) {
      createErrors.verifyCode = t('auth.codeRequired')
      isValid = false
    } else if (!/^\d{6}$/.test(createForm.verifyCode.trim())) {
      createErrors.verifyCode = t('auth.invalidCode')
      isValid = false
    }
  }

  if ((requiresInvitation.value || invitationCodeEnabled.value) && !createForm.invitationCode.trim()) {
    actionError.value = t('auth.invitationCodeRequired')
    isValid = false
  }

  return isValid
}

async function handleSendCreateVerifyCode(): Promise<void> {
  createErrors.email = ''
  createErrors.turnstile = ''
  actionError.value = ''

  if (!supportsVerifiedEmailCreate.value) {
    return
  }

  if (!createForm.email.trim()) {
    createErrors.email = t('auth.emailRequired')
    return
  }
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(createForm.email)) {
    createErrors.email = t('auth.invalidEmail')
    return
  }

  if (turnstileEnabled.value && !showCreateTurnstile.value && !turnstileToken.value) {
    showCreateTurnstile.value = true
    return
  }
  if (turnstileEnabled.value && showCreateTurnstile.value && !turnstileToken.value) {
    createErrors.turnstile = t('auth.completeVerification')
    return
  }

  isSendingCreateCode.value = true
  try {
    const response = await sendVerifyCode({
      email: createForm.email.trim(),
      turnstile_token: turnstileEnabled.value ? turnstileToken.value : undefined,
      pending_oauth_token: pendingOAuthToken.value
    })
    createCodeSent.value = true
    showCreateTurnstile.value = false
    resetTurnstile()
    startCreateCodeCountdown(response.countdown)
    appStore.showSuccess(t('auth.codeSentSuccess'))
  } catch (error: unknown) {
    createErrors.turnstile = ''
    actionError.value = extractErrorMessage(error, t('auth.sendCodeFailed'))
  } finally {
    isSendingCreateCode.value = false
  }
}

async function finalizeOAuthLogin(response: OAuthTokenPairResponse): Promise<void> {
  if (bindingIntent.value && authStore.isAuthenticated) {
    await authStore.refreshUser()
    appStore.showSuccess(t('profile.bindings.connected'))
    await router.replace(redirectTo.value)
    return
  }

  if (response.refresh_token) {
    setRefreshToken(response.refresh_token)
  }
  if (response.expires_in) {
    setTokenExpiresAt(response.expires_in)
  }

  await authStore.setToken(response.access_token)
  appStore.showSuccess(t('auth.loginSuccess'))
  await router.replace(redirectTo.value)
}

function goBackToChoices(): void {
  actionError.value = ''
  bindErrors.email = ''
  bindErrors.password = ''
  bindErrors.turnstile = ''
  createErrors.email = ''
  createErrors.verifyCode = ''
  createErrors.turnstile = ''
  resetTurnstile()
  showCreateTurnstile.value = false
  mode.value = bindingIntent.value ? 'bind' : 'choose'
}

async function handleBindExistingAccount(): Promise<void> {
  actionError.value = ''
  if (!validateBindForm()) {
    return
  }

  isBinding.value = true
  try {
    const response = await bindOAuthLogin(props.provider, {
      pendingOAuthToken: pendingOAuthToken.value,
      email: bindForm.email.trim(),
      password: bindForm.password,
      turnstileToken: turnstileEnabled.value ? turnstileToken.value : undefined
    })
    if (isOAuthBind2FAResponse(response)) {
      totpTempToken.value = response.temp_token || ''
      totpPendingOAuthToken.value = response.pending_oauth_token || pendingOAuthToken.value
      totpUserEmailMasked.value = response.user_email_masked || ''
      show2FAModal.value = true
      return
    }
    await finalizeOAuthLogin(response as OAuthTokenPairResponse)
  } catch (error: unknown) {
    resetTurnstile()
    actionError.value = extractErrorMessage(error, t('auth.thirdParty.bindFailed'))
  } finally {
    isBinding.value = false
  }
}

async function handleCreateAccount(): Promise<void> {
  actionError.value = ''
  if (bindingIntent.value) {
    mode.value = 'bind'
    actionError.value = t('auth.thirdParty.bindingOnly', { providerName: resolvedProviderName.value })
    return
  }
  if (!validateCreateForm()) return

  isCreating.value = true
  try {
    const response = await createOAuthAccount(props.provider, {
      pendingOAuthToken: pendingOAuthToken.value,
      email: supportsVerifiedEmailCreate.value ? createForm.email.trim() : undefined,
      verifyCode: supportsVerifiedEmailCreate.value ? createForm.verifyCode.trim() : undefined,
      invitationCode: createForm.invitationCode.trim() || undefined
    })
    await finalizeOAuthLogin(response)
  } catch (error: unknown) {
    actionError.value = extractErrorMessage(error, t('auth.thirdParty.createFailed'))
  } finally {
    isCreating.value = false
  }
}

async function handle2FAVerify(code: string): Promise<void> {
  if (totpModalRef.value) {
    totpModalRef.value.setVerifying(true)
  }

  try {
    await authStore.login2FA(totpTempToken.value, code, totpPendingOAuthToken.value || undefined)
    show2FAModal.value = false
    totpTempToken.value = ''
    totpPendingOAuthToken.value = ''
    totpUserEmailMasked.value = ''
    appStore.showSuccess(bindingIntent.value ? t('profile.bindings.connected') : t('auth.loginSuccess'))
    if (bindingIntent.value && authStore.isAuthenticated) {
      await authStore.refreshUser()
    }
    await router.replace(redirectTo.value)
  } catch (error: unknown) {
    const message = extractErrorMessage(error, t('profile.totp.loginFailed'))
    if (totpModalRef.value) {
      totpModalRef.value.setError(message)
      totpModalRef.value.setVerifying(false)
    }
  }
}

function handle2FACancel(): void {
  show2FAModal.value = false
  totpTempToken.value = ''
  totpPendingOAuthToken.value = ''
  totpUserEmailMasked.value = ''
}

function setFatalError(message: string): void {
  errorMessage.value = message
  appStore.showError(message)
  isProcessing.value = false
}

function isLegacyWechatPaymentCallback(params: URLSearchParams): boolean {
  if (props.provider !== 'wechat') return false
  return Boolean(
    readParam(params, 'openid') ||
    readParam(params, 'wechat_resume') ||
    readParam(params, 'payment_type') ||
    readParam(params, 'order_type')
  )
}

async function loadPublicSettings(): Promise<void> {
  try {
    const settings = await getPublicSettings()
    invitationCodeEnabled.value = settings.invitation_code_enabled
    turnstileEnabled.value = settings.turnstile_enabled
    turnstileSiteKey.value = settings.turnstile_site_key || ''
    if (props.provider === 'oidc') {
      oidcProviderName.value = settings.oidc_oauth_provider_name?.trim() || 'OIDC'
    }
  } catch {
    // Ignore and keep fallbacks so callback completion can continue.
  }
}

onMounted(async () => {
  await loadPublicSettings()

  const params = parseFragmentParams()

  if (isLegacyWechatPaymentCallback(params)) {
    await router.replace({
      path: wechatPaymentCallbackPath,
      query: route.query,
      hash: typeof window !== 'undefined' ? window.location.hash : ''
    })
    return
  }

  const token = readParam(params, 'access_token')
  const refreshToken = readParam(params, 'refresh_token')
  const expiresInStr = readParam(params, 'expires_in')
  const redirect = sanitizeRedirectPath(
    readParam(params, 'redirect') || (route.query.redirect as string | undefined) || '/dashboard'
  )
  const error = readParam(params, 'error')
  const errorDesc =
    readParam(params, 'error_description') ||
    readParam(params, 'error_message') ||
    readParam(params, 'message')

  bindingIntent.value = readParam(params, 'intent') === 'bind' || hasBindingIntent(redirect)
  redirectTo.value = bindingIntent.value ? stripBindingIntent(redirect) : redirect
  bindForm.email =
    readParam(params, 'email') ||
    readParam(params, 'email_hint') ||
    readParam(params, 'login_email') ||
    ''
  createForm.email = bindForm.email

  if (error) {
    pendingOAuthToken.value = readPendingOAuthToken(params)

    if (invitationRequiredErrors.has(error)) {
      requiresInvitation.value = true
      canReturnToChoices.value = false
      if (!pendingOAuthToken.value) {
        const message =
          props.provider === 'linuxdo'
            ? t('auth.linuxdo.invalidPendingToken')
            : props.provider === 'wechat'
              ? t('auth.wechat.invalidPendingToken')
              : t('auth.oidc.invalidPendingToken')
        setFatalError(message)
        return
      }
      mode.value = 'create'
      isProcessing.value = false
      return
    }

    if (bindingRequiredErrors.has(error)) {
      canReturnToChoices.value = !bindingIntent.value
      if (!pendingOAuthToken.value) {
        setFatalError(t('auth.thirdParty.invalidPendingToken'))
        return
      }
      mode.value = bindingIntent.value ? 'bind' : 'choose'
      isProcessing.value = false
      return
    }

    setFatalError(errorDesc || error)
    return
  }

  if (!token) {
    const message =
      props.provider === 'linuxdo'
        ? t('auth.linuxdo.callbackMissingToken')
        : props.provider === 'wechat'
          ? t('auth.wechat.callbackMissingToken')
          : t('auth.oidc.callbackMissingToken')
    setFatalError(message)
    return
  }

  try {
    await finalizeOAuthLogin({
      access_token: token,
      refresh_token: refreshToken || undefined,
      expires_in: expiresInStr ? parseInt(expiresInStr, 10) : undefined,
      token_type: 'Bearer'
    })
  } catch (error: unknown) {
    setFatalError(extractErrorMessage(error, t('auth.loginFailed')))
  }
})

onUnmounted(() => {
  if (createCodeCountdownTimer) {
    clearInterval(createCodeCountdownTimer)
    createCodeCountdownTimer = null
  }
})
</script>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: all 0.3s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}
</style>
