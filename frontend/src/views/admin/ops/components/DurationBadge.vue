<template>
  <template v-if="ms == null">
    <span class="text-gray-400">—</span>
  </template>
  <template v-else>
    <!-- 主要耗时数值 -->
    <span
      class="inline-flex items-center gap-1 font-mono font-semibold"
      :class="levelClass"
      :title="tooltipText"
    >
      {{ displayText }}
      <!-- 速度等级圆点 -->
      <span class="h-1.5 w-1.5 rounded-full flex-shrink-0" :class="dotClass" />
    </span>
  </template>
</template>

<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  ms: number | null
}

const props = defineProps<Props>()

/** 格式化耗时：< 1000ms 显示 ms，>= 1000ms 显示 s */
const displayText = computed(() => {
  if (props.ms == null) return '—'
  if (props.ms < 1000) return `${props.ms} ms`
  return `${(props.ms / 1000).toFixed(1)} s`
})

/** 速度等级：fast < 2s, medium < 8s, slow >= 8s */
const level = computed(() => {
  if (props.ms == null) return 'unknown'
  if (props.ms < 2000) return 'fast'
  if (props.ms < 8000) return 'medium'
  return 'slow'
})

const levelClass = computed(() => {
  switch (level.value) {
    case 'fast': return 'text-emerald-600 dark:text-emerald-400'
    case 'medium': return 'text-amber-600 dark:text-amber-400'
    case 'slow': return 'text-red-600 dark:text-red-400'
    default: return 'text-gray-500 dark:text-gray-400'
  }
})

const dotClass = computed(() => {
  switch (level.value) {
    case 'fast': return 'bg-emerald-500'
    case 'medium': return 'bg-amber-500'
    case 'slow': return 'bg-red-500'
    default: return 'bg-gray-400'
  }
})

const tooltipText = computed(() => {
  if (props.ms == null) return ''
  return `${props.ms} ms`
})
</script>
