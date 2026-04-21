<template>
  <div class="card">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <h2 class="text-lg font-medium text-gray-900 dark:text-white">
        {{ t('profile.passkey.title') }}
      </h2>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        {{ t('profile.passkey.description') }}
      </p>
    </div>
    <div class="px-6 py-6">
      <!-- Loading state -->
      <div v-if="loading" class="flex items-center justify-center py-8">
        <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-primary-500"></div>
      </div>

      <!-- Feature disabled globally -->
      <div v-else-if="status && !status.feature_enabled" class="flex items-center gap-4 py-4">
        <div class="flex-shrink-0 rounded-full bg-gray-100 p-3 dark:bg-dark-700">
          <svg class="h-6 w-6 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126zM12 15.75h.007v.008H12v-.008z" />
          </svg>
        </div>
        <div>
          <p class="font-medium text-gray-700 dark:text-gray-300">
            {{ t('profile.passkey.featureDisabled') }}
          </p>
          <p class="text-sm text-gray-500 dark:text-gray-400">
            {{ t('profile.passkey.featureDisabledHint') }}
          </p>
        </div>
      </div>

      <!-- Active Passkeys -->
      <div v-else>
        <!-- Recent Auth Required Banner -->
        <div v-if="status && !status.can_manage" class="flex items-center gap-4 py-4 mb-6 bg-yellow-50 dark:bg-yellow-900/20 rounded-lg px-4 border border-yellow-100 dark:border-yellow-900/30">
          <div class="flex-shrink-0 rounded-full bg-yellow-100 p-2 dark:bg-yellow-900/50">
            <svg class="h-5 w-5 text-yellow-600 dark:text-yellow-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
          </div>
          <div>
            <p class="font-medium text-yellow-800 dark:text-yellow-200 text-sm">
              {{ t('profile.passkey.recentAuthRequired') }}
            </p>
            <p class="text-xs text-yellow-700 dark:text-yellow-300 mt-0.5">
              {{ t('profile.passkey.recentAuthRequiredHint') }}
            </p>
          </div>
        </div>

        <div v-if="passkeys && passkeys.length > 0" class="space-y-4 mb-6">
          <div v-for="passkey in passkeys" :key="passkey.credential_id" class="flex items-center justify-between p-4 border border-gray-200 dark:border-dark-600 rounded-lg" data-testid="passkey-row">
            <div class="flex items-center gap-4">
              <div class="flex-shrink-0 rounded-full bg-primary-100 p-3 dark:bg-primary-900/30">
                <svg class="h-6 w-6 text-primary-600 dark:text-primary-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M15.75 5.25a3 3 0 013 3m3 0a6 6 0 01-7.029 5.912c-.563-.097-1.159.026-1.563.43L10.5 17.25H8.25v2.25H6v2.25H2.25v-2.818c0-.597.237-1.17.659-1.591l6.499-6.499c.404-.404.527-1 .43-1.563A6 6 0 1121.75 8.25z" />
                </svg>
              </div>
              <div>
                <div class="flex items-center gap-2">
                  <p class="font-medium text-gray-900 dark:text-white">
                    {{ passkey.friendly_name }}
                  </p>
                  <span v-if="passkey.synced" class="inline-flex items-center rounded-md bg-blue-50 px-2 py-1 text-xs font-medium text-blue-700 ring-1 ring-inset ring-blue-700/10 dark:bg-blue-900/30 dark:text-blue-400 dark:ring-blue-400/20">
                    {{ t('profile.passkey.synced') }}
                  </span>
                  <span v-else-if="passkey.backup_eligible" class="inline-flex items-center rounded-md bg-gray-50 px-2 py-1 text-xs font-medium text-gray-600 ring-1 ring-inset ring-gray-500/10 dark:bg-gray-800 dark:text-gray-400 dark:ring-gray-400/20">
                    {{ t('profile.passkey.backupEligible') }}
                  </span>
                </div>
                <p class="text-sm text-gray-500 dark:text-gray-400">
                  {{ t('profile.passkey.createdAt') }}: {{ formatDate(passkey.created_at) }}
                  <template v-if="passkey.last_used_at">
                    · {{ t('profile.passkey.lastUsedAt') }}: {{ formatDate(passkey.last_used_at) }}
                  </template>
                </p>
              </div>
            </div>
            <div class="flex items-center gap-2">
              <button
                type="button"
                class="btn btn-outline-secondary btn-sm"
                :disabled="!status?.can_manage"
                @click="openRenameDialog(passkey)"
              >
                {{ t('common.rename') }}
              </button>
              <button
                type="button"
                class="btn btn-outline-danger btn-sm"
                data-testid="passkey-revoke-button"
                :disabled="!status?.can_manage"
                @click="openRevokeDialog(passkey)"
              >
                {{ t('common.revoke') }}
              </button>
            </div>
          </div>
        </div>

        <div v-else class="flex items-center gap-4 py-4 mb-6">
          <div class="flex-shrink-0 rounded-full bg-gray-100 p-3 dark:bg-dark-700">
            <svg class="h-6 w-6 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M15.75 5.25a3 3 0 013 3m3 0a6 6 0 01-7.029 5.912c-.563-.097-1.159.026-1.563.43L10.5 17.25H8.25v2.25H6v2.25H2.25v-2.818c0-.597.237-1.17.659-1.591l6.499-6.499c.404-.404.527-1 .43-1.563A6 6 0 1121.75 8.25z" />
            </svg>
          </div>
          <div>
            <p class="font-medium text-gray-700 dark:text-gray-300">
              {{ t('profile.passkey.noPasskeys') }}
            </p>
            <p class="text-sm text-gray-500 dark:text-gray-400">
              {{ t('profile.passkey.noPasskeysHint') }}
            </p>
          </div>
        </div>

        <div class="flex justify-end">
          <button
            type="button"
            class="btn btn-primary"
            data-testid="passkey-enroll-button"
            :disabled="enrolling || !status?.can_manage"
            @click="startEnrollment"
          >
            <span v-if="enrolling" class="mr-2 animate-spin rounded-full h-4 w-4 border-b-2 border-white"></span>
            {{ t('profile.passkey.addPasskey') }}
          </button>
        </div>
      </div>
    </div>

    <!-- Rename Dialog -->
    <PasskeyRenameDialog
      v-if="showRenameDialog && selectedPasskey"
      :passkey="selectedPasskey"
      @close="showRenameDialog = false"
      @success="handleRenameSuccess"
    />

    <!-- Revoke Dialog -->
    <PasskeyRevokeDialog
      v-if="showRevokeDialog && selectedPasskey"
      :passkey="selectedPasskey"
      @close="showRevokeDialog = false"
      @success="handleRevokeSuccess"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { startRegistration } from '@simplewebauthn/browser'
