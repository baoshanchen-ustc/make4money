<template>
  <div>
    <div v-if="loading" class="flex h-[256px] items-center justify-center text-xs text-gray-400 dark:text-gray-500">
      <svg class="mr-2 h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
      </svg>
      加载中…
    </div>
    <div v-else-if="loadError" class="flex h-[256px] items-center justify-center text-xs text-red-500">
      加载失败：{{ loadError }}
    </div>
    <div v-else-if="!data || data.days.length === 0"
         class="flex h-[256px] items-center justify-center text-xs text-gray-400 dark:text-gray-500">
      暂无数据
    </div>
    <div v-else class="h-[256px]">
      <canvas ref="chartRef" class="!h-full !w-full" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import {
  Chart,
  LineController,
  LineElement,
  PointElement,
  LinearScale,
  CategoryScale,
  Tooltip,
  Legend,
  Filler,
} from 'chart.js'
import { getCopilotAccountsDailyStats } from '@/api/admin/copilotAnalytics'
import type { CopilotAccountsDailyStatsResult } from '@/api/admin/copilotAnalytics'
import { extractErrorMessage } from '@/api/client'

Chart.register(LineController, LineElement, PointElement, LinearScale, CategoryScale, Tooltip, Legend, Filler)

const props = defineProps<{ days: number }>()

// Palette — cycles for many accounts
const LINE_COLORS = [
  '#10b981', '#6366f1', '#f59e0b', '#3b82f6',
  '#f43f5e', '#06b6d4', '#ec4899', '#8b5cf6',
  '#84cc16', '#fb923c',
]

const chartRef = ref<HTMLCanvasElement | null>(null)
let chart: Chart | null = null

const loading = ref(false)
const loadError = ref<string | null>(null)
const data = ref<CopilotAccountsDailyStatsResult | null>(null)

// Dark mode reactive state
const isDark = ref(document.documentElement.classList.contains('dark'))
let darkObserver: MutationObserver | null = null

function buildChartData() {
  if (!data.value) return { labels: [], datasets: [] }

  const dates = Array.from(new Set(data.value.days.map(d => d.date))).sort()

  const countMap = new Map<number, Map<string, number>>()
  for (const entry of data.value.days) {
    if (!countMap.has(entry.account_id)) countMap.set(entry.account_id, new Map())
    countMap.get(entry.account_id)!.set(entry.date, entry.count)
  }

  const accountsSorted = [...data.value.accounts].sort((a, b) => a.account_id - b.account_id)

  const datasets = accountsSorted.map((acc, idx) => {
    const color = LINE_COLORS[idx % LINE_COLORS.length]
    const dayMap = countMap.get(acc.account_id) ?? new Map()
    return {
      label: acc.name,
      data: dates.map(d => dayMap.get(d) ?? 0),
      borderColor: color,
      backgroundColor: color + '18',
      borderWidth: 2,
      pointRadius: dates.length <= 14 ? 3 : 0,
      pointHoverRadius: 5,
      tension: 0.3,
      fill: false,
    }
  })

  return { labels: dates, datasets }
}

function buildChartOptions() {
  const dark = isDark.value
  const tickColor  = dark ? '#9ca3af' : '#6b7280'
  const gridColor  = dark ? 'rgba(255,255,255,0.08)' : 'rgba(0,0,0,0.06)'
  const legendColor = dark ? '#d1d5db' : '#374151'

  return {
    responsive: true,
    maintainAspectRatio: false,
    interaction: { mode: 'index' as const, intersect: false },
    plugins: {
      legend: {
        display: true,
        position: 'bottom' as const,
        labels: { boxWidth: 12, font: { size: 11 }, padding: 16, color: legendColor },
      },
      tooltip: {
        callbacks: {
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          title: (items: any[]) => items[0]?.label ?? '',
        },
      },
    },
    scales: {
      x: {
        grid: { display: false },
        ticks: { font: { size: 10 }, maxTicksLimit: 14, color: tickColor },
      },
      y: {
        beginAtZero: true,
        grid: { color: gridColor },
        ticks: { font: { size: 10 }, precision: 0, color: tickColor },
      },
    },
  }
}

async function renderChart() {
  if (!data.value || data.value.days.length === 0) return

  // Show canvas before creating chart so chartRef.value is valid
  loading.value = false
  await nextTick()

  if (!chartRef.value) return

  if (chart) {
    // Smooth update without full re-create
    chart.data = buildChartData()
    // Update options in-place for dark mode changes
    const opts = buildChartOptions()
    chart.options = opts as typeof chart.options
    chart.update('active')
  } else {
    chart = new Chart(chartRef.value, {
      type: 'line',
      data: buildChartData(),
      options: buildChartOptions() as any, // eslint-disable-line @typescript-eslint/no-explicit-any
    })
  }
}

async function load() {
  loading.value = true
  loadError.value = null
  chart?.destroy()
  chart = null
  try {
    data.value = await getCopilotAccountsDailyStats({ days: props.days })
    if (!data.value || data.value.days.length === 0) {
      // Empty state — show the "暂无数据" branch
      loading.value = false
      return
    }
    await renderChart()
  } catch (e: unknown) {
    loadError.value = extractErrorMessage(e)
    loading.value = false
  }
}

onMounted(() => {
  darkObserver = new MutationObserver(() => {
    const newDark = document.documentElement.classList.contains('dark')
    if (newDark !== isDark.value) {
      isDark.value = newDark
      // Rebuild chart with new colors
      chart?.destroy()
      chart = null
      renderChart()
    }
  })
  darkObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })

  load()
})

watch(() => props.days, load)

onBeforeUnmount(() => {
  chart?.destroy()
  chart = null
  darkObserver?.disconnect()
  darkObserver = null
})
</script>
