<template>
  <div v-if="wechatAuthEnabled" class="card">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <h2 class="text-lg font-medium text-gray-900 dark:text-white">
        {{ t('profile.wechatBind.title') }}
      </h2>
    </div>
    <div class="px-6 py-6">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-3">
          <div class="flex h-10 w-10 items-center justify-center rounded-full" :class="isBound ? 'bg-green-100 dark:bg-green-900/30' : 'bg-gray-100 dark:bg-dark-700'">
            <svg viewBox="0 0 24 24" width="24" height="24" fill="currentColor" :class="isBound ? 'text-green-600 dark:text-green-400' : 'text-gray-400 dark:text-dark-400'">
              <path d="M8.691 2.188C3.891 2.188 0 5.476 0 9.53c0 2.212 1.17 4.203 3.002 5.55a.59.59 0 0 1 .213.665l-.39 1.48c-.019.07-.048.141-.048.213 0 .163.13.295.29.295a.32.32 0 0 0 .186-.057l2.019-1.179a.73.73 0 0 1 .59-.056c.797.263 1.66.404 2.554.404.186 0 .368-.008.55-.022-.096-.312-.148-.635-.148-.968 0-3.347 3.291-6.056 7.357-6.056.204 0 .405.011.603.029-.622-3.439-4.3-5.64-8.087-5.64zM5.785 5.991c.642 0 1.162.529 1.162 1.18 0 .651-.52 1.18-1.162 1.18-.642 0-1.162-.529-1.162-1.18 0-.651.52-1.18 1.162-1.18zm5.813 0c.642 0 1.162.529 1.162 1.18 0 .651-.52 1.18-1.162 1.18-.642 0-1.162-.529-1.162-1.18 0-.651.52-1.18 1.162-1.18z" />
              <path d="M23.8 14.617c0-3.185-3.148-5.767-7.03-5.767-3.883 0-7.031 2.582-7.031 5.767 0 3.186 3.148 5.768 7.03 5.768.718 0 1.408-.092 2.059-.264a.614.614 0 0 1 .5.048l1.63.955a.269.269 0 0 0 .158.048c.131 0 .243-.111.243-.247 0-.059-.024-.12-.04-.178l-.315-1.195a.505.505 0 0 1 .181-.566c1.48-1.082 2.415-2.686 2.415-4.37zm-9.433-1.18c-.538 0-.974-.443-.974-.99s.436-.99.974-.99c.537 0 .974.443.974.99s-.437.99-.974.99zm4.807 0c-.538 0-.974-.443-.974-.99s.436-.99.974-.99c.537 0 .973.443.973.99s-.436.99-.973.99z" />
            </svg>
          </div>
          <div>
            <p class="font-medium text-gray-900 dark:text-white">{{ t('profile.wechatBind.wechatAccount') }}</p>
            <p v-if="isBound" class="text-sm text-green-600 dark:text-green-400">
              {{ t('profile.wechatBind.boundStatus') }}
            </p>
            <p v-else class="text-sm text-gray-500 dark:text-dark-400">{{ t('profile.wechatBind.description') }}</p>
          </div>
        </div>
        <div v-if="isBound" class="flex items-center gap-2">
          <span class="inline-flex items-center rounded-full bg-green-100 px-2.5 py-0.5 text-xs font-medium text-green-800 dark:bg-green-900/30 dark:text-green-400">
            <svg class="mr-1 h-3 w-3" fill="currentColor" viewBox="0 0 20 20">
              <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
            </svg>
            {{ t('profile.wechatBind.bound') }}
          </span>
          <button
            type="button"
            class="btn btn-secondary btn-sm text-red-600 hover:text-red-700 hover:bg-red-50 dark:text-red-400 dark:hover:text-red-300 dark:hover:bg-red-900/20"
            @click="showUnbindConfirm = true"
          >
            {{ t('profile.wechatBind.unbindButton') }}
          </button>
        </div>
        <button v-else type="button" class="btn btn-secondary" @click="openBindModal">
          {{ t('profile.wechatBind.bindButton') }}
        </button>
      </div>
    </div>

    <!-- WeChat Bind Modal -->
    <Teleport to="body">
      <transition name="modal-fade">
        <div
          v-if="showBindModal"
          class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
          @click.self="closeModal"
        >
          <div
            class="w-full max-w-md rounded-2xl bg-white p-6 shadow-xl dark:bg-dark-800"
            @click.stop
          >
            <div class="flex items-center justify-between mb-4">
              <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('profile.wechatBind.modalTitle') }}
              </h3>
              <button
                type="button"
                @click="closeModal"
                class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
              >
                <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                </svg>
              </button>
            </div>

            <!-- QR Code -->
            <div class="flex flex-col items-center space-y-4">
              <div class="rounded-lg border border-gray-200 bg-white p-2 dark:border-dark-600 relative">
                <!-- Loading state (subscription mode) -->
                <div
                  v-if="scanLoading"
                  class="flex h-48 w-48 items-center justify-center"
                >
                  <svg class="h-8 w-8 animate-spin text-gray-400" fill="none" viewBox="0 0 24 24">
                    <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                    <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                  </svg>
                </div>
                <!-- QR code image -->
                <img
                  v-else-if="displayQrCodeUrl"
                  :src="displayQrCodeUrl"
                  :alt="t('auth.wechat.qrCodeAlt')"
                  class="h-48 w-48 object-contain"
                  :class="{ 'opacity-30': scanExpired }"
                />
                <!-- No QR code -->
                <div
                  v-else
                  class="flex h-48 w-48 items-center justify-center text-gray-400"
                >
                  {{ t('auth.wechat.noQrCode') }}
                </div>
                <!-- Expired overlay (subscription mode) -->
                <div
                  v-if="scanExpired && !scanLoading && isSubscriptionMode"
                  class="absolute inset-0 flex flex-col items-center justify-center bg-white/80 dark:bg-dark-800/80"
                >
                  <p class="text-sm text-gray-500 dark:text-dark-400 mb-2">{{ t('auth.wechat.scanExpired') }}</p>
                  <button
                    type="button"
                    class="btn btn-secondary btn-sm"
                    @click="initScanBind"
                  >
                    {{ t('auth.wechat.scanRefresh') }}
                  </button>
                </div>
              </div>

              <!-- Status message for subscription mode -->
              <div v-if="isSubscriptionMode" class="text-center">
                <p v-if="scanStatus === 'waiting'" class="text-sm text-gray-500 dark:text-dark-400">
                  {{ t('auth.wechat.scanAutoTip') }}
                </p>
                <p v-else-if="scanStatus === 'confirmed'" class="text-sm text-green-600 dark:text-green-400">
                  {{ t('profile.wechatBind.bindSuccess') }}
                </p>
                <p v-if="scanStatus === 'waiting' && !scanExpired && !scanLoading" class="text-xs text-gray-400 dark:text-dark-500 mt-1">
                  {{ t('auth.wechat.scanWaiting') }}
                </p>
              </div>
              <!-- Status message for unverified_official mode -->
              <p v-else class="text-center text-sm text-gray-500 dark:text-dark-400">
                {{ t('auth.wechat.scanTipOfficial') }}
              </p>
            </div>

            <!-- Error message for scan mode -->
            <p v-if="scanError && isSubscriptionMode" class="mt-4 text-center text-sm text-red-500">
              {{ scanError }}
            </p>

            <!-- Verification Code Input (only for unverified_official mode) -->
            <div v-if="!isSubscriptionMode" class="mt-6 space-y-4">
              <div>
                <label for="wechat-bind-code" class="input-label">
                  {{ t('auth.wechat.codeLabel') }}
                </label>
                <input
                  id="wechat-bind-code"
                  v-model="verifyCode"
                  type="text"
                  :disabled="isLoading"
                  class="input text-center text-lg tracking-widest"
                  :class="{ 'input-error': codeError }"
                  :placeholder="t('auth.wechat.codePlaceholder')"
                  maxlength="6"
                  @keyup.enter="handleWeChatBind"
                />
                <p v-if="codeError" class="input-error-text mt-1">
                  {{ codeError }}
                </p>
              </div>

              <button
                type="button"
                :disabled="isLoading || !verifyCode.trim()"
                class="btn btn-primary w-full"
                @click="handleWeChatBind"
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
                {{ isLoading ? t('profile.wechatBind.binding') : t('profile.wechatBind.bindButton') }}
              </button>
            </div>
          </div>
        </div>
      </transition>
    </Teleport>

    <!-- WeChat Unbind Confirm Modal -->
    <Teleport to="body">
      <transition name="modal-fade">
        <div
          v-if="showUnbindConfirm"
          class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
          @click.self="showUnbindConfirm = false"
        >
          <div
            class="w-full max-w-sm rounded-2xl bg-white p-6 shadow-xl dark:bg-dark-800"
            @click.stop
          >
            <div class="flex items-center justify-center mb-4">
              <div class="flex h-12 w-12 items-center justify-center rounded-full bg-red-100 dark:bg-red-900/30">
                <svg class="h-6 w-6 text-red-600 dark:text-red-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                </svg>
              </div>
            </div>
            <h3 class="text-lg font-semibold text-center text-gray-900 dark:text-white mb-2">
              {{ t('profile.wechatBind.unbindConfirmTitle') }}
            </h3>
            <p class="text-sm text-center text-gray-500 dark:text-dark-400 mb-6">
              {{ t('profile.wechatBind.unbindConfirmMessage') }}
            </p>
            <div class="flex gap-3">
              <button
                type="button"
                class="btn btn-secondary flex-1"
                :disabled="isUnbinding"
                @click="showUnbindConfirm = false"
              >
                {{ t('common.cancel') }}
              </button>
              <button
                type="button"
                class="btn flex-1 bg-red-600 text-white hover:bg-red-700 focus:ring-red-500"
                :disabled="isUnbinding"
                @click="handleWeChatUnbind"
              >
                <svg
                  v-if="isUnbinding"
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
                {{ isUnbinding ? t('profile.wechatBind.unbinding') : t('profile.wechatBind.confirmUnbind') }}
              </button>
            </div>
          </div>
        </div>
      </transition>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { authAPI } from '@/api'
