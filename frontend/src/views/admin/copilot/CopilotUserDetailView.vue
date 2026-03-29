<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- 面包屑 -->
      <nav class="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400">
        <router-link
          to="/admin/copilot/users"
          class="font-medium text-blue-600 transition-colors hover:text-blue-800 dark:text-blue-400 dark:hover:text-blue-300"
        >
          用户分析
        </router-link>
        <span class="text-gray-300 dark:text-gray-600">›</span>
        <span class="truncate font-medium text-gray-700 dark:text-gray-200">
          {{ summary?.username ?? `用户 #${userId}` }}
        </span>
      </nav>

      <!-- 加载中 -->
      <div
        v-if="summaryLoading"
        class="flex min-h-64 items-center justify-center rounded-lg border border-gray-200 bg-white shadow-sm dark:border-gray-700 dark:bg-gray-800"
      >
        <LoadingSpinner />
      </div>

      <!-- 错误 -->
      <div
        v-else-if="summaryError"
        class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-400"
      >
        {{ summaryError }}
      </div>

      <template v-else-if="summary">
        <!-- 用户信息卡 -->
        <div class="overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm dark:border-gray-700 dark:bg-gray-800">
          <div class="h-1 bg-gradient-to-r from-blue-500 via-cyan-500 to-emerald-500" />
          <div class="p-5">
            <div class="flex flex-col gap-5 sm:flex-row sm:items-center sm:justify-between">
              <!-- 头像 + 名称 -->
              <div class="flex items-center gap-4">
                <div class="flex h-14 w-14 flex-shrink-0 items-center justify-center rounded-full bg-blue-100 text-2xl font-bold text-blue-700 dark:bg-blue-900/40 dark:text-blue-400">
                  {{ avatarLetter }}
                </div>
                <div>
                  <h1 class="text-xl font-bold text-gray-900 dark:text-white">{{ summary.username }}</h1>
                  <p class="mt-0.5 text-sm text-gray-500 dark:text-gray-400">
                    首次 {{ formatDate(summary.first_request_at) }} ·
                    最近活跃 {{ formatDate(summary.last_request_at) }}
                  </p>
                </div>
              </div>
              <!-- 统计数字 -->
              <div class="flex gap-6">
                <div class="text-center">
                  <p class="text-2xl font-bold text-green-600 dark:text-green-400">{{ summary.total_premium_requests.toLocaleString() }}</p>
                  <p class="text-xs text-gray-500">Premium</p>
                </div>
                <div class="text-center">
                  <p class="text-2xl font-bold text-blue-600 dark:text-blue-400">{{ summary.total_agent_requests.toLocaleString() }}</p>
                  <p class="text-xs text-gray-500">Agent</p>
                </div>
                <div class="text-center">
                  <p class="text-2xl font-bold text-gray-700 dark:text-gray-200">{{ totalRequests.toLocaleString() }}</p>
                  <p class="text-xs text-gray-500">总量</p>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- 图表双列 -->
        <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
          <!-- 近30日堆叠柱状图 -->
          <div class="rounded-lg border border-gray-200 bg-white shadow-sm dark:border-gray-700 dark:bg-gray-800">
            <div class="border-b border-gray-100 px-4 py-3 dark:border-gray-700">
              <h2 class="text-sm font-semibold text-gray-900 dark:text-white">近 30 日每日请求</h2>
              <p class="text-xs text-gray-400">Premium / Agent 堆叠展示</p>
            </div>
            <div class="p-4">
              <div v-if="dailyLoading" class="flex h-64 items-center justify-center">
                <LoadingSpinner />
              </div>
              <div v-else-if="dailyError" class="flex h-64 items-center justify-center text-sm text-red-500">
                {{ dailyError }}
              </div>
              <div v-else style="height: 260px;">
                <canvas ref="barChartRef" style="height: 100%; width: 100%;" />
              </div>
            </div>
          </div>

          <!-- 模型分布环形图 -->
          <div class="rounded-lg border border-gray-200 bg-white shadow-sm dark:border-gray-700 dark:bg-gray-800">
            <div class="border-b border-gray-100 px-4 py-3 dark:border-gray-700">
              <h2 class="text-sm font-semibold text-gray-900 dark:text-white">模型使用分布</h2>
              <p class="text-xs text-gray-400">Top {{ topModels.length }} 模型使用占比</p>
            </div>
            <div class="p-4">
              <div
                v-if="topModels.length === 0"
                class="flex h-64 items-center justify-center text-sm text-gray-400"
              >
                暂无模型数据
              </div>
              <div v-else class="flex items-center gap-4">
                <canvas ref="donutChartRef" width="180" height="180" style="flex-shrink: 0;" />
                <ul class="flex-1 space-y-2 text-xs overflow-hidden">
                  <li
                    v-for="(m, i) in topModels"
                    :key="m.model"
                    class="flex items-center justify-between gap-2"
                  >
                    <div class="flex items-center gap-1.5 min-w-0">
                      <span
                        class="h-2.5 w-2.5 flex-shrink-0 rounded-full"
                        :style="{ backgroundColor: MODEL_COLORS[i % MODEL_COLORS.length] }"
                      />
                      <span class="truncate text-gray-700 dark:text-gray-300">{{ m.model }}</span>
                    </div>
                    <span class="flex-shrink-0 font-semibold text-gray-500 dark:text-gray-400">
                      {{ m.percentage.toFixed(1) }}%
                    </span>
                  </li>
                </ul>
              </div>
            </div>
          </div>
        </div>

        <!-- 24h 热力图 -->
        <div class="rounded-lg border border-gray-200 bg-white shadow-sm dark:border-gray-700 dark:bg-gray-800">
          <div class="flex items-center justify-between border-b border-gray-100 px-4 py-3 dark:border-gray-700">
            <h2 class="text-sm font-semibold text-gray-900 dark:text-white">活跃时段热力图（近7天）</h2>
            <p class="text-xs text-gray-400">颜色越深，请求越多</p>
          </div>
          <div class="p-4">
            <UserHeatmap :user-id="userId" :days="7" />
          </div>
        </div>

        <!-- 请求日志 -->
        <div class="rounded-lg border border-gray-200 bg-white shadow-sm dark:border-gray-700 dark:bg-gray-800">
          <div class="flex flex-col gap-3 border-b border-gray-100 px-4 py-3 dark:border-gray-700 sm:flex-row sm:items-center sm:justify-between">
            <h2 class="text-sm font-semibold text-gray-900 dark:text-white">请求日志</h2>
            <input
              v-model="selectedDate"
              type="date"
              class="rounded border border-gray-300 px-2 py-1 text-xs focus:border-blue-500 focus:outline-none dark:border-gray-600 dark:bg-gray-700 dark:text-white"
            />
          </div>
          <div class="p-4">
            <UserRequestTree :user-id="userId" :date="selectedDate" />
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import {
  Chart,
  BarController,
  BarElement,
  DoughnutController,
  ArcElement,
  LinearScale,
  CategoryScale,
  Tooltip,
  Legend,
} from 'chart.js'
import { getCopilotUserSummary, getCopilotUsersDailyStats } from '@/api/admin/copilotAnalytics'
import type { CopilotUserSummaryResult, CopilotUsersDailyStatsResult } from '@/api/admin/copilotAnalytics'
import { extractErrorMessage } from '@/api/client'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import UserHeatmap from '@/components/admin/copilot/UserHeatmap.vue'
import UserRequestTree from '@/components/admin/copilot/UserRequestTree.vue'

