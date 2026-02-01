<template>
  <main class="container mx-auto max-w-6xl px-4 py-6">
    <!-- 页面标题 -->
    <div class="mb-6">
      <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
        {{ t('admin.recharge.title') }}
      </h1>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.recharge.description') }}
      </p>
    </div>

    <!-- 筛选器 -->
    <div class="mb-6 rounded-2xl bg-white p-4 shadow-card dark:bg-dark-800">
      <div class="flex flex-wrap items-end gap-4">
        <!-- 用户ID筛选 -->
        <div class="flex-1 min-w-[150px]">
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            {{ t('admin.recharge.filterUserId') }}
          </label>
          <input
            v-model="filters.userId"
            type="number"
            min="1"
            :placeholder="t('admin.recharge.filterUserIdPlaceholder')"
            class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500 dark:border-gray-600 dark:bg-dark-700 dark:text-white"
            @keyup.enter="handleSearch"
          />
        </div>

        <!-- 状态筛选 -->
        <div class="flex-1 min-w-[150px]">
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            {{ t('rechargeRecords.filterStatus') }}
          </label>
          <select
            v-model="filters.status"
            class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500 dark:border-gray-600 dark:bg-dark-700 dark:text-white"
            @change="handleSearch"
          >
            <option value="">{{ t('rechargeRecords.allStatus') }}</option>
            <option value="pending">{{ t('recharge.statusPending') }}</option>
            <option value="paid">{{ t('recharge.statusPaid') }}</option>
            <option value="failed">{{ t('recharge.statusFailed') }}</option>
            <option value="expired">{{ t('recharge.statusExpired') }}</option>
            <option value="cancelled">{{ t('recharge.statusCancelled') }}</option>
            <option value="refunded">{{ t('recharge.statusRefunded') }}</option>
          </select>
        </div>

        <!-- 搜索按钮 -->
        <button
          type="button"
          class="btn btn-primary"
          @click="handleSearch"
        >
          {{ t('common.search') }}
        </button>

        <!-- 重置按钮 -->
        <button
          type="button"
          class="px-4 py-2 text-sm text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white"
          @click="resetFilters"
        >
          {{ t('rechargeRecords.resetFilters') }}
        </button>
      </div>
    </div>

    <!-- 加载状态 -->
    <div v-if="loading" class="flex flex-col items-center justify-center py-20">
      <div class="h-8 w-8 animate-spin rounded-full border-4 border-primary-500 border-t-transparent"></div>
      <span class="mt-4 text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</span>
    </div>

    <!-- 空状态 -->
    <div
      v-else-if="orders.length === 0"
      class="flex flex-col items-center justify-center py-20 text-center"
    >
      <svg class="h-16 w-16 text-gray-300 dark:text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
      </svg>
      <h3 class="mt-4 text-lg font-medium text-gray-900 dark:text-white">
        {{ t('rechargeRecords.noRecords') }}
      </h3>
      <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.recharge.noRecordsDesc') }}
      </p>
    </div>

    <!-- 订单表格 -->
    <div v-else class="overflow-hidden rounded-2xl bg-white shadow-card dark:bg-dark-800">
      <table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
        <thead class="bg-gray-50 dark:bg-dark-700">
          <tr>
            <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
              {{ t('admin.recharge.orderNo') }}
            </th>
            <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
              {{ t('admin.recharge.userId') }}
            </th>
            <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
              {{ t('admin.recharge.amount') }}
            </th>
            <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
              {{ t('admin.recharge.status') }}
            </th>
            <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
              {{ t('admin.recharge.createdAt') }}
            </th>
            <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
              {{ t('admin.recharge.actions') }}
            </th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-200 dark:divide-gray-700">
          <tr v-for="order in orders" :key="order.order_no" class="hover:bg-gray-50 dark:hover:bg-dark-700">
            <td class="px-4 py-3">
              <span class="font-mono text-sm text-gray-900 dark:text-white">
                {{ order.order_no }}
              </span>
            </td>
            <td class="px-4 py-3 text-sm text-gray-500 dark:text-gray-400">
              {{ order.user_id }}
            </td>
            <td class="px-4 py-3 text-sm font-medium text-gray-900 dark:text-white">
              ¥{{ order.amount.toFixed(2) }}
            </td>
            <td class="px-4 py-3">
              <span
                class="rounded-full px-2 py-0.5 text-xs font-medium"
                :class="getStatusClass(order.status)"
              >
                {{ getStatusText(order.status) }}
              </span>
            </td>
            <td class="px-4 py-3 text-sm text-gray-500 dark:text-gray-400">
              {{ formatDateTime(order.created_at) }}
            </td>
            <td class="px-4 py-3">
              <div class="flex gap-2">
                <button
                  type="button"
                  class="text-sm text-primary-600 hover:text-primary-800 dark:text-primary-400"
                  @click="showOrderDetail(order.order_no)"
                >
                  {{ t('common.view') }}
                </button>
                <button
                  v-if="order.status === 'paid'"
                  type="button"
                  class="text-sm text-red-600 hover:text-red-800 dark:text-red-400"
                  @click="openRefundDialog(order)"
                >
                  {{ t('admin.recharge.refund') }}
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>

      <!-- 分页 -->
      <div v-if="pagination.pages > 1" class="flex items-center justify-between border-t border-gray-200 px-4 py-3 dark:border-gray-700">
        <div class="text-sm text-gray-500 dark:text-gray-400">
          {{ t('rechargeRecords.totalRecords', { count: pagination.total }) }}
        </div>
        <div class="flex gap-2">
          <button
            type="button"
            :disabled="pagination.page <= 1"
            class="rounded-lg border border-gray-300 px-3 py-1.5 text-sm dark:border-gray-600"
            :class="pagination.page <= 1 ? 'opacity-50 cursor-not-allowed' : 'hover:bg-gray-100 dark:hover:bg-dark-700'"
            @click="goToPage(pagination.page - 1)"
          >
            {{ t('common.previousPage') }}
          </button>
          <span class="flex items-center text-sm text-gray-500 dark:text-gray-400">
            {{ pagination.page }} / {{ pagination.pages }}
          </span>
          <button
            type="button"
            :disabled="pagination.page >= pagination.pages"
            class="rounded-lg border border-gray-300 px-3 py-1.5 text-sm dark:border-gray-600"
            :class="pagination.page >= pagination.pages ? 'opacity-50 cursor-not-allowed' : 'hover:bg-gray-100 dark:hover:bg-dark-700'"
            @click="goToPage(pagination.page + 1)"
          >
            {{ t('common.nextPage') }}
          </button>
        </div>
      </div>
    </div>

    <!-- 订单详情弹窗 -->
    <div
      v-if="showDetail"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
      @click.self="showDetail = false"
    >
      <div class="w-full max-w-lg rounded-2xl bg-white p-6 dark:bg-dark-800">
        <div class="mb-4 flex items-center justify-between">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('admin.recharge.orderDetail') }}
          </h3>
          <button
            type="button"
            class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
            @click="showDetail = false"
          >
            <svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <div v-if="detailLoading" class="flex justify-center py-8">
          <div class="h-8 w-8 animate-spin rounded-full border-4 border-primary-500 border-t-transparent"></div>
        </div>

        <div v-else-if="orderDetail" class="space-y-4">
          <div class="grid grid-cols-2 gap-4 text-sm">
            <div>
              <span class="text-gray-500 dark:text-gray-400">{{ t('admin.recharge.orderNo') }}:</span>
              <span class="ml-2 font-mono text-gray-900 dark:text-white">{{ orderDetail.order_no }}</span>
            </div>
            <div>
              <span class="text-gray-500 dark:text-gray-400">{{ t('admin.recharge.userId') }}:</span>
              <span class="ml-2 text-gray-900 dark:text-white">{{ orderDetail.user_id }}</span>
            </div>
            <div>
              <span class="text-gray-500 dark:text-gray-400">{{ t('admin.recharge.amount') }}:</span>
              <span class="ml-2 font-medium text-gray-900 dark:text-white">¥{{ orderDetail.amount.toFixed(2) }}</span>
            </div>
            <div>
              <span class="text-gray-500 dark:text-gray-400">{{ t('admin.recharge.status') }}:</span>
              <span
                class="ml-2 rounded-full px-2 py-0.5 text-xs font-medium"
                :class="getStatusClass(orderDetail.status)"
              >
                {{ getStatusText(orderDetail.status) }}
              </span>
            </div>
            <div>
              <span class="text-gray-500 dark:text-gray-400">{{ t('admin.recharge.paymentMethod') }}:</span>
              <span class="ml-2 text-gray-900 dark:text-white">{{ orderDetail.payment_method }}</span>
            </div>
            <div>
              <span class="text-gray-500 dark:text-gray-400">{{ t('admin.recharge.createdAt') }}:</span>
              <span class="ml-2 text-gray-900 dark:text-white">{{ formatDateTime(orderDetail.created_at) }}</span>
            </div>
            <div v-if="orderDetail.paid_at">
              <span class="text-gray-500 dark:text-gray-400">{{ t('admin.recharge.paidAt') }}:</span>
              <span class="ml-2 text-gray-900 dark:text-white">{{ formatDateTime(orderDetail.paid_at) }}</span>
            </div>
            <div v-if="orderDetail.transaction_id">
              <span class="text-gray-500 dark:text-gray-400">{{ t('admin.recharge.transactionId') }}:</span>
              <span class="ml-2 font-mono text-xs text-gray-900 dark:text-white">{{ orderDetail.transaction_id }}</span>
            </div>
            <div v-if="orderDetail.notes" class="col-span-2">
              <span class="text-gray-500 dark:text-gray-400">{{ t('admin.recharge.notes') }}:</span>
              <span class="ml-2 text-gray-900 dark:text-white">{{ orderDetail.notes }}</span>
            </div>
          </div>

          <!-- 退款按钮 -->
          <div v-if="orderDetail.status === 'paid'" class="mt-6 flex justify-end">
            <button
              type="button"
              class="btn bg-red-600 text-white hover:bg-red-700"
              @click="openRefundDialog(orderDetail)"
            >
              {{ t('admin.recharge.refund') }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- 退款确认弹窗 -->
    <div
      v-if="showRefundDialog"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
      @click.self="showRefundDialog = false"
    >
      <div class="w-full max-w-md rounded-2xl bg-white p-6 dark:bg-dark-800">
        <div class="mb-4 flex items-center justify-between">
          <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
            {{ t('admin.recharge.refundConfirm') }}
          </h3>
          <button
            type="button"
            class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
            @click="showRefundDialog = false"
          >
            <svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <div class="mb-4 text-sm text-gray-600 dark:text-gray-300">
          <p>{{ t('admin.recharge.refundConfirmMessage', { orderNo: refundOrder?.order_no, amount: refundOrder?.amount.toFixed(2) }) }}</p>
        </div>

        <div class="mb-4">
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            {{ t('admin.recharge.refundReason') }} *
          </label>
          <textarea
            v-model="refundReason"
            rows="3"
            :placeholder="t('admin.recharge.refundReasonPlaceholder')"
            class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500 dark:border-gray-600 dark:bg-dark-700 dark:text-white"
          ></textarea>
        </div>

        <div class="flex justify-end gap-3">
          <button
            type="button"
            class="btn btn-secondary"
            @click="showRefundDialog = false"
          >
            {{ t('common.cancel') }}
          </button>
          <button
            type="button"
            class="btn bg-red-600 text-white hover:bg-red-700"
            :disabled="refunding || !refundReason.trim() || refundReason.trim().length < 2"
            @click="handleRefund"
          >
            <span v-if="refunding" class="flex items-center gap-2">
              <div class="h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent"></div>
              {{ t('common.processing') }}
            </span>
            <span v-else>{{ t('admin.recharge.confirmRefund') }}</span>
          </button>
        </div>
      </div>
    </div>
  </main>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { AdminRechargeOrderListItem, AdminRechargeOrder } from '@/api/admin/recharge'

const { t } = useI18n()
const appStore = useAppStore()

// 加载状态
const loading = ref(true)

// 订单列表
const orders = ref<AdminRechargeOrderListItem[]>([])

// 分页信息
const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0,
  pages: 0
})

