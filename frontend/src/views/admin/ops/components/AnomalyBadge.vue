<template>
  <span
    class="inline-flex items-center rounded-full font-bold ring-1 ring-inset"
    :class="[props.small ? 'px-2 py-0.5 text-[10px]' : 'px-2.5 py-1 text-xs', toneClassMap[props.type]]"
  >
    {{ t(`admin.ops.anomaly.types.${props.type}`) }}
  </span>
</template>

<script setup lang="ts">
import type { AnomalyType } from '@/api/admin/ops'
import { useI18n } from 'vue-i18n'

interface Props {
  type: AnomalyType
  small?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  small: false
})

const { t } = useI18n()

const toneClassMap: Record<AnomalyType, string> = {
  zero_token:
    'bg-amber-50 text-amber-700 ring-amber-600/20 dark:bg-amber-900/30 dark:text-amber-300 dark:ring-amber-500/30',
  slow_request:
    'bg-orange-50 text-orange-700 ring-orange-600/20 dark:bg-orange-900/30 dark:text-orange-300 dark:ring-orange-500/30',
  timeout: 'bg-red-50 text-red-700 ring-red-600/20 dark:bg-red-900/30 dark:text-red-300 dark:ring-red-500/30',
  error: 'bg-rose-50 text-rose-700 ring-rose-600/20 dark:bg-rose-900/30 dark:text-rose-300 dark:ring-rose-500/30',
  quota_exhaustion_suspected:
    'bg-violet-50 text-violet-700 ring-violet-600/20 dark:bg-violet-900/30 dark:text-violet-300 dark:ring-violet-500/30'
}
</script>
