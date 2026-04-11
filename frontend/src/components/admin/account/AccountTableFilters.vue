<template>
  <div class="flex flex-wrap items-center gap-3">
    <SearchInput
      :model-value="searchQuery"
      :placeholder="t('admin.accounts.searchAccounts')"
      class="w-full sm:w-64"
      @update:model-value="$emit('update:searchQuery', $event)"
      @search="$emit('change')"
    />
    <Select
      :model-value="filters.platform"
      class="w-40"
      :options="pOpts"
      @update:model-value="updatePlatform"
      @change="$emit('change')"
    />
    <Select
      :model-value="filters.type"
      class="w-40"
      :options="tOpts"
      @update:model-value="updateType"
      @change="$emit('change')"
    />
    <Select
      :model-value="filters.status"
      class="w-40"
      :options="sOpts"
      @update:model-value="updateStatus"
      @change="$emit('change')"
    />

    <div class="relative" ref="planTypeDropdownRef">
      <button
        type="button"
        data-testid="account-plan-types-trigger"
        class="flex w-44 items-center justify-between gap-2 rounded-xl border border-gray-200 bg-white px-4 py-2.5 text-sm text-gray-900 transition-all duration-200 hover:border-gray-300 focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-500/30 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-100 dark:hover:border-dark-500"
        :class="planTypeDropdownOpen && 'border-primary-500 ring-2 ring-primary-500/30'"
        :title="planTypeButtonTitle"
        @click="togglePlanTypeDropdown"
      >
        <span class="truncate">{{ planTypeButtonLabel }}</span>
        <svg
          class="h-4 w-4 flex-shrink-0 text-gray-400 transition-transform duration-200"
          :class="planTypeDropdownOpen && 'rotate-180'"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
          stroke-width="1.5"
        >
          <path stroke-linecap="round" stroke-linejoin="round" d="M19.5 8.25L12 15.75 4.5 8.25" />
        </svg>
      </button>

      <div
        v-if="planTypeDropdownOpen"
        class="absolute left-0 z-50 mt-2 w-56 rounded-xl border border-gray-200 bg-white shadow-lg dark:border-gray-700 dark:bg-gray-800"
      >
        <div class="max-h-64 overflow-y-auto p-2">
          <label
            v-for="option in planTypeOpts"
            :key="String(option.value)"
            :data-testid="`account-plan-types-option-${String(option.value)}`"
            class="flex cursor-pointer items-center gap-2 rounded-md px-3 py-2 text-sm text-gray-700 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-gray-700"
          >
            <input
              type="checkbox"
              class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
              :checked="selectedPlanTypes.includes(String(option.value))"
              @change="togglePlanType(String(option.value))"
            />
            <span>{{ option.label }}</span>
          </label>
        </div>
        <div class="flex items-center justify-between border-t border-gray-100 px-3 py-2 dark:border-gray-700">
          <button
            type="button"
            data-testid="account-plan-types-reset"
            class="text-xs font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
            @click="clearPlanTypes"
          >
            {{ t('common.reset') }}
          </button>
          <span class="text-xs text-gray-500 dark:text-gray-400">
            {{ selectedPlanTypes.length ? t('admin.accounts.planTypesSelected', { count: selectedPlanTypes.length }) : t('common.all') }}
          </span>
        </div>
      </div>
    </div>

    <Select
      :model-value="filters.privacy_mode"
      class="w-40"
      :options="privacyOpts"
      @update:model-value="updatePrivacyMode"
      @change="$emit('change')"
    />
    <Select
      :model-value="filters.group"
      class="w-40"
      :options="gOpts"
      @update:model-value="updateGroup"
      @change="$emit('change')"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import type { AdminGroup } from '@/types'

const props = defineProps<{ searchQuery: string; filters: Record<string, any>; groups?: AdminGroup[] }>()
const emit = defineEmits(['update:searchQuery', 'update:filters', 'change'])
const { t } = useI18n()

const planTypeDropdownRef = ref<HTMLElement | null>(null)
const planTypeDropdownOpen = ref(false)

const normalizePlanTypes = (value: unknown): string[] => {
  if (Array.isArray(value)) {
    return value.map(item => String(item).trim()).filter(Boolean)
  }
  if (typeof value === 'string') {
    return value
      .split(',')
      .map(item => item.trim())
      .filter(Boolean)
  }
  return []
}

const selectedPlanTypes = computed<string[]>(() => normalizePlanTypes(props.filters.plan_types))

