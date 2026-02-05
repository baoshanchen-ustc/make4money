<template>
  <AppLayout>
    <TablePageLayout>
      <!-- Filters Section -->
      <template #filters>
        <div class="flex flex-wrap items-start justify-between gap-4">
          <!-- Left: Group selector + date range + search -->
          <div class="flex flex-1 flex-wrap items-center gap-3">
            <!-- Group Selector -->
            <div class="w-full sm:w-48">
              <Select
                v-model="selectedGroupId"
                :options="groupOptions"
                :placeholder="t('admin.balance.selectGroup')"
                @change="handleGroupChange"
              />
            </div>

            <!-- Date Range Picker -->
            <div class="w-full sm:w-auto">
              <DateRangePicker
                :start-date="startDate"
                :end-date="endDate"
                @update:startDate="updateStartDate"
                @update:endDate="updateEndDate"
                @change="handleDateChange"
              />
            </div>

            <!-- Search Input -->
            <div class="relative w-full sm:w-64">
              <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
              <input
                v-model="searchKeyword"
                type="text"
                :placeholder="t('admin.balance.searchPlaceholder')"
                class="input pl-10"
                @input="debounceSearch"
              />
            </div>
          </div>

          <!-- Right: Refresh -->
          <div class="ml-auto flex items-center gap-3">
            <button @click="loadData" :disabled="loading" class="btn btn-secondary">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
          </div>
        </div>
      </template>

      <!-- Table Section -->
      <template #table>
        <!-- No standard groups hint -->
        <div v-if="!groupsLoading && standardGroups.length === 0" class="flex items-center justify-center py-20">
          <EmptyState
            :title="t('admin.balance.noStandardGroups')"
            :description="t('admin.balance.noStandardGroupsDesc')"
          />
        </div>

        <!-- No group selected -->
        <div v-else-if="!selectedGroupId" class="flex items-center justify-center py-20">
          <EmptyState
            :title="t('admin.balance.selectGroupHint')"
            :description="t('admin.balance.selectGroupHintDesc')"
          />
        </div>

        <!-- Data Table -->
        <DataTable
          v-else
          :columns="columns"
          :data="users"
          :loading="loading"
          :server-side-sort="true"
          :actions-count="3"
          sort-storage-key="admin-balance"
          @sort="handleSort"
        >
          <!-- User column -->
          <template #cell-email="{ row }">
            <div class="flex items-center gap-2">
              <div class="flex h-8 w-8 items-center justify-center rounded-full bg-primary-100 dark:bg-primary-900/30">
                <span class="text-sm font-medium text-primary-700 dark:text-primary-300">
                  {{ (row.email || row.username || '?').charAt(0).toUpperCase() }}
                </span>
              </div>
              <div>
                <div class="font-medium text-gray-900 dark:text-gray-100">{{ row.email }}</div>
                <div v-if="row.username" class="text-xs text-gray-500">{{ row.username }}</div>
              </div>
            </div>
          </template>

          <!-- Balance column -->
          <template #cell-balance="{ value, row }">
            <div class="flex items-center gap-2">
              <button
                @click="handleBalanceHistory(row)"
                class="font-medium text-gray-900 underline decoration-dashed decoration-gray-300 underline-offset-4 hover:decoration-primary-400 dark:text-gray-100"
              >
                ${{ formatCost(value) }}
              </button>
              <button
                @click.stop="handleDeposit(row)"
                class="text-xs font-medium text-emerald-600 hover:text-emerald-700"
              >
                {{ t('admin.users.deposit') }}
              </button>
            </div>
          </template>

          <!-- Cost columns -->
          <template #cell-total_cost="{ value }">
            <span class="font-medium">${{ formatCost(value) }}</span>
          </template>

          <template #cell-actual_cost="{ value }">
            <span class="font-medium text-green-600">${{ formatCost(value) }}</span>
          </template>

          <!-- Token columns -->
          <template #cell-input_tokens="{ value }">
            {{ formatTokens(value) }}
          </template>

          <template #cell-output_tokens="{ value }">
            {{ formatTokens(value) }}
          </template>

          <template #cell-cache_read_tokens="{ value }">
            {{ formatTokens(value) }}
          </template>

          <!-- Total requests -->
          <template #cell-total_requests="{ value }">
            {{ formatNumber(value) }}
          </template>

          <!-- Actions -->
          <template #cell-actions="{ row }">
            <div class="flex items-center gap-1">
              <button
                @click="handleModelDist(row)"
                class="flex flex-col items-center gap-0.5 rounded px-2 py-1 text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:text-gray-400 dark:hover:bg-dark-600 dark:hover:text-gray-200"
              >
                <Icon name="chart" size="sm" />
                <span class="text-xs">{{ t('admin.balance.modelDist') }}</span>
              </button>
              <button
                @click="handleDeposit(row)"
                class="flex flex-col items-center gap-0.5 rounded px-2 py-1 text-emerald-500 hover:bg-emerald-50 hover:text-emerald-700 dark:hover:bg-emerald-900/20"
              >
                <Icon name="plus" size="sm" />
                <span class="text-xs">{{ t('admin.users.deposit') }}</span>
              </button>
              <button
                @click="handleBalanceHistory(row)"
                class="flex flex-col items-center gap-0.5 rounded px-2 py-1 text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:text-gray-400 dark:hover:bg-dark-600 dark:hover:text-gray-200"
              >
                <Icon name="clock" size="sm" />
                <span class="text-xs">{{ t('admin.balance.history') }}</span>
              </button>
            </div>
          </template>

          <!-- Empty state -->
          <template #empty>
            <EmptyState
              :title="t('admin.balance.noData')"
              :description="t('admin.balance.noDataDesc')"
            />
          </template>
        </DataTable>
      </template>

      <!-- Pagination Section -->
      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          :show-page-size-selector="true"
          :show-jump="false"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <!-- Model Distribution Dialog -->
    <BaseDialog
      :show="showModelDistDialog"
      :title="t('admin.balance.modelDistTitle', { user: modelDistUser?.email || '' })"
      width="wide"
      @close="closeModelDistDialog"
    >
      <div v-if="modelDistLoading" class="flex h-48 items-center justify-center">
        <div class="text-gray-500">{{ t('common.loading') }}...</div>
      </div>
      <div v-else-if="modelStats.length === 0" class="flex h-48 items-center justify-center">
        <div class="text-gray-400">{{ t('admin.balance.noModelData') }}</div>
      </div>
      <div v-else>
        <ModelDistributionChart :model-stats="modelStats" :loading="false" />
      </div>
    </BaseDialog>

    <!-- Balance Modal -->
    <UserBalanceModal
      :show="showBalanceModal"
      :user="balanceUser"
      :operation="balanceOperation"
      @close="closeBalanceModal"
      @success="handleBalanceSuccess"
    />

    <!-- Balance History Modal -->
    <UserBalanceHistoryModal
      :show="showBalanceHistoryModal"
      :user="balanceHistoryUser"
      @close="closeBalanceHistoryModal"
      @deposit="handleDepositFromHistory"
      @withdraw="handleWithdrawFromHistory"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'