Chart.register(BarController, BarElement, DoughnutController, ArcElement, LinearScale, CategoryScale, Tooltip, Legend)

// ─────────────────────────────────────────────
// Constants
// ─────────────────────────────────────────────

const MODEL_COLORS = ['#3b82f6', '#10b981', '#f59e0b', '#ef4444', '#8b5cf6', '#06b6d4']
const BAR_DAYS = 30

// ─────────────────────────────────────────────
// Route / state
// ─────────────────────────────────────────────

const route = useRoute()
const userId = computed(() => Number(route.params.id))

const summary = ref<CopilotUserSummaryResult | null>(null)
const dailyStats = ref<CopilotUsersDailyStatsResult | null>(null)
const summaryLoading = ref(false)
const dailyLoading = ref(false)
const summaryError = ref<string | null>(null)
const dailyError = ref<string | null>(null)

const barChartRef = ref<HTMLCanvasElement | null>(null)
const donutChartRef = ref<HTMLCanvasElement | null>(null)
let barChart: Chart | null = null
let donutChart: Chart | null = null

// ─────────────────────────────────────────────
// Computed
// ─────────────────────────────────────────────

const avatarLetter = computed(() =>
  (summary.value?.username?.trim().charAt(0) ?? '?').toUpperCase(),
)

const totalRequests = computed(() => {
  if (!summary.value) return 0
  return summary.value.total_premium_requests + summary.value.total_agent_requests
})

