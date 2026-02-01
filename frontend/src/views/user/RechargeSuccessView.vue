<template>
  <main class="container mx-auto max-w-3xl px-4 py-6">
    <!-- 页面加载状态 -->
    <div v-if="loading" class="flex flex-col items-center justify-center py-20">
      <div class="h-8 w-8 animate-spin rounded-full border-4 border-primary-500 border-t-transparent"></div>
      <span class="mt-4 text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</span>
    </div>

    <!-- 成功页面内容 -->
    <div v-else class="space-y-6">
      <!-- 成功卡片 -->
      <div class="rounded-2xl bg-white p-8 shadow-card dark:bg-dark-800">
        <!-- 成功图标 -->
        <div class="mx-auto mb-6 flex h-20 w-20 items-center justify-center rounded-full bg-emerald-100 dark:bg-emerald-900/30">
          <svg class="h-10 w-10 text-emerald-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
          </svg>
        </div>

        <!-- 成功标题 -->
        <h1 class="mb-2 text-center text-2xl font-bold text-gray-900 dark:text-white">
          {{ t('recharge.successTitle') }}
        </h1>
        <p class="mb-8 text-center text-gray-500 dark:text-gray-400">
          {{ t('recharge.successSubtitle') }}
        </p>

        <!-- 充值金额（大字体） -->
        <div class="mb-8 text-center">
          <span class="text-sm text-gray-500 dark:text-gray-400">{{ t('recharge.rechargeAmount') }}</span>
          <div class="mt-2 text-5xl font-bold text-emerald-500">
            ¥{{ order?.amount?.toFixed(2) || '--' }}
          </div>
        </div>

        <!-- 订单信息 -->
        <div class="mb-8 rounded-xl bg-gray-50 p-4 dark:bg-dark-700">
          <div class="flex justify-between text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{ t('recharge.orderNo') }}</span>
            <span class="font-mono text-gray-900 dark:text-white">{{ orderNo }}</span>
          </div>
          <div class="mt-3 flex justify-between text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{ t('recharge.paidTime') }}</span>
            <span class="text-gray-900 dark:text-white">{{ formattedPaidTime }}</span>
          </div>
        </div>

        <!-- 当前账户余额 -->
        <div class="mb-8 rounded-xl bg-gradient-to-r from-primary-500 to-primary-600 p-4 text-white">
          <div class="flex items-center justify-between">
            <span class="text-sm opacity-80">{{ t('recharge.currentBalance') }}</span>
            <span class="text-2xl font-bold">${{ formattedBalance }}</span>
          </div>
        </div>

        <!-- 操作按钮 -->
        <div class="flex flex-col gap-3 sm:flex-row">
          <button
            type="button"
            class="btn btn-secondary flex-1 py-3"
            @click="goToDashboard"
          >
            {{ t('recharge.backToDashboard') }}
          </button>
          <button
            type="button"
            class="btn btn-primary flex-1 py-3"
            @click="continueRecharge"
          >
            {{ t('recharge.continueRecharge') }}
          </button>
        </div>
      </div>
    </div>
  </main>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter, useRoute } from 'vue-router'
import { rechargeAPI, type RechargeOrder } from '@/api/recharge'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

// 订单号
const orderNo = computed(() => route.params.orderNo as string)

// 页面加载状态
const loading = ref(true)

// 订单信息
const order = ref<RechargeOrder | null>(null)

// 格式化余额
const formattedBalance = computed(() => {
  return authStore.user?.balance?.toFixed(2) || '0.00'
})

// 格式化支付时间
const formattedPaidTime = computed(() => {
  if (!order.value?.paid_at) return '--'
  const date = new Date(order.value.paid_at)
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
})

// 返回仪表盘
const goToDashboard = () => {
  router.push({ name: 'Dashboard' })
}

// 继续充值
const continueRecharge = () => {
  router.push({ name: 'Recharge' })
}

// 加载订单信息
const loadOrder = async () => {
  try {
    const result = await rechargeAPI.getOrder(orderNo.value)
    order.value = result

    // 刷新用户信息以获取最新余额
    await authStore.refreshUser()
  } catch (error) {
    console.error('[RechargeSuccess] Failed to load order:', error)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadOrder()
})
</script>
