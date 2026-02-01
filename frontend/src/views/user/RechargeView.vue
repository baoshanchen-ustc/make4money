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
        <span class="mt-2 block text-5xl font-bold">${{ formattedBalance }}</span>
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

        <!-- 汇率提示（非 1:1 时显示） -->
        <div
          v-if="selectedAmount && rechargeStore.exchangeRate !== 1"
          class="mt-4 rounded-lg bg-blue-50 p-3 text-sm text-blue-700 dark:bg-blue-900/20 dark:text-blue-300"
        >
          {{ t('recharge.exchangeRateInfo', {
            payAmount: selectedAmount.toFixed(2),
            creditAmount: creditedAmountPreview,
            rate: rechargeStore.exchangeRate.toFixed(2)
          }) }}
        </div>

        <!-- 支付方式选择器 -->
        <div class="mt-6">
          <PaymentMethodSelector v-model="selectedPaymentMethod" />
        </div>

        <!-- Turnstile 验证码（IP 高频时显示） -->
        <div v-if="showCaptcha && turnstileSiteKey" class="mt-6">
          <TurnstileWidget
            ref="turnstileRef"
            :site-key="turnstileSiteKey"
            @verify="onTurnstileVerify"
            @expire="onTurnstileExpire"
            @error="onTurnstileError"
          />
        </div>

        <!-- 错误提示 -->
        <div
          v-if="errorMessage"
          class="mt-4 rounded-lg bg-red-50 p-3 text-sm text-red-600 dark:bg-red-900/20 dark:text-red-400"
        >
          {{ errorMessage }}
        </div>

        <!-- 提交按钮 -->
        <div class="mt-6">
          <button
            type="button"
            :disabled="!canSubmit || submitting || rateLimitCountdown > 0"
            :aria-label="submitButtonText"
            class="btn btn-primary w-full py-3 text-base font-medium transition-all duration-200"
            :class="{
              'opacity-50 cursor-not-allowed': !canSubmit || submitting || rateLimitCountdown > 0,
              'hover:shadow-lg': canSubmit && !submitting && rateLimitCountdown <= 0
            }"
            @click="handleSubmit"
          >
            <span v-if="rateLimitCountdown > 0" class="flex items-center justify-center gap-2">
              <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                <path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              {{ t('recharge.rateLimitCountdown', { seconds: rateLimitCountdown }) }}
            </span>
            <span v-else-if="submitting" class="flex items-center justify-center gap-2">
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
              {{ t('recharge.orderCreating') }}
            </span>
            <span v-else>{{ submitButtonText }}</span>
          </button>
        </div>

        <!-- 待支付订单提示 -->
        <div
          v-if="pendingOrder"
          class="mt-4 rounded-lg bg-green-50 p-4 dark:bg-green-900/20"
        >
          <div class="flex items-start justify-between">
            <div>
              <h4 class="font-medium text-green-800 dark:text-green-300">
                {{ t('recharge.pendingOrderCreated') }}
              </h4>
              <p class="mt-1 text-sm text-green-700 dark:text-green-400">
                {{ t('recharge.pendingOrderHint') }}
              </p>
            </div>
            <button
              type="button"
              class="btn btn-outline border-green-600 text-green-700 hover:bg-green-50 dark:border-green-500 dark:text-green-400 dark:hover:bg-green-900/30"
              @click="openPaymentModal(pendingOrder.order_no)"
            >
              {{ t('recharge.viewQrcode') }}
            </button>
          </div>
        </div>
      </div>

      <!-- 充值记录区域 -->
      <div class="rounded-2xl bg-white p-6 shadow-card dark:bg-dark-800">
        <div class="mb-4 flex items-center justify-between">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('recharge.records.title') }}
          </h2>
          <button
            type="button"
            class="flex items-center gap-1 text-sm text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
            :disabled="ordersLoading"
            @click="loadOrders"
          >
            <svg
              class="h-4 w-4"
              :class="{ 'animate-spin': ordersLoading }"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="2"
            >
              <path stroke-linecap="round" stroke-linejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
            {{ t('recharge.records.refresh') }}
          </button>
        </div>

        <!-- 订单列表 -->
        <div v-if="ordersLoading && orders.length === 0" class="flex justify-center py-8">
          <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
        </div>

        <div v-else-if="orders.length === 0" class="py-8 text-center">
          <svg class="mx-auto h-12 w-12 text-gray-300 dark:text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
          </svg>
          <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">{{ t('rechargeRecords.noRecords') }}</p>
        </div>

        <div v-else class="overflow-x-auto">
          <table class="w-full">
            <thead>
              <tr class="border-b border-gray-100 dark:border-dark-600">
                <th class="pb-3 text-left text-sm font-medium text-gray-500 dark:text-gray-400">
                  {{ t('recharge.orderNo') }}
                </th>
                <th class="pb-3 text-left text-sm font-medium text-gray-500 dark:text-gray-400">
                  {{ t('recharge.orderAmount') }}
                </th>
                <th class="pb-3 text-left text-sm font-medium text-gray-500 dark:text-gray-400">
                  {{ t('recharge.orderStatus') }}
                </th>
                <th class="pb-3 text-left text-sm font-medium text-gray-500 dark:text-gray-400">
                  {{ t('recharge.modal.createdAt') }}
                </th>
                <th class="pb-3 text-right text-sm font-medium text-gray-500 dark:text-gray-400">
                  {{ t('recharge.records.actions') }}
                </th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-600">
              <tr v-for="item in orders" :key="item.order_no" class="hover:bg-gray-50 dark:hover:bg-dark-700">
                <td class="py-4">
                  <button
                    type="button"
                    class="font-mono text-sm text-primary-600 hover:text-primary-700 dark:text-primary-400"
                    @click="openDetailModal(item.order_no)"
                  >
                    {{ truncateOrderNo(item.order_no) }}
                  </button>
                </td>
                <td class="py-4 font-medium text-gray-900 dark:text-white">
                  ¥{{ item.amount.toFixed(2) }}
                </td>
                <td class="py-4">
                  <span
                    class="rounded-full px-2 py-0.5 text-xs font-medium"
                    :class="getStatusClass(item.status)"
                  >
                    {{ getStatusText(item.status) }}
                  </span>
                </td>
                <td class="py-4 text-sm text-gray-500 dark:text-gray-400">
                  {{ formatDate(item.created_at) }}
                </td>
                <td class="py-4 text-right">
                  <div class="flex justify-end gap-2">
                    <button
                      type="button"
                      class="btn btn-outline btn-sm"
                      @click="openDetailModal(item.order_no)"
                    >
                      {{ t('recharge.records.viewDetail') }}
                    </button>
                    <button
                      v-if="canRepay(item)"
                      type="button"
                      class="btn btn-primary btn-sm"
                      @click="openPaymentModal(item.order_no)"
                    >
                      {{ t('recharge.modal.repay') }}
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- 加载更多 -->
        <div v-if="hasMoreOrders" class="mt-4 text-center">
          <button
            type="button"
            class="text-sm text-primary-600 hover:text-primary-700 dark:text-primary-400"
            :disabled="ordersLoading"
            @click="loadMoreOrders"
          >
            {{ ordersLoading ? t('common.loading') : t('recharge.records.loadMore') }}
          </button>
        </div>
      </div>
    </div>

    <!-- 支付弹框 -->
    <PaymentModal
      v-model:visible="paymentModalVisible"
      :order-no="currentPaymentOrderNo"
      @paid="onPaymentSuccess"
      @expired="onPaymentExpired"
      @close="onPaymentModalClose"
    />

    <!-- 订单详情弹框 -->
    <OrderDetailModal
      v-model:visible="detailModalVisible"
      :order-no="currentDetailOrderNo"
      @repay="openPaymentModal"
      @status-changed="loadOrders"
    />
  </main>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useRechargeStore, useAppStore } from '@/stores'
