<template>
  <div class="card">
    <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
        {{ t('checkIn.title') }}
      </h2>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        {{ t('checkIn.description') }}
      </p>
    </div>

    <div class="space-y-4 p-6">
      <div v-if="loading" class="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400">
        <div class="h-4 w-4 animate-spin rounded-full border-b-2 border-primary-600"></div>
        {{ t('common.loading') }}
      </div>

      <template v-else-if="status">
        <div
          v-if="!status.enabled"
          class="rounded-lg border border-gray-200 bg-gray-50 px-4 py-3 text-sm text-gray-600 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300"
        >
          {{ t('checkIn.disabled') }}
        </div>

        <template v-else>
          <div
            class="rounded-xl border border-primary-200 bg-primary-50 px-4 py-3 dark:border-primary-900/40 dark:bg-primary-900/20"
          >
            <div class="flex items-center justify-between gap-3">
              <div class="min-w-0">
                <p class="text-sm font-medium text-primary-900 dark:text-primary-200">
                  {{
                    status.checked_in_today
                      ? t('checkIn.checkedInToday')
                      : t('checkIn.readyToCheckIn', { date: status.check_in_date })
                  }}
                </p>
                <p class="mt-1 text-xs text-primary-700 dark:text-primary-300">
                  {{ t('checkIn.rewardAmount', { amount: formatCurrency(status.reward_amount) }) }}
                </p>
              </div>

              <button
                type="button"
                class="btn btn-primary btn-sm"
                :disabled="checkingIn || status.checked_in_today"
                @click="handleCheckIn"
              >
                <svg
                  v-if="checkingIn"
                  class="mr-1 h-4 w-4 animate-spin"
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
                {{
                  status.checked_in_today
                    ? t('checkIn.checkedInButton')
                    : checkingIn
                      ? t('checkIn.checkingIn')
                      : t('checkIn.checkInNow')
                }}
              </button>
            </div>
          </div>

          <div class="grid grid-cols-1 gap-3 text-sm sm:grid-cols-2">
            <div class="rounded-lg border border-gray-200 px-3 py-2 dark:border-dark-600">
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('checkIn.currentStreak') }}</p>
              <p class="mt-1 font-semibold text-gray-900 dark:text-white">{{ status.current_streak }}</p>
            </div>
            <div class="rounded-lg border border-gray-200 px-3 py-2 dark:border-dark-600">
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('checkIn.totalCheckIns') }}</p>
              <p class="mt-1 font-semibold text-gray-900 dark:text-white">{{ status.total_checkins }}</p>
            </div>
            <div class="rounded-lg border border-gray-200 px-3 py-2 dark:border-dark-600">
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('checkIn.timezone') }}</p>
              <p class="mt-1 truncate font-medium text-gray-900 dark:text-white">{{ status.timezone }}</p>
            </div>
            <div class="rounded-lg border border-gray-200 px-3 py-2 dark:border-dark-600">
              <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('checkIn.lastCheckInAt') }}</p>
              <p class="mt-1 font-medium text-gray-900 dark:text-white">
                {{ status.last_check_in_at ? formatDateTime(status.last_check_in_at) : '-' }}
              </p>
            </div>
          </div>

          <div v-if="status.history_visible" class="space-y-2 border-t border-gray-100 pt-4 dark:border-dark-700">
            <div class="flex items-center justify-between">
              <p class="text-sm font-medium text-gray-900 dark:text-white">
                {{ t('checkIn.historyTitle') }}
              </p>
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="loadingHistory"
                @click="loadHistory"
              >
                <Icon name="refresh" size="sm" :class="loadingHistory ? 'animate-spin' : ''" />
              </button>
            </div>

            <div
              v-if="loadingHistory"
              class="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400"
            >
              <div class="h-4 w-4 animate-spin rounded-full border-b-2 border-primary-600"></div>
              {{ t('common.loading') }}
            </div>

            <div
              v-else-if="history.length === 0"
              class="rounded-lg border border-dashed border-gray-300 px-3 py-2 text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400"
            >
              {{ t('checkIn.noHistory') }}
            </div>

            <ul v-else class="space-y-2">
              <li
                v-for="item in history.slice(0, 7)"
                :key="item.id"
                class="flex items-center justify-between rounded-lg border border-gray-100 px-3 py-2 text-sm dark:border-dark-700"
              >
                <div class="min-w-0">
                  <p class="font-medium text-gray-900 dark:text-white">{{ item.check_in_date }}</p>
                  <p class="text-xs text-gray-500 dark:text-gray-400">
                    {{ formatDateTime(item.checked_in_at) }}
                  </p>
                </div>
                <p class="text-sm font-semibold text-emerald-600 dark:text-emerald-400">
                  +{{ formatCurrency(item.reward_amount) }}
                </p>
              </li>
            </ul>
          </div>
        </template>
      </template>

      <div
        v-else
        class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/40 dark:bg-red-900/20 dark:text-red-300"
      >
        <p>{{ t('checkIn.loadFailed') }}</p>
        <button type="button" class="btn btn-secondary btn-sm mt-3" @click="loadStatus">
          {{ t('common.refresh') }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { checkInAPI } from '@/api'
import type { CheckInHistoryItem, CheckInStatus } from '@/types'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { formatCurrency, formatDateTime } from '@/utils/format'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const loading = ref(true)
const checkingIn = ref(false)
const loadingHistory = ref(false)
const status = ref<CheckInStatus | null>(null)
const history = ref<CheckInHistoryItem[]>([])

async function loadStatus() {
  loading.value = true
  try {
    status.value = await checkInAPI.getStatus()
    if (status.value.enabled && status.value.history_visible) {
      await loadHistory()
    } else {
      history.value = []
    }
  } catch (error: any) {
    console.error('Failed to load check-in status:', error)
    status.value = null
  } finally {
    loading.value = false
  }
}

async function loadHistory() {
  if (!status.value?.enabled || !status.value.history_visible) {
    history.value = []
    return
  }

  loadingHistory.value = true
  try {
    history.value = await checkInAPI.getHistory()
  } catch (error: any) {
    console.error('Failed to load check-in history:', error)
    history.value = []
  } finally {
    loadingHistory.value = false
  }
}

async function handleCheckIn() {
  if (!status.value || status.value.checked_in_today || checkingIn.value) {
    return
  }

  checkingIn.value = true
  try {
    const result = await checkInAPI.checkIn()

    if (result.already_checked_in) {
      appStore.showInfo(t('checkIn.alreadyCheckedIn'))
    } else {
      appStore.showSuccess(
        t('checkIn.checkInSuccess', { amount: formatCurrency(result.reward.amount) })
      )
    }

    await Promise.all([loadStatus(), authStore.refreshUser().catch(() => undefined)])
  } catch (error: any) {
    appStore.showError(error.message || t('checkIn.checkInFailed'))
  } finally {
    checkingIn.value = false
  }
}

onMounted(() => {
  loadStatus()
})
</script>
