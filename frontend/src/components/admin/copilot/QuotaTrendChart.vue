<template>
  <div>
    <div v-if="loading" class="flex h-[180px] items-center justify-center text-xs text-gray-400 dark:text-gray-500">
      加载中…
    </div>
    <div v-else-if="loadError" class="flex h-[180px] items-center justify-center text-xs text-red-500">
      加载失败：{{ loadError }}
    </div>
    <div v-else-if="!data || data.trend.length === 0"
         class="flex h-[180px] items-center justify-center text-xs text-gray-400 dark:text-gray-500">
      暂无趋势数据
    </div>
    <div v-else class="h-[180px]">
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
import { getCopilotAccountQuotaTrend } from '@/api/admin/copilotAnalytics'
import type { CopilotAccountQuotaTrendResult } from '@/api/admin/copilotAnalytics'
import { extractErrorMessage } from '@/api/client'

Chart.register(LineController, LineElement, PointElement, LinearScale, CategoryScale, Tooltip, Legend, Filler)

const props = defineProps<{ accountId: number; days?: number }>()

const chartRef = ref<HTMLCanvasElement | null>(null)
let chart: Chart | null = null

const loading = ref(false)
const loadError = ref<string | null>(null)
const data = ref<CopilotAccountQuotaTrendResult | null>(null)

// Dark mode reactive state
const isDark = ref(document.documentElement.classList.contains('dark'))
let darkObserver: MutationObserver | null = null

function buildChartData() {
  const trend = data.value?.trend ?? []
  const dark = isDark.value
  const limitColor = dark ? '#4b5563' : '#d1d5db'

  return {
    labels: trend.map(t => t.snapshot_date),
    datasets: [
      {
        label: '已用配额',
        data: trend.map(t => t.premium_used),
        borderColor: 'rgb(59, 130, 246)',
        backgroundColor: 'rgba(59, 130, 246, 0.1)',
        fill: true,
        tension: 0.3,
        pointRadius: 3,
        pointHoverRadius: 5,
      },
      {
        label: '配额上限',
        data: trend.map(t => t.premium_entitlement),
        borderColor: limitColor,
        borderDash: [4, 4],
        fill: false,
        tension: 0.3,
        pointRadius: 0,
        pointHoverRadius: 0,
      },
    ],
  }
}

function buildChartOptions() {
  const dark = isDark.value
  const tickColor  = dark ? '#9ca3af' : '#6b7280'
  const gridColor  = dark ? 'rgba(255,255,255,0.08)' : 'rgba(0,0,0,0.06)'
  const legendColor = dark ? '#d1d5db' : '#374151'

  return {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: {
        display: true,
        position: 'bottom' as const,
        labels: { boxWidth: 12, font: { size: 11 }, color: legendColor },
      },
      tooltip: { mode: 'index' as const, intersect: false },
    },
    scales: {
      x: {
        grid: { display: false },
        ticks: { font: { size: 10 }, maxTicksLimit: 10, color: tickColor },
      },
      y: {
        beginAtZero: true,
        grid: { color: gridColor },
        ticks: { font: { size: 10 }, color: tickColor },
      },
    },
  }
}

async function renderChart() {
  if (!data.value || data.value.trend.length === 0) return

  loading.value = false
  await nextTick()

  if (!chartRef.value) return

  if (chart) {
    chart.data = buildChartData()
    chart.options = buildChartOptions() as typeof chart.options
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
    data.value = await getCopilotAccountQuotaTrend(props.accountId, { days: props.days ?? 30 })
    if (!data.value || data.value.trend.length === 0) {
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
      chart?.destroy()
      chart = null
      renderChart()
    }
  })
  darkObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })

  load()
})

watch(() => [props.accountId, props.days], load)

onBeforeUnmount(() => {
  chart?.destroy()
  chart = null
  darkObserver?.disconnect()
  darkObserver = null
})
</script>
