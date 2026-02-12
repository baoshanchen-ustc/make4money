<template>
  <!-- eslint-disable vue/no-mutating-props -->
  <div class="space-y-6">
    <!-- WeChat Pay (read-only) -->
    <div class="card">
      <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('admin.settings.wechatPay.title') }}
        </h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.settings.wechatPay.description') }}
        </p>
      </div>
      <div class="space-y-4 p-6">
        <div v-if="wechatPayLoading" class="flex items-center gap-2 text-gray-500">
          <div class="h-4 w-4 animate-spin rounded-full border-b-2 border-primary-600"></div>
          {{ t('common.loading') }}
        </div>

        <template v-else>
          <div class="flex items-center justify-between">
            <span class="text-sm text-gray-700 dark:text-gray-300">
              {{ t('admin.settings.wechatPay.status') }}
            </span>
            <span
              :class="[
                'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium',
                wechatPayStatus?.enabled
                  ? 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
                  : 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300'
              ]"
            >
              {{ wechatPayStatus?.enabled
                ? t('admin.settings.wechatPay.enabled')
                : t('admin.settings.wechatPay.disabled')
              }}
            </span>
          </div>

          <div class="flex items-center justify-between">
            <span class="text-sm text-gray-700 dark:text-gray-300">
              {{ t('admin.settings.wechatPay.configStatus') }}
            </span>
            <span
              :class="[
                'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium',
                wechatPayStatus?.configured
                  ? 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400'
                  : 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400'
              ]"
            >
              {{ wechatPayStatus?.configured
                ? t('admin.settings.wechatPay.configured')
                : t('admin.settings.wechatPay.notConfigured')
              }}
            </span>
          </div>

          <div v-if="wechatPayStatus?.app_id_masked" class="flex items-center justify-between">
            <span class="text-sm text-gray-700 dark:text-gray-300">AppID</span>
            <code class="text-sm font-mono text-gray-600 dark:text-gray-400">
              {{ wechatPayStatus.app_id_masked }}
            </code>
          </div>

          <div v-if="wechatPayStatus?.mch_id_masked" class="flex items-center justify-between">
            <span class="text-sm text-gray-700 dark:text-gray-300">
              {{ t('admin.settings.wechatPay.mchId') }}
            </span>
            <code class="text-sm font-mono text-gray-600 dark:text-gray-400">
              {{ wechatPayStatus.mch_id_masked }}
            </code>
          </div>

          <div v-if="wechatPayStatus?.notify_url" class="flex items-center justify-between">
            <span class="text-sm text-gray-700 dark:text-gray-300">
              {{ t('admin.settings.wechatPay.notifyUrl') }}
            </span>
            <code class="text-sm font-mono text-gray-600 dark:text-gray-400 truncate max-w-xs" :title="wechatPayStatus.notify_url">
              {{ wechatPayStatus.notify_url }}
            </code>
          </div>

          <div v-if="!wechatPayStatus?.configured" class="mt-4 p-3 bg-yellow-50 dark:bg-yellow-900/20 rounded-lg">
            <p class="text-sm text-yellow-800 dark:text-yellow-200">
              {{ t('admin.settings.wechatPay.configHint') }}
            </p>
          </div>
        </template>
      </div>
    </div>

    <!-- Recharge Settings -->
    <div class="card">
      <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('admin.settings.recharge.title') }}
        </h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.settings.recharge.description') }}
        </p>
      </div>
      <div class="space-y-5 p-6">
        <div v-if="rechargeLoading" class="flex items-center gap-2 text-gray-500">
          <div class="h-4 w-4 animate-spin rounded-full border-b-2 border-primary-600"></div>
          {{ t('common.loading') }}
        </div>

        <template v-else>
          <div class="grid grid-cols-1 gap-6 md:grid-cols-2">
            <div>
              <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('admin.settings.recharge.minAmount') }}
              </label>
              <input
                v-model.number="rechargeForm.min_amount"
                type="number"
                step="0.01"
                min="0.01"
                class="input"
                placeholder="1.00"
              />
            </div>

            <div>
              <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('admin.settings.recharge.maxAmount') }}
              </label>
              <input
                v-model.number="rechargeForm.max_amount"
                type="number"
                step="0.01"
                min="0.01"
                class="input"
                placeholder="1000.00"
              />
            </div>
          </div>

          <div>
            <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.settings.recharge.orderExpireMinutes') }}
            </label>
            <div class="flex items-center gap-2">
              <input
                v-model.number="rechargeForm.order_expire_minutes"
                type="number"
                min="1"
                max="1440"
                class="input w-32"
                placeholder="120"
              />
              <span class="text-sm text-gray-500 dark:text-gray-400">
                {{ t('admin.settings.recharge.minutes') }}
              </span>
            </div>
            <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.recharge.expireHint') }}
            </p>
          </div>

          <div>
            <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.settings.recharge.exchangeRate') }}
            </label>
            <div class="flex items-center gap-2">
              <input
                v-model.number="rechargeForm.exchange_rate"
                type="number"
                step="0.01"
                min="0.01"
                class="input w-32"
                placeholder="1.00"
              />
              <span class="text-sm text-gray-500 dark:text-gray-400">
                {{ t('admin.settings.recharge.exchangeRateUnit') }}
              </span>
            </div>
            <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.recharge.exchangeRateHint') }}
            </p>
          </div>

          <!-- Default amounts -->
          <div>
            <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.settings.recharge.defaultAmounts') }}
            </label>
            <div class="flex flex-wrap gap-2 mb-2">
              <span
                v-for="(amount, index) in rechargeForm.default_amounts"
                :key="index"
                class="inline-flex items-center gap-1 px-3 py-1 rounded-full bg-primary-100 text-primary-800 dark:bg-primary-900/30 dark:text-primary-300 text-sm"
              >
                ¥{{ amount }}
                <button
                  type="button"
                  class="hover:text-primary-600 dark:hover:text-primary-200"
                  @click="removeRechargeAmount(index)"
                >
                  <svg class="h-3.5 w-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </span>
            </div>
            <div class="flex items-center gap-2">
              <input
                v-model.number="newRechargeAmount"
                type="number"
                step="0.01"
                min="0.01"
                class="input w-32"
                :placeholder="t('admin.settings.recharge.addAmount')"
                @keyup.enter="addRechargeAmount"
              />
              <button
                type="button"
                class="btn btn-secondary"
                :disabled="!newRechargeAmount || newRechargeAmount <= 0"
                @click="addRechargeAmount"
              >
                {{ t('admin.settings.recharge.addAmount') }}
              </button>
            </div>
            <p class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.settings.recharge.amountsHint') }}
            </p>
          </div>

          <!-- Save Button -->
          <div class="flex justify-end pt-4 border-t border-gray-100 dark:border-dark-700">
            <button
              type="button"
              class="btn btn-primary"
              :disabled="rechargeSaving"
              @click="saveRechargeSettings"
            >
              <svg v-if="rechargeSaving" class="mr-2 h-4 w-4 animate-spin" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" fill="none" />
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
              </svg>
              {{ rechargeSaving ? t('common.saving') : t('common.save') }}
            </button>
          </div>
        </template>
      </div>
    </div>

    <!-- Balance Lot Expiry Settings -->
    <div class="card">
      <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('admin.settings.balanceLotExpiry.title') }}
        </h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.settings.balanceLotExpiry.description') }}
        </p>
      </div>
      <div class="space-y-4 p-6">
        <div>
          <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.balanceLotExpiry.expiryDays') }}
          </label>
          <input
            v-model.number="form.balance_lot_expiry_days"
            type="number"
            min="1"
            max="365"
            class="input w-40"
          />
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.settings.balanceLotExpiry.expiryDaysHint') }}
          </p>
        </div>
        <div class="flex items-center gap-3">
          <input
            v-model="form.balance_expiry_reminder_enabled"
            type="checkbox"
            class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
          />
          <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.balanceLotExpiry.reminderEnabled') }}
          </label>
        </div>
        <div v-if="form.balance_expiry_reminder_enabled">
          <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.balanceLotExpiry.reminderAdvanceDays') }}
          </label>
          <input
            v-model.number="form.balance_expiry_reminder_advance_days"
            type="number"
            min="1"
            max="30"
            class="input w-40"
          />
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.settings.balanceLotExpiry.reminderAdvanceDaysHint') }}
          </p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
