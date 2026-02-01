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
          @click="$emit('update:visible', false)"
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
            @click="$emit('update:visible', false)"
          >
            <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>

          <!-- 标题 -->
          <h3 class="mb-6 text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('recharge.modal.orderDetail') }}
          </h3>

          <!-- 加载状态 -->
          <div v-if="loading" class="flex flex-col items-center justify-center py-12">
            <div class="h-8 w-8 animate-spin rounded-full border-4 border-primary-500 border-t-transparent"></div>
          </div>

          <!-- 订单详情 -->
          <div v-else-if="order" class="space-y-4">
            <!-- 订单号 -->
            <div class="flex justify-between">
              <span class="text-gray-500 dark:text-gray-400">{{ t('recharge.orderNo') }}</span>
              <span class="font-mono text-sm text-gray-900 dark:text-white">{{ order.order_no }}</span>
            </div>

            <!-- 订单状态 -->
            <div class="flex justify-between">
              <span class="text-gray-500 dark:text-gray-400">{{ t('recharge.orderStatus') }}</span>
              <span
                class="rounded-full px-2 py-0.5 text-xs font-medium"
                :class="statusClass"
              >
                {{ statusText }}
              </span>
            </div>

            <!-- 充值金额 -->
            <div class="flex justify-between">
              <span class="text-gray-500 dark:text-gray-400">{{ t('recharge.orderAmount') }}</span>
              <span class="font-semibold text-gray-900 dark:text-white">¥{{ order.amount?.toFixed(2) }}</span>
            </div>

            <!-- 创建时间 -->
            <div class="flex justify-between">
              <span class="text-gray-500 dark:text-gray-400">{{ t('recharge.modal.createdAt') }}</span>
              <span class="text-gray-900 dark:text-white">{{ formatDate(order.created_at) }}</span>
            </div>

            <!-- 支付方式 -->
            <div class="flex justify-between">
              <span class="text-gray-500 dark:text-gray-400">{{ t('recharge.paymentMethod') }}</span>
              <span class="text-gray-900 dark:text-white">{{ t('recharge.wechatPay') }}</span>
            </div>

            <!-- 支付时间（已支付时显示） -->
            <div v-if="order.paid_at" class="flex justify-between">
              <span class="text-gray-500 dark:text-gray-400">{{ t('recharge.paidTime') }}</span>
              <span class="text-gray-900 dark:text-white">{{ formatDate(order.paid_at) }}</span>
            </div>

            <!-- 操作按钮 -->
            <div class="mt-6 flex gap-3">
              <button
                type="button"
                class="btn btn-outline flex-1"
                :disabled="syncing"
                @click="handleSync"
              >
                <span v-if="syncing" class="flex items-center justify-center gap-2">
                  <div class="h-4 w-4 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
                  {{ t('recharge.paying.syncing') }}
                </span>
                <span v-else>{{ t('recharge.modal.refreshStatus') }}</span>
              </button>
              <button
                v-if="canRepay"
                type="button"
                class="btn btn-primary flex-1"
                @click="handleRepay"
              >
                {{ t('recharge.modal.repay') }}
              </button>
            </div>
          </div>

          <!-- 错误状态 -->
          <div v-else class="py-12 text-center">
            <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('recharge.modal.loadError') }}</p>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { rechargeAPI, type RechargeOrder } from '@/api/recharge'
import { useAppStore } from '@/stores/app'

const props = defineProps<{
  visible: boolean
  orderNo: string
}>()

const emit = defineEmits<{
  'update:visible': [value: boolean]
  'repay': [orderNo: string]
  'statusChanged': []
}>()

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const syncing = ref(false)
const order = ref<RechargeOrder | null>(null)

// 是否可以重新支付
const canRepay = computed(() => {
  if (!order.value) return false
  if (order.value.status !== 'pending') return false
  // 检查是否过期
  if (order.value.expire_at) {
    return new Date(order.value.expire_at) > new Date()
  }
  return true
})

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
    case 'cancelled':
      return 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300'
    case 'refunded':
      return 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400'
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
    case 'cancelled':
      return t('recharge.statusCancelled')
    case 'refunded':
      return t('recharge.statusRefunded')
    default:
      return t('recharge.statusUnknown')
  }
})

// 格式化日期
const formatDate = (dateStr: string) => {
  const date = new Date(dateStr)
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

// 加载订单
const loadOrder = async () => {
  if (!props.orderNo) return

  loading.value = true
  try {
    order.value = await rechargeAPI.getOrder(props.orderNo)
  } catch (error) {
    console.error('[OrderDetailModal] Load order failed:', error)
  } finally {
    loading.value = false
  }
}

// 刷新状态
const handleSync = async () => {
  syncing.value = true
  try {
    await rechargeAPI.syncOrderStatus(props.orderNo)
    const result = await rechargeAPI.getOrder(props.orderNo)
    order.value = result
    emit('statusChanged')

    if (result.status === 'paid') {
      appStore.showSuccess(t('recharge.paying.syncResult.paid'))
    } else if (result.status === 'pending') {
      appStore.showInfo(t('recharge.paying.syncResult.pending'))
    }
  } catch {
    appStore.showError(t('recharge.paying.syncError'))
  } finally {
    syncing.value = false
  }
}

// 重新支付
const handleRepay = () => {
  emit('update:visible', false)
  emit('repay', props.orderNo)
}

// 监听 visible 变化
watch(() => props.visible, (newVal) => {
  if (newVal) {
    loadOrder()
  }
})

// 监听 orderNo 变化
watch(() => props.orderNo, () => {
  if (props.visible) {
    loadOrder()
  }
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
