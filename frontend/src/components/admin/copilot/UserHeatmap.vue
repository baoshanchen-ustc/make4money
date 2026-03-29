<template>
  <div>
    <div v-if="loading" class="flex h-24 items-center justify-center">
      <LoadingSpinner />
    </div>
    <div v-else-if="error" class="text-sm text-red-500">{{ error }}</div>
    <div v-else class="relative">
      <!-- 小时列标签 -->
      <div class="mb-1 grid grid-cols-[2rem_repeat(24,1fr)] gap-0.5 text-center">
        <span />
        <span
          v-for="h in 24"
          :key="h"
          class="text-[10px] text-gray-400 dark:text-gray-500"
        >{{ (h - 1).toString().padStart(2, '0') }}</span>
      </div>
      <!-- 每一天一行 -->
      <div
        v-for="row in rows"
        :key="row.date"
        class="grid grid-cols-[2rem_repeat(24,1fr)] gap-0.5"
      >
        <span class="text-right text-[10px] leading-4 text-gray-400 dark:text-gray-500 pr-1">
          {{ row.label }}
        </span>
        <div
          v-for="cell in row.cells"
          :key="cell.hour"
          class="h-4 rounded-sm cursor-default transition-opacity hover:opacity-80"
          :style="{ backgroundColor: heatColor(cell.count, maxCount) }"
          @mouseenter="(e) => showTooltip(e, row.date, cell)"
          @mouseleave="hideTooltip"
        />
      </div>
      <!-- 图例 -->
      <div class="mt-2 flex items-center gap-1 justify-end">
        <span class="text-[10px] text-gray-400">少</span>
        <div v-for="step in legendSteps" :key="step" class="h-3 w-5 rounded-sm" :style="{ backgroundColor: heatColor(step, 5) }" />
        <span class="text-[10px] text-gray-400">多</span>
      </div>

      <!-- 自定义 Tooltip -->
      <Teleport to="body">
        <div
          v-if="tooltip.visible"
          class="pointer-events-none fixed z-[99999] rounded-lg border border-gray-200 bg-white px-3 py-2 shadow-lg dark:border-gray-700 dark:bg-gray-800"
          :style="{ left: tooltip.x + 'px', top: tooltip.y + 'px' }"
        >
          <p class="text-xs font-semibold text-gray-700 dark:text-gray-200">
            {{ tooltip.date }} {{ tooltip.hour.toString().padStart(2, '0') }}:00
          </p>
          <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
            请求数：<span class="font-bold text-blue-600 dark:text-blue-400">{{ tooltip.count }}</span> 次
          </p>
        </div>
      </Teleport>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, watch, onMounted } from 'vue'
import { getCopilotUserTimeline } from '@/api/admin/copilotAnalytics'
import { extractErrorMessage } from '@/api/client'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'

const props = defineProps<{
  userId: number
  days?: number
}>()

interface HeatCell { hour: number; count: number }
interface HeatRow { date: string; label: string; cells: HeatCell[] }

const loading = ref(false)
const error = ref<string | null>(null)
const rows = ref<HeatRow[]>([])
const maxCount = ref(1)
const legendSteps = [0, 1, 2, 3, 4, 5]

const tooltip = reactive({
  visible: false,
  x: 0,
  y: 0,
  date: '',
  hour: 0,
  count: 0,
})

function showTooltip(event: MouseEvent, date: string, cell: HeatCell) {
  const rect = (event.target as HTMLElement).getBoundingClientRect()
  // Position tooltip above the cell, centered
  tooltip.x = rect.left + rect.width / 2 - 70
  tooltip.y = rect.top - 60
  tooltip.date = date
  tooltip.hour = cell.hour
  tooltip.count = cell.count
  tooltip.visible = true
}

function hideTooltip() {
  tooltip.visible = false
}

function localDateStr(offset: number): string {
  const d = new Date()
  d.setDate(d.getDate() + offset)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

function heatColor(count: number, max: number): string {
  if (count === 0 || max === 0) return '#e5e7eb'
  const ratio = Math.min(count / max, 1)
  const r = Math.round(219 - ratio * 170)
  const g = Math.round(234 - ratio * 130)
  const b = Math.round(254 - ratio * 60)
  return `rgb(${r},${g},${b})`
}

async function load() {
  if (!props.userId) return
  loading.value = true
  error.value = null
  const numDays = props.days ?? 7
  try {
    const results = await Promise.all(
      Array.from({ length: numDays }, (_, i) => {
        const date = localDateStr(-(numDays - 1 - i))
        return getCopilotUserTimeline(props.userId, { date }).then(r => ({ date, hourly: r.hourly }))
      }),
    )
    let globalMax = 0
    rows.value = results.map(({ date, hourly }) => {
      const cells = hourly.map(h => {
        const count = h.premium_count + h.agent_count
        if (count > globalMax) globalMax = count
        return { hour: h.hour, count }
      })
      const [, m, d] = date.split('-')
      return { date, label: `${m}/${d}`, cells }
    })
    maxCount.value = globalMax || 1
  } catch (e: unknown) {
    error.value = extractErrorMessage(e)
  } finally {
    loading.value = false
  }
}

onMounted(load)
watch(() => [props.userId, props.days], load)
</script>
