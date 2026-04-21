<template>
  <div class="fixed inset-0 z-50 overflow-y-auto" @click.self="$emit('close')">
    <div class="flex min-h-full items-center justify-center p-4">
      <div class="fixed inset-0 bg-black/50 transition-opacity" @click="$emit('close')"></div>

      <div class="relative w-full max-w-md transform rounded-xl bg-white p-6 shadow-xl transition-all dark:bg-dark-800">
        <div class="mb-6">
          <div class="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-red-100 dark:bg-red-900/30">
            <svg class="h-6 w-6 text-red-600 dark:text-red-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
            </svg>
          </div>
          <h3 class="mt-4 text-center text-xl font-semibold text-gray-900 dark:text-white">
            {{ t('profile.passkey.revokeTitle') }}
          </h3>
          <p class="mt-2 text-center text-sm text-gray-500 dark:text-gray-400">
            {{ t('profile.passkey.revokeWarning', { name: passkey.friendly_name }) }}
          </p>
        </div>

        <div v-if="error" class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-700 dark:bg-red-900/30 dark:text-red-400">
          {{ error }}
        </div>

        <div class="flex justify-end gap-3 pt-4">
          <button type="button" class="btn btn-secondary" @click="$emit('close')">
            {{ t('common.cancel') }}
          </button>
          <button
            type="button"
            class="btn btn-danger"
            data-testid="passkey-revoke-confirm-button"
            :disabled="loading"
            @click="handleRevoke"
          >
            <span v-if="loading" class="mr-2 animate-spin rounded-full h-4 w-4 border-b-2 border-white"></span>
            {{ t('profile.passkey.confirmRevoke') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { authAPI } from '@/api'
import type { PasskeyCredentialSummary } from '@/types'

const props = defineProps<{
  passkey: PasskeyCredentialSummary
}>()

const emit = defineEmits<{
  close: []
  success: []
}>()

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const error = ref('')

const handleRevoke = async () => {
  loading.value = true
  error.value = ''

  try {
    await authAPI.revokePasskey(props.passkey.credential_id)
    appStore.showSuccess(t('profile.passkey.revokeSuccess'))
    emit('success')
  } catch (err: any) {
    if (err.response?.data?.code === 'RECENT_AUTH_REQUIRED') {
      error.value = t('profile.passkey.recentAuthRequiredHint')
    } else {
      error.value = err.response?.data?.message || t('profile.passkey.revokeFailed')
    }
  } finally {
    loading.value = false
  }
}
</script>