import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ModelDistributionChart from '@/components/charts/ModelDistributionChart.vue'
import UserBalanceModal from '@/components/admin/user/UserBalanceModal.vue'
import UserBalanceHistoryModal from '@/components/admin/user/UserBalanceHistoryModal.vue'

import { adminAPI } from '@/api/admin'
import type { BalanceGroupUserStats } from '@/api/admin/balance'
import type { AdminGroup, AdminUser, ModelStat } from '@/types'
import type { Column } from '@/components/common/types'

const { t } = useI18n()

// --- Group selector ---
const groupsLoading = ref(false)
const allGroups = ref<AdminGroup[]>([])
const selectedGroupId = ref<number | null>(null)

const standardGroups = computed(() =>
  allGroups.value.filter(g => g.subscription_type === 'standard' && g.status === 'active')
)

const groupOptions = computed(() =>
  standardGroups.value.map(g => ({
    value: g.id,
    label: g.name
  }))
)

const loadGroups = async () => {
  groupsLoading.value = true
  try {
    allGroups.value = await adminAPI.groups.getAll()
    // Auto-select first standard group
    if (standardGroups.value.length > 0 && !selectedGroupId.value) {
      selectedGroupId.value = standardGroups.value[0].id
    }
  } catch (error) {
    console.error('Failed to load groups:', error)
  } finally {
    groupsLoading.value = false
  }
}

