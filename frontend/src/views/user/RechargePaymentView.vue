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
          <!-- 倒计时（仅在待支付状态显示） -->
          <div v-if="order?.status === 'pending' && order?.expire_at" class="flex justify-between text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{ t('recharge.remainingTime') }}</span>
            <OrderCountdown
              :expire-at="order.expire_at"
              @expired="onCountdownExpired"
            />
          </div>
        </div>

        <!-- 支付二维码区域（Native 支付） -->
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

          <!-- 手动同步按钮 -->
          <div class="mt-4 text-center">
            <button
              type="button"
              :disabled="syncCooldown > 0 || isSyncing"
              :class="[
                'px-4 py-2 rounded-lg text-sm font-medium transition-colors',
                syncCooldown > 0 || isSyncing
                  ? 'bg-gray-200 text-gray-500 cursor-not-allowed dark:bg-gray-700 dark:text-gray-400'
                  : 'bg-primary-100 text-primary-700 hover:bg-primary-200 dark:bg-primary-900/30 dark:text-primary-400'
              ]"
              @click="handleSyncStatus"
            >
              <span v-if="isSyncing" class="flex items-center gap-2">
                <div class="h-4 w-4 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
                {{ t('recharge.paying.syncing') }}
              </span>
              <span v-else-if="syncCooldown > 0">
                {{ t('recharge.paying.syncCooldown', { seconds: syncCooldown }) }}
              </span>
              <span v-else>
                {{ t('recharge.paying.syncStatus') }}
              </span>
            </button>
            <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
              {{ t('recharge.paying.syncHint') }}
            </p>
          </div>
        </div>

        <!-- 二维码加载中（无 qrcode_url 但是 native 支付） -->
        <div
          v-else-if="order?.payment_channel === 'native' && !order?.qrcode_url && order?.status === 'pending'"
          class="mt-6 flex flex-col items-center justify-center rounded-xl bg-gray-50 p-8 dark:bg-dark-700"
        >
          <div class="h-8 w-8 animate-spin rounded-full border-4 border-primary-500 border-t-transparent"></div>
          <span class="mt-4 text-sm text-gray-500 dark:text-gray-400">{{ t('recharge.qrcodeLoading') }}</span>
        </div>

        <!-- JSAPI 支付区域 -->
        <div
          v-else-if="order?.payment_channel === 'jsapi' && order?.status === 'pending'"
          class="mt-6 flex flex-col items-center justify-center rounded-xl bg-gray-50 p-8 dark:bg-dark-700"
        >
          <!-- 微信环境检测提示 -->
          <div v-if="!isInWeChatBrowser" class="text-center">
            <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
            <p class="mt-4 text-sm text-gray-500 dark:text-gray-400">
              {{ t('recharge.pleaseOpenInWeChat') }}
            </p>
          </div>

          <!-- 微信环境内显示支付按钮 -->
          <div v-else class="w-full text-center">
            <!-- 支付中状态 -->
            <div v-if="jsapiPaymentStatus === 'paying'" class="flex flex-col items-center">
              <div class="h-8 w-8 animate-spin rounded-full border-4 border-primary-500 border-t-transparent"></div>
              <span class="mt-4 text-sm text-gray-500 dark:text-gray-400">{{ t('recharge.paymentProcessing') }}</span>
            </div>

            <!-- 支付失败状态 -->
            <div v-else-if="jsapiPaymentStatus === 'failed'" class="flex flex-col items-center">
              <svg class="h-12 w-12 text-red-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              <p class="mt-4 text-sm text-red-500 dark:text-red-400">{{ jsapiErrorMessage }}</p>
              <button
                type="button"
                class="mt-4 btn btn-primary"
                @click="initiateJSAPIPayment"
              >
                {{ t('recharge.retryPayment') }}
              </button>
            </div>

            <!-- 待支付状态：显示支付按钮 -->
            <div v-else class="flex flex-col items-center">
              <svg class="h-16 w-16 text-green-500" viewBox="0 0 24 24" fill="currentColor">
                <path d="M9.5 3C6.46 3 4 5.46 4 8.5c0 .74.15 1.45.42 2.1C2.39 11.31 1 13.3 1 15.5 1 19.09 3.91 22 7.5 22c2.2 0 4.19-1.39 5.5-3.5 1.31 2.11 3.3 3.5 5.5 3.5 3.59 0 6.5-2.91 6.5-6.5 0-2.2-1.39-4.19-3.42-4.9.27-.65.42-1.36.42-2.1C22 5.46 19.54 3 16.5 3c-1.85 0-3.47.92-4.5 2.31C10.97 3.92 9.35 3 7.5 3h2zm0 2c1.93 0 3.5 1.57 3.5 3.5 0 .5-.12.97-.32 1.4l-.1.21-.17.37.37.17.21.1c.43.2.9.32 1.4.32.5 0 .97-.12 1.4-.32l.21-.1.37-.17.17.37.1.21c.2.43.32.9.32 1.4 0 2.48-2.02 4.5-4.5 4.5-1.38 0-2.62-.63-3.44-1.62l-.06-.08-.56-.72-.56.72-.06.08C8.12 17.87 6.88 18.5 5.5 18.5 3.02 18.5 1 16.48 1 14c0-1.38.63-2.62 1.62-3.44l.08-.06.72-.56-.72-.56-.08-.06C1.63 8.5 1 7.26 1 5.88 1 3.4 3.02 1.38 5.5 1.38c1.38 0 2.62.63 3.44 1.62l.06.08.56.72.56-.72.06-.08C11.12.38 12.36-.25 13.74-.25c2.48 0 4.5 2.02 4.5 4.5 0 1.38-.63 2.62-1.62 3.44l-.08.06-.72.56.72.56.08.06c.99.82 1.62 2.06 1.62 3.44 0 2.48-2.02 4.5-4.5 4.5-1.38 0-2.62-.63-3.44-1.62l-.06-.08-.56-.72-.56.72-.06.08c-.82.99-2.06 1.62-3.44 1.62-2.48 0-4.5-2.02-4.5-4.5z"/>
              </svg>
              <p class="mt-4 text-sm text-gray-500 dark:text-gray-400">
                {{ t('recharge.clickToPayInWeChat') }}
              </p>
              <button
                type="button"
                class="mt-4 btn btn-primary px-8 py-3 text-base"
                @click="initiateJSAPIPayment"
              >
                {{ t('recharge.payNow') }}
              </button>
            </div>

            <!-- 轮询状态指示器 -->
            <div v-if="isPolling" class="mt-4 flex items-center justify-center gap-2 text-sm text-gray-500 dark:text-gray-400">
              <div class="h-4 w-4 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
              <span>{{ t('recharge.waitingPayment') }}</span>
            </div>

            <!-- 手动同步按钮 -->
            <div class="mt-4 text-center">
              <button
                type="button"
                :disabled="syncCooldown > 0 || isSyncing"
                :class="[
                  'px-4 py-2 rounded-lg text-sm font-medium transition-colors',
                  syncCooldown > 0 || isSyncing
                    ? 'bg-gray-200 text-gray-500 cursor-not-allowed dark:bg-gray-700 dark:text-gray-400'
                    : 'bg-primary-100 text-primary-700 hover:bg-primary-200 dark:bg-primary-900/30 dark:text-primary-400'
                ]"
                @click="handleSyncStatus"
              >
                <span v-if="isSyncing" class="flex items-center gap-2">
                  <div class="h-4 w-4 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
                  {{ t('recharge.paying.syncing') }}
                </span>
                <span v-else-if="syncCooldown > 0">
                  {{ t('recharge.paying.syncCooldown', { seconds: syncCooldown }) }}
                </span>
                <span v-else>
                  {{ t('recharge.paying.syncStatus') }}
                </span>
              </button>
              <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
                {{ t('recharge.paying.syncHint') }}
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  </main>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter, useRoute } from 'vue-router'
import { rechargeAPI, type RechargeOrder, type JSAPIPaymentParams } from '@/api/recharge'
import { isWeChatBrowser, invokeWeChatPay } from '@/utils/wechat'
import { useAppStore } from '@/stores/app'
import QRCodeDisplay from '@/components/user/recharge/QRCodeDisplay.vue'
import OrderCountdown from '@/components/user/recharge/OrderCountdown.vue'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()
const appStore = useAppStore()