const topModels = computed(() => summary.value?.top_models.slice(0, 6) ?? [])

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

/** Locale-safe YYYY-MM-DD for today + offset days (no UTC drift) */
function localDateStr(offset = 0): string {
  const d = new Date()
  d.setDate(d.getDate() + offset)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

const selectedDate = ref(localDateStr())

function formatDate(iso: string | null): string {
  if (!iso) return '—'
  return new Date(iso).toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

// ─────────────────────────────────────────────
// Charts
// ─────────────────────────────────────────────

function buildBarChart() {
  if (!barChartRef.value || !dailyStats.value || !userId.value) return

  // Build date labels for the last BAR_DAYS days
  const dates: string[] = Array.from({ length: BAR_DAYS }, (_, i) => localDateStr(-(BAR_DAYS - 1 - i)))
  const labels = dates.map(d => d.slice(5)) // MM-DD

  // Index this user's data by date
  const dayMap = new Map<string, { premium: number; agent: number }>()
  for (const entry of dailyStats.value.days) {
    if (entry.user_id === userId.value) {
      dayMap.set(entry.date, { premium: entry.premium_count, agent: entry.agent_count })
    }
  }

  barChart?.destroy()
  barChart = new Chart(barChartRef.value, {
    type: 'bar',
    data: {
      labels,
      datasets: [
        {
          label: 'Premium',
          data: dates.map(d => dayMap.get(d)?.premium ?? 0),
          backgroundColor: '#3b82f6',
          stack: 'stack',
        },
        {
          label: 'Agent',
          data: dates.map(d => dayMap.get(d)?.agent ?? 0),
          backgroundColor: '#10b981',
          stack: 'stack',
        },
      ],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      interaction: { mode: 'index' },
      plugins: {
        legend: { position: 'bottom', labels: { boxWidth: 12, padding: 16 } },
        tooltip: {
          callbacks: {
            label: ctx => ` ${ctx.dataset.label}: ${ctx.parsed.y} 次`,
          },
        },
      },
      scales: {
        x: { stacked: true, grid: { display: false }, ticks: { maxTicksLimit: 10, font: { size: 10 } } },
        y: { stacked: true, beginAtZero: true, ticks: { precision: 0, font: { size: 10 } } },
      },
    },
  })
}

function buildDonutChart() {
  if (!donutChartRef.value || !topModels.value.length) return

  donutChart?.destroy()
  donutChart = new Chart(donutChartRef.value, {
    type: 'doughnut',
    data: {
      labels: topModels.value.map(m => m.model),
      datasets: [{
        data: topModels.value.map(m => m.count),
        backgroundColor: MODEL_COLORS.slice(0, topModels.value.length),
        borderWidth: 2,
        hoverOffset: 8,
      }],
    },
    options: {
      responsive: false,
      cutout: '60%',
      plugins: {
        legend: { display: false },
        tooltip: {
          callbacks: {
            label: ctx => {
              const m = topModels.value[ctx.dataIndex]
              return m ? ` ${m.model}: ${m.count} 次 (${m.percentage.toFixed(1)}%)` : ''
            },
          },
        },
      },
    },
  })
}

// ─────────────────────────────────────────────
// Data loading
// ─────────────────────────────────────────────

async function loadAll() {
  if (!userId.value) return

  summaryLoading.value = true
  dailyLoading.value = true
  summaryError.value = null
  dailyError.value = null
  summary.value = null
  dailyStats.value = null
  barChart?.destroy()
  barChart = null
  donutChart?.destroy()
  donutChart = null

  const [sumResult, dailyResult] = await Promise.allSettled([
    getCopilotUserSummary(userId.value),
    getCopilotUsersDailyStats({ days: BAR_DAYS }),
  ])

  if (sumResult.status === 'fulfilled') {
    summary.value = sumResult.value
  } else {
    summaryError.value = extractErrorMessage(sumResult.reason)
  }

  if (dailyResult.status === 'fulfilled') {
    dailyStats.value = dailyResult.value
  } else {
    dailyError.value = extractErrorMessage(dailyResult.reason)
  }

  summaryLoading.value = false
  dailyLoading.value = false

  // Wait for Vue to render the canvas elements before drawing charts
  if (summary.value) {
    await nextTick()
    buildBarChart()
    buildDonutChart()
  }
}

watch(userId, loadAll, { immediate: true })

onBeforeUnmount(() => {
  barChart?.destroy()
  donutChart?.destroy()
})
</script>
