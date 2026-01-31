<template>
  <main class="container mx-auto max-w-3xl px-4 py-6">
    <!-- 页面加载状态 -->
    <div v-if="loading" class="flex flex-col items-center justify-center py-20">
      <div class="h-8 w-8 animate-spin rounded-full border-4 border-primary-500 border-t-transparent"></div>
      <span class="mt-4 text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</span>
    </div>

    <!-- 主体内容 -->
    <div v-else class="space-y-6">
      <!-- 余额展示卡片 -->
      <div
        class="balance-card bg-gradient-to-r from-primary-500 to-primary-600 rounded-2xl p-10 text-center text-white shadow-lg"
      >
        <!-- 钱包图标 -->
        <div class="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-white/20">
          <svg class="h-8 w-8 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              d="M21 12a2.25 2.25 0 00-2.25-2.25H15a3 3 0 11-6 0H5.25A2.25 2.25 0 003 12m18 0v6a2.25 2.25 0 01-2.25 2.25H5.25A2.25 2.25 0 013 18v-6m18 0V9M3 12V9m18 0a2.25 2.25 0 00-2.25-2.25H5.25A2.25 2.25 0 003 9m18 0V6a2.25 2.25 0 00-2.25-2.25H5.25A2.25 2.25 0 003 6v3"
            />
          </svg>
        </div>
        <!-- 余额标签 -->
        <span class="block text-sm opacity-80">{{ t('recharge.currentBalance') }}</span>
        <!-- 余额数值 -->
        <span class="mt-2 block text-5xl font-bold">¥{{ formattedBalance }}</span>
      </div>

      <!-- 充值表单区域 -->
      <div class="rounded-2xl bg-white p-6 shadow-card dark:bg-dark-800">
        <h2 class="mb-2 text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('recharge.title') }}
        </h2>
        <p class="mb-6 text-sm text-gray-500 dark:text-gray-400">
          {{ t('recharge.subtitle') }}
        </p>

        <!-- 金额选择器 -->
        <AmountSelector
          v-model="selectedAmount"
          :default-amounts="rechargeStore.defaultAmounts"
          :min-amount="rechargeStore.minAmount"
          :max-amount="rechargeStore.maxAmount"
        />

        <!-- 提交按钮 -->
        <div class="mt-6">
          <button
            type="button"
            :disabled="!isAmountValid || submitting"
            :aria-label="submitButtonText"
            class="btn btn-primary w-full py-3 text-base font-medium transition-all duration-200"
            :class="{
              'opacity-50 cursor-not-allowed': !isAmountValid || submitting,
              'hover:shadow-lg': isAmountValid && !submitting
            }"
            @click="handleSubmit"
          >
            <span v-if="submitting" class="flex items-center justify-center gap-2">
              <svg class="h-5 w-5 animate-spin" viewBox="0 0 24 24" fill="none">
                <circle
                  class="opacity-25"
                  cx="12"
                  cy="12"
                  r="10"
                  stroke="currentColor"
                  stroke-width="4"
                />
                <path
                  class="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                />
              </svg>
              {{ t('recharge.submitting') }}
            </span>
            <span v-else>{{ submitButtonText }}</span>
          </button>
        </div>

        <!-- 支付方式提示（后续 Story 实现） -->
        <p class="mt-4 text-center text-sm text-gray-400 dark:text-gray-500">
          {{ t('recharge.comingSoon') }}
        </p>
      </div>
    </div>
  </main>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useRechargeStore } from '@/stores'
import AmountSelector from '@/components/user/recharge/AmountSelector.vue'

const { t } = useI18n()
const authStore = useAuthStore()
const rechargeStore = useRechargeStore()

// 页面加载状态
const loading = ref(true)

// 选中的充值金额
const selectedAmount = ref<number | null>(null)

// 提交中状态
const submitting = ref(false)

// 用户余额
const balance = computed(() => authStore.user?.balance ?? 0)

// 格式化余额显示（保留两位小数）
const formattedBalance = computed(() => balance.value.toFixed(2))

// 金额有效性验证
const isAmountValid = computed(() => {
  if (selectedAmount.value === null) {
    return false
  }
  const amount = selectedAmount.value
  return amount >= rechargeStore.minAmount && amount <= rechargeStore.maxAmount
})

// 提交按钮文案
const submitButtonText = computed(() => {
  if (selectedAmount.value !== null && isAmountValid.value) {
    return t('recharge.submitButton', { amount: selectedAmount.value })
  }
  return t('recharge.submitButtonDefault')
})

// 提交处理（后续 Story 实现具体逻辑）
const handleSubmit = async () => {
  if (!isAmountValid.value || submitting.value) {
    return
  }

  submitting.value = true

  try {
    // TODO: Story 2-4/2-5 实现订单创建逻辑
    console.log('Creating order for amount:', selectedAmount.value)

    // 模拟异步操作（后续替换为真实 API 调用）
    await new Promise((resolve) => setTimeout(resolve, 1000))
  } catch (error) {
    console.error('Failed to create order:', error)
  } finally {
    submitting.value = false
  }
}

// 页面加载时刷新用户数据以获取最新余额
onMounted(async () => {
  try {
    // 并行加载用户数据和充值配置
    await Promise.all([authStore.refreshUser(), rechargeStore.fetchConfig()])
  } catch (error) {
    console.error('Failed to refresh user data:', error)
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
/* 余额卡片渐变背景 */
.balance-card {
  background: linear-gradient(135deg, #d97757 0%, #c45a3a 100%);
}
</style>