// 筛选条件
const filters = reactive({
  userId: '',
  status: ''
})

// 订单详情
const showDetail = ref(false)
const detailLoading = ref(false)
const orderDetail = ref<AdminRechargeOrder | null>(null)

// 退款相关
const showRefundDialog = ref(false)
const refundOrder = ref<AdminRechargeOrderListItem | AdminRechargeOrder | null>(null)
const refundReason = ref('')
const refunding = ref(false)

// 加载订单列表
async function loadOrders() {
  loading.value = true
  try {
    const result = await adminAPI.recharge.listOrders(
      pagination.page,
      pagination.pageSize,
      {
        status: filters.status || undefined,
        user_id: filters.userId ? parseInt(filters.userId) : undefined
      }
    )
    orders.value = result.orders
    pagination.total = result.total
    pagination.pages = Math.ceil(result.total / pagination.pageSize)
  } catch (error: any) {
    appStore.showError(error.message || t('common.loadFailed'))
  } finally {
    loading.value = false
  }
}

// 搜索
function handleSearch() {
  pagination.page = 1
  loadOrders()
}

// 重置筛选
function resetFilters() {
  filters.userId = ''
  filters.status = ''
  pagination.page = 1
  loadOrders()
}

// 翻页
function goToPage(page: number) {
  if (page < 1 || page > pagination.pages) return
  pagination.page = page
  loadOrders()
}

