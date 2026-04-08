<template>
  <div class="relative">
    <div v-if="!userId" class="flex h-[300px] items-center justify-center text-sm text-gray-400">
      请选择一个用户
    </div>
    <div v-else-if="!dailyData" class="flex h-[300px] items-center justify-center text-sm text-gray-400">
      暂无数据
    </div>
    <div v-else class="h-[300px]">
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
import type { CopilotUsersDailyStatsResult } from '@/api/admin/copilotAnalytics'

Chart.register(LineController, LineElement, PointElement, LinearScale, CategoryScale, Tooltip, Legend, Filler)

const props = withDefaults(defineProps<{
  days?: number
  userId: number | null
  dailyData: CopilotUsersDailyStatsResult | null
}>(), {
  days: 30,
})

const chartRef = ref<HTMLCanvasElement | null>(null)
let chart: Chart | null = null

/** 生成最近 N 天的日期字符串数组（本地时区，无 UTC 偏移） */
function buildDateRange(days: number): string[] {
  const dates: string[] = []
  const today = new Date()
  for (let i = days - 1; i >= 0; i--) {
    const d = new Date(today)
    d.setDate(d.getDate() - i)
    dates.push(
      `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`,
    )
  }
  return dates
}

function buildChart() {
  if (!props.userId || !props.dailyData || !chartRef.value) return

  const dates = buildDateRange(props.days ?? 30)

  // 按日期汇总该用户的 premium / agent
  const premiumByDate = new Map<string, number>()
  const agentByDate = new Map<string, number>()

  for (const entry of props.dailyData.days) {
    if (entry.user_id !== props.userId) continue
    premiumByDate.set(entry.date, (premiumByDate.get(entry.date) ?? 0) + entry.premium_count)
    agentByDate.set(entry.date, (agentByDate.get(entry.date) ?? 0) + entry.agent_count)
  }

  const premiumData = dates.map(d => premiumByDate.get(d) ?? 0)
  const agentData = dates.map(d => agentByDate.get(d) ?? 0)
  const totalData = dates.map((_, i) => premiumData[i] + agentData[i])

  const dark = document.documentElement.classList.contains('dark')
  const tickColor = dark ? '#9ca3af' : '#6b7280'
  const gridColor = dark ? 'rgba(255,255,255,0.08)' : 'rgba(0,0,0,0.06)'
  const legendColor = dark ? '#d1d5db' : '#374151'

  const pointRadius = (props.days ?? 30) <= 14 ? 3 : 0

  chart?.destroy()
  chart = new Chart(chartRef.value, {
    type: 'line',
    data: {
      labels: dates,
      datasets: [
        {
          label: 'Premium',
          data: premiumData,
          borderColor: '#10b981',
          backgroundColor: '#10b98118',
          borderWidth: 2,
          pointRadius,
          pointHoverRadius: 5,
          tension: 0.3,
          fill: false,
        },
        {
          label: 'Agent',
          data: agentData,
          borderColor: '#3b82f6',
          backgroundColor: '#3b82f618',
          borderWidth: 2,
          pointRadius,
          pointHoverRadius: 5,
          tension: 0.3,
          fill: false,
        },
        {
          label: '总量',
          data: totalData,
          borderColor: '#8b5cf6',
          backgroundColor: '#8b5cf618',
          borderWidth: 2,
          pointRadius,
          pointHoverRadius: 5,
          tension: 0.3,
          fill: false,
        },
      ],
    },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      interaction: { mode: 'index', intersect: false },
      plugins: {
        legend: {
          position: 'bottom',
          labels: { boxWidth: 12, padding: 16, font: { size: 12 }, color: legendColor },
        },
        tooltip: {
          callbacks: {
            title: (items) => items[0]?.label ?? '',
            label: (item) => ` ${item.dataset.label}: ${item.parsed.y} 次`,
          },
        },
      },
      scales: {
        x: {
          grid: { display: false },
          ticks: { maxTicksLimit: 10, font: { size: 11 }, color: tickColor },
        },
        y: {
          beginAtZero: true,
          grid: { color: gridColor },
          ticks: { font: { size: 11 }, color: tickColor },
        },
      },
    },
  })
}

async function rebuild() {
  await nextTick()
  buildChart()
}

onMounted(rebuild)
watch(() => [props.userId, props.dailyData, props.days], rebuild)
onBeforeUnmount(() => chart?.destroy())
</script>