import { wechatBind, wechatUnbind, wechatScanInit, wechatScanBindPoll } from '@/api/auth'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const wechatAuthEnabled = ref(false)
const wechatAccountType = ref('subscription')
const qrCodeUrl = ref('')
const showBindModal = ref(false)
const showUnbindConfirm = ref(false)
const verifyCode = ref('')
const codeError = ref('')
const isLoading = ref(false)
const isUnbinding = ref(false)

// Scan bind state (for subscription mode)
const scanSceneId = ref('')
const scanQrCodeUrl = ref('')
const scanStatus = ref<'waiting' | 'confirmed'>('waiting')
const scanExpired = ref(false)
const scanLoading = ref(false)
const scanError = ref('')
let pollTimer: ReturnType<typeof setInterval> | null = null
let expireTimer: ReturnType<typeof setTimeout> | null = null

const isSubscriptionMode = computed(() => wechatAccountType.value === 'subscription')

// Display either the dynamic scan QR code or the static QR code
const displayQrCodeUrl = computed(() => {
  if (isSubscriptionMode.value && scanQrCodeUrl.value) {
    return scanQrCodeUrl.value
  }
  return qrCodeUrl.value
})

// Check if user has already bound WeChat
const isBound = computed(() => {
  return !!(authStore.user?.wechat_openid && authStore.user.wechat_openid.length > 0)
})

