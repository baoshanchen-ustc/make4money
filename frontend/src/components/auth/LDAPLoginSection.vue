<template>
  <div class="space-y-4">
    <!-- LDAP 登录表单 -->
    <form @submit.prevent="handleLDAPLogin" class="space-y-4">
      <!-- 用户名输入框 -->
      <div>
        <label for="ldap-username" class="input-label">
          {{ t('auth.ldap.usernameLabel') }}
        </label>
        <div class="relative">
          <div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5">
            <Icon name="user" size="md" class="text-gray-400 dark:text-dark-500" />
          </div>
          <input
            id="ldap-username"
            v-model="formData.username"
            type="text"
            required
            autocomplete="username"
            :disabled="disabled || isLoading"
            class="input pl-11"
            :class="{ 'input-error': errors.username }"
            :placeholder="t('auth.ldap.usernamePlaceholder')"
          />
        </div>
        <p v-if="errors.username" class="input-error-text">
          {{ errors.username }}
        </p>
      </div>

      <!-- 密码输入框 -->
      <div>
        <label for="ldap-password" class="input-label">
          {{ t('auth.ldap.passwordLabel') }}
        </label>
        <div class="relative">
          <div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5">
            <Icon name="lock" size="md" class="text-gray-400 dark:text-dark-500" />
          </div>
          <input
            id="ldap-password"
            v-model="formData.password"
            :type="showPassword ? 'text' : 'password'"
            required
            autocomplete="current-password"
            :disabled="disabled || isLoading"
            class="input pl-11 pr-11"
            :class="{ 'input-error': errors.password }"
            :placeholder="t('auth.ldap.passwordPlaceholder')"
          />
          <button
            type="button"
            @click="showPassword = !showPassword"
            class="absolute inset-y-0 right-0 flex items-center pr-3.5 text-gray-400 transition-colors hover:text-gray-600 dark:hover:text-dark-300"
          >
            <Icon v-if="showPassword" name="eyeOff" size="md" />
            <Icon v-else name="eye" size="md" />
          </button>
        </div>
        <p v-if="errors.password" class="input-error-text">
          {{ errors.password }}
        </p>
      </div>

      <!-- 错误消息 -->
      <transition name="fade">
        <div
          v-if="errorMessage"
          class="rounded-xl border border-red-200 bg-red-50 p-4 dark:border-red-800/50 dark:bg-red-900/20"
        >
          <div class="flex items-start gap-3">
            <div class="flex-shrink-0">
              <Icon name="exclamationCircle" size="md" class="text-red-500" />
            </div>
            <p class="text-sm text-red-700 dark:text-red-400">
              {{ errorMessage }}
            </p>
          </div>
        </div>
      </transition>

      <!-- LDAP 登录按钮 -->
      <button
        type="submit"
        :disabled="disabled || isLoading"
        class="btn btn-secondary w-full"
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
        <Icon v-else name="login" size="md" class="mr-2" />
        {{ isLoading ? t('auth.ldap.signingIn') : t('auth.ldap.signIn') }}
      </button>
    </form>

    <!-- 分隔线 -->
    <div class="flex items-center gap-3">
      <div class="h-px flex-1 bg-gray-200 dark:bg-dark-700"></div>
      <span class="text-xs text-gray-500 dark:text-dark-400">
        {{ t('auth.ldap.orContinue') }}
      </span>
      <div class="h-px flex-1 bg-gray-200 dark:bg-dark-700"></div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore, useAuthStore } from '@/stores'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()

defineProps<{
  disabled?: boolean
}>()

// 状态
const isLoading = ref<boolean>(false)
const errorMessage = ref<string>('')
const showPassword = ref<boolean>(false)

const formData = reactive({
  username: '',
  password: ''
})

const errors = reactive({
  username: '',
  password: ''
})

// 表单验证
function validateForm(): boolean {
  errors.username = ''
  errors.password = ''

  let isValid = true

  if (!formData.username.trim()) {
    errors.username = t('auth.ldap.usernameRequired')
    isValid = false
  }

  if (!formData.password) {
    errors.password = t('auth.ldap.passwordRequired')
    isValid = false
  }

  return isValid
}

// LDAP 登录处理
async function handleLDAPLogin(): Promise<void> {
  errorMessage.value = ''

  if (!validateForm()) {
    return
  }

  isLoading.value = true

  try {
    await authStore.ldapLogin({
      username: formData.username,
      password: formData.password
    })

    appStore.showSuccess(t('auth.ldap.loginSuccess'))

    const redirectTo = (router.currentRoute.value.query.redirect as string) || '/dashboard'
    await router.push(redirectTo)
  } catch (error: unknown) {
    const err = error as { message?: string; response?: { data?: { detail?: string } } }

    if (err.response?.data?.detail) {
      errorMessage.value = err.response.data.detail
    } else if (err.message) {
      errorMessage.value = err.message
    } else {
      errorMessage.value = t('auth.ldap.loginFailed')
    }

    appStore.showError(errorMessage.value)
  } finally {
    isLoading.value = false
  }
}
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
