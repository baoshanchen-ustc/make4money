<template>
  <Teleport to="body">
    <transition name="modal-fade">
      <div
        v-if="show"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
        @click.self="handleOverlayClick"
      >
        <div
          class="w-full max-w-md rounded-2xl bg-white p-6 shadow-xl dark:bg-dark-800"
          @click.stop
        >
          <div class="flex items-start justify-between mb-4">
            <div class="flex items-start gap-3">
              <div class="flex-shrink-0">
                <svg
                  class="h-6 w-6 text-amber-500"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                  />
                </svg>
              </div>
              <div>
                <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
                  {{ required ? t('auth.emailBind.titleRequired') : t('auth.emailBind.title') }}
                </h3>
              </div>
            </div>
            <button
              v-if="!required"
              type="button"
              @click="handleCancel"
              class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
            >
              <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>

          <!-- 提示内容 -->
          <div class="mb-6 space-y-3">
            <p class="text-sm text-gray-600 dark:text-dark-400">
              {{ required ? t('auth.emailBind.descriptionRequired') : t('auth.emailBind.description') }}
            </p>
            <ul v-if="!required" class="space-y-2 text-sm text-gray-600 dark:text-dark-400">
              <li class="flex items-start gap-2">
                <svg class="h-5 w-5 flex-shrink-0 text-green-500 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                </svg>
                <span>{{ t('auth.emailBind.benefit1') }}</span>
              </li>
              <li class="flex items-start gap-2">
                <svg class="h-5 w-5 flex-shrink-0 text-green-500 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                </svg>
                <span>{{ t('auth.emailBind.benefit2') }}</span>
              </li>
              <li class="flex items-start gap-2">
                <svg class="h-5 w-5 flex-shrink-0 text-green-500 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                </svg>
                <span>{{ t('auth.emailBind.benefit3') }}</span>
              </li>
            </ul>
          </div>

          <!-- Email Input -->
          <div class="mb-4 space-y-4">
            <div>
              <label for="email-bind-email" class="input-label">
                {{ t('auth.emailBind.emailLabel') }}
              </label>
              <input
                id="email-bind-email"
                v-model="email"
                type="email"
                :disabled="isLoading"
                class="input"
                :class="{ 'input-error': emailError }"
                :placeholder="t('auth.emailBind.emailPlaceholder')"
                @keyup.enter="handleSendCode"
              />
              <p v-if="emailError" class="input-error-text mt-1">
                {{ emailError }}
              </p>
            </div>

            <!-- Verification Code Input -->
            <div>
              <label for="email-bind-code" class="input-label">
                {{ t('auth.emailBind.codeLabel') }}
              </label>
              <div class="flex gap-2">
                <input
                  id="email-bind-code"
                  v-model="verifyCode"
                  type="text"
                  :disabled="isLoading || !codeSent"
                  class="input flex-1 text-center text-lg tracking-widest"
                  :class="{ 'input-error': codeError }"
                  :placeholder="t('auth.emailBind.codePlaceholder')"
                  maxlength="6"
                  @keyup.enter="handleBind"
                />
                <button
                  type="button"
                  :disabled="isLoading || isSendingCode || countdown > 0 || !isValidEmail"
                  class="btn btn-secondary whitespace-nowrap"
                  @click="handleSendCode"
                >
                  <svg
                    v-if="isSendingCode"
                    class="-ml-1 mr-2 h-4 w-4 animate-spin"
                    fill="none"
                    viewBox="0 0 24 24"
                  >
                    <circle
                      class="opacity-25"
                      cx="12"
                      cy="12"
                      r="10"
                      stroke="currentColor"
                      stroke-width="4"
                    ></circle>
                    <path
                      class="opacity-75"
                      fill="currentColor"
                      d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                    ></path>
                  </svg>
                  {{ countdown > 0 ? `${countdown}s` : t('auth.emailBind.sendCode') }}
                </button>
              </div>
              <p v-if="codeError" class="input-error-text mt-1">
                {{ codeError }}
              </p>
            </div>

            <!-- Password Input (required 模式下显示) -->
            <template v-if="required">
              <div>
                <label for="email-bind-password" class="input-label">
                  {{ t('auth.emailBind.passwordLabel') }}
                </label>
                <input
                  id="email-bind-password"
                  v-model="password"
                  type="password"
                  :disabled="isLoading"
                  class="input"
                  :class="{ 'input-error': passwordError }"
                  :placeholder="t('auth.emailBind.passwordPlaceholder')"
                  autocomplete="new-password"
                />
                <p v-if="passwordError" class="input-error-text mt-1">
                  {{ passwordError }}
                </p>
              </div>

              <div>
                <label for="email-bind-confirm-password" class="input-label">
                  {{ t('auth.emailBind.confirmPasswordLabel') }}
                </label>
                <input
                  id="email-bind-confirm-password"
                  v-model="confirmPassword"
                  type="password"
                  :disabled="isLoading"
                  class="input"
                  :class="{ 'input-error': confirmPasswordError }"
                  :placeholder="t('auth.emailBind.confirmPasswordPlaceholder')"
                  autocomplete="new-password"
                  @keyup.enter="handleBind"
                />
                <p v-if="confirmPasswordError" class="input-error-text mt-1">
                  {{ confirmPasswordError }}
                </p>
              </div>
            </template>

            <!-- Turnstile Widget -->
            <div v-if="turnstileEnabled && turnstileSiteKey">
              <TurnstileWidget
                ref="turnstileRef"
                :site-key="turnstileSiteKey"
                @verify="onTurnstileVerify"
                @expire="onTurnstileExpire"
                @error="onTurnstileError"
              />
              <p v-if="turnstileError" class="input-error-text mt-2 text-center">
                {{ turnstileError }}
              </p>
            </div>

            <div class="flex gap-3">
              <button
                v-if="!required"
                type="button"
                :disabled="isLoading"
                class="btn btn-secondary flex-1"
                @click="handleSkip"
              >
                {{ t('auth.emailBind.skip') }}
              </button>
              <button
                type="button"
                :disabled="isLoading || !verifyCode.trim() || !codeSent"
                class="btn btn-primary flex-1"
                @click="handleBind"
              >
                <svg
                  v-if="isLoading"
                  class="-ml-1 mr-2 h-4 w-4 animate-spin text-white"
                  fill="none"
                  viewBox="0 0 24 24"
                >
                  <circle
                    class="opacity-25"
                    cx="12"
                    cy="12"
                    r="10"
                    stroke="currentColor"
                    stroke-width="4"
                  ></circle>
                  <path
                    class="opacity-75"
                    fill="currentColor"
                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                  ></path>
                </svg>
                {{ isLoading ? t('auth.emailBind.binding') : t('auth.emailBind.bind') }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore, useAuthStore } from '@/stores'
