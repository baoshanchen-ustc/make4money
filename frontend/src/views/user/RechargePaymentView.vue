<template>
  <main class="container mx-auto max-w-3xl px-4 py-6">
    <!-- 页面加载状态 -->
    <div v-if="loading" class="flex flex-col items-center justify-center py-20">
      <div class="h-8 w-8 animate-spin rounded-full border-4 border-primary-500 border-t-transparent"></div>
      <span class="mt-4 text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</span>
    </div>

    <!-- 支付页面内容 -->
    <div v-else class="space-y-6">
      <!-- 返回按钮 -->
      <button
        type="button"
        class="flex items-center gap-2 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
        @click="goBack"
      >
        <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M15 19l-7-7 7-7" />
        </svg>
        {{ t('common.back') }}
      </button>

      <!-- 订单信息卡片 -->
      <div class="rounded-2xl bg-white p-6 shadow-card dark:bg-dark-800">
        <h2 class="mb-4 text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('recharge.orderInfo') }}
        </h2>

        <!-- 订单详情 -->
        <div class="space-y-3">
          <div class="flex justify-between text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{ t('recharge.orderNo') }}</span>
            <span class="font-mono text-gray-900 dark:text-white">{{ orderNo }}</span>
          </div>
          <div class="flex justify-between text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{ t('recharge.orderAmount') }}</span>
            <span class="text-lg font-semibold text-primary-500">¥{{ order?.amount?.toFixed(2) || '--' }}</span>
          </div>
          <div class="flex justify-between text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{ t('recharge.orderStatus') }}</span>
            <span
              class="rounded-full px-2 py-0.5 text-xs font-medium"
              :class="statusClass"
            >
              {{ statusText }}
            </span>
          </div>
        </div>

        <!-- 支付二维码区域 -->
        <div
          v-if="order?.payment_channel === 'native' && order?.qrcode_url"
          class="mt-6 flex flex-col items-center justify-center rounded-xl bg-gray-50 p-8 dark:bg-dark-700"
        >
          <QRCodeDisplay
            :code-url="order.qrcode_url"
            :size="200"
            @generated="onQRCodeGenerated"
            @error="onQRCodeError"
          />
          <!-- 轮询状态指示器 -->
          <div v-if="isPolling" class="mt-4 flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400">
            <div class="h-4 w-4 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
            <span>{{ t('recharge.waitingPayment') }}</span>
          </div>
        </div>

        <!-- 二维码加载中（无 qrcode_url 但是 native 支付） -->
        <div
          v-else-if="order?.payment_channel === 'native'"
          class="mt-6 flex flex-col items-center justify-center rounded-xl bg-gray-50 p-8 dark:bg-dark-700"
        >
          <div class="h-8 w-8 animate-spin rounded-full border-4 border-primary-500 border-t-transparent"></div>
          <span class="mt-4 text-sm text-gray-500 dark:text-gray-400">{{ t('recharge.qrcodeLoading') }}</span>
        </div>
      </div>
    </div>
  </main>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter, useRoute } from 'vue-router'
import { rechargeAPI, type RechargeOrder } from '@/api/recharge'
import QRCodeDisplay from '@/components/user/recharge/QRCodeDisplay.vue'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()

// 订单号
const orderNo = computed(() => route.params.orderNo as string)

// 页面加载状态
const loading = ref(true)

// 订单信息
const order = ref<RechargeOrder | null>(null)

// 轮询相关
const POLL_INTERVAL = 3000 // 3秒
const MAX_POLL_COUNT = 40 // 最多轮询40次（2分钟）
let pollTimer: ReturnType<typeof setInterval> | null = null
const pollCount = ref(0)
const isPolling = ref(false)

// 订单状态样式
const statusClass = computed(() => {
  switch (order.value?.status) {
    case 'pending':
      return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400'
    case 'paid':
      return 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400'
    case 'failed':
    case 'expired':
      return 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400'
    default:
      return 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300'
  }
})

// 订单状态文案
const statusText = computed(() => {
  switch (order.value?.status) {
    case 'pending':
      return t('recharge.statusPending')
    case 'paid':
      return t('recharge.statusPaid')
    case 'failed':
      return t('recharge.statusFailed')
    case 'expired':
      return t('recharge.statusExpired')
    default:
      return t('recharge.statusUnknown')
  }
})

// 返回上一页
const goBack = () => {
  router.push({ name: 'Recharge' })
}

// 二维码生成成功
const onQRCodeGenerated = () => {
  console.log('[RechargePayment] QR code generated successfully')
}

// 二维码生成失败
const onQRCodeError = (error: Error) => {
  console.error('[RechargePayment] QR code generation failed:', error)
}

// 停止轮询
const stopPolling = () => {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
  isPolling.value = false
}

// 处理订单状态变化
const handleStatusChange = (status: string) => {
  if (status === 'paid') {
    stopPolling()
    router.push({ name: 'RechargeSuccess', params: { orderNo: orderNo.value } })
  } else if (status === 'failed' || status === 'expired') {
    stopPolling()
    router.push({ name: 'RechargeFailed', params: { orderNo: orderNo.value } })
  }
}

// 轮询订单状态
const pollOrderStatus = async () => {
  try {
    pollCount.value++
    console.log(`[RechargePayment] Polling order status (${pollCount.value}/${MAX_POLL_COUNT})`)

    const result = await rechargeAPI.getOrder(orderNo.value)
    order.value = result

    // 检查状态变化
    handleStatusChange(result.status)

    // 检查是否达到最大轮询次数
    if (pollCount.value >= MAX_POLL_COUNT) {
      console.log('[RechargePayment] Max poll count reached, stopping polling')
      stopPolling()
    }
  } catch (error) {
    console.error('[RechargePayment] Failed to poll order status:', error)
  }
}

// 开始轮询
const startPolling = () => {
  if (pollTimer) return
  isPolling.value = true
  pollTimer = setInterval(pollOrderStatus, POLL_INTERVAL)
}

// 加载订单信息
const loadOrder = async () => {
  try {
    const result = await rechargeAPI.getOrder(orderNo.value)
    order.value = result

    // 检查初始状态
    handleStatusChange(result.status)

    // 如果订单状态为 pending，开始轮询
    if (result.status === 'pending') {
      startPolling()
    }
  } catch (error) {
    console.error('[RechargePayment] Failed to load order:', error)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadOrder()
})

onUnmounted(() => {
  stopPolling()
})
</script>
