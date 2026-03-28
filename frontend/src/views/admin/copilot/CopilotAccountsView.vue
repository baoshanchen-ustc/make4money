<template>
  <AppLayout>
    <div class="space-y-6">
    <!-- Header -->
    <div class="mb-6">
      <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
        {{ t('admin.copilot.accounts.title') }}
      </h1>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.copilot.accounts.description') }}
      </p>
    </div>

    <!-- Summary cards -->
    <div class="mb-6 grid grid-cols-1 gap-4 sm:grid-cols-4">
      <SummaryCard
        title="总估算月费用"
        :value="overview?.estimated_monthly_cost ?? 0"
        :loading="loading"
        color="green"
      />
      <SummaryCard
        title="今日 Premium 请求"
        :value="overview?.today_premium_requests ?? 0"
        :loading="loading"
        color="blue"
      />
      <SummaryCard
        title="Copilot 账户数"
        :value="overview?.total_accounts ?? 0"
        :loading="loading"
        color="purple"
      />
      <SummaryCard
        title="告警账户数"
        :value="overview?.alert_count ?? 0"
        :loading="loading"
        color="red"
      />
    </div>

    <!-- Error state -->
    <div v-if="error" class="mb-6 rounded-md bg-red-50 p-4 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-400">
      {{ error }}
    </div>

    <!-- Toast -->
    <Transition name="fade">
      <div
        v-if="toast"
        class="fixed bottom-6 right-6 z-50 rounded-md px-4 py-3 text-sm shadow-lg"
        :class="toast.type === 'success'
          ? 'bg-green-600 text-white'
          : 'bg-red-600 text-white'"
      >
        {{ toast.msg }}
      </div>
    </Transition>

    <!-- Loading -->
    <div v-if="loading" class="flex h-32 items-center justify-center">
      <LoadingSpinner />
    </div>

    <!-- No data -->
    <div
      v-else-if="!overview || overview.accounts.length === 0"
      class="flex h-32 items-center justify-center rounded-lg border border-gray-200 text-sm text-gray-500 dark:border-gray-700 dark:text-gray-400"
    >
      {{ t('admin.copilot.accounts.noData') }}
    </div>

    <!-- Account cards grid -->
    <div v-else class="space-y-6">
      <template v-for="account in overview.accounts" :key="account.account_id">
        <div class="rounded-lg border border-gray-200 bg-white shadow-sm dark:border-gray-700 dark:bg-gray-800">
          <!-- Card header row -->
          <div class="flex flex-wrap items-start gap-4 p-4">
            <!-- Account cost card -->
            <div class="min-w-[260px] flex-1">
              <AccountCostCard :account="account" />
            </div>

            <!-- Actions -->
            <div class="flex shrink-0 flex-col gap-2">
              <button
                :disabled="refreshing.has(account.account_id)"
                class="rounded-md border border-gray-300 px-3 py-1.5 text-xs font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50 dark:border-gray-600 dark:text-gray-300 dark:hover:bg-gray-700"
                @click="doRefresh(account)"
              >
                {{ refreshing.has(account.account_id)
                  ? t('admin.copilot.accounts.refreshing')
                  : t('admin.copilot.accounts.refreshQuota') }}
              </button>
              <button
                class="rounded-md border border-blue-300 px-3 py-1.5 text-xs font-medium text-blue-700 hover:bg-blue-50 dark:border-blue-600 dark:text-blue-400 dark:hover:bg-blue-900/20"
                @click="openBudget(account)"
              >
                {{ t('admin.copilot.accounts.setBudget') }}
              </button>
              <button
                class="text-xs text-gray-500 underline hover:text-gray-700 dark:text-gray-400"
                @click="toggleTrend(account.account_id)"
              >
                {{ trendExpanded.has(account.account_id) ? '▲ 收起趋势' : '▼ 查看趋势' }}
              </button>
            </div>
          </div>

          <!-- Quota trend chart (expandable) -->
          <div v-if="trendExpanded.has(account.account_id)" class="border-t border-gray-100 p-4 dark:border-gray-700">
            <p class="mb-2 text-xs font-medium text-gray-500 dark:text-gray-400">配额趋势（近 30 天）</p>
            <QuotaTrendChart :account-id="account.account_id" :days="30" />
          </div>
        </div>
      </template>
    </div>

    <!-- Budget alert dialog -->
    <BudgetAlertDialog
      :visible="budgetDialog.visible"
      :account-id="budgetDialog.accountId"
      :initial="budgetDialog.initial"
      @close="budgetDialog.visible = false"
      @saved="onBudgetSaved"
      @error="onBudgetError"
    />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, onMounted, reactive } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  getCopilotAccountsOverview,
  refreshCopilotAccountQuota,
} from '@/api/admin/copilotAnalytics'
import type {
  CopilotAccountsOverviewResult,
  CopilotAccountOverviewEntry,
  CopilotAccountBudgetAlertInfo,
} from '@/api/admin/copilotAnalytics'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import SummaryCard from '@/components/admin/copilot/CopilotSummaryCard.vue'
import AccountCostCard from '@/components/admin/copilot/AccountCostCard.vue'
import QuotaTrendChart from '@/components/admin/copilot/QuotaTrendChart.vue'
import BudgetAlertDialog from '@/components/admin/copilot/BudgetAlertDialog.vue'

const { t } = useI18n()

const loading = ref(false)
const error = ref<string | null>(null)
const overview = ref<CopilotAccountsOverviewResult | null>(null)

const refreshing = ref(new Set<number>())
const trendExpanded = ref(new Set<number>())

const budgetDialog = reactive<{
  visible: boolean
  accountId: number
  initial: CopilotAccountBudgetAlertInfo | null
}>({
  visible: false,
  accountId: 0,
  initial: null,
})

const toast = ref<{ type: 'success' | 'error'; msg: string } | null>(null)

function showToast(type: 'success' | 'error', msg: string) {
  toast.value = { type, msg }
  setTimeout(() => { toast.value = null }, 3000)
}

async function loadOverview() {
  loading.value = true
  error.value = null
  try {
    overview.value = await getCopilotAccountsOverview()
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

async function doRefresh(account: CopilotAccountOverviewEntry) {
  refreshing.value.add(account.account_id)
  try {
    await refreshCopilotAccountQuota(account.account_id)
    showToast('success', t('admin.copilot.accounts.refreshSuccess'))
    await loadOverview()
  } catch (e: unknown) {
    showToast('error', t('admin.copilot.accounts.refreshFailed'))
  } finally {
    refreshing.value.delete(account.account_id)
  }
}

function toggleTrend(accountId: number) {
  if (trendExpanded.value.has(accountId)) {
    trendExpanded.value.delete(accountId)
  } else {
    trendExpanded.value.add(accountId)
  }
}

function openBudget(account: CopilotAccountOverviewEntry) {
  budgetDialog.accountId = account.account_id
  budgetDialog.initial = account.budget_alert
  budgetDialog.visible = true
}

async function onBudgetSaved() {
  budgetDialog.visible = false
  showToast('success', t('admin.copilot.accounts.budgetSaved'))
  await loadOverview()
}

function onBudgetError(msg: string) {
  showToast('error', `${t('admin.copilot.accounts.budgetFailed')}: ${msg}`)
}

onMounted(loadOverview)
</script>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.3s;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