import { rechargeAPI, isRateLimitError, isCaptchaRequiredError, type OrderListItem } from '@/api'
import AmountSelector from '@/components/user/recharge/AmountSelector.vue'
import PaymentMethodSelector from '@/components/user/recharge/PaymentMethodSelector.vue'
import TurnstileWidget from '@/components/TurnstileWidget.vue'
import PaymentModal from '@/components/user/recharge/PaymentModal.vue'
import OrderDetailModal from '@/components/user/recharge/OrderDetailModal.vue'

const { t } = useI18n()
const authStore = useAuthStore()
const rechargeStore = useRechargeStore()
const appStore = useAppStore()

// 页面加载状态
const loading = ref(true)

// 选中的充值金额
const selectedAmount = ref<number | null>(null)

// 选中的支付方式
const selectedPaymentMethod = ref<string | null>(null)

// 提交中状态
const submitting = ref(false)

// 错误信息
const errorMessage = ref('')

// 限流倒计时（秒）
const rateLimitCountdown = ref(0)
let rateLimitTimer: ReturnType<typeof setInterval> | null = null

// 日级限流标记
const isDailyLimited = ref(false)

// 验证码相关状态
const showCaptcha = ref(false)
const captchaToken = ref('')
const turnstileSiteKey = ref('')
const turnstileRef = ref<InstanceType<typeof TurnstileWidget> | null>(null)

