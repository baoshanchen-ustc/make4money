<template>
  <div class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
    <h3 class="text-sm font-black uppercase tracking-wider text-gray-900 dark:text-white">
      {{ t('admin.ops.requestInspect.latency.title') }}
    </h3>

    <!-- 无数据提示 -->
    <p
      v-if="!hasStageData"
      class="mt-3 text-xs text-gray-400 dark:text-gray-500"
    >
      {{ t('admin.ops.requestInspect.latency.noData') }}
    </p>

    <template v-else>
      <!-- 总耗时 + TTFB 概要行 -->
      <div class="mt-3 flex flex-wrap items-baseline gap-x-4 gap-y-1">
        <!-- 分段之和（与 Gantt 同口径） -->
        <span class="text-base font-bold text-gray-900 dark:text-white">
          {{ totalMs != null ? totalMs + ' ms' : '—' }}
          <span class="ml-1 text-xs font-normal text-gray-400">{{ t('admin.ops.requestInspect.latency.total') }}</span>
        </span>
        <!-- end-to-end duration_ms（若与分段和不同则额外展示） -->
        <span
          v-if="detail.duration_ms != null && stageSum != null && Math.abs(detail.duration_ms - stageSum) > 50"
          class="text-sm font-semibold text-gray-500 dark:text-gray-400"
          :title="t('admin.ops.requestInspect.latency.e2eDesc')"
        >
          {{ detail.duration_ms }} ms
          <span class="ml-1 text-xs font-normal text-gray-400">{{ t('admin.ops.requestInspect.latency.e2e') }}</span>
        </span>
        <span v-if="detail.first_token_ms != null" class="text-sm font-semibold text-indigo-600 dark:text-indigo-400">
          {{ detail.first_token_ms }} ms
          <span class="ml-1 text-xs font-normal text-gray-400">{{ t('admin.ops.requestInspect.latency.ttfb') }}</span>
        </span>
        <span v-if="tokensPerSec != null" class="text-sm font-semibold text-emerald-600 dark:text-emerald-400">
          {{ tokensPerSec }} {{ t('admin.ops.requestInspect.latency.tokensPerSec') }}
        </span>
      </div>

      <!-- Gantt 时间轴 -->
      <div class="mt-4 space-y-2">
        <div
          v-for="segment in segments"
          :key="segment.key"
          class="flex items-center gap-3"
        >
          <!-- 标签 -->
          <div class="w-20 flex-shrink-0 text-right text-[11px] font-semibold text-gray-500 dark:text-gray-400">
            {{ segment.label }}
          </div>
          <!-- 进度条轨道 -->
          <div
            class="relative h-5 flex-1 overflow-hidden rounded bg-gray-100 dark:bg-dark-700"
            :title="segment.tooltip"
          >
            <!-- 宽度为该段占总耗时百分比；ms 为 null 时不渲染色块 -->
            <div
              v-if="segment.ms != null"
              class="absolute left-0 top-0 h-full rounded transition-all duration-300"
              :class="segment.colorClass"
              :style="{ width: Math.max(segment.pct, 2) + '%' }"
            />
            <!-- 数值标注：若足够宽则放在条内，否则放在条外 -->
            <span
              class="absolute inset-y-0 flex items-center px-1.5 text-[10px] font-bold"
              :class="segment.ms != null && segment.pct > 18 ? 'left-0 text-white' : 'text-gray-600 dark:text-gray-300'"
              :style="segment.ms != null && segment.pct <= 18 ? { left: Math.max(segment.pct, 2) + '%' } : {}"
            >
              {{ segment.ms != null ? segment.ms + ' ms' : '—' }}
            </span>
          </div>
        </div>
      </div>

      <!-- 图例说明 -->
      <div class="mt-3 grid grid-cols-2 gap-1 sm:grid-cols-4">
        <div
          v-for="segment in segments"
          :key="'legend-' + segment.key"
          class="flex items-start gap-1.5 rounded-lg px-2 py-1.5 hover:bg-gray-50 dark:hover:bg-dark-700"
          :title="segment.tooltip"
        >
          <span class="mt-0.5 h-2.5 w-2.5 flex-shrink-0 rounded-sm" :class="segment.colorClass" />
          <div class="min-w-0">
            <div class="text-[11px] font-semibold text-gray-700 dark:text-gray-200">{{ segment.label }}</div>
            <div class="text-[10px] leading-tight text-gray-400 dark:text-gray-500">{{ segment.tooltip }}</div>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

