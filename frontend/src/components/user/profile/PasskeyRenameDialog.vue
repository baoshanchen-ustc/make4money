<template>
  <div class="fixed inset-0 z-50 overflow-y-auto" @click.self="$emit('close')">
    <div class="flex min-h-full items-center justify-center p-4">
      <div class="fixed inset-0 bg-black/50 transition-opacity" @click="$emit('close')"></div>

      <div class="relative w-full max-w-md transform rounded-xl bg-white p-6 shadow-xl transition-all dark:bg-dark-800">
        <div class="mb-6">
          <h3 class="text-lg font-medium text-gray-900 dark:text-white">
            {{ t('profile.passkey.renameTitle') }}
          </h3>
        </div>

        <form @submit.prevent="handleRename" class="space-y-4">
          <div>
            <label for="friendlyName" class="input-label">
              {{ t('profile.passkey.friendlyName') }}
            </label>
            <input
              id="friendlyName"
              v-model="friendlyName"
              type="text"
              class="input"
              data-testid="passkey-rename-input"
              :placeholder="t('profile.passkey.friendlyNamePlaceholder')"
              required
              autofocus
            />
          </div>

          <div v-if="error" class="rounded-lg bg-red-50 p-3 text-sm text-red-700 dark:bg-red-900/30 dark:text-red-400">
            {{ error }}
          </div>

          <div class="flex justify-end gap-3 pt-4">
            <button type="button" class="btn btn-secondary" @click="$emit('close')">
              {{ t('common.cancel') }}
            </button>
            <button
              type="submit"
              class="btn btn-primary"
              data-testid="passkey-rename-confirm-button"
              :disabled="loading || !friendlyName.trim() || friendlyName.trim() === passkey.friendly_name"
            >
              <span v-if="loading" class="mr-2 animate-spin rounded-full h-4 w-4 border-b-2 border-white"></span>
              {{ t('common.save') }}
            </button>
          </div>
        </form>
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

const friendlyName = ref(props.passkey.friendly_name)
const loading = ref(false)
const error = ref('')

const handleRename = async () => {
  const newName = friendlyName.value.trim()
  if (!newName || newName === props.passkey.friendly_name) return

  loading.value = true
  error.value = ''

  try {
    await authAPI.renamePasskey(props.passkey.credential_id, newName)
    appStore.showSuccess(t('profile.passkey.renameSuccess'))
    emit('success')
  } catch (err: any) {
    if (err.response?.data?.code === 'RECENT_AUTH_REQUIRED') {
      error.value = t('profile.passkey.recentAuthRequiredHint')
    } else {
      error.value = err.response?.data?.message || t('profile.passkey.renameFailed')
    }
  } finally {
    loading.value = false
  }
}
</script>