// 待支付订单（刚创建的）
const pendingOrder = ref<{ order_no: string; amount: number } | null>(null)

// 订单列表相关
const orders = ref<OrderListItem[]>([])
const ordersLoading = ref(false)
const ordersPage = ref(1)
const ordersTotal = ref(0)
const hasMoreOrders = computed(() => orders.value.length < ordersTotal.value)

// 弹框相关
const paymentModalVisible = ref(false)
const currentPaymentOrderNo = ref('')
const detailModalVisible = ref(false)
const currentDetailOrderNo = ref('')

// 用户余额
const balance = computed(() => authStore.user?.balance ?? 0)

// 格式化余额显示（保留两位小数）
const formattedBalance = computed(() => balance.value.toFixed(2))

// 预览到账额度（汇率转换后）
const creditedAmountPreview = computed(() => {
  if (!selectedAmount.value || !rechargeStore.exchangeRate || rechargeStore.exchangeRate <= 0) {
    return '0.00'
  }
  return (selectedAmount.value / rechargeStore.exchangeRate).toFixed(2)
})

// 金额有效性验证
const isAmountValid = computed(() => {
  if (selectedAmount.value === null) {
    return false
  }
  const amount = selectedAmount.value
  return amount >= rechargeStore.minAmount && amount <= rechargeStore.maxAmount
})

// 支付方式有效性验证
const isPaymentMethodValid = computed(() => {
  return selectedPaymentMethod.value !== null
})

// 是否可以提交
const canSubmit = computed(() => {
  return isAmountValid.value && isPaymentMethodValid.value && !isDailyLimited.value
})

// 提交按钮文案
const submitButtonText = computed(() => {
  if (isDailyLimited.value) {
    return t('recharge.dailyLimitReached')
  }
  if (!isPaymentMethodValid.value) {
    return t('recharge.selectPaymentMethodFirst')
  }
  if (selectedAmount.value !== null && isAmountValid.value) {
    return t('recharge.submitButton', { amount: selectedAmount.value })
  }
  return t('recharge.submitButtonDefault')
})

// 启动限流倒计时
const startRateLimitCountdown = (seconds: number) => {
  if (rateLimitTimer) {
    clearInterval(rateLimitTimer)
  }

  rateLimitCountdown.value = seconds
  rateLimitTimer = setInterval(() => {
    rateLimitCountdown.value--
    if (rateLimitCountdown.value <= 0) {
      if (rateLimitTimer) {
        clearInterval(rateLimitTimer)
        rateLimitTimer = null
      }
      errorMessage.value = ''
    }
  }, 1000)
}

// Turnstile 回调处理
const onTurnstileVerify = (token: string) => {
  captchaToken.value = token
}

const onTurnstileExpire = () => {
  captchaToken.value = ''
}

const onTurnstileError = () => {
  captchaToken.value = ''
  errorMessage.value = t('recharge.captchaError')
}

// 重置验证码
const resetCaptcha = () => {
  captchaToken.value = ''
  if (turnstileRef.value) {
    turnstileRef.value.reset()
  }
}

