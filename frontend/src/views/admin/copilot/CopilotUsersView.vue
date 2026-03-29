<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- 页头 -->
      <div class="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
        <div>
          <p class="text-xs font-semibold uppercase tracking-widest text-blue-600 dark:text-blue-400">
            Copilot Analytics
          </p>
          <h1 class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">
            {{ t('admin.copilot.users.title') }}
          </h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.copilot.users.description') }}
          </p>
        </div>
        <!-- 时间范围选择 -->
        <div class="flex items-center gap-1 rounded-xl border border-gray-200 bg-white p-1 shadow-sm dark:border-gray-700 dark:bg-gray-800">
          <button
            v-for="opt in DAY_OPTIONS"
            :key="opt.value"
            class="rounded-lg px-3 py-1.5 text-sm font-medium transition-colors"
            :class="selectedDays === opt.value
              ? 'bg-blue-600 text-white shadow-sm'
              : 'text-gray-500 hover:bg-gray-100 hover:text-gray-900 dark:text-gray-400 dark:hover:bg-gray-700 dark:hover:text-gray-100'"
            @click="selectedDays = opt.value"
          >
            {{ opt.label }}
          </button>
        </div>
      </div>

      <!-- 错误提示 -->
      <div
        v-if="error"
        class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-400"
      >
        {{ error }}
      </div>

      <!-- KPI 卡片 -->
      <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <SummaryCard
          :title="`近 ${selectedDays} 日 Premium`"
          :value="kpiTotalPremium"
          :loading="loading"
          color="green"
        />
        <SummaryCard
          title="活跃用户数"
          :value="kpiActiveUsers"
          :loading="loading"
          color="blue"
        />
        <SummaryCard
          title="人均请求数"
          :value="kpiAvgRequests"
          :loading="loading"
          color="purple"
        />
        <!-- Top 消耗用户（自定义卡片） -->
        <div class="relative overflow-hidden rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-gray-700 dark:bg-gray-800">
          <div class="absolute inset-x-0 top-0 h-0.5 bg-gradient-to-r from-amber-400 via-orange-500 to-pink-500" />
          <p class="text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">Top 消耗用户</p>
          <div v-if="loading" class="mt-3 space-y-2">
            <div class="h-5 w-32 animate-pulse rounded bg-gray-200 dark:bg-gray-700" />
            <div class="h-7 w-20 animate-pulse rounded bg-gray-200 dark:bg-gray-700" />
          </div>
          <template v-else-if="topUser">
            <p class="mt-2 truncate text-lg font-bold text-gray-900 dark:text-white">{{ topUser.username }}</p>
            <p class="text-2xl font-bold text-amber-600 dark:text-amber-400">{{ topUser.totalRequests.toLocaleString() }}</p>
            <p class="mt-0.5 text-xs text-gray-400">总请求数</p>
          </template>
          <template v-else>
            <p class="mt-2 text-sm text-gray-400">暂无活跃用户</p>
          </template>
        </div>
      </div>

      <!-- 趋势折线图卡片 -->
      <div class="rounded-lg border border-gray-200 bg-white shadow-sm dark:border-gray-700 dark:bg-gray-800">
        <div class="flex flex-col gap-3 border-b border-gray-100 px-4 py-3 dark:border-gray-700 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 class="text-sm font-semibold text-gray-900 dark:text-white">用户请求趋势</h2>
            <p class="text-xs text-gray-400">近 {{ selectedDays }} 天按用户拆分</p>
          </div>
          <!-- 指标切换 -->
          <div class="flex items-center gap-1 rounded-lg border border-gray-200 p-1 dark:border-gray-700">
            <button
              v-for="m in METRIC_OPTIONS"
              :key="m.value"
              class="rounded px-2.5 py-1 text-xs font-semibold transition-colors"
              :class="chartMetric === m.value
                ? 'bg-gray-900 text-white dark:bg-white dark:text-gray-900'
                : 'text-gray-500 hover:bg-gray-100 hover:text-gray-800 dark:text-gray-400 dark:hover:bg-gray-700'"
              @click="chartMetric = m.value"
            >
              {{ m.label }}
            </button>
          </div>
        </div>
        <div class="p-4">
          <UsersDailyChart :days="selectedDays" :metric="chartMetric" />
        </div>
      </div>

      <!-- 用户排行表 -->
      <div class="rounded-lg border border-gray-200 bg-white shadow-sm dark:border-gray-700 dark:bg-gray-800">
        <div class="flex flex-col gap-3 border-b border-gray-100 px-4 py-3 dark:border-gray-700 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 class="text-sm font-semibold text-gray-900 dark:text-white">用户排行</h2>
            <p class="text-xs text-gray-400">共 {{ filteredUsers.length }} / {{ aggregatedUsers.length }} 位活跃用户</p>
          </div>
          <div class="flex flex-col gap-2 sm:flex-row">
            <select
              v-model="sortKey"
              class="rounded-lg border border-gray-200 bg-white px-3 py-1.5 text-xs focus:border-blue-500 focus:outline-none dark:border-gray-600 dark:bg-gray-700 dark:text-white"
            >
              <option v-for="s in SORT_OPTIONS" :key="s.value" :value="s.value">{{ s.label }}</option>
            </select>
            <input
              v-model="searchQuery"
              type="text"
              placeholder="搜索用户名…"
              class="rounded-lg border border-gray-200 px-3 py-1.5 text-xs focus:border-blue-500 focus:outline-none dark:border-gray-600 dark:bg-gray-700 dark:text-white"
            />
          </div>
        </div>

        <div v-if="loading" class="flex h-36 items-center justify-center">
          <LoadingSpinner />
        </div>
        <div v-else-if="filteredUsers.length === 0" class="flex h-36 items-center justify-center text-sm text-gray-400">
          暂无匹配数据
        </div>
        <div v-else class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-100 dark:divide-gray-700">
            <thead class="bg-gray-50 dark:bg-gray-900/40">
              <tr>
                <th class="w-12 px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">#</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">用户</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500">Premium</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500">Agent</th>
                <th class="hidden px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 md:table-cell">
                  7 日趋势
                </th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500">Top 模型</th>
                <th class="px-4 py-3" />
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-gray-700">
              <tr
                v-for="(user, idx) in filteredUsers"
                :key="user.userId"
                class="transition-colors hover:bg-gray-50 dark:hover:bg-gray-700/30"
              >
                <td class="px-4 py-3 text-sm">
                  <span v-if="idx === 0" class="text-lg">🥇</span>
                  <span v-else-if="idx === 1" class="text-lg">🥈</span>
                  <span v-else-if="idx === 2" class="text-lg">🥉</span>
                  <span v-else class="text-xs text-gray-400">{{ idx + 1 }}</span>
                </td>
                <td class="px-4 py-3">
                  <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ user.username }}</p>
                  <p class="text-xs text-gray-400">{{ formatDatetime(user.lastRequestAt) }}</p>
                </td>
                <td class="px-4 py-3 text-right">
                  <span class="inline-flex items-center rounded-full bg-green-100 px-2.5 py-0.5 text-xs font-semibold text-green-700 dark:bg-green-900/30 dark:text-green-400">
                    {{ user.premiumRequests.toLocaleString() }}
                  </span>
                </td>
                <td class="px-4 py-3 text-right">
                  <span class="inline-flex items-center rounded-full bg-blue-100 px-2.5 py-0.5 text-xs font-semibold text-blue-700 dark:bg-blue-900/30 dark:text-blue-400">
                    {{ user.agentRequests.toLocaleString() }}
                  </span>
                </td>
                <td class="hidden px-4 py-3 md:table-cell">
                  <UserSparkline :data="user.sparkline" :width="88" :height="28" color="#8b5cf6" />
                  <p class="mt-0.5 text-xs text-gray-400">总 {{ user.totalRequests.toLocaleString() }}</p>
                </td>
                <td class="px-4 py-3">
                  <span
                    v-if="user.topModel"
                    class="inline-flex max-w-[160px] items-center truncate rounded-full bg-purple-100 px-2.5 py-0.5 text-xs text-purple-700 dark:bg-purple-900/30 dark:text-purple-400"
                  >
                    {{ user.topModel }}
                  </span>
                  <span v-else class="text-xs text-gray-400">—</span>
                  <span v-if="user.extraModelCount > 0" class="ml-1 text-xs text-gray-400">
                    +{{ user.extraModelCount }}
                  </span>
                </td>
                <td class="px-4 py-3 text-right">
                  <router-link
                    :to="{ name: 'AdminCopilotUserDetail', params: { id: user.userId } }"
                    class="text-xs font-medium text-blue-600 hover:text-blue-800 dark:text-blue-400 dark:hover:text-blue-300"
                  >
                    详情 →
                  </router-link>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  getCopilotUsersDailyStats,
  getCopilotUserStats,
} from '@/api/admin/copilotAnalytics'
import type {
  CopilotUserStatsResult,
  CopilotUsersDailyStatsResult,
} from '@/api/admin/copilotAnalytics'
import { extractErrorMessage } from '@/api/client'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import SummaryCard from '@/components/admin/copilot/CopilotSummaryCard.vue'
import UsersDailyChart from '@/components/admin/copilot/UsersDailyChart.vue'
import UserSparkline from '@/components/admin/copilot/UserSparkline.vue'