onMounted(async () => {
  try {
    const settings = await authAPI.getPublicSettings()
    wechatAuthEnabled.value = settings.wechat_auth_enabled
    wechatAccountType.value = settings.wechat_account_type || 'subscription'
    qrCodeUrl.value = settings.wechat_account_qrcode_data || settings.wechat_account_qrcode_url || ''
  } catch (error) {
    console.error('Failed to load public settings:', error)
  }
})

onUnmounted(() => {
  clearTimers()
})

function clearTimers(): void {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
  if (expireTimer) {
    clearTimeout(expireTimer)
    expireTimer = null
  }
}

function closeModal(): void {
  showBindModal.value = false
  verifyCode.value = ''
  codeError.value = ''
  scanSceneId.value = ''
  scanQrCodeUrl.value = ''
  scanStatus.value = 'waiting'
  scanExpired.value = false
  scanLoading.value = false
  scanError.value = ''
  clearTimers()
}

async function openBindModal(): Promise<void> {
  showBindModal.value = true

  // For subscription mode, initialize scan bind
  if (isSubscriptionMode.value) {
    await initScanBind()
  }
}

async function initScanBind(): Promise<void> {
  scanLoading.value = true
  scanError.value = ''
  scanExpired.value = false
  scanStatus.value = 'waiting'
  clearTimers()

  try {
    const result = await wechatScanInit()
    scanSceneId.value = result.scene_id
    scanQrCodeUrl.value = result.qr_code_url

    // Start polling
    startBindPolling()

    // Set expiration timer
    expireTimer = setTimeout(() => {
      scanExpired.value = true
      clearTimers()
    }, result.expire_seconds * 1000)
  } catch (error: unknown) {
    const err = error as { response?: { data?: { detail?: string } }; message?: string }
    scanError.value = err.response?.data?.detail || err.message || t('auth.wechat.scanError')
  } finally {
    scanLoading.value = false
  }
}