// 订单号
const orderNo = computed(() => route.params.orderNo as string)

// 页面加载状态
const loading = ref(true)

// 订单信息
const order = ref<RechargeOrder | null>(null)

// 微信环境检测
const isInWeChatBrowser = ref(false)

// JSAPI 支付状态
const jsapiPaymentStatus = ref<'idle' | 'paying' | 'failed'>('idle')
const jsapiErrorMessage = ref('')
const jsapiParams = ref<JSAPIPaymentParams | null>(null)

// 轮询相关
const POLL_INTERVAL = 3000 // 3秒
const MAX_POLL_COUNT = 40 // 最多轮询40次（2分钟）
const SYNC_EVERY_N_POLLS = 5 // 每5次轮询调用一次 syncOrderStatus（约15秒）
let pollTimer: ReturnType<typeof setInterval> | null = null
const pollCount = ref(0)
const isPolling = ref(false)

// 手动同步相关
const isSyncing = ref(false)
const syncCooldown = ref(0)
let syncCooldownTimer: ReturnType<typeof setInterval> | null = null

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

// 倒计时归零
const onCountdownExpired = () => {
  console.log('[RechargePayment] Order countdown expired')
  stopPolling()
  router.push({ name: 'RechargeFailed', params: { orderNo: orderNo.value } })
}

