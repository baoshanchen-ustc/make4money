<template>
  <div class="card overflow-hidden">
    <div
      class="border-b border-gray-100 bg-gradient-to-r from-primary-500/10 to-primary-600/5 px-6 py-5 dark:border-dark-700 dark:from-primary-500/20 dark:to-primary-600/10"
    >
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
        {{ t('profile.bindings.title') }}
      </h2>
      <p class="mt-1 text-sm text-gray-600 dark:text-gray-400">
        {{ t('profile.bindings.description') }}
      </p>
    </div>

    <div class="divide-y divide-gray-100 dark:divide-dark-700">
      <div
        v-for="binding in bindings"
        :key="binding.provider"
        class="flex flex-col gap-4 px-6 py-5 sm:flex-row sm:items-center sm:justify-between"
      >
        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-3">
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ t(`profile.bindings.providers.${binding.provider}`) }}
            </h3>
            <span :class="binding.bound ? 'badge badge-success' : 'badge badge-gray'">
              {{
                binding.bound
                  ? t('profile.bindings.status.bound')
                  : t('profile.bindings.status.notBound')
              }}
            </span>
          </div>
          <p class="mt-1 text-sm text-gray-600 dark:text-gray-400">
            {{ t(`profile.bindings.providerDescriptions.${binding.provider}`) }}
          </p>
          <p v-if="binding.value" class="mt-2 truncate text-sm font-medium text-gray-900 dark:text-white">
            {{ binding.value }}
          </p>
          <p v-else class="mt-2 text-sm text-gray-500 dark:text-gray-500">
            {{ t(`profile.bindings.providerHints.${binding.provider}`) }}
          </p>
          <p
            v-if="binding.availabilityHintKey"
            class="mt-2 text-sm text-gray-500 dark:text-gray-400"
          >
            {{ t(binding.availabilityHintKey) }}
          </p>
          <div
            v-if="binding.provider === 'email' && !binding.bound"
            class="mt-4 grid gap-3 sm:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)]"
          >
            <input
              v-model.trim="emailBindingForm.email"
              type="email"
              class="input"
              :placeholder="t('profile.balanceNotify.emailPlaceholder')"
              :disabled="isSendingEmailCode || isBindingEmail"
            />
            <button
              type="button"
              class="btn btn-secondary"
              :disabled="isSendingEmailCode || isBindingEmail || (turnstileEnabled && !turnstileToken)"
              @click="handleSendEmailCode"
            >
              {{ isSendingEmailCode ? t('auth.sendingCode') : t('profile.balanceNotify.sendCode') }}
            </button>
            <input
              v-model.trim="emailBindingForm.verifyCode"
              type="text"
              inputmode="numeric"
              maxlength="6"
              class="input"
              :placeholder="t('profile.balanceNotify.codePlaceholder')"
              :disabled="isBindingEmail"
            />
            <input
              v-model="emailBindingForm.password"
              type="password"
              class="input"
              :placeholder="t('auth.passwordLabel')"
              :disabled="isBindingEmail"
            />
            <button
              type="button"
              class="btn btn-primary sm:col-span-2"
              :disabled="isBindingEmail"
              @click="handleBindEmail"
            >
              {{ isBindingEmail ? t('auth.verifying') : t('profile.balanceNotify.verify') }}
            </button>
            <div v-if="turnstileEnabled && turnstileSiteKey" class="sm:col-span-2">
              <TurnstileWidget
                ref="turnstileRef"
                :site-key="turnstileSiteKey"
                @verify="onTurnstileVerify"
                @expire="onTurnstileExpire"
                @error="onTurnstileError"
              />
              <p v-if="turnstileError" class="input-error-text mt-2">
                {{ turnstileError }}
              </p>
            </div>
          </div>
        </div>

        <div class="flex shrink-0 items-center gap-3">
          <a
            v-if="binding.connectUrl && !binding.bound"
            :href="binding.connectUrl"
            class="btn btn-secondary"
            @click.prevent="handleConnect(binding.connectUrl)"
          >
            {{ t('profile.bindings.actions.connect') }}
          </a>
          <button
            v-else-if="binding.connectDisabled"
            type="button"
            class="btn btn-secondary cursor-not-allowed opacity-60"
            disabled
          >
            {{ t('profile.bindings.actions.connect') }}
          </button>
          <button
            v-else-if="binding.bound && binding.canDisconnect"
            type="button"
            class="btn btn-secondary"
            :disabled="disconnectingProvider === binding.provider"
            @click="handleDisconnect(binding.provider)"
          >
            {{
              disconnectingProvider === binding.provider
                ? t('auth.verifying')
                : t('profile.bindings.actions.disconnect')
            }}
          </button>
          <span
            v-else-if="binding.bound"
            class="text-sm font-medium text-green-600 dark:text-green-400"
          >
            {{ t('profile.bindings.actions.connected') }}
          </span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { prepareOAuthBindAccessTokenCookie, sendVerifyCode } from '@/api/auth'