const updatePlatform = (value: string | number | boolean | null) => {
  emit('update:filters', { ...props.filters, platform: value })
}
const updateType = (value: string | number | boolean | null) => {
  emit('update:filters', { ...props.filters, type: value })
}
const updateStatus = (value: string | number | boolean | null) => {
  emit('update:filters', { ...props.filters, status: value })
}
const updatePrivacyMode = (value: string | number | boolean | null) => {
  emit('update:filters', { ...props.filters, privacy_mode: value })
}
const updateGroup = (value: string | number | boolean | null) => {
  emit('update:filters', { ...props.filters, group: value })
}
const updatePlanTypes = (planTypes: string[]) => {
  emit('update:filters', { ...props.filters, plan_types: planTypes })
  emit('change')
}

const togglePlanTypeDropdown = () => {
  planTypeDropdownOpen.value = !planTypeDropdownOpen.value
}

const togglePlanType = (value: string) => {
  const nextPlanTypes = selectedPlanTypes.value.includes(value)
    ? selectedPlanTypes.value.filter(item => item !== value)
    : [...selectedPlanTypes.value, value]
  updatePlanTypes(nextPlanTypes)
}

const clearPlanTypes = () => {
  if (selectedPlanTypes.value.length === 0) {
    planTypeDropdownOpen.value = false
    return
  }
  updatePlanTypes([])
}

const handleClickOutside = (event: MouseEvent) => {
  if (!planTypeDropdownRef.value) return
  const target = event.target as Node | null
  if (target && !planTypeDropdownRef.value.contains(target)) {
    planTypeDropdownOpen.value = false
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
})

const pOpts = computed(() => [
  { value: '', label: t('admin.accounts.allPlatforms') },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'openai', label: 'OpenAI' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'antigravity', label: 'Antigravity' }
])
const tOpts = computed(() => [
  { value: '', label: t('admin.accounts.allTypes') },
  { value: 'oauth', label: t('admin.accounts.oauthType') },
  { value: 'setup-token', label: t('admin.accounts.setupToken') },
  { value: 'apikey', label: t('admin.accounts.apiKey') },
  { value: 'bedrock', label: 'AWS Bedrock' }
])
const sOpts = computed(() => [
  { value: '', label: t('admin.accounts.allStatus') },
  { value: 'active', label: t('admin.accounts.status.active') },
  { value: 'inactive', label: t('admin.accounts.status.inactive') },
  { value: 'error', label: t('admin.accounts.status.error') },
  { value: 'rate_limited', label: t('admin.accounts.status.rateLimited') },
  { value: 'temp_unschedulable', label: t('admin.accounts.status.tempUnschedulable') },
  { value: 'unschedulable', label: t('admin.accounts.status.unschedulable') }
])
const planTypeOpts = computed(() => [
  { value: 'free', label: t('admin.accounts.planType.free') },
  { value: 'team', label: t('admin.accounts.planType.team') },
  { value: 'plus', label: t('admin.accounts.planType.plus') },
  { value: 'pro', label: t('admin.accounts.planType.pro') },
  { value: 'unknown', label: t('admin.accounts.planType.unknown') }
])
const privacyOpts = computed(() => [
  { value: '', label: t('admin.accounts.allPrivacyModes') },
  { value: '__unset__', label: t('admin.accounts.privacyUnset') },
  { value: 'training_off', label: 'Privacy' },
  { value: 'training_set_cf_blocked', label: 'CF' },
  { value: 'training_set_failed', label: 'Fail' }
])
const gOpts = computed(() => [
  { value: '', label: t('admin.accounts.allGroups') },
  { value: 'ungrouped', label: t('admin.accounts.ungroupedGroup') },
  ...(props.groups || []).map(g => ({ value: String(g.id), label: g.name }))
])

const selectedPlanTypeLabels = computed(() => {
  const labelMap = new Map(planTypeOpts.value.map(option => [String(option.value), option.label]))
  return selectedPlanTypes.value.map(value => labelMap.get(value) || value)
})

const planTypeButtonLabel = computed(() => {
  if (selectedPlanTypeLabels.value.length === 0) {
    return t('admin.accounts.allPlanTypes')
  }
  if (selectedPlanTypeLabels.value.length <= 2) {
    return selectedPlanTypeLabels.value.join(' / ')
  }
  return t('admin.accounts.planTypesSelected', { count: selectedPlanTypeLabels.value.length })
})

const planTypeButtonTitle = computed(() => {
  if (selectedPlanTypeLabels.value.length === 0) {
    return t('admin.accounts.allPlanTypes')
  }
  return selectedPlanTypeLabels.value.join(', ')
})
</script>
