<template>
  <div
    class="rounded-lg border bg-white p-4 shadow-sm dark:bg-gray-800"
    :class="borderClass"
  >
    <div class="flex items-start justify-between gap-2">
      <div class="min-w-0">
        <p class="truncate text-sm font-semibold text-gray-900 dark:text-white">{{ account.name }}</p>
        <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
          {{ account.plan_type || '—' }}
          <span v-if="account.seat_count > 0" class="ml-1">· {{ account.seat_count }} {{ t('admin.copilot.accounts.seats') }}</span>
        </p>
      </div>
      <span
        class="shrink-0 rounded-full px-2 py-0.5 text-xs font-medium"
        :class="statusBadgeClass"
      >
        {{ statusLabel }}
      </span>
    </div>

    <!-- Quota bar -->
    <div class="mt-3">
      <div class="mb-1 flex justify-between text-xs text-gray-500 dark:text-gray-400">
        <span>{{ t('admin.copilot.accounts.premiumUsed') }}</span>
        <span v-if="account.quota_snapshot">
          {{ premiumUsed }} / {{ premiumTotal }}
        </span>
        <span v-else>—</span>
      </div>
      <div class="h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-gray-700">
        <div
          class="h-full rounded-full transition-all duration-500"
          :class="barClass"
          :style="{ width: `${usagePercent}%` }"
        />
      </div>
    </div>

    <!-- Monthly cost -->
    <div class="mt-3 flex items-end justify-between">
      <div>
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.copilot.accounts.monthlyCost') }}</p>
        <p class="text-lg font-bold text-gray-900 dark:text-white">${{ account.monthly_cost.toFixed(0) }}</p>
      </div>
      <div class="text-right">
        <p class="text-xs text-gray-500 dark:text-gray-400">今日 / 本月</p>
        <p class="text-xs font-medium text-gray-700 dark:text-gray-300">
          {{ account.system_today_premium_requests }} / {{ account.system_month_premium_requests }}
        </p>
      </div>
    </div>

    <!-- Budget alert info -->
    <div v-if="account.budget_alert?.enabled" class="mt-2 border-t border-gray-100 pt-2 dark:border-gray-700">
      <p class="text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.copilot.accounts.budgetAlert') }}:
        <span class="font-medium text-gray-700 dark:text-gray-300">${{ account.budget_alert.monthly_budget }}</span>
        · {{ account.budget_alert.alert_threshold }}% 阈值
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { CopilotAccountOverviewEntry } from '@/api/admin/copilotAnalytics'

const { t } = useI18n()

const props = defineProps<{
  account: CopilotAccountOverviewEntry
}>()

const premiumUsed = computed(() => {
  const snap = props.account.quota_snapshot
  if (!snap) return 0
  // github_total_used is the authoritative total from GitHub API.
  // external_used is a derived sub-component (github_total_used - system usage),
  // so we must NOT add them together.
  return snap.github_total_used
})

const premiumTotal = computed(() => {
  const snap = props.account.quota_snapshot
  if (!snap) return 0
  return snap.unlimited ? '∞' : snap.entitlement
})

const usagePercent = computed(() => {
  const snap = props.account.quota_snapshot
  if (!snap || snap.unlimited || snap.entitlement === 0) return 0
  return Math.min(100, Math.round((snap.github_total_used / snap.entitlement) * 100))
})

const statusBadgeClass = computed(() => {
  switch (props.account.alert_status) {
    case 'critical': return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
    case 'warning':  return 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400'
    default:         return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
  }
})

const borderClass = computed(() => {
  switch (props.account.alert_status) {
    case 'critical': return 'border-red-300 dark:border-red-700'
    case 'warning':  return 'border-yellow-300 dark:border-yellow-700'
    default:         return 'border-gray-200 dark:border-gray-700'
  }
})

const barClass = computed(() => {
  if (usagePercent.value >= 95) return 'bg-red-500'
  if (usagePercent.value >= 70) return 'bg-yellow-500'
  return 'bg-green-500'
})

const statusLabel = computed(() => {
  switch (props.account.alert_status) {
    case 'critical': return 'Critical'
    case 'warning':  return 'Warning'
    default:         return 'OK'
  }
})
</script>