import type { PasskeyCredentialSummary, PasskeyRegistrationCredentialJSON } from '@/types'
import PasskeyRenameDialog from './PasskeyRenameDialog.vue'
import PasskeyRevokeDialog from './PasskeyRevokeDialog.vue'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()

const loading = ref(true)
const enrolling = ref(false)
const showRenameDialog = ref(false)
const showRevokeDialog = ref(false)
const selectedPasskey = ref<PasskeyCredentialSummary | null>(null)

const status = computed(() => authStore.passkeyStatus)
const passkeys = computed(() => authStore.passkeys)

const loadData = async () => {
  loading.value = true
  try {
    await authStore.refreshPasskeyManagement()
  } catch (error) {
    console.error('Failed to load passkey management data:', error)
  } finally {
    loading.value = false
  }
}

const startEnrollment = async () => {
  enrolling.value = true
  try {
    const beginResponse = await authStore.beginPasskeyEnrollment()
    
    let credential
    try {
      credential = await startRegistration({ optionsJSON: beginResponse.options.publicKey as any })
    } catch (error: any) {
      if (error.name === 'NotAllowedError' || error.name === 'AbortError') {
        return
      }
      throw error
    }

    await authStore.finishPasskeyEnrollment(credential as unknown as PasskeyRegistrationCredentialJSON)
    appStore.showSuccess(t('profile.passkey.enrollSuccess'))
  } catch (error: any) {
    console.error('Passkey enrollment failed:', error)
    if (error.response?.data?.code === 'RECENT_AUTH_REQUIRED') {
      appStore.showError(t('profile.passkey.recentAuthRequiredHint'))
    } else {
      appStore.showError(error.response?.data?.message || t('profile.passkey.enrollFailed'))
    }
  } finally {
    enrolling.value = false
  }
}

const openRenameDialog = (passkey: PasskeyCredentialSummary) => {
  selectedPasskey.value = passkey
  showRenameDialog.value = true
}

const openRevokeDialog = (passkey: PasskeyCredentialSummary) => {
  selectedPasskey.value = passkey
  showRevokeDialog.value = true
}

const handleRenameSuccess = () => {
  showRenameDialog.value = false
  loadData()
}

const handleRevokeSuccess = () => {
  showRevokeDialog.value = false
  loadData()
}

const formatDate = (timestamp: number) => {
  const date = new Date(timestamp * 1000)
  return date.toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit'
  })
}

onMounted(() => {
  loadData()
})
</script>