// ─────────────────────────────────────────────
// Types
// ─────────────────────────────────────────────

type ChartMetric = 'premium' | 'agent'
type SortKey = 'premium' | 'agent' | 'total'

interface UserRow {
  userId: number
  username: string
  premiumRequests: number
  agentRequests: number
  totalRequests: number
  sparkline: number[]
  topModel: string
  extraModelCount: number
  lastRequestAt: string | null
}

// ─────────────────────────────────────────────
// Constants
// ─────────────────────────────────────────────

const DAY_OPTIONS = [
  { label: '7天', value: 7 },
  { label: '14天', value: 14 },
  { label: '30天', value: 30 },
  { label: '60天', value: 60 },
] as const

const METRIC_OPTIONS = [
  { label: 'Premium', value: 'premium' as ChartMetric },
  { label: 'Agent', value: 'agent' as ChartMetric },
]

const SORT_OPTIONS = [
  { label: '按 Premium 排序', value: 'premium' as SortKey },
  { label: '按 Agent 排序', value: 'agent' as SortKey },
  { label: '按总请求排序', value: 'total' as SortKey },
]

// ─────────────────────────────────────────────
// State
// ─────────────────────────────────────────────

const { t } = useI18n()

const selectedDays = ref<number>(30)
const chartMetric = ref<ChartMetric>('premium')
const sortKey = ref<SortKey>('premium')
const searchQuery = ref('')
const loading = ref(false)
const error = ref<string | null>(null)
const dailyData = ref<CopilotUsersDailyStatsResult | null>(null)
const todayData = ref<CopilotUserStatsResult | null>(null)

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

