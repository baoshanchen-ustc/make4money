<template>
  <Teleport to="body">
    <Transition name="modal">
      <div
        v-if="visible"
        class="fixed inset-0 z-50 flex items-center justify-center p-4"
      >
        <!-- 背景遮罩 -->
        <div
          class="absolute inset-0 bg-black/50 backdrop-blur-sm"
          @click="handleClose"
        ></div>

        <!-- 弹框内容 -->
        <div
          class="relative w-full max-w-md rounded-2xl bg-white p-6 shadow-2xl dark:bg-dark-800"
          @click.stop
        >
          <!-- 关闭按钮 -->
          <button
            type="button"
            class="absolute right-4 top-4 rounded-full p-2 text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700"
            @click="handleClose"
          >
            <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>

          <!-- 标题 -->
          <h3 class="mb-6 text-center text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('subscriptionPlan.orderInfo') }}
          </h3>

          <!-- 加载状态 -->
          <div v-if="loading" class="flex flex-col items-center justify-center py-12">
            <div class="h-8 w-8 animate-spin rounded-full border-4 border-primary-500 border-t-transparent"></div>
            <span class="mt-4 text-sm text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</span>
          </div>

          <!-- 支付内容 -->
          <div v-else-if="order" class="text-center">
            <!-- 套餐名称 -->
            <div class="mb-2 text-base font-medium text-gray-700 dark:text-gray-300">
              {{ order.group_name || t('subscriptionPlan.subscription') }}
            </div>

            <!-- 支付方式标识 -->
            <div class="mb-2 text-sm text-gray-500 dark:text-gray-400">
              {{ t('recharge.wechatPay') }}
            </div>

            <!-- 订单号 -->
            <div class="mb-2 text-xs text-gray-400 dark:text-gray-500">
              {{ t('recharge.orderNo') }}：{{ order.order_no }}
            </div>

            <!-- 金额 -->
            <div class="mb-6 text-3xl font-bold text-red-500">
              ¥{{ order.amount?.toFixed(2) }}
            </div>

            <!-- 二维码区域 -->
            <div v-if="order.qrcode_url" class="mx-auto mb-4 w-fit rounded-xl border-2 border-gray-100 p-4 dark:border-dark-600">
              <QRCodeDisplay
                :code-url="order.qrcode_url"
                :size="180"
              />
            </div>

            <!-- 二维码加载中 -->
            <div
              v-else-if="order.status === 'pending'"
              class="mx-auto mb-4 flex h-[212px] w-[212px] items-center justify-center rounded-xl bg-gray-50 dark:bg-dark-700"
            >
              <div class="text-center">
                <div class="mx-auto h-6 w-6 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
                <span class="mt-2 block text-xs text-gray-500">{{ t('recharge.qrcodeLoading') }}</span>
              </div>
            </div>

            <!-- 提示文字 -->
            <p class="mb-2 text-sm text-gray-600 dark:text-gray-400">
              {{ t('recharge.qrcodeHint') }}
            </p>

            <!-- 倒计时 -->
            <div v-if="order.status === 'pending' && order.expire_at" class="mb-4 text-sm text-gray-500 dark:text-gray-400">
              {{ t('recharge.modal.qrcodeExpire') }}
              <OrderCountdown
                :expire-at="order.expire_at"
                @expired="onCountdownExpired"
              />
            </div>

            <!-- 轮询状态指示器 -->
            <div v-if="isPolling" class="mb-4 flex items-center justify-center gap-2 text-xs text-gray-500 dark:text-gray-400">
              <div class="h-3 w-3 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
              <span>{{ t('recharge.waitingPayment') }}</span>
            </div>

            <!-- 操作按钮 -->
            <div class="flex justify-center gap-3">
              <button
                type="button"
                class="btn btn-outline px-6"
                @click="handleCancel"
              >
                {{ t('recharge.modal.cancelPayment') }}
              </button>
              <button
                type="button"
                class="btn btn-primary px-6"
                @click="handlePaidConfirm"
              >
                {{ t('recharge.modal.confirmPaid') }}
              </button>
            </div>
          </div>

          <!-- 错误状态 -->
          <div v-else class="py-12 text-center">
            <svg class="mx-auto h-12 w-12 text-red-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
            <p class="mt-4 text-sm text-gray-500 dark:text-gray-400">{{ t('recharge.modal.loadError') }}</p>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, watch, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { subscriptionPlanAPI, type SubscriptionOrder } from '@/api/subscriptionPlan'