// 提交处理
const handleSubmit = async () => {
  if (!canSubmit.value || submitting.value || rateLimitCountdown.value > 0) {
    return
  }

  if (showCaptcha.value && !captchaToken.value) {
    errorMessage.value = t('recharge.captchaRequired')
    return
  }

  errorMessage.value = ''
  submitting.value = true

  try {
    const order = await rechargeAPI.createOrder({
      amount: selectedAmount.value!,
      payment_method: selectedPaymentMethod.value!,
      captcha_token: captchaToken.value || undefined
    })

    // 订单创建成功，显示提示并打开支付弹框
    appStore.showSuccess(t('recharge.orderCreateSuccess'))
    pendingOrder.value = { order_no: order.order_no, amount: order.amount }
    currentPaymentOrderNo.value = order.order_no
    paymentModalVisible.value = true

    // 刷新订单列表
    loadOrders()
  } catch (error) {
    console.error('Failed to create order:', error)
    if (isCaptchaRequiredError(error)) {
      showCaptcha.value = true
      errorMessage.value = error.message
      if (!turnstileSiteKey.value) {
        const settings = await appStore.fetchPublicSettings()
        if (settings?.turnstile_site_key) {
          turnstileSiteKey.value = settings.turnstile_site_key
        }
      }
    } else if (isRateLimitError(error)) {
      errorMessage.value = error.message
      if (error.isDaily) {
        isDailyLimited.value = true
      } else if (error.retryAfter) {
        startRateLimitCountdown(error.retryAfter)
      }
      resetCaptcha()
    } else {
      errorMessage.value = t('recharge.orderCreateFailed')
      resetCaptcha()
    }
  } finally {
    submitting.value = false
  }
}

// 加载订单列表
const loadOrders = async () => {
  ordersLoading.value = true
  ordersPage.value = 1
  try {
    const result = await rechargeAPI.listOrders({ page: 1, page_size: 10 })
    orders.value = result.orders
    ordersTotal.value = result.total
  } catch (error) {
    console.error('Failed to load orders:', error)
  } finally {
    ordersLoading.value = false
  }
}

// 加载更多订单
const loadMoreOrders = async () => {
  if (ordersLoading.value) return
  ordersLoading.value = true
  try {
    const nextPage = ordersPage.value + 1
    const result = await rechargeAPI.listOrders({ page: nextPage, page_size: 10 })
    orders.value = [...orders.value, ...result.orders]
    ordersPage.value = nextPage
    ordersTotal.value = result.total
  } catch (error) {
    console.error('Failed to load more orders:', error)
  } finally {
    ordersLoading.value = false
  }
}

// 打开支付弹框
const openPaymentModal = (orderNo: string) => {
  currentPaymentOrderNo.value = orderNo
  paymentModalVisible.value = true
}

// 打开订单详情弹框
const openDetailModal = (orderNo: string) => {
  currentDetailOrderNo.value = orderNo
  detailModalVisible.value = true
}

// 支付成功
const onPaymentSuccess = () => {
  pendingOrder.value = null
  authStore.refreshUser()
  loadOrders()
}

// 支付过期
const onPaymentExpired = () => {
  pendingOrder.value = null
  loadOrders()
}

// 支付弹框关闭
const onPaymentModalClose = () => {
  // 关闭弹框后，如果有待支付订单，显示提示条
  // pendingOrder 已经在创建时设置
}

// 判断订单是否可以重新支付
const canRepay = (item: OrderListItem) => {
  if (item.status !== 'pending') return false
  // 需要获取完整订单信息来判断是否过期
  // 这里简单处理：如果状态是 pending 就认为可以支付
  return true
}

// 获取状态样式
const getStatusClass = (status: string) => {
  switch (status) {
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
}

// 获取状态文案
const getStatusText = (status: string) => {
  switch (status) {
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
}

// 截断订单号显示
const truncateOrderNo = (orderNo: string) => {
  if (orderNo.length <= 20) return orderNo
  return orderNo.slice(0, 16) + '...' + orderNo.slice(-4)
}

// 格式化日期
const formatDate = (dateStr: string) => {
  const date = new Date(dateStr)
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  })
}

// 页面加载时刷新用户数据以获取最新余额
onMounted(async () => {
  try {
    await Promise.all([
      authStore.refreshUser(),
      rechargeStore.fetchConfig(),
      loadOrders()
    ])
  } catch (error) {
    console.error('Failed to refresh data:', error)
  } finally {
    loading.value = false
  }
})

// 组件卸载时清理计时器
onUnmounted(() => {
  if (rateLimitTimer) {
    clearInterval(rateLimitTimer)
  }
})
</script>

<style scoped>
/* 余额卡片渐变背景 */
.balance-card {
  background: linear-gradient(135deg, #d97757 0%, #c45a3a 100%);
}

/* 按钮小尺寸 */
.btn-sm {
  padding: 0.375rem 0.75rem;
  font-size: 0.75rem;
}
</style>