// --- Date range ---
const getDefaultDates = () => {
  const now = new Date()
  const end = now.toISOString().slice(0, 10)
  const start = new Date(now.getTime() - 6 * 24 * 60 * 60 * 1000).toISOString().slice(0, 10)
  return { start, end }
}

const defaultDates = getDefaultDates()
const startDate = ref(defaultDates.start)
const endDate = ref(defaultDates.end)

const updateStartDate = (date: string) => {
  startDate.value = date
}
const updateEndDate = (date: string) => {
  endDate.value = date
}
const handleDateChange = () => {
  // Validate 90 day max
  const s = new Date(startDate.value)
  const e = new Date(endDate.value)
  const diffDays = (e.getTime() - s.getTime()) / (1000 * 60 * 60 * 24)
  if (diffDays > 90) {
    // Adjust start date
    const newStart = new Date(e.getTime() - 90 * 24 * 60 * 60 * 1000)
    startDate.value = newStart.toISOString().slice(0, 10)
  }
  pagination.page = 1
  loadData()
}

// --- Search ---
const searchKeyword = ref('')
let searchTimer: ReturnType<typeof setTimeout> | null = null

const debounceSearch = () => {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    pagination.page = 1
    loadData()
  }, 300)
}

// --- Sort ---
const sortBy = ref('total_cost')
const sortOrder = ref<'asc' | 'desc'>('desc')

const handleSort = (key: string, order: 'asc' | 'desc') => {
  sortBy.value = key
  sortOrder.value = order
  loadData()
}

// --- Pagination ---
const pagination = reactive({
  page: 1,
  page_size: 20,
  total: 0
})

const handlePageChange = (page: number) => {
  pagination.page = page
  loadData()
}

const handlePageSizeChange = (pageSize: number) => {
  pagination.page_size = pageSize
  pagination.page = 1
  loadData()
}

// --- Data ---
const users = ref<BalanceGroupUserStats[]>([])
const loading = ref(false)
let abortController: AbortController | null = null

const loadData = async () => {
  if (!selectedGroupId.value) return
  // Cancel previous request
  if (abortController) abortController.abort()
  abortController = new AbortController()

  loading.value = true
  try {
    const result = await adminAPI.balance.getBalanceGroupUserStats({
      group_id: selectedGroupId.value,
      start_date: startDate.value,
      end_date: endDate.value,
      page: pagination.page,
      page_size: pagination.page_size,
      sort_by: sortBy.value,
      sort_order: sortOrder.value,
      search: searchKeyword.value || undefined
    }, { signal: abortController.signal })
    users.value = result.users || []
    pagination.total = result.total || 0
  } catch (error: any) {
    if (error?.name === 'AbortError' || error?.name === 'CanceledError') return
    console.error('Failed to load balance stats:', error)
    users.value = []
    pagination.total = 0
  } finally {
    loading.value = false
  }
}

const handleGroupChange = () => {
  pagination.page = 1
  loadData()
}