function startBindPolling(): void {
  if (pollTimer) return

  pollTimer = setInterval(async () => {
    if (!scanSceneId.value || scanExpired.value) {
      clearTimers()
      return
    }

    try {
      const result = await wechatScanBindPoll(scanSceneId.value)

      if (result.status === 'confirmed') {
        scanStatus.value = 'confirmed'
        clearTimers()

        // Refresh user data to update wechat_openid
        await authStore.refreshUser()

        // Show success
        appStore.showSuccess(t('profile.wechatBind.bindSuccess'))

        // Close modal after a short delay
        setTimeout(() => {
          closeModal()
        }, 500)
      }
    } catch (error: unknown) {
      const err = error as { status?: number; message?: string }
      // If it's a conflict error (already bound), show error and stop polling
      if (err.status === 409) {
        scanError.value = err.message || t('profile.wechatBind.bindFailed')
        appStore.showError(scanError.value)
        clearTimers()
      }
      // Ignore other polling errors, will retry
    }
  }, 2000)
}

async function handleWeChatBind(): Promise<void> {
  if (!verifyCode.value.trim()) {
    codeError.value = t('auth.wechat.codeRequired')
    return
  }

  codeError.value = ''
  isLoading.value = true

  try {
    await wechatBind(verifyCode.value.trim())

    // Refresh user data to update wechat_openid
    await authStore.refreshUser()

    // Show success
    appStore.showSuccess(t('profile.wechatBind.bindSuccess'))

    // Close modal
    closeModal()
  } catch (error: unknown) {
    const err = error as { message?: string; response?: { data?: { detail?: string } } }

    if (err.response?.data?.detail) {
      codeError.value = err.response.data.detail
    } else if (err.message) {
      codeError.value = err.message
    } else {
      codeError.value = t('profile.wechatBind.bindFailed')
    }

    appStore.showError(codeError.value)
  } finally {
    isLoading.value = false
  }
}

async function handleWeChatUnbind(): Promise<void> {
  isUnbinding.value = true

  try {
    await wechatUnbind()

    // Refresh user data to update wechat_openid
    await authStore.refreshUser()

    // Show success
    appStore.showSuccess(t('profile.wechatBind.unbindSuccess'))

    // Close modal
    showUnbindConfirm.value = false
  } catch (error: unknown) {
    const err = error as { message?: string; response?: { data?: { detail?: string } } }

    let errorMessage = t('profile.wechatBind.unbindFailed')
    if (err.response?.data?.detail) {
      errorMessage = err.response.data.detail
    } else if (err.message) {
      errorMessage = err.message
    }

    appStore.showError(errorMessage)
  } finally {
    isUnbinding.value = false
  }
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