import { bindEmail, sendBindEmailCode } from '@/api/auth'
import TurnstileWidget from '@/components/TurnstileWidget.vue'

const props = defineProps<{
  show: boolean
  required?: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'skip'): void
  (e: 'success'): void
}>()

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const email = ref('')
const verifyCode = ref('')
const password = ref('')
const confirmPassword = ref('')
const emailError = ref('')
const codeError = ref('')
const passwordError = ref('')
const confirmPasswordError = ref('')
const isLoading = ref(false)
const isSendingCode = ref(false)
const codeSent = ref(false)
const countdown = ref(0)

// Turnstile
const turnstileEnabled = ref(false)
const turnstileSiteKey = ref('')
const turnstileRef = ref<InstanceType<typeof TurnstileWidget> | null>(null)
const turnstileToken = ref('')
const turnstileError = ref('')

// Email validation
const isValidEmail = computed(() => {
  const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
  return emailRegex.test(email.value.trim())
})

onMounted(async () => {
  // Get Turnstile settings
  const settings = appStore.cachedPublicSettings
  if (settings) {
    turnstileEnabled.value = settings.turnstile_enabled
    turnstileSiteKey.value = settings.turnstile_site_key || ''
  }
})

function handleOverlayClick(): void {
  if (!props.required) {
    emit('close')
  }
}