// --- Columns ---
const columns = computed<Column[]>(() => [
  { key: 'email', label: t('admin.balance.columns.user'), sortable: false },
  { key: 'balance', label: t('admin.balance.columns.balance'), sortable: true },
  { key: 'total_cost', label: t('admin.balance.columns.totalCost'), sortable: true },
  { key: 'actual_cost', label: t('admin.balance.columns.actualCost'), sortable: true },
  { key: 'total_requests', label: t('admin.balance.columns.requests'), sortable: true },
  { key: 'input_tokens', label: t('admin.balance.columns.inputTokens'), sortable: true },
  { key: 'output_tokens', label: t('admin.balance.columns.outputTokens'), sortable: true },
  { key: 'cache_read_tokens', label: t('admin.balance.columns.cacheTokens'), sortable: true },
  { key: 'actions', label: t('admin.balance.columns.actions'), sortable: false }
])

// --- Formatting ---
const formatCost = (value: number) => {
  if (value == null) return '0.00'
  return value.toFixed(4)
}

const formatTokens = (value: number) => {
  if (value == null || value === 0) return '0'
  if (value >= 1000000) return (value / 1000000).toFixed(1) + 'M'
  if (value >= 1000) return (value / 1000).toFixed(1) + 'K'
  return value.toString()
}

const formatNumber = (value: number) => {
  if (value == null || value === 0) return '0'
  return value.toLocaleString()
}

// --- Model Distribution Dialog ---
const showModelDistDialog = ref(false)
const modelDistUser = ref<BalanceGroupUserStats | null>(null)
const modelDistLoading = ref(false)
const modelStats = ref<ModelStat[]>([])

const handleModelDist = async (row: BalanceGroupUserStats) => {
  modelDistUser.value = row
  showModelDistDialog.value = true
  modelDistLoading.value = true
  try {
    const result = await adminAPI.dashboard.getModelStats({
      user_id: row.user_id,
      start_date: startDate.value,
      end_date: endDate.value
    })
    modelStats.value = result.models || []
  } catch (error) {
    console.error('Failed to load model stats:', error)
    modelStats.value = []
  } finally {
    modelDistLoading.value = false
  }
}

const closeModelDistDialog = () => {
  showModelDistDialog.value = false
  modelDistUser.value = null
  modelStats.value = []
}

// --- Balance Modal ---
const showBalanceModal = ref(false)
const balanceUser = ref<AdminUser | null>(null)
const balanceOperation = ref<'add' | 'subtract'>('add')

const toAdminUser = (row: BalanceGroupUserStats): AdminUser => {
  return {
    id: row.user_id,
    email: row.email,
    username: row.username,
    balance: row.balance,
    role: 'user',
    concurrency: 0,
    status: 'active',
    allowed_groups: null,
    notes: '',
    created_at: '',
    updated_at: ''
  } as AdminUser
}

const handleDeposit = (row: BalanceGroupUserStats) => {
  balanceUser.value = toAdminUser(row)
  balanceOperation.value = 'add'
  showBalanceModal.value = true
}

const closeBalanceModal = () => {
  showBalanceModal.value = false
  balanceUser.value = null
}

const handleBalanceSuccess = () => {
  closeBalanceModal()
  loadData()
}

// --- Balance History Modal ---
const showBalanceHistoryModal = ref(false)
const balanceHistoryUser = ref<AdminUser | null>(null)

const handleBalanceHistory = (row: BalanceGroupUserStats) => {
  balanceHistoryUser.value = toAdminUser(row)
  showBalanceHistoryModal.value = true
}

const closeBalanceHistoryModal = () => {
  showBalanceHistoryModal.value = false
  balanceHistoryUser.value = null
}

const handleDepositFromHistory = () => {
  if (balanceHistoryUser.value) {
    balanceUser.value = balanceHistoryUser.value
    balanceOperation.value = 'add'
    showBalanceModal.value = true
  }
}

const handleWithdrawFromHistory = () => {
  if (balanceHistoryUser.value) {
    balanceUser.value = balanceHistoryUser.value
    balanceOperation.value = 'subtract'
    showBalanceModal.value = true
  }
}

// --- Lifecycle ---
onMounted(async () => {
  await loadGroups()
  if (selectedGroupId.value) {
    loadData()
  }
})

onUnmounted(() => {
  if (searchTimer) clearTimeout(searchTimer)
  if (abortController) abortController.abort()
})
</script>