import TurnstileWidget from '@/components/TurnstileWidget.vue'
import { userAPI } from '@/api/user'
import {
  resolveUserBinding,
  resolveUserBindings,
  type UserAccountBindingProvider
} from '@/components/user/profile/profileUser'
import { useAppStore, useAuthStore } from '@/stores'

const props = defineProps<{
  user: Record<string, unknown> | null
}>()

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const emailBindingForm = reactive({
  email: '',
  verifyCode: '',
  password: ''
})
const isSendingEmailCode = ref(false)
const isBindingEmail = ref(false)
const disconnectingProvider = ref<UserAccountBindingProvider | null>(null)
const turnstileRef = ref<InstanceType<typeof TurnstileWidget> | null>(null)
const turnstileToken = ref('')
const turnstileError = ref('')

const userRecord = computed(() => (props.user ?? {}) as Record<string, unknown>)
const emailBinding = computed(() => resolveUserBinding(userRecord.value, 'email'))
const publicSettings = computed(() => appStore.cachedPublicSettings ?? null)
const turnstileEnabled = computed(() => publicSettings.value?.turnstile_enabled === true)
const turnstileSiteKey = computed(() => publicSettings.value?.turnstile_site_key ?? '')

watch(emailBinding, (binding) => {
  if (binding.bound || emailBindingForm.email) {
    return
  }
  emailBindingForm.email = binding.value
}, { immediate: true })

const bindings = computed(() => resolveUserBindings(userRecord.value))

function validateEmailBindingForm(requireCode: boolean): boolean {
  if (!emailBindingForm.email) {
    appStore.showError(t('auth.emailRequired'))
    return false
  }
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(emailBindingForm.email)) {
    appStore.showError(t('auth.invalidEmail'))
    return false
  }
  if (requireCode && !emailBindingForm.verifyCode) {
    appStore.showError(t('auth.codeRequired'))
    return false
  }
  if (requireCode && !emailBindingForm.password) {
    appStore.showError(t('auth.passwordRequired'))
    return false
  }
  if (requireCode && emailBindingForm.password.length < 6) {
    appStore.showError(t('auth.passwordMinLength'))
    return false
  }
  return true
}

async function handleSendEmailCode() {
  if (!validateEmailBindingForm(false)) {
    return
  }

  isSendingEmailCode.value = true
  try {
    await sendVerifyCode({
      email: emailBindingForm.email,
      turnstile_token: turnstileEnabled.value ? turnstileToken.value : undefined
    })
    appStore.showSuccess(t('profile.balanceNotify.codeSentTo', { email: emailBindingForm.email }))
    resetTurnstile()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || t('auth.sendCodeFailed'))
    resetTurnstile()
  } finally {
    isSendingEmailCode.value = false
  }
}

async function handleBindEmail() {
  if (!validateEmailBindingForm(true)) {
    return
  }

  isBindingEmail.value = true
  try {
    await userAPI.bindAccount('email', {
      email: emailBindingForm.email,
      verifyCode: emailBindingForm.verifyCode,
      password: emailBindingForm.password
    })
    emailBindingForm.verifyCode = ''
    emailBindingForm.password = ''
    await authStore.refreshUser()
    appStore.showSuccess(t('profile.balanceNotify.verifySuccess'))
  } catch (error) {
    appStore.showError((error as { message?: string }).message || t('auth.verifyFailed'))
  } finally {
    isBindingEmail.value = false
  }
}

function handleConnect(connectUrl: string) {
  prepareOAuthBindAccessTokenCookie()
  window.location.href = connectUrl
}

function onTurnstileVerify(token: string) {
  turnstileToken.value = token
  turnstileError.value = ''
}

function onTurnstileExpire() {
  turnstileToken.value = ''
  turnstileError.value = t('auth.turnstileExpired')
}

function onTurnstileError() {
  turnstileToken.value = ''
  turnstileError.value = t('auth.turnstileFailed')
}

function resetTurnstile() {
  if (!turnstileEnabled.value) {
    return
  }
  turnstileRef.value?.reset()
  turnstileToken.value = ''
}

async function handleDisconnect(provider: UserAccountBindingProvider) {
  if (provider === 'email') {
    return
  }
  if (!window.confirm(t('profile.bindings.disconnectConfirm'))) {
    return
  }

  disconnectingProvider.value = provider
  try {
    await userAPI.unbindAccount(provider)
    await authStore.refreshUser()
    appStore.showSuccess(t('profile.bindings.disconnectSuccess'))
  } catch (error) {
    appStore.showError((error as { message?: string }).message || t('profile.bindings.disconnectFailed'))
  } finally {
    disconnectingProvider.value = null
  }
}
</script>
