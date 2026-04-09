<template>
  <div class="space-y-6">
    <!-- Tab navigation -->
    <div class="border-b border-gray-200 dark:border-dark-700">
      <nav class="-mb-px flex space-x-6 overflow-x-auto">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          @click="activeTab = tab.key"
          class="whitespace-nowrap border-b-2 px-1 pb-3 text-sm font-medium transition-colors"
          :class="[
            activeTab === tab.key
              ? 'border-primary-500 text-primary-600 dark:text-primary-400'
              : 'border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700 dark:text-dark-400 dark:hover:text-dark-200'
          ]"
        >
          {{ tab.label }}
        </button>
      </nav>
    </div>

    <!-- Dashboard Tab -->
    <template v-if="activeTab === 'dashboard'">
      <div v-if="dashboardLoading" class="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>
      <template v-else-if="dashboardStats">
        <UserDashboardStats :stats="dashboardStats" :balance="0" :is-simple="false" />
        <!-- Trend/Charts section -->
        <div class="card p-4">
          <div class="flex flex-wrap items-center justify-between gap-3 mb-4">
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">使用趋势</h3>
            <div class="flex items-center gap-3">
              <input
                v-model="startDate"
                type="date"
                class="input text-sm"
                @change="loadCharts"
              />
              <span class="text-gray-400">—</span>
              <input
                v-model="endDate"
                type="date"
                class="input text-sm"
                @change="loadCharts"
              />
            </div>
          </div>
          <div v-if="chartsLoading" class="flex items-center justify-center py-8">
            <LoadingSpinner />
          </div>
          <div v-else-if="trendData.length > 0" class="space-y-2">
            <div class="overflow-x-auto">
              <table class="w-full text-sm">
                <thead>
                  <tr class="border-b border-gray-100 dark:border-dark-700">
                    <th class="pb-2 text-left text-xs font-medium text-gray-500 dark:text-dark-400">日期</th>
                    <th class="pb-2 text-right text-xs font-medium text-gray-500 dark:text-dark-400">请求数</th>
                    <th class="pb-2 text-right text-xs font-medium text-gray-500 dark:text-dark-400">Token</th>
                    <th class="pb-2 text-right text-xs font-medium text-gray-500 dark:text-dark-400">费用</th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="point in trendData"
                    :key="point.date"
                    class="border-b border-gray-50 dark:border-dark-800"
                  >
                    <td class="py-1.5 text-gray-700 dark:text-gray-300">{{ point.date }}</td>
                    <td class="py-1.5 text-right text-gray-700 dark:text-gray-300">{{ point.requests }}</td>
                    <td class="py-1.5 text-right text-gray-700 dark:text-gray-300">{{ formatTokens(point.total_tokens) }}</td>
                    <td class="py-1.5 text-right text-gray-700 dark:text-gray-300">${{ point.actual_cost.toFixed(4) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
          <div v-else class="py-8 text-center text-sm text-gray-500 dark:text-dark-400">
            暂无趋势数据
          </div>
        </div>
      </template>
      <div v-else class="py-12 text-center text-sm text-gray-500 dark:text-dark-400">
        加载失败
      </div>
    </template>

    <!-- API Keys Tab -->
    <template v-if="activeTab === 'apikeys'">
      <div v-if="apiKeysLoading" class="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>
      <div v-else class="card overflow-hidden">
        <div class="p-4 border-b border-gray-100 dark:border-dark-700">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
            API 密钥
            <span class="ml-2 text-xs font-normal text-gray-500 dark:text-dark-400">(共 {{ apiKeys.length }} 个)</span>
          </h3>
        </div>
        <div v-if="apiKeys.length > 0" class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead class="bg-gray-50 dark:bg-dark-700/50">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-dark-400">名称</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-dark-400">Key</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-dark-400">状态</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-dark-400">创建时间</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="key in apiKeys" :key="key.id" class="hover:bg-gray-50 dark:hover:bg-dark-700/30">
                <td class="px-4 py-3 font-medium text-gray-900 dark:text-white">{{ key.name }}</td>
                <td class="px-4 py-3 font-mono text-xs text-gray-500 dark:text-dark-400">
                  {{ key.key ? key.key.substring(0, 12) + '...' : '—' }}
                </td>
                <td class="px-4 py-3">
                  <span
                    class="inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium"
                    :class="key.status === 'active'
                      ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                      : 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'"
                  >
                    <span class="h-1.5 w-1.5 rounded-full" :class="key.status === 'active' ? 'bg-green-500' : 'bg-red-500'"></span>
                    {{ key.status === 'active' ? '正常' : '已禁用' }}
                  </span>
                </td>
                <td class="px-4 py-3 text-gray-500 dark:text-dark-400">{{ formatDateTime(key.created_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else class="py-12 text-center text-sm text-gray-500 dark:text-dark-400">
          该用户暂无 API 密钥
        </div>
      </div>
    </template>

    <!-- Usage Tab -->
    <template v-if="activeTab === 'usage'">
      <div v-if="usageLoading" class="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>
      <div v-else class="card overflow-hidden">
        <div class="p-4 border-b border-gray-100 dark:border-dark-700">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
            使用记录
            <span class="ml-2 text-xs font-normal text-gray-500 dark:text-dark-400">(共 {{ usageTotal }} 条)</span>
          </h3>
        </div>
        <div v-if="usageLogs.length > 0" class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead class="bg-gray-50 dark:bg-dark-700/50">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-dark-400">时间</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-dark-400">模型</th>
                <th class="px-4 py-3 text-right text-xs font-medium text-gray-500 dark:text-dark-400">Token</th>
                <th class="px-4 py-3 text-right text-xs font-medium text-gray-500 dark:text-dark-400">费用</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-dark-400">状态</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="log in usageLogs" :key="log.id" class="hover:bg-gray-50 dark:hover:bg-dark-700/30">
                <td class="px-4 py-3 text-gray-500 dark:text-dark-400 text-xs">{{ formatDateTime(log.created_at) }}</td>
                <td class="px-4 py-3 font-mono text-xs text-gray-700 dark:text-gray-300">{{ log.model }}</td>
                <td class="px-4 py-3 text-right text-gray-700 dark:text-gray-300">{{ formatTokens((log.input_tokens || 0) + (log.output_tokens || 0)) }}</td>
                <td class="px-4 py-3 text-right text-gray-700 dark:text-gray-300">${{ (log.actual_cost || 0).toFixed(4) }}</td>
                <td class="px-4 py-3">
                  <span
                    class="inline-flex rounded-full px-2 py-0.5 text-xs font-medium"
                    :class="log.status === 'success'
                      ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                      : 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'"
                  >
                    {{ log.status === 'success' ? '成功' : '失败' }}
                  </span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else class="py-12 text-center text-sm text-gray-500 dark:text-dark-400">
          暂无使用记录
        </div>
      </div>
    </template>

    <!-- Subscriptions Tab -->
    <template v-if="activeTab === 'subscriptions'">
      <div v-if="subsLoading" class="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>
      <div v-else class="card overflow-hidden">
        <div class="p-4 border-b border-gray-100 dark:border-dark-700">
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
            订阅
            <span class="ml-2 text-xs font-normal text-gray-500 dark:text-dark-400">(共 {{ subscriptions.length }} 个)</span>
          </h3>
        </div>
        <div v-if="subscriptions.length > 0" class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead class="bg-gray-50 dark:bg-dark-700/50">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-dark-400">分组</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-dark-400">平台</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-dark-400">状态</th>
                <th class="px-4 py-3 text-left text-xs font-medium text-gray-500 dark:text-dark-400">到期时间</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="sub in subscriptions" :key="sub.id" class="hover:bg-gray-50 dark:hover:bg-dark-700/30">
                <td class="px-4 py-3 font-medium text-gray-900 dark:text-white">{{ sub.group?.name || '—' }}</td>
                <td class="px-4 py-3 text-gray-500 dark:text-dark-400">{{ sub.group?.platform || '—' }}</td>
                <td class="px-4 py-3">
                  <span
                    class="inline-flex rounded-full px-2 py-0.5 text-xs font-medium"
                    :class="sub.status === 'active'
                      ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                      : 'bg-gray-100 text-gray-600 dark:bg-dark-600 dark:text-dark-300'"
                  >
                    {{ sub.status === 'active' ? '有效' : sub.status === 'expired' ? '已过期' : '已撤销' }}
                  </span>
                </td>
                <td class="px-4 py-3 text-gray-500 dark:text-dark-400">
                  {{ sub.expires_at ? formatDateTime(sub.expires_at) : '永不过期' }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else class="py-12 text-center text-sm text-gray-500 dark:text-dark-400">
          该用户暂无订阅
        </div>
      </div>
    </template>

    <!-- Profile Tab -->
    <template v-if="activeTab === 'profile'">
      <div v-if="profileLoading" class="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>
      <div v-else-if="userProfile" class="card p-6 space-y-4">
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white mb-4">用户信息</h3>
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <p class="text-xs text-gray-500 dark:text-dark-400">邮箱</p>
            <p class="mt-1 text-sm font-medium text-gray-900 dark:text-white">{{ userProfile.email }}</p>
          </div>
          <div>
            <p class="text-xs text-gray-500 dark:text-dark-400">用户名</p>
            <p class="mt-1 text-sm font-medium text-gray-900 dark:text-white">{{ userProfile.username || '—' }}</p>
          </div>
          <div>
            <p class="text-xs text-gray-500 dark:text-dark-400">用户ID</p>
            <p class="mt-1 text-sm font-medium text-gray-900 dark:text-white">{{ userProfile.id }}</p>
          </div>
          <div>
            <p class="text-xs text-gray-500 dark:text-dark-400">角色</p>
            <p class="mt-1 text-sm font-medium text-gray-900 dark:text-white">
              <span :class="['badge', userProfile.role === 'admin' ? 'badge-purple' : 'badge-gray']">
                {{ userProfile.role === 'admin' ? '管理员' : '用户' }}
              </span>
            </p>
          </div>
          <div>
            <p class="text-xs text-gray-500 dark:text-dark-400">状态</p>
            <p class="mt-1 flex items-center gap-1.5 text-sm font-medium text-gray-900 dark:text-white">
              <span
                class="inline-block h-2 w-2 rounded-full"
                :class="userProfile.status === 'active' ? 'bg-green-500' : 'bg-red-500'"
              ></span>
              {{ userProfile.status === 'active' ? '正常' : '已禁用' }}
            </p>
          </div>
          <div>
            <p class="text-xs text-gray-500 dark:text-dark-400">余额</p>
            <p class="mt-1 text-sm font-medium text-emerald-600 dark:text-emerald-400">
              ${{ userProfile.balance?.toFixed(2) ?? '0.00' }}
            </p>
          </div>
          <div>
            <p class="text-xs text-gray-500 dark:text-dark-400">并发数</p>
            <p class="mt-1 text-sm font-medium text-gray-900 dark:text-white">{{ userProfile.concurrency }}</p>
          </div>
          <div>
            <p class="text-xs text-gray-500 dark:text-dark-400">注册时间</p>
            <p class="mt-1 text-sm font-medium text-gray-900 dark:text-white">{{ formatDateTime(userProfile.created_at) }}</p>
          </div>
        </div>
        <div v-if="userProfile.notes">
          <p class="text-xs text-gray-500 dark:text-dark-400">备注</p>
          <p class="mt-1 text-sm text-gray-700 dark:text-gray-300">{{ userProfile.notes }}</p>
        </div>
      </div>
      <div v-else class="py-12 text-center text-sm text-gray-500 dark:text-dark-400">
        加载失败
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { formatDateTime } from '@/utils/format'
import { adminUserViewAPI } from '@/api/admin/userView'
import { adminAPI } from '@/api/admin'
import { adminUsageAPI } from '@/api/admin/usage'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import UserDashboardStats from '@/components/user/dashboard/UserDashboardStats.vue'
import type { UserDashboardStats as UserStatsType } from '@/api/usage'
import type { TrendDataPoint, ModelStat, ApiKey, UserSubscription, AdminUser } from '@/types'

const props = defineProps<{
  userId: number
}>()

type TabKey = 'dashboard' | 'apikeys' | 'usage' | 'subscriptions' | 'profile'

const tabs: { key: TabKey; label: string }[] = [
  { key: 'dashboard', label: '仪表盘' },
  { key: 'apikeys', label: 'API 密钥' },
  { key: 'usage', label: '使用记录' },
  { key: 'subscriptions', label: '订阅' },
  { key: 'profile', label: '资料' },
]

const activeTab = ref<TabKey>('dashboard')

// ---- Dashboard ----
const dashboardStats = ref<UserStatsType | null>(null)
const dashboardLoading = ref(false)
const trendData = ref<TrendDataPoint[]>([])
const chartsLoading = ref(false)

const formatLD = (d: Date) => d.toISOString().split('T')[0]
const startDate = ref(formatLD(new Date(Date.now() - 6 * 86400000)))
const endDate = ref(formatLD(new Date()))

const loadDashboard = async () => {
  dashboardLoading.value = true
  try {
    dashboardStats.value = await adminUserViewAPI.getDashboardStats(props.userId)
  } catch (error) {
    console.error('Failed to load user dashboard stats:', error)
  } finally {
    dashboardLoading.value = false
  }
}

const loadCharts = async () => {
  chartsLoading.value = true
  try {
    const res = await adminUserViewAPI.getDashboardTrend(props.userId, {
      start_date: startDate.value,
      end_date: endDate.value,
      granularity: 'day'
    })
    trendData.value = res.trend || []
  } catch (error) {
    console.error('Failed to load trend data:', error)
  } finally {
    chartsLoading.value = false
  }
}

// ---- API Keys ----
const apiKeys = ref<ApiKey[]>([])
const apiKeysLoading = ref(false)

const loadApiKeys = async () => {
  if (apiKeys.value.length > 0) return
  apiKeysLoading.value = true
  try {
    const res = await adminAPI.users.getUserApiKeys(props.userId)
    apiKeys.value = res.items
  } catch (error) {
    console.error('Failed to load user API keys:', error)
  } finally {
    apiKeysLoading.value = false
  }
}

// ---- Usage ----
const usageLogs = ref<any[]>([])
const usageTotal = ref(0)
const usageLoading = ref(false)

const loadUsage = async () => {
  if (usageLogs.value.length > 0) return
  usageLoading.value = true
  try {
    const res = await adminUsageAPI.list({ user_id: props.userId, page: 1, page_size: 20 })
    usageLogs.value = res.items
    usageTotal.value = res.total
  } catch (error) {
    console.error('Failed to load usage logs:', error)
  } finally {
    usageLoading.value = false
  }
}

// ---- Subscriptions ----
const subscriptions = ref<UserSubscription[]>([])
const subsLoading = ref(false)

const loadSubscriptions = async () => {
  if (subscriptions.value.length > 0) return
  subsLoading.value = true
  try {
    const res = await adminAPI.subscriptions.list(1, 20, { user_id: props.userId })
    subscriptions.value = res.items
  } catch (error) {
    console.error('Failed to load subscriptions:', error)
  } finally {
    subsLoading.value = false
  }
}

// ---- Profile ----
const userProfile = ref<AdminUser | null>(null)
const profileLoading = ref(false)

const loadProfile = async () => {
  if (userProfile.value) return
  profileLoading.value = true
  try {
    userProfile.value = await adminAPI.users.getById(props.userId)
  } catch (error) {
    console.error('Failed to load user profile:', error)
  } finally {
    profileLoading.value = false
  }
}

// Helper formatters
const formatTokens = (n: number) => {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1000) return `${(n / 1000).toFixed(1)}K`
  return n.toString()
}

// Tab switching: lazy-load data
watch(activeTab, (tab) => {
  if (tab === 'dashboard') {
    loadDashboard()
    loadCharts()
  } else if (tab === 'apikeys') {
    loadApiKeys()
  } else if (tab === 'usage') {
    loadUsage()
  } else if (tab === 'subscriptions') {
    loadSubscriptions()
  } else if (tab === 'profile') {
    loadProfile()
  }
})

// Re-load when userId changes
watch(() => props.userId, () => {
  // Reset all data
  dashboardStats.value = null
  trendData.value = []
  apiKeys.value = []
  usageLogs.value = []
  usageTotal.value = 0
  subscriptions.value = []
  userProfile.value = null
  // Load current tab
  if (activeTab.value === 'dashboard') {
    loadDashboard()
    loadCharts()
  } else if (activeTab.value === 'apikeys') {
    loadApiKeys()
  } else if (activeTab.value === 'usage') {
    loadUsage()
  } else if (activeTab.value === 'subscriptions') {
    loadSubscriptions()
  } else if (activeTab.value === 'profile') {
    loadProfile()
  }
})

onMounted(() => {
  loadDashboard()
  loadCharts()
})
</script>