/** Returns a locale-safe YYYY-MM-DD string for `today - offset` days */
function localDateStr(offsetDays: number): string {
  const d = new Date()
  d.setDate(d.getDate() - offsetDays)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

function formatDatetime(iso: string | null): string {
  if (!iso) return '暂无记录'
  return new Date(iso).toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

// ─────────────────────────────────────────────
// Aggregation
// ─────────────────────────────────────────────

/**
 * Merges daily trend data with today's per-user model/last-request metadata,
 * and computes 7-day sparklines from the daily data.
 */
const aggregatedUsers = computed<UserRow[]>(() => {
  if (!dailyData.value) return []

  // Accumulate premium + agent per user across all dates
  const premiumByUser = new Map<number, number>()
  const agentByUser = new Map<number, number>()
  const dailyByUser = new Map<number, Map<string, number>>()

  for (const entry of dailyData.value.days) {
    premiumByUser.set(entry.user_id, (premiumByUser.get(entry.user_id) ?? 0) + entry.premium_count)
    agentByUser.set(entry.user_id, (agentByUser.get(entry.user_id) ?? 0) + entry.agent_count)

    if (!dailyByUser.has(entry.user_id)) {
      dailyByUser.set(entry.user_id, new Map())
    }
    const existing = dailyByUser.get(entry.user_id)!.get(entry.date) ?? 0
    dailyByUser.get(entry.user_id)!.set(entry.date, existing + entry.premium_count + entry.agent_count)
  }

  // Build model + last-request lookup from today's data
  const modelsByUser = new Map<number, string[]>()
  const lastRequestByUser = new Map<number, string>()
  if (todayData.value) {
    for (const u of todayData.value.users) {
      if (u.models?.length) modelsByUser.set(u.user_id, u.models)
      if (u.last_request_at) lastRequestByUser.set(u.user_id, u.last_request_at)
    }
  }

  // Last 7 days for sparkline (locale-safe, no UTC drift)
  const sparklineDates = Array.from({ length: 7 }, (_, i) => localDateStr(6 - i))

  return dailyData.value.users.map((user) => {
    const premium = premiumByUser.get(user.user_id) ?? 0
    const agent = agentByUser.get(user.user_id) ?? 0
    const dayMap = dailyByUser.get(user.user_id) ?? new Map()
    const models = modelsByUser.get(user.user_id) ?? []

    return {
      userId: user.user_id,
      username: user.username,
      premiumRequests: premium,
      agentRequests: agent,
      totalRequests: premium + agent,
      sparkline: sparklineDates.map(d => dayMap.get(d) ?? 0),
      topModel: models[0] ?? '',
      extraModelCount: Math.max(models.length - 1, 0),
      lastRequestAt: lastRequestByUser.get(user.user_id) ?? null,
    }
  }).filter(u => u.totalRequests > 0)
})

// ─────────────────────────────────────────────
// KPI computeds
// ─────────────────────────────────────────────

const kpiTotalPremium = computed(() =>
  aggregatedUsers.value.reduce((sum, u) => sum + u.premiumRequests, 0),
)

const kpiActiveUsers = computed(() => aggregatedUsers.value.length)

const kpiAvgRequests = computed(() => {
  if (kpiActiveUsers.value === 0) return 0
  const total = aggregatedUsers.value.reduce((sum, u) => sum + u.totalRequests, 0)
  return Math.round(total / kpiActiveUsers.value)
})

const topUser = computed<UserRow | null>(() => {
  if (!aggregatedUsers.value.length) return null
  return aggregatedUsers.value.reduce((best, u) =>
    u.totalRequests > best.totalRequests ? u : best,
  )
})

// ─────────────────────────────────────────────
// Filtered / sorted leaderboard
// ─────────────────────────────────────────────

const filteredUsers = computed<UserRow[]>(() => {
  const q = searchQuery.value.trim().toLowerCase()
  const list = q
    ? aggregatedUsers.value.filter(u => u.username.toLowerCase().includes(q))
    : aggregatedUsers.value

  return [...list].sort((a, b) => {
    const keyMap: Record<SortKey, keyof UserRow> = {
      premium: 'premiumRequests',
      agent: 'agentRequests',
      total: 'totalRequests',
    }
    const key = keyMap[sortKey.value]
    return (b[key] as number) - (a[key] as number)
  })
})

// ─────────────────────────────────────────────
// Data loading
// ─────────────────────────────────────────────

async function loadDashboard(): Promise<void> {
  loading.value = true
  error.value = null
  try {
    const [daily, today] = await Promise.all([
      getCopilotUsersDailyStats({ days: selectedDays.value }),
      getCopilotUserStats({}),
    ])
    dailyData.value = daily
    todayData.value = today
  } catch (e: unknown) {
    error.value = extractErrorMessage(e)
  } finally {
    loading.value = false
  }
}

onMounted(loadDashboard)
watch(selectedDays, loadDashboard)
</script>
