<template>
  <div class="rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-gray-700 dark:bg-gray-800">
    <p class="text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">{{ title }}</p>
    <div v-if="loading" class="mt-2 h-8 w-16 animate-pulse rounded bg-gray-200 dark:bg-gray-700" />
    <p v-else class="mt-1 text-2xl font-bold" :class="valueClass">{{ value.toLocaleString() }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  title: string
  value: number
  loading?: boolean
  color?: 'green' | 'blue' | 'purple' | 'yellow' | 'red'
}>()

const valueClass = computed(() => ({
  'text-green-600 dark:text-green-400': props.color === 'green',
  'text-blue-600 dark:text-blue-400': props.color === 'blue',
  'text-purple-600 dark:text-purple-400': props.color === 'purple',
  'text-yellow-600 dark:text-yellow-400': props.color === 'yellow',
  'text-red-600 dark:text-red-400': props.color === 'red',
  'text-gray-900 dark:text-white': !props.color,
}))
</script>