// 最小公共接口，兼容 OpsUsageInspectDetail 和 OpsErrorDetail
interface LatencyData {
  duration_ms?: number | null
  first_token_ms?: number | null
  output_tokens?: number | null
  auth_latency_ms?: number | null
  routing_latency_ms?: number | null
  upstream_latency_ms?: number | null
  response_latency_ms?: number | null
}

interface Props {
  detail: LatencyData
}

const props = defineProps<Props>()
const { t } = useI18n()

// 是否有任何阶段数据
const hasStageData = computed(() => {
  const d = props.detail
  return (
    d.auth_latency_ms != null ||
    d.routing_latency_ms != null ||
    d.upstream_latency_ms != null ||
    d.response_latency_ms != null
  )
})

// 总耗时：优先用各阶段之和（与分段同口径，避免 duration_ms 口径偏差导致百分比失真）
// 若阶段数据全为 null，回退到 duration_ms
const stageSum = computed(() => {
  const d = props.detail
  const vals = [d.auth_latency_ms, d.routing_latency_ms, d.upstream_latency_ms, d.response_latency_ms]
  const known = vals.filter((v): v is number => v != null)
  return known.length > 0 ? known.reduce((a, b) => a + b, 0) : null
})

const totalMs = computed(() => stageSum.value ?? props.detail.duration_ms ?? null)

// tokens/s 生成速率（使用 duration_ms 保持与实际传输时间一致）
const tokensPerSec = computed(() => {
  const d = props.detail
  const dur = d.duration_ms
  const out = d.output_tokens
  if (!dur || !out || dur <= 0) return null
  const rate = (out / (dur / 1000)).toFixed(1)
  return rate
})

interface Segment {
  key: string
  label: string
  tooltip: string
  ms: number | null
  pct: number
  colorClass: string
}

const segments = computed((): Segment[] => {
  const d = props.detail
  const total = totalMs.value ?? 0

  const calcPct = (ms: number | null | undefined): number => {
    if (ms == null || total <= 0) return 0
    return Math.min(100, Math.round((ms / total) * 100))
  }

  return [
    {
      key: 'auth',
      label: t('admin.ops.requestInspect.latency.auth'),
      tooltip: t('admin.ops.requestInspect.latency.authDesc'),
      ms: d.auth_latency_ms ?? null,
      pct: calcPct(d.auth_latency_ms),
      colorClass: 'bg-violet-400 dark:bg-violet-500'
    },
    {
      key: 'routing',
      label: t('admin.ops.requestInspect.latency.routing'),
      tooltip: t('admin.ops.requestInspect.latency.routingDesc'),
      ms: d.routing_latency_ms ?? null,
      pct: calcPct(d.routing_latency_ms),
      colorClass: 'bg-sky-400 dark:bg-sky-500'
    },
    {
      key: 'upstream',
      label: t('admin.ops.requestInspect.latency.upstream'),
      tooltip: t('admin.ops.requestInspect.latency.upstreamDesc'),
      ms: d.upstream_latency_ms ?? null,
      pct: calcPct(d.upstream_latency_ms),
      colorClass: 'bg-amber-400 dark:bg-amber-500'
    },
    {
      key: 'response',
      label: t('admin.ops.requestInspect.latency.response'),
      tooltip: t('admin.ops.requestInspect.latency.responseDesc'),
      ms: d.response_latency_ms ?? null,
      pct: calcPct(d.response_latency_ms),
      colorClass: 'bg-emerald-400 dark:bg-emerald-500'
    }
  ]
})
</script>
