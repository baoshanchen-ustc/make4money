<template>
  <main class="container mx-auto max-w-4xl px-4 py-6">
    <!-- 页面标题 -->
    <div class="mb-6">
      <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
        {{ t('rechargeRecords.title') }}
      </h1>
      <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
        {{ t('rechargeRecords.description') }}
      </p>
    </div>

    <!-- 筛选器 -->
    <div class="mb-6 rounded-2xl bg-white p-4 shadow-card dark:bg-dark-800">
      <div class="flex flex-wrap items-end gap-4">
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
          </select>
        </div>

        <!-- 开始日期 -->
        <div class="flex-1 min-w-[150px]">
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            {{ t('rechargeRecords.filterStartDate') }}
          </label>
          <input
            v-model="filters.startDate"
            type="date"
            class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500 dark:border-gray-600 dark:bg-dark-700 dark:text-white"
            @change="handleSearch"
          />
        </div>

        <!-- 结束日期 -->
        <div class="flex-1 min-w-[150px]">
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            {{ t('rechargeRecords.filterEndDate') }}
          </label>
          <input
            v-model="filters.endDate"
            type="date"
            class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500 dark:border-gray-600 dark:bg-dark-700 dark:text-white"
            @change="handleSearch"
          />
        </div>

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
        {{ t('rechargeRecords.noRecordsDesc') }}
      </p>
      <button
        type="button"
        class="mt-4 btn btn-primary"
        @click="$router.push({ name: 'Recharge' })"
      >
        {{ t('rechargeRecords.goRecharge') }}
      </button>
    </div>

    <!-- 记录列表 -->
    <div v-else class="space-y-4">
      <div
        v-for="order in orders"
        :key="order.order_no"
        class="rounded-2xl bg-white p-4 shadow-card dark:bg-dark-800"
      >
        <div class="flex items-center justify-between">
          <!-- 左侧信息 -->
          <div class="flex-1">
            <div class="flex items-center gap-3">
              <span class="font-mono text-sm text-gray-500 dark:text-gray-400">
                {{ order.order_no }}
              </span>
              <span
                class="rounded-full px-2 py-0.5 text-xs font-medium"
                :class="getStatusClass(order.status)"
              >
                {{ getStatusText(order.status) }}
              </span>
            </div>
            <div class="mt-1 text-xs text-gray-400 dark:text-gray-500">
              {{ formatDateTime(order.created_at) }}
              <span v-if="order.paid_at" class="ml-2">
                {{ t('rechargeRecords.paidAt') }}: {{ formatDateTime(order.paid_at) }}
              </span>
            </div>
          </div>

          <!-- 右侧金额 -->
          <div class="text-right">
            <span class="text-xl font-semibold text-gray-900 dark:text-white">
              ¥{{ order.amount.toFixed(2) }}
            </span>
          </div>
        </div>
      </div>

      <!-- 分页 -->
      <div v-if="pagination.pages > 1" class="flex items-center justify-center gap-2 pt-4">
        <button
          type="button"
          :disabled="pagination.page <= 1"
          class="rounded-lg border border-gray-300 px-3 py-1.5 text-sm dark:border-gray-600"
          :class="pagination.page <= 1 ? 'opacity-50 cursor-not-allowed' : 'hover:bg-gray-100 dark:hover:bg-dark-700'"
          @click="goToPage(pagination.page - 1)"
        >
          {{ t('common.previousPage') }}
        </button>

        <span class="text-sm text-gray-500 dark:text-gray-400">
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

      <!-- 总数显示 -->
      <div class="text-center text-sm text-gray-500 dark:text-gray-400">
        {{ t('rechargeRecords.totalRecords', { count: pagination.total }) }}
      </div>
    </div>
  </main>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { rechargeAPI, type OrderListItem } from '@/api/recharge'

const { t } = useI18n()

// 加载状态
const loading = ref(true)

// 订单列表
const orders = ref<OrderListItem[]>([])

// 分页信息
const pagination = reactive({
  page: 1,
  pageSize: 10,
  total: 0,
  pages: 0
})

// 筛选条件
const filters = reactive({
  status: '',
  startDate: '',
  endDate: ''
})

// 获取状态样式类
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
    default:
      return t('recharge.statusUnknown')
  }
}

// 格式化日期时间
const formatDateTime = (dateStr: string) => {
  const date = new Date(dateStr)
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  })
}

// 加载订单列表
const loadOrders = async () => {
  loading.value = true
  try {
    const result = await rechargeAPI.listOrders({
      page: pagination.page,
      page_size: pagination.pageSize,
      status: filters.status || undefined,
      start_time: filters.startDate || undefined,
      end_time: filters.endDate || undefined
    })

    orders.value = result.orders
    pagination.total = result.total
    pagination.page = result.page
    pagination.pageSize = result.page_size
    pagination.pages = Math.ceil(result.total / result.page_size)
  } catch (error) {
    console.error('Failed to load orders:', error)
  } finally {
    loading.value = false
  }
}

// 搜索处理
const handleSearch = () => {
  pagination.page = 1
  loadOrders()
}

// 重置筛选条件
const resetFilters = () => {
  filters.status = ''
  filters.startDate = ''
  filters.endDate = ''
  pagination.page = 1
  loadOrders()
}

// 跳转到指定页
const goToPage = (page: number) => {
  if (page < 1 || page > pagination.pages) return
  pagination.page = page
  loadOrders()
}

onMounted(() => {
  loadOrders()
})
</script>
