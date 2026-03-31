<template>
  <div class="flex flex-wrap items-center gap-2">
    <button
      v-for="type in ALL_TYPES"
      :key="type"
      type="button"
      class="inline-flex items-center rounded-full px-2.5 py-1 text-xs font-bold ring-1 ring-inset transition-colors"
      :class="isSelected(type) ? filledClassMap[type] : outlinedClassMap[type]"
      :aria-pressed="isSelected(type)"
      @click="toggleType(type)"
    >
      {{ t(`admin.ops.anomaly.types.${type}`) }}
    </button>

    <button
      v-if="hasSelection"
      type="button"
      class="ml-1 text-xs font-semibold text-gray-500 underline-offset-4 hover:text-gray-700 hover:underline dark:text-gray-400 dark:hover:text-gray-200"
      @click="clearAll"
    >
      {{ t('admin.ops.anomaly.clearFilter') }}
    </button>
  </div>
</template>

<script setup lang="ts">
import type { AnomalyType } from '@/api/admin/ops'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

interface Props {
  modelValue: AnomalyType[]
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: AnomalyType[]): void
}>()

const { t } = useI18n()

const ALL_TYPES: AnomalyType[] = ['zero_token', 'slow_request', 'timeout', 'error', 'quota_exhaustion_suspected']

const filledClassMap: Record<AnomalyType, string> = {
  zero_token:
    'bg-amber-100 text-amber-800 ring-amber-600/20 hover:bg-amber-200 dark:bg-amber-900/40 dark:text-amber-200 dark:ring-amber-500/30 dark:hover:bg-amber-900/60',
  slow_request:
    'bg-orange-100 text-orange-800 ring-orange-600/20 hover:bg-orange-200 dark:bg-orange-900/40 dark:text-orange-200 dark:ring-orange-500/30 dark:hover:bg-orange-900/60',
  timeout:
    'bg-red-100 text-red-800 ring-red-600/20 hover:bg-red-200 dark:bg-red-900/40 dark:text-red-200 dark:ring-red-500/30 dark:hover:bg-red-900/60',
  error:
    'bg-rose-100 text-rose-800 ring-rose-600/20 hover:bg-rose-200 dark:bg-rose-900/40 dark:text-rose-200 dark:ring-rose-500/30 dark:hover:bg-rose-900/60',
  quota_exhaustion_suspected:
    'bg-violet-100 text-violet-800 ring-violet-600/20 hover:bg-violet-200 dark:bg-violet-900/40 dark:text-violet-200 dark:ring-violet-500/30 dark:hover:bg-violet-900/60'
}

const outlinedClassMap: Record<AnomalyType, string> = {
  zero_token:
    'bg-transparent text-amber-700 ring-amber-200 hover:bg-amber-50 dark:text-amber-300 dark:ring-amber-800/60 dark:hover:bg-amber-900/20',
  slow_request:
    'bg-transparent text-orange-700 ring-orange-200 hover:bg-orange-50 dark:text-orange-300 dark:ring-orange-800/60 dark:hover:bg-orange-900/20',
  timeout:
    'bg-transparent text-red-700 ring-red-200 hover:bg-red-50 dark:text-red-300 dark:ring-red-800/60 dark:hover:bg-red-900/20',
  error:
    'bg-transparent text-rose-700 ring-rose-200 hover:bg-rose-50 dark:text-rose-300 dark:ring-rose-800/60 dark:hover:bg-rose-900/20',
  quota_exhaustion_suspected:
    'bg-transparent text-violet-700 ring-violet-200 hover:bg-violet-50 dark:text-violet-300 dark:ring-violet-800/60 dark:hover:bg-violet-900/20'
}

const hasSelection = computed(() => (props.modelValue || []).length > 0)

function isSelected(type: AnomalyType): boolean {
  return (props.modelValue || []).includes(type)
}

function toggleType(type: AnomalyType) {
  const next = new Set(props.modelValue || [])
  if (next.has(type)) {
    next.delete(type)
  } else {
    next.add(type)
  }
  emit(
    'update:modelValue',
    ALL_TYPES.filter((item) => next.has(item))
  )
}

function clearAll() {
  emit('update:modelValue', [])
}
</script>