/* eslint-disable vue/no-mutating-props */
import { ref, reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api'
import type { WeChatPayStatus, RechargeSettings } from '@/api/admin/settings'
import { useAppStore } from '@/stores'

defineProps<{
  form: any
}>()

const { t } = useI18n()
const appStore = useAppStore()

// WeChat Pay status (read-only)
const wechatPayLoading = ref(true)
const wechatPayStatus = ref<WeChatPayStatus | null>(null)

// Recharge settings
const rechargeLoading = ref(true)
const rechargeSaving = ref(false)
const newRechargeAmount = ref<number | null>(null)
const rechargeForm = reactive<RechargeSettings>({
  min_amount: 1,
  max_amount: 1000,
  default_amounts: [10, 50, 100, 200, 500],
  order_expire_minutes: 120,
  exchange_rate: 1
})

async function loadWeChatPayStatus() {
  wechatPayLoading.value = true
  try {
    wechatPayStatus.value = await adminAPI.settings.getWeChatPayStatus()
  } catch (error: any) {
    console.error('Failed to load WeChat Pay status:', error)
  } finally {
    wechatPayLoading.value = false
  }
}

async function loadRechargeSettings() {
  rechargeLoading.value = true
  try {
    const settings = await adminAPI.settings.getRechargeSettings()
    Object.assign(rechargeForm, settings)
  } catch (error: any) {
    console.error('Failed to load recharge settings:', error)
  } finally {
    rechargeLoading.value = false
  }
}

function addRechargeAmount() {
  if (!newRechargeAmount.value || newRechargeAmount.value <= 0) return
  if (rechargeForm.default_amounts.includes(newRechargeAmount.value)) {
    newRechargeAmount.value = null
    return
  }
  rechargeForm.default_amounts.push(newRechargeAmount.value)
  rechargeForm.default_amounts.sort((a, b) => a - b)
  newRechargeAmount.value = null
}

function removeRechargeAmount(index: number) {
  rechargeForm.default_amounts.splice(index, 1)
}

async function saveRechargeSettings() {
  if (rechargeForm.min_amount > rechargeForm.max_amount) {
    appStore.showError(t('admin.settings.recharge.errors.minGreaterThanMax'))
    return
  }

  if (rechargeForm.exchange_rate <= 0) {
    appStore.showError(t('admin.settings.recharge.errors.invalidExchangeRate'))
    return
  }

  for (const amount of rechargeForm.default_amounts) {
    if (amount < rechargeForm.min_amount || amount > rechargeForm.max_amount) {
      appStore.showError(t('admin.settings.recharge.errors.amountOutOfRange'))
      return
    }
  }

  rechargeSaving.value = true
  try {
    const updated = await adminAPI.settings.updateRechargeSettings({
      min_amount: rechargeForm.min_amount,
      max_amount: rechargeForm.max_amount,
      default_amounts: rechargeForm.default_amounts,
      order_expire_minutes: rechargeForm.order_expire_minutes,
      exchange_rate: rechargeForm.exchange_rate
    })
    Object.assign(rechargeForm, updated)
    appStore.showSuccess(t('admin.settings.recharge.saveSuccess'))
  } catch (error: any) {
    appStore.showError(
      t('admin.settings.recharge.saveFailed') + ': ' + (error.message || t('common.unknownError'))
    )
  } finally {
    rechargeSaving.value = false
  }
}

onMounted(() => {
  loadWeChatPayStatus()
  loadRechargeSettings()
})
</script>
