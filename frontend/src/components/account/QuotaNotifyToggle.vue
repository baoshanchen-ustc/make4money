<script setup lang="ts">
import { QUOTA_THRESHOLD_TYPE_FIXED, QUOTA_THRESHOLD_TYPE_PERCENTAGE, type QuotaThresholdType } from '@/constants/account'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

defineProps<{
  enabled: boolean | null
  threshold: number | null
  thresholdType: QuotaThresholdType | null
}>()

const emit = defineEmits<{
  'update:enabled': [value: boolean | null]
  'update:threshold': [value: number | null]
  'update:thresholdType': [value: QuotaThresholdType | null]
}>()

function toggleType(current: string | null) {
  emit('update:thresholdType', current === QUOTA_THRESHOLD_TYPE_PERCENTAGE ? QUOTA_THRESHOLD_TYPE_FIXED : QUOTA_THRESHOLD_TYPE_PERCENTAGE)
}
</script>

<template>
  <div class="flex items-center gap-1.5">
    <label class="text-sm text-gray-500 whitespace-nowrap">{{ t('admin.accounts.quotaNotify.alert') }}</label>
    <button
      type="button"
      @click="emit('update:enabled', !enabled)"
      :class="[
        'relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none',
        enabled ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
      ]"
    >
      <span
        :class="[
          'pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
          enabled ? 'translate-x-4' : 'translate-x-0'
        ]"
      />
    </button>
    <template v-if="enabled">
      <button
        type="button"
        class="px-1.5 py-0.5 text-xs font-medium rounded border transition-colors"
        :class="(!thresholdType || thresholdType === QUOTA_THRESHOLD_TYPE_FIXED) ? 'bg-primary-100 text-primary-700 border-primary-300 dark:bg-primary-900/30 dark:text-primary-400 dark:border-primary-700' : 'bg-gray-100 text-gray-500 border-gray-200 dark:bg-dark-600 dark:text-gray-400 dark:border-dark-500'"
        @click="toggleType(thresholdType)"
      >
        $
      </button>
      <button
        type="button"
        class="px-1.5 py-0.5 text-xs font-medium rounded border transition-colors"
        :class="thresholdType === QUOTA_THRESHOLD_TYPE_PERCENTAGE ? 'bg-primary-100 text-primary-700 border-primary-300 dark:bg-primary-900/30 dark:text-primary-400 dark:border-primary-700' : 'bg-gray-100 text-gray-500 border-gray-200 dark:bg-dark-600 dark:text-gray-400 dark:border-dark-500'"
        @click="toggleType(thresholdType)"
      >
        %
      </button>
      <div class="relative flex-1">
        <input
          :value="threshold"
          @input="emit('update:threshold', parseFloat(($event.target as HTMLInputElement).value) || null)"
          type="number"
          min="0"
          :max="thresholdType === QUOTA_THRESHOLD_TYPE_PERCENTAGE ? 100 : undefined"
          :step="thresholdType === QUOTA_THRESHOLD_TYPE_PERCENTAGE ? 1 : 0.01"
          class="input py-1 text-sm w-full pr-7"
          :placeholder="thresholdType === QUOTA_THRESHOLD_TYPE_PERCENTAGE ? t('admin.accounts.quotaNotify.thresholdPlaceholder') : t('admin.accounts.quotaNotify.threshold')"
        />
        <span class="absolute right-2.5 top-1/2 -translate-y-1/2 text-xs text-gray-400 pointer-events-none">
          {{ thresholdType === QUOTA_THRESHOLD_TYPE_PERCENTAGE ? '%' : '$' }}
        </span>
      </div>
    </template>
  </div>
</template>
