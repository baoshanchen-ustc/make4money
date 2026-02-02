<template>
  <div class="space-y-4">
    <button type="button" :disabled="disabled" class="btn btn-secondary w-full" @click="showWeChatModal = true">
      <svg
        class="mr-2"
        viewBox="0 0 24 24"
        width="20"
        height="20"
        fill="currentColor"
        style="color: #07c160"
      >
        <path
          d="M8.691 2.188C3.891 2.188 0 5.476 0 9.53c0 2.212 1.17 4.203 3.002 5.55a.59.59 0 0 1 .213.665l-.39 1.48c-.019.07-.048.141-.048.213 0 .163.13.295.29.295a.32.32 0 0 0 .186-.057l2.019-1.179a.73.73 0 0 1 .59-.056c.797.263 1.66.404 2.554.404.186 0 .368-.008.55-.022-.096-.312-.148-.635-.148-.968 0-3.347 3.291-6.056 7.357-6.056.204 0 .405.011.603.029-.622-3.439-4.3-5.64-8.087-5.64zM5.785 5.991c.642 0 1.162.529 1.162 1.18 0 .651-.52 1.18-1.162 1.18-.642 0-1.162-.529-1.162-1.18 0-.651.52-1.18 1.162-1.18zm5.813 0c.642 0 1.162.529 1.162 1.18 0 .651-.52 1.18-1.162 1.18-.642 0-1.162-.529-1.162-1.18 0-.651.52-1.18 1.162-1.18z"
        />
        <path
          d="M23.8 14.617c0-3.185-3.148-5.767-7.03-5.767-3.883 0-7.031 2.582-7.031 5.767 0 3.186 3.148 5.768 7.03 5.768.718 0 1.408-.092 2.059-.264a.614.614 0 0 1 .5.048l1.63.955a.269.269 0 0 0 .158.048c.131 0 .243-.111.243-.247 0-.059-.024-.12-.04-.178l-.315-1.195a.505.505 0 0 1 .181-.566c1.48-1.082 2.415-2.686 2.415-4.37zm-9.433-1.18c-.538 0-.974-.443-.974-.99s.436-.99.974-.99c.537 0 .974.443.974.99s-.437.99-.974.99zm4.807 0c-.538 0-.974-.443-.974-.99s.436-.99.974-.99c.537 0 .973.443.973.99s-.436.99-.973.99z"
        />
      </svg>
      {{ t('auth.wechat.signIn') }}
    </button>

    <div class="flex items-center gap-3">
      <div class="h-px flex-1 bg-gray-200 dark:bg-dark-700"></div>
      <span class="text-xs text-gray-500 dark:text-dark-400">
        {{ t('auth.wechat.orContinue') }}
      </span>
      <div class="h-px flex-1 bg-gray-200 dark:bg-dark-700"></div>
    </div>

    <!-- WeChat Login Modal -->
    <Teleport to="body">
      <transition name="modal-fade">
        <div
          v-if="showWeChatModal"
          class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
          @click.self="closeModal"
        >
          <div
            class="w-full max-w-md rounded-2xl bg-white p-6 shadow-xl dark:bg-dark-800"
            @click.stop
          >
            <div class="flex items-center justify-between mb-4">
              <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
                {{ t('auth.wechat.modalTitle') }}
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
              <div class="rounded-lg border border-gray-200 bg-white p-2 dark:border-dark-600">
                <img
                  v-if="qrCodeUrl"
                  :src="qrCodeUrl"
                  :alt="t('auth.wechat.qrCodeAlt')"
                  class="h-48 w-48 object-contain"
                />
                <div
                  v-else
                  class="flex h-48 w-48 items-center justify-center text-gray-400"
                >
                  {{ t('auth.wechat.noQrCode') }}
                </div>
              </div>
              <p class="text-center text-sm text-gray-500 dark:text-dark-400">
                {{ accountType === 'unverified_official' ? t('auth.wechat.scanTipOfficial') : t('auth.wechat.scanTip') }}
              </p>
            </div>

            <!-- Verification Code Input -->
            <div class="mt-6 space-y-4">
              <div>
                <label for="wechat-code" class="input-label">
                  {{ t('auth.wechat.codeLabel') }}
                </label>
                <input
                  id="wechat-code"
                  v-model="verifyCode"
                  type="text"
                  :disabled="isLoading"
                  class="input text-center text-lg tracking-widest"
                  :class="{ 'input-error': codeError }"
                  :placeholder="t('auth.wechat.codePlaceholder')"
                  maxlength="6"
                  @keyup.enter="handleWeChatLogin"
                />
                <p v-if="codeError" class="input-error-text mt-1">
                  {{ codeError }}
                </p>
              </div>

              <button
                type="button"
                :disabled="isLoading || !verifyCode.trim()"
                class="btn btn-primary w-full"
                @click="handleWeChatLogin"
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
                {{ isLoading ? t('auth.wechat.verifying') : t('auth.wechat.verify') }}
              </button>
            </div>
          </div>
        </div>
      </transition>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import { wechatAuth } from '@/api/auth'

defineProps<{
  disabled?: boolean
  qrCodeUrl?: string
  accountType?: string
}>()

const emit = defineEmits<{
  (e: 'needEmailBind'): void
}>()

const router = useRouter()
const route = useRoute()
const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()

const showWeChatModal = ref(false)
const verifyCode = ref('')
const codeError = ref('')
const isLoading = ref(false)

function closeModal(): void {
  showWeChatModal.value = false
  verifyCode.value = ''
  codeError.value = ''
}

async function handleWeChatLogin(): Promise<void> {
  if (!verifyCode.value.trim()) {
    codeError.value = t('auth.wechat.codeRequired')
    return
  }

  codeError.value = ''
  isLoading.value = true

  try {
    await wechatAuth(verifyCode.value.trim())

    // Re-load auth state from localStorage (wechatAuth already saved token and user)
    await authStore.checkAuth()

    // Show success
    appStore.showSuccess(t('auth.loginSuccess'))

    // Close modal
    closeModal()

    // Check if user needs to bind email (WeChat login users with synthetic email)
    const user = authStore.user
    if (user && user.email.endsWith('@wechat-auth.invalid')) {
      // Emit event to show email bind modal
      emit('needEmailBind')
      return
    }

    // Redirect
    const redirectTo = (route.query.redirect as string) || '/dashboard'
    await router.push(redirectTo)
  } catch (error: unknown) {
    const err = error as { message?: string; response?: { data?: { detail?: string } } }

    if (err.response?.data?.detail) {
      codeError.value = err.response.data.detail
    } else if (err.message) {
      codeError.value = err.message
    } else {
      codeError.value = t('auth.wechat.verifyFailed')
    }

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
