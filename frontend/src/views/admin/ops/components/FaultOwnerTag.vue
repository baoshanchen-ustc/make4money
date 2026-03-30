<script setup lang="ts">
import type { FaultOwner } from '@/api/admin/ops'

const props = defineProps<{
  owner: FaultOwner | null | undefined
}>()

const CONFIG: Record<string, { label: string; classes: string }> = {
  client:   { label: '客户端',  classes: 'bg-amber-100   text-amber-700   dark:bg-amber-900/30  dark:text-amber-400'  },
  upstream: { label: '上游',    classes: 'bg-red-100     text-red-700     dark:bg-red-900/30    dark:text-red-400'    },
  platform: { label: 'sub2api', classes: 'bg-purple-100  text-purple-700  dark:bg-purple-900/30 dark:text-purple-400' },
  ok:       { label: '正常',    classes: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400'},
}

const config = () => CONFIG[props.owner ?? 'ok'] ?? CONFIG['ok']
</script>

<template>
  <span
    class="inline-flex items-center rounded-full px-2 py-0.5 text-[10px] font-bold"
    :class="config().classes"
  >
    {{ config().label }}
  </span>
</template>
