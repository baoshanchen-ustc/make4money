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

        <!-- 支付按钮区域（后续 Story 实现） -->
        <div class="mt-6 flex flex-col items-center justify-center py-4 text-center">
          <svg
            class="mb-4 h-12 w-12 text-gray-300 dark:text-gray-600"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            stroke-width="1.5"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              d="M2.25 8.25h19.5M2.25 9h19.5m-16.5 5.25h6m-6 2.25h3m-3.75 3h15a2.25 2.25 0 002.25-2.25V6.75A2.25 2.25 0 0019.5 4.5h-15a2.25 2.25 0 00-2.25 2.25v10.5A2.25 2.25 0 004.5 19.5z"
            />
          </svg>
          <p class="text-gray-400 dark:text-gray-500">{{ t('recharge.comingSoon') }}</p>
        </div>
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

// 用户余额
const balance = computed(() => authStore.user?.balance ?? 0)

// 格式化余额显示（保留两位小数）
const formattedBalance = computed(() => balance.value.toFixed(2))

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
