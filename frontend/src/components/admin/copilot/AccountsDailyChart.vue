<template>
  <div>
    <div v-if="loading" class="flex h-48 items-center justify-center text-xs text-gray-400 dark:text-gray-500">
      <svg class="mr-2 h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
      </svg>
      加载中…
    </div>
    <div v-else-if="loadError" class="flex h-48 items-center justify-center text-xs text-red-500">
      加载失败：{{ loadError }}
    </div>
    <div v-else-if="!data || data.days.length === 0" class="flex h-48 items-center justify-center text-xs text-gray-400 dark:text-gray-500">
      暂无数据
    </div>
    <Line v-else :data="chartData" :options="chartOptions" class="max-h-64" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onUnmounted } from 'vue'
import { Line } from 'vue-chartjs'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Tooltip,
  Legend,
  Filler,
} from 'chart.js'
import { getCopilotAccountsDailyStats } from '@/api/admin/copilotAnalytics'
import type { CopilotAccountsDailyStatsResult } from '@/api/admin/copilotAnalytics'
import { extractErrorMessage } from '@/api/client'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend, Filler)

const props = defineProps<{ days: number }>()

const loading = ref(false)
const loadError = ref<string | null>(null)
const data = ref<CopilotAccountsDailyStatsResult | null>(null)

// Palette — cycles for many accounts
const LINE_COLORS = [
  '#10b981', '#6366f1', '#f59e0b', '#3b82f6',
  '#f43f5e', '#06b6d4', '#ec4899', '#8b5cf6',
  '#84cc16', '#fb923c',
]

async function load() {
  loading.value = true
  loadError.value = null
  try {
    data.value = await getCopilotAccountsDailyStats({ days: props.days })
  } catch (e: unknown) {
    loadError.value = extractErrorMessage(e)
  } finally {
    loading.value = false
  }
}

watch(() => props.days, load, { immediate: true })

// Build sorted date labels covering the full range (fills zeros for missing dates)
const dateLabels = computed<string[]>(() => {
  if (!data.value || data.value.days.length === 0) return []
  const dates = new Set(data.value.days.map(d => d.date))
  return Array.from(dates).sort()
})

const chartData = computed(() => {
  if (!data.value) return { labels: [], datasets: [] }

  const labels = dateLabels.value

  // Build a map: accountId → (date → count)
  const countMap = new Map<number, Map<string, number>>()
  for (const entry of data.value.days) {
    if (!countMap.has(entry.account_id)) countMap.set(entry.account_id, new Map())
    countMap.get(entry.account_id)!.set(entry.date, entry.count)
  }

  // One dataset per account — sorted by account_id for stable color assignment
  const accountsSorted = [...data.value.accounts].sort((a, b) => a.account_id - b.account_id)

  const datasets = accountsSorted.map((acc, idx) => {
    const color = LINE_COLORS[idx % LINE_COLORS.length]
    const dayMap = countMap.get(acc.account_id) ?? new Map()
    return {
      label: acc.name,
      data: labels.map(d => dayMap.get(d) ?? 0),
      borderColor: color,
      backgroundColor: color + '18',
      borderWidth: 2,
      pointRadius: labels.length <= 14 ? 3 : 0,
      pointHoverRadius: 5,
      tension: 0.3,
      fill: false,
    }
  })

  return { labels, datasets }
})

const chartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  interaction: { mode: 'index' as const, intersect: false },
  plugins: {
    legend: {
      display: true,
      position: 'bottom' as const,
      labels: { boxWidth: 12, font: { size: 11 }, padding: 16 },
    },
    tooltip: {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      callbacks: { title: (items: any[]) => items[0]?.label ?? '' },
    },
  },
  scales: {
    x: {
      grid: { display: false },
      ticks: { font: { size: 10 }, maxTicksLimit: 14 },
    },
    y: {
      beginAtZero: true,
      grid: { color: 'rgba(156,163,175,0.12)' },
      ticks: { font: { size: 10 }, precision: 0 },
    },
  },
}

onUnmounted(() => {
  // No timers to clean up; fetch is fire-and-forget within the watch
})
</script>