// 停止轮询
const stopPolling = () => {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
  isPolling.value = false
}

// 启动同步冷却倒计时
const startSyncCooldown = () => {
  syncCooldown.value = 5
  syncCooldownTimer = setInterval(() => {
    syncCooldown.value--
    if (syncCooldown.value <= 0 && syncCooldownTimer) {
      clearInterval(syncCooldownTimer)
      syncCooldownTimer = null
    }
  }, 1000)
}

// 停止同步冷却倒计时
const stopSyncCooldown = () => {
  if (syncCooldownTimer) {
    clearInterval(syncCooldownTimer)
    syncCooldownTimer = null
  }
}

// 手动同步订单状态
const handleSyncStatus = async () => {
  if (isSyncing.value || syncCooldown.value > 0) return

  isSyncing.value = true
  try {
    const result = await rechargeAPI.syncOrderStatus(orderNo.value)
    console.log('[RechargePayment] Sync order status result:', result)

    // 根据状态显示提示并跳转
    switch (result.status) {
      case 'paid':
        appStore.showSuccess(t('recharge.paying.syncResult.paid'))
        router.push({ name: 'RechargeSuccess', params: { orderNo: orderNo.value } })
        break
      case 'pending':
        appStore.showInfo(t('recharge.paying.syncResult.pending'))
        break
      case 'failed':
      case 'expired':
        appStore.showWarning(t('recharge.paying.syncResult.failed'))
        router.push({ name: 'RechargeFailed', params: { orderNo: orderNo.value } })
        break
      default:
        appStore.showInfo(t('recharge.paying.syncResult.unknown'))
    }
  } catch (error: unknown) {
    console.error('[RechargePayment] Sync order status failed:', error)
    const message = error instanceof Error ? error.message : t('recharge.paying.syncError')
    appStore.showError(message)
  } finally {
    isSyncing.value = false
    startSyncCooldown()
  }
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
    const shouldSync = pollCount.value % SYNC_EVERY_N_POLLS === 0
    console.log(`[RechargePayment] Polling order status (${pollCount.value}/${MAX_POLL_COUNT})${shouldSync ? ' [with sync]' : ''}`)

    if (shouldSync) {
      // 每 SYNC_EVERY_N_POLLS 次调用 syncOrderStatus 查询微信支付状态
      try {
        await rechargeAPI.syncOrderStatus(orderNo.value)
      } catch (syncError) {
        // sync 失败时不中断轮询，继续使用 getOrder
        console.warn('[RechargePayment] Sync failed, continuing with getOrder:', syncError)
      }
    }

    // 获取订单最新状态
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

// 发起支付（调用后端获取支付参数）
const initiatePayment = async () => {
  try {
    console.log('[RechargePayment] Initiating payment for order:', orderNo.value)
    const result = await rechargeAPI.initiatePayment(orderNo.value)

    // 更新订单信息
    if (result.qrcode_url) {
      order.value = { ...order.value!, qrcode_url: result.qrcode_url }
    }
    if (result.prepay_id) {
      order.value = { ...order.value!, prepay_id: result.prepay_id }
    }
    if (result.jsapi_params) {
      jsapiParams.value = result.jsapi_params
    }

    return result
  } catch (error) {
    console.error('[RechargePayment] Failed to initiate payment:', error)
    throw error
  }
}

// 发起 JSAPI 支付
const initiateJSAPIPayment = async () => {
  if (jsapiPaymentStatus.value === 'paying') return

  jsapiPaymentStatus.value = 'paying'
  jsapiErrorMessage.value = ''

  try {
    // 每次支付都重新获取 JSAPI 参数（签名包含时间戳，需要最新的）
    const result = await initiatePayment()
    if (!result.jsapi_params) {
      throw new Error('未获取到支付参数')
    }
    jsapiParams.value = result.jsapi_params

    // 调用微信支付
    console.log('[RechargePayment] Invoking WeChat JSAPI payment')
    const payResult = await invokeWeChatPay(jsapiParams.value)

    if (payResult.success) {
      // 微信支付前端返回成功，但需要等待后端回调确认
      // 这里不直接跳转，而是依赖轮询来检测支付状态
      console.log('[RechargePayment] JSAPI payment completed, waiting for backend confirmation')
      jsapiPaymentStatus.value = 'idle'
      // 确保轮询正在运行
      if (!isPolling.value) {
        startPolling()
      }
    } else if (payResult.cancelled) {
      // 用户取消支付
      console.log('[RechargePayment] JSAPI payment cancelled')
      jsapiPaymentStatus.value = 'idle'
    } else {
      // 支付失败
      console.error('[RechargePayment] JSAPI payment failed:', payResult.errorMessage)
      jsapiPaymentStatus.value = 'failed'
      jsapiErrorMessage.value = payResult.errorMessage || t('recharge.paymentFailed')
    }
  } catch (error) {
    console.error('[RechargePayment] JSAPI payment error:', error)
    jsapiPaymentStatus.value = 'failed'
    jsapiErrorMessage.value = error instanceof Error ? error.message : t('recharge.paymentFailed')
  }
}

// 加载订单信息并发起支付
const loadOrder = async () => {
  try {
    // 获取订单详情
    const result = await rechargeAPI.getOrder(orderNo.value)
    order.value = result

    // 检查初始状态
    handleStatusChange(result.status)

    // 如果订单状态为 pending，发起支付并开始轮询
    if (result.status === 'pending') {
      // 如果还没有支付参数，调用发起支付接口
      if (!result.qrcode_url && !result.prepay_id) {
        try {
          await initiatePayment()
        } catch (error) {
          console.error('[RechargePayment] Failed to initiate payment:', error)
        }
      }

      startPolling()
    }
  } catch (error) {
    console.error('[RechargePayment] Failed to load order:', error)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  // 检测微信环境
  isInWeChatBrowser.value = isWeChatBrowser()
  console.log('[RechargePayment] Is WeChat browser:', isInWeChatBrowser.value)

  loadOrder()
})

onUnmounted(() => {
  stopPolling()
  stopSyncCooldown()
})
</script>
