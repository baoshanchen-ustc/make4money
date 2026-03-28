<template>
  <div>
    <div v-if="loading" class="flex h-32 items-center justify-center text-xs text-gray-400">加载中…</div>
    <div v-else-if="loadError" class="flex h-32 items-center justify-center text-xs text-red-500">
      加载失败：{{ loadError }}
    </div>
    <div v-else-if="!data || data.trend.length === 0" class="flex h-32 items-center justify-center text-xs text-gray-400">
      暂无趋势数据
    </div>
    <Line v-else :data="chartData" :options="chartOptions" class="max-h-48" />
  </div>
</template>

<script setup lang="ts">
import { computed, watch, ref } from 'vue'
import { Line } from 'vue-chartjs'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler,
} from 'chart.js'
import { getCopilotAccountQuotaTrend } from '@/api/admin/copilotAnalytics'
import type { CopilotAccountQuotaTrendResult } from '@/api/admin/copilotAnalytics'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Title, Tooltip, Legend, Filler)

const props = defineProps<{ accountId: number; days?: number }>()

const loading = ref(false)
const data = ref<CopilotAccountQuotaTrendResult | null>(null)
const loadError = ref<string | null>(null)

async function load() {
  loading.value = true
  loadError.value = null
  try {
    data.value = await getCopilotAccountQuotaTrend(props.accountId, { days: props.days ?? 30 })
  } catch (e: unknown) {
    loadError.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

watch(() => [props.accountId, props.days], load, { immediate: true })

const chartData = computed(() => {
  const trend = data.value?.trend ?? []
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
      },
      {
        label: '配额上限',
        data: trend.map(t => t.premium_entitlement),
        borderColor: 'rgb(209, 213, 219)',
        borderDash: [4, 4],
        fill: false,
        tension: 0.3,
        pointRadius: 0,
      },
    ],
  }
})

const chartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: { display: true, position: 'bottom' as const, labels: { boxWidth: 12, font: { size: 11 } } },
    tooltip: { mode: 'index' as const, intersect: false },
  },
  scales: {
    x: { ticks: { font: { size: 10 }, maxTicksLimit: 10 } },
    y: { beginAtZero: true, ticks: { font: { size: 10 } } },
  },
}
</script>
