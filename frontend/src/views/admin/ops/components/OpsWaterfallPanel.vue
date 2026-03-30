<script setup lang="ts">
import { computed } from 'vue'
import type { OpsSpan, OpsRequestDetail } from '@/api/admin/ops'

const props = defineProps<{
  row: OpsRequestDetail
}>()

interface WaterfallBar {
  label: string
  durationMs: number
  offsetMs: number
  color: string
  status: string
}

const PHASE_COLORS: Record<string, string> = {
  'token.fetch':     'bg-yellow-400',
  'translate.req':   'bg-blue-300',
  'upstream.post':   'bg-indigo-500',
  'failover.select': 'bg-orange-400',
  'routing.select':  'bg-emerald-400',
  'auth.verify':     'bg-teal-400',
  'translate.resp':  'bg-sky-300',
  'body.truncation': 'bg-red-400',
}

const PHASE_LABELS: Record<string, string> = {
  'token.fetch':     'Token 获取',
  'translate.req':   '请求转译',
  'upstream.post':   '上游请求',
  'failover.select': 'Failover 切换',
  'routing.select':  '路由选择',
  'auth.verify':     'Auth 验证',
  'translate.resp':  '响应转译',
  'body.truncation': 'Body 截断',
}

const bars = computed((): WaterfallBar[] => {
  const spans = props.row.spans
  if (!spans || spans.length === 0) return []

  const t0 = Math.min(...spans.map((s: OpsSpan) => s.start_unix_ms))

  return spans.map((span: OpsSpan) => ({
    label: PHASE_LABELS[span.name] ?? span.name,
    durationMs: span.duration_ms,
    offsetMs: span.start_unix_ms - t0,
    color: span.status === 'error'
      ? 'bg-red-500'
      : (PHASE_COLORS[span.name] ?? 'bg-gray-400'),
    status: span.status ?? 'ok',
  }))
})

const totalMs = computed(() => props.row.duration_ms ?? 1)
</script>

<template>
  <div class="space-y-1 py-2">
    <div
      v-for="(bar, i) in bars"
      :key="i"
      class="flex items-center gap-2 text-xs"
    >
      <div class="w-28 flex-shrink-0 truncate text-right text-[11px] text-gray-500 dark:text-gray-400">
        {{ bar.label }}
      </div>
      <div class="relative flex-1 h-4 rounded bg-gray-100 dark:bg-dark-800 overflow-hidden">
        <div
          class="absolute top-0 h-4 rounded transition-all"
          :class="bar.color"
          :style="{
            left: `${(bar.offsetMs / totalMs) * 100}%`,
            width: `${Math.max((bar.durationMs / totalMs) * 100, 0.5)}%`,
          }"
        />
      </div>
      <div class="w-16 flex-shrink-0 text-right font-mono text-[11px] text-gray-600 dark:text-gray-300">
        {{ bar.durationMs }}ms
      </div>
    </div>

    <div
      v-if="bars.length === 0"
      class="py-4 text-center text-xs text-gray-400"
    >
      暂无 Span 数据（此请求发生在埋点上线之前）
    </div>
  </div>
</template>
