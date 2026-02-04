<template>
  <Teleport to="body">
    <transition name="modal-fade">
      <div
        v-if="show"
        class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
        @click.self="handleCancel"
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
                  {{ t('auth.wechatBind.title') }}
                </h3>
              </div>
            </div>
            <button
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
              {{ t('auth.wechatBind.description') }}
            </p>
            <ul class="space-y-2 text-sm text-gray-600 dark:text-dark-400">
              <li class="flex items-start gap-2">
                <svg class="h-5 w-5 flex-shrink-0 text-green-500 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                </svg>
                <span>{{ t('auth.wechatBind.benefit1') }}</span>
              </li>
              <li class="flex items-start gap-2">
                <svg class="h-5 w-5 flex-shrink-0 text-green-500 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                </svg>
                <span>{{ t('auth.wechatBind.benefit2') }}</span>
              </li>
              <li class="flex items-start gap-2">
                <svg class="h-5 w-5 flex-shrink-0 text-green-500 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
                </svg>
                <span>{{ t('auth.wechatBind.benefit3') }}</span>
              </li>
            </ul>
          </div>

          <!-- QR Code -->
          <div class="mb-6 flex flex-col items-center space-y-4">
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
                {{ t('auth.wechatBind.success') }}
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
          <p v-if="scanError && isSubscriptionMode" class="mb-4 text-center text-sm text-red-500">
            {{ scanError }}
          </p>

          <!-- Verification Code Input (only for unverified_official mode) -->
          <div v-if="!isSubscriptionMode" class="mb-6 space-y-4">
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
                @keyup.enter="handleBind"
              />
              <p v-if="codeError" class="input-error-text mt-1">
                {{ codeError }}
              </p>
            </div>

            <div class="flex gap-3">
              <button
                type="button"
                :disabled="isLoading"
                class="btn btn-secondary flex-1"
                @click="handleSkip"
              >
                {{ t('auth.wechatBind.skip') }}
              </button>
              <button
                type="button"
                :disabled="isLoading || !verifyCode.trim()"
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
                {{ isLoading ? t('auth.wechat.verifying') : t('auth.wechatBind.bind') }}
              </button>
            </div>
          </div>

          <!-- Skip button for subscription mode -->
          <div v-if="isSubscriptionMode" class="flex gap-3">
            <button
              type="button"
              class="btn btn-secondary flex-1"
              @click="handleSkip"
            >
              {{ t('auth.wechatBind.skip') }}
            </button>
          </div>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, watch, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore, useAuthStore } from '@/stores'
import { wechatBind, wechatScanInit, wechatScanBindPoll } from '@/api/auth'

const props = defineProps<{
  show: boolean
  qrCodeUrl?: string
  accountType?: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'skip'): void
  (e: 'success'): void
}>()

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const verifyCode = ref('')
const codeError = ref('')
const isLoading = ref(false)

// Scan bind state (for subscription mode)
const scanSceneId = ref('')
const scanQrCodeUrl = ref('')
const scanStatus = ref<'waiting' | 'confirmed'>('waiting')
const scanExpired = ref(false)
const scanLoading = ref(false)
const scanError = ref('')
let pollTimer: ReturnType<typeof setInterval> | null = null
let expireTimer: ReturnType<typeof setTimeout> | null = null

const isSubscriptionMode = computed(() => props.accountType === 'subscription')

// Display either the dynamic scan QR code or the static prop QR code
const displayQrCodeUrl = computed(() => {
  if (isSubscriptionMode.value && scanQrCodeUrl.value) {
    return scanQrCodeUrl.value
  }
  return props.qrCodeUrl
})

// Watch for modal open to initialize scan bind
watch(() => props.show, (newVal) => {
  if (newVal && isSubscriptionMode.value) {
    initScanBind()
  }
  if (!newVal) {
    resetState()
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

function resetState(): void {
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

function handleCancel(): void {
  emit('close')
}

function handleSkip(): void {
  emit('skip')
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
    const err = error as { message?: string }
    scanError.value = err.message || t('auth.wechat.scanError')
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

        // Refresh user info
        await authStore.checkAuth()

        // Show success
        appStore.showSuccess(t('auth.wechatBind.success'))

        // Emit success event after a short delay
        setTimeout(() => {
          emit('success')
        }, 500)
      }
    } catch (error: unknown) {
      const err = error as { status?: number; message?: string }
      // If it's a conflict error (already bound), show error and stop polling
      if (err.status === 409) {
        scanError.value = err.message || t('auth.wechatBind.bindFailed')
        appStore.showError(scanError.value)
        clearTimers()
      }
      // Ignore other polling errors, will retry
    }
  }, 2000)
}

async function handleBind(): Promise<void> {
  if (!verifyCode.value.trim()) {
    codeError.value = t('auth.wechat.codeRequired')
    return
  }

  codeError.value = ''
  isLoading.value = true

  try {
    await wechatBind(verifyCode.value.trim())

    // Refresh user info
    await authStore.checkAuth()

    // Show success
    appStore.showSuccess(t('auth.wechatBind.success'))

    // Emit success event
    emit('success')
  } catch (error: unknown) {
    const err = error as { message?: string }

    codeError.value = err.message || t('auth.wechatBind.bindFailed')
    appStore.showError(codeError.value)
  } finally {
    isLoading.value = false
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
