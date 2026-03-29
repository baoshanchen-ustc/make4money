<template>
  <div class="relative">
    <div v-if="loading" class="flex h-[300px] items-center justify-center">
      <LoadingSpinner />
    </div>
    <div v-else-if="error" class="flex h-[300px] items-center justify-center text-sm text-red-500">
      {{ error }}
    </div>
    <div v-else class="h-[300px]">
      <canvas ref="chartRef" class="!h-full !w-full" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onBeforeUnmount } from 'vue'
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
import { getCopilotUsersDailyStats } from '@/api/admin/copilotAnalytics'
import type { CopilotUsersDailyStatsResult } from '@/api/admin/copilotAnalytics'
import { extractErrorMessage } from '@/api/client'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'

Chart.register(LineController, LineElement, PointElement, LinearScale, CategoryScale, Tooltip, Legend, Filler)

const props = withDefaults(defineProps<{
  days?: number
  metric?: 'premium' | 'agent' | 'total'
}>(), {
  days: 30,
  metric: 'premium',
})

const PALETTE = [
  '#3b82f6', '#10b981', '#f59e0b', '#ef4444', '#8b5cf6',
  '#06b6d4', '#f97316', '#84cc16', '#ec4899', '#6366f1',
]

const chartRef = ref<HTMLCanvasElement | null>(null)
let chart: Chart | null = null
const loading = ref(false)
const error = ref<string | null>(null)

async function buildChart() {
  loading.value = true
  error.value = null
  try {
    const result: CopilotUsersDailyStatsResult = await getCopilotUsersDailyStats({ days: props.days })

    const dates: string[] = []
    const today = new Date()
    for (let i = props.days - 1; i >= 0; i--) {
      const d = new Date(today)
      d.setDate(d.getDate() - i)
      dates.push(`${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`)
    }

    const countsByUser = new Map<number, Map<string, number>>()
    for (const entry of result.days) {
      if (!countsByUser.has(entry.user_id)) {
        countsByUser.set(entry.user_id, new Map())
      }
      const val = props.metric === 'premium'
        ? entry.premium_count
        : props.metric === 'agent'
          ? entry.agent_count
          : entry.premium_count + entry.agent_count
      countsByUser.get(entry.user_id)!.set(entry.date, val)
    }

    const datasets = result.users.map((user, idx) => {
      const userMap = countsByUser.get(user.user_id) ?? new Map()
      return {
        label: user.username,
        data: dates.map(d => userMap.get(d) ?? 0),
        borderColor: PALETTE[idx % PALETTE.length],
        backgroundColor: PALETTE[idx % PALETTE.length] + '18',
        borderWidth: 2,
        pointRadius: props.days <= 14 ? 3 : 0,
        pointHoverRadius: 5,
        tension: 0.3,
        fill: false,
      }
    })

    chart?.destroy()
    if (!chartRef.value) return
    chart = new Chart(chartRef.value, {
      type: 'line',
      data: { labels: dates, datasets },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        interaction: { mode: 'index', intersect: false },
        plugins: {
          legend: {
            position: 'bottom',
            labels: { boxWidth: 12, padding: 16, font: { size: 12 } },
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
            ticks: { maxTicksLimit: 10, font: { size: 11 } },
          },
          y: {
            beginAtZero: true,
            grid: { color: 'rgba(0,0,0,0.06)' },
            ticks: { font: { size: 11 } },
          },
        },
      },
    })
  } catch (e: unknown) {
    error.value = extractErrorMessage(e)
  } finally {
    loading.value = false
  }
}

onMounted(buildChart)
watch(() => [props.days, props.metric], buildChart)
onBeforeUnmount(() => chart?.destroy())
</script>