// 显示订单详情
async function showOrderDetail(orderNo: string) {
  showDetail.value = true
  detailLoading.value = true
  orderDetail.value = null
  try {
    orderDetail.value = await adminAPI.recharge.getOrder(orderNo)
  } catch (error: any) {
    appStore.showError(error.message || t('common.loadFailed'))
    showDetail.value = false
  } finally {
    detailLoading.value = false
  }
}

// 打开退款对话框
function openRefundDialog(order: AdminRechargeOrderListItem | AdminRechargeOrder) {
  refundOrder.value = order
  refundReason.value = ''
  showRefundDialog.value = true
}

// 执行退款
async function handleRefund() {
  if (!refundOrder.value || !refundReason.value.trim()) return

  refunding.value = true
  try {
    await adminAPI.recharge.refundOrder(refundOrder.value.order_no, {
      reason: refundReason.value.trim()
    })
    appStore.showSuccess(t('admin.recharge.refundSuccess'))
    showRefundDialog.value = false
    showDetail.value = false
    loadOrders()
  } catch (error: any) {
    appStore.showError(error.message || t('admin.recharge.refundFailed'))
  } finally {
    refunding.value = false
  }
}

// 状态样式
function getStatusClass(status: string): string {
  const classes: Record<string, string> = {
    pending: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400',
    paid: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400',
    failed: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400',
    expired: 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-400',
    cancelled: 'bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-400',
    refunded: 'bg-orange-100 text-orange-800 dark:bg-orange-900/30 dark:text-orange-400'
  }
  return classes[status] || classes.pending
}

// 状态文本
function getStatusText(status: string): string {
  const texts: Record<string, string> = {
    pending: t('recharge.statusPending'),
    paid: t('recharge.statusPaid'),
    failed: t('recharge.statusFailed'),
    expired: t('recharge.statusExpired'),
    cancelled: t('recharge.statusCancelled'),
    refunded: t('recharge.statusRefunded')
  }
  return texts[status] || status
}

// 格式化日期时间
function formatDateTime(dateStr: string): string {
  const date = new Date(dateStr)
  return date.toLocaleString()
}

onMounted(() => {
  loadOrders()
})
</script>