function handleCancel(): void {
  emit('close')
}

function handleSkip(): void {
  emit('skip')
}

function onTurnstileVerify(token: string): void {
  turnstileToken.value = token
  turnstileError.value = ''
}

function onTurnstileExpire(): void {
  turnstileToken.value = ''
  turnstileError.value = t('auth.turnstileExpired')
}

function onTurnstileError(): void {
  turnstileToken.value = ''
  turnstileError.value = t('auth.turnstileFailed')
}

async function handleSendCode(): Promise<void> {
  emailError.value = ''
  codeError.value = ''

  if (!email.value.trim()) {
    emailError.value = t('auth.emailRequired')
    return
  }

  if (!isValidEmail.value) {
    emailError.value = t('auth.invalidEmail')
    return
  }

  // Check Turnstile if enabled
  if (turnstileEnabled.value && !turnstileToken.value) {
    turnstileError.value = t('auth.completeVerification')
    return
  }

  isSendingCode.value = true

  try {
    const response = await sendBindEmailCode({
      email: email.value.trim(),
      turnstile_token: turnstileEnabled.value ? turnstileToken.value : undefined
    })

    codeSent.value = true
    countdown.value = response.countdown || 60

    // Start countdown timer
    const timer = setInterval(() => {
      countdown.value--
      if (countdown.value <= 0) {
        clearInterval(timer)
      }
    }, 1000)

    appStore.showSuccess(t('auth.emailBind.codeSent'))
  } catch (error: unknown) {
    // Reset Turnstile on error
    if (turnstileRef.value) {
      turnstileRef.value.reset()
      turnstileToken.value = ''
    }

    const err = error as { message?: string; response?: { data?: { detail?: string } } }
    if (err.response?.data?.detail) {
      emailError.value = err.response.data.detail
    } else if (err.message) {
      emailError.value = err.message
    } else {
      emailError.value = t('auth.emailBind.sendCodeFailed')
    }

    appStore.showError(emailError.value)
  } finally {
    isSendingCode.value = false
  }
}

function validatePassword(): boolean {
  passwordError.value = ''
  confirmPasswordError.value = ''

  if (!password.value) {
    passwordError.value = t('auth.emailBind.passwordRequired')
    return false
  }

  if (password.value.length < 6) {
    passwordError.value = t('auth.emailBind.passwordMinLength')
    return false
  }

  if (password.value !== confirmPassword.value) {
    confirmPasswordError.value = t('auth.emailBind.passwordMismatch')
    return false
  }

  return true
}

async function handleBind(): Promise<void> {
  if (!verifyCode.value.trim()) {
    codeError.value = t('auth.emailBind.codeRequired')
    return
  }

  // required 模式下验证密码
  if (props.required && !validatePassword()) {
    return
  }

  codeError.value = ''
  isLoading.value = true

  try {
    await bindEmail({
      email: email.value.trim(),
      verify_code: verifyCode.value.trim(),
      password: props.required ? password.value : undefined
    })
  } catch (error: unknown) {
    const err = error as { message?: string; response?: { data?: { detail?: string } } }

    if (err.response?.data?.detail) {
      codeError.value = err.response.data.detail
    } else if (err.message) {
      codeError.value = err.message
    } else {
      codeError.value = t('auth.emailBind.bindFailed')
    }

    appStore.showError(codeError.value)
    isLoading.value = false
    return
  }

  // 绑定成功后刷新用户信息
  // refreshUser 可能因密码设置导致 TokenVersion 变更而失败，不阻断成功流程
  try {
    await authStore.refreshUser()
  } catch {
    // 预期行为：设置密码后旧 token 失效，跳转后会获取新 token
  }

  isLoading.value = false
  appStore.showSuccess(t('auth.emailBind.success'))
  emit('success')
}
</script>

<style scoped>
.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: opacity 0.2s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}
</style>