import { useAppStore } from '@/stores/app'
import QRCodeDisplay from '../recharge/QRCodeDisplay.vue'
import OrderCountdown from '../recharge/OrderCountdown.vue'

export interface PaidEventPayload {
  orderNo: string
  orderType?: string
}

const props = defineProps<{
  visible: boolean
  orderNo: string
}>()

const emit = defineEmits<{
  'update:visible': [value: boolean]
  'close': []
  'paid': [payload: PaidEventPayload]
  'expired': [orderNo: string]
}>()

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const order = ref<SubscriptionOrder | null>(null)

// 轮询相关
const POLL_INTERVAL = 3000
const MAX_POLL_COUNT = 40
const SYNC_EVERY_N_POLLS = 5
let pollTimer: ReturnType<typeof setInterval> | null = null
const pollCount = ref(0)
const isPolling = ref(false)

// 停止轮询
const stopPolling = () => {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
  isPolling.value = false
}

// 处理状态变化
const handleStatusChange = (status: string) => {
  if (status === 'paid') {
    stopPolling()
    emit('update:visible', false)
    emit('paid', {
      orderNo: props.orderNo,
      orderType: order.value?.order_type
    })
    appStore.showSuccess(t('subscriptionPlan.paymentSuccess'))
  } else if (status === 'failed' || status === 'expired' || status === 'cancelled') {
    stopPolling()
    emit('expired', props.orderNo)
  }
}

// 轮询订单状态
const pollOrderStatus = async () => {
  try {
    pollCount.value++
    const shouldSync = pollCount.value % SYNC_EVERY_N_POLLS === 0

    if (shouldSync) {
      try {
        await subscriptionPlanAPI.syncOrderStatus(props.orderNo)
      } catch {
        // sync 失败时继续
      }
    }

    const result = await subscriptionPlanAPI.getOrder(props.orderNo)
    order.value = result
    handleStatusChange(result.status)

    if (pollCount.value >= MAX_POLL_COUNT) {
      stopPolling()
    }
  } catch (error) {
    console.error('[SubscriptionPaymentModal] Poll failed:', error)
  }
}

// 开始轮询
const startPolling = () => {
  if (pollTimer) return
  isPolling.value = true
  pollTimer = setInterval(pollOrderStatus, POLL_INTERVAL)
}

// 加载订单并发起支付
const loadOrder = async () => {
  if (!props.orderNo) return

  loading.value = true
  try {
    const result = await subscriptionPlanAPI.getOrder(props.orderNo)
    order.value = result

    if (result.status === 'pending') {
      // 如果没有二维码，调用发起支付
      if (!result.qrcode_url && !result.prepay_id) {
        const payResult = await subscriptionPlanAPI.initiatePayment(props.orderNo)
        if (payResult.qrcode_url) {
          order.value = { ...order.value!, qrcode_url: payResult.qrcode_url }
        }
      }
      startPolling()
    } else {
      handleStatusChange(result.status)
    }
  } catch (error) {
    console.error('[SubscriptionPaymentModal] Load order failed:', error)
  } finally {
    loading.value = false
  }
}

// 关闭弹框
const handleClose = () => {
  stopPolling()
  emit('close')
  emit('update:visible', false)
}

// 取消支付（仅关闭弹框，不取消订单）
const handleCancel = () => {
  handleClose()
}

// 确认已支付
const handlePaidConfirm = async () => {
  try {
    const result = await subscriptionPlanAPI.syncOrderStatus(props.orderNo)
    if (result.status === 'paid') {
      handleStatusChange('paid')
    } else {
      appStore.showInfo(t('recharge.paying.syncResult.pending'))
    }
  } catch {
    appStore.showError(t('recharge.paying.syncError'))
  }
}

// 倒计时归零
const onCountdownExpired = () => {
  stopPolling()
  emit('expired', props.orderNo)
}

// 监听 visible 变化
watch(() => props.visible, (newVal) => {
  if (newVal) {
    pollCount.value = 0
    loadOrder()
  } else {
    stopPolling()
  }
})

// 监听 orderNo 变化
watch(() => props.orderNo, () => {
  if (props.visible) {
    pollCount.value = 0
    loadOrder()
  }
})

onUnmounted(() => {
  stopPolling()
})
</script>

<style scoped>
.modal-enter-active,
.modal-leave-active {
  transition: all 0.3s ease;
}

.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}

.modal-enter-from .relative,
.modal-leave-to .relative {
  transform: scale(0.95);
}
</style>
