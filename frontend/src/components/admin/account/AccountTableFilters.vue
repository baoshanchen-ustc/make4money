<template>
  <div class="flex flex-1 flex-wrap items-center gap-2">
    <SearchInput
      :model-value="searchQuery"
      :placeholder="t('admin.accounts.searchAccounts')"
      class="w-full sm:w-64"
      @update:model-value="$emit('update:searchQuery', $event)"
      @search="$emit('change')"
    />

    <div ref="filterDropdownRef" class="relative">
      <button
        type="button"
        class="btn btn-secondary px-3"
        @click="showFilterMenu = !showFilterMenu"
      >
        <Icon name="filter" size="sm" class="mr-1.5" />
        <span>筛选</span>
        <span
          v-if="activeFilterCount > 0"
          class="ml-1.5 rounded-full bg-primary-100 px-1.5 py-0.5 text-xs font-medium text-primary-700 dark:bg-primary-900/40 dark:text-primary-300"
        >
          {{ activeFilterCount }}
        </span>
      </button>

      <div
        v-if="showFilterMenu"
        class="absolute left-0 z-50 mt-2 w-[min(22rem,calc(100vw-2rem))] rounded-lg border border-gray-200 bg-white p-3 shadow-lg dark:border-dark-600 dark:bg-dark-800"
      >
        <div class="space-y-3">
          <div>
            <div class="mb-2 text-xs font-semibold uppercase text-gray-500 dark:text-dark-400">账号</div>
            <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
              <Select :model-value="filters.platform" :options="platformOptions" @update:model-value="updateFilter('platform', $event)" @change="$emit('change')" />
              <Select :model-value="filters.type" :options="typeOptions" @update:model-value="updateFilter('type', $event)" @change="$emit('change')" />
              <Select :model-value="filters.privacy_mode" :options="privacyOptions" @update:model-value="updateFilter('privacy_mode', $event)" @change="$emit('change')" />
              <Select :model-value="filters.group" :options="groupOptions" @update:model-value="updateFilter('group', $event)" @change="$emit('change')" />
            </div>
          </div>

          <div>
            <div class="mb-2 text-xs font-semibold uppercase text-gray-500 dark:text-dark-400">状态</div>
            <Select :model-value="filters.status" :options="statusOptions" @update:model-value="updateFilter('status', $event)" @change="$emit('change')" />
          </div>

          <div>
            <div class="mb-2 text-xs font-semibold uppercase text-gray-500 dark:text-dark-400">Plan</div>
            <Select :model-value="filters.plan_type" :options="planOptions" @update:model-value="updateFilter('plan_type', $event)" @change="$emit('change')" />
          </div>
        </div>
      </div>
    </div>

    <div v-if="activeTags.length > 0" class="flex flex-wrap items-center gap-1.5">
      <button
        v-for="tag in activeTags"
        :key="tag.key"
        type="button"
        class="inline-flex max-w-[14rem] items-center gap-1 rounded-md bg-gray-100 px-2 py-1 text-xs text-gray-700 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-200 dark:hover:bg-dark-600"
        @click="clearFilter(tag.key)"
      >
        <span class="truncate">{{ tag.label }}</span>
        <Icon name="x" size="xs" />
      </button>
      <button
        type="button"
        class="px-2 py-1 text-xs text-gray-500 hover:text-gray-700 dark:text-dark-400 dark:hover:text-dark-200"
        @click="clearAllFilters"
      >
        全部清除
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import Icon from '@/components/icons/Icon.vue'
import type { AdminGroup } from '@/types'

const props = defineProps<{
  searchQuery: string
  filters: Record<string, any>
  groups?: AdminGroup[]
}>()

const emit = defineEmits(['update:searchQuery', 'update:filters', 'change'])
const { t } = useI18n()

const showFilterMenu = ref(false)
const filterDropdownRef = ref<HTMLElement | null>(null)

const platformOptions = computed(() => [
  { value: '', label: t('admin.accounts.allPlatforms') },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'openai', label: 'OpenAI' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'antigravity', label: 'Antigravity' }
])

const typeOptions = computed(() => [
  { value: '', label: t('admin.accounts.allTypes') },
  { value: 'oauth', label: t('admin.accounts.oauthType') },
  { value: 'setup-token', label: t('admin.accounts.setupToken') },
  { value: 'apikey', label: t('admin.accounts.apiKey') },
  { value: 'bedrock', label: 'AWS Bedrock' }
])

const statusOptions = computed(() => [
  { value: '', label: t('admin.accounts.allStatus') },
  { value: 'active', label: t('admin.accounts.status.active') },
  { value: 'inactive', label: t('admin.accounts.status.inactive') },
  { value: 'error', label: t('admin.accounts.status.error') },
  { value: 'rate_limited', label: t('admin.accounts.status.rateLimited') },
  { value: 'temp_unschedulable', label: t('admin.accounts.status.tempUnschedulable') },
  { value: 'unschedulable', label: t('admin.accounts.status.unschedulable') }
])

const privacyOptions = computed(() => [
  { value: '', label: t('admin.accounts.allPrivacyModes') },
  { value: '__unset__', label: t('admin.accounts.privacyUnset') },
  { value: 'training_off', label: 'Privacy' },
  { value: 'training_set_cf_blocked', label: 'CF' },
  { value: 'training_set_failed', label: 'Fail' }
])

const groupOptions = computed(() => [
  { value: '', label: t('admin.accounts.allGroups') },
  { value: 'ungrouped', label: t('admin.accounts.ungroupedGroup') },
  ...(props.groups || []).map((group) => ({ value: String(group.id), label: group.name }))
])

const planOptions = computed(() => [
  { value: '', label: '全部 Plan' },
  { value: 'free', label: 'Free' },
  { value: 'plus', label: 'Plus' },
  { value: 'team', label: 'Team' },
  { value: 'pro', label: 'Pro' },
  { value: '__unset__', label: '未识别' }
])

const optionGroups = computed<Record<string, Array<{ value: string | number | boolean | null; label: string }>>>(() => ({
  platform: platformOptions.value,
  type: typeOptions.value,
  status: statusOptions.value,
  privacy_mode: privacyOptions.value,
  group: groupOptions.value,
  plan_type: planOptions.value
}))

const filterNames: Record<string, string> = {
  platform: '平台',
  type: '类型',
  status: '状态',
  privacy_mode: '隐私',
  group: '分组',
  plan_type: 'Plan'
}

const filterKeys = ['platform', 'type', 'status', 'privacy_mode', 'group', 'plan_type']

const activeTags = computed(() => filterKeys
  .map((key) => {
    const value = props.filters[key]
    if (value === '' || value === null || value === undefined) return null
    const option = optionGroups.value[key]?.find((item) => String(item.value) === String(value))
    return {
      key,
      label: `${filterNames[key]}: ${option?.label ?? value}`
    }
  })
  .filter(Boolean) as Array<{ key: string; label: string }>)

const activeFilterCount = computed(() => activeTags.value.length)

const updateFilter = (key: string, value: string | number | boolean | null) => {
  emit('update:filters', { ...props.filters, [key]: value ?? '' })
}

const clearFilter = (key: string) => {
  emit('update:filters', { ...props.filters, [key]: '' })
  emit('change')
}

const clearAllFilters = () => {
  const next = { ...props.filters }
  for (const key of filterKeys) {
    next[key] = ''
  }
  emit('update:filters', next)
  emit('change')
}

const handleClickOutside = (event: MouseEvent) => {
  const target = event.target as Node
  if (filterDropdownRef.value && !filterDropdownRef.value.contains(target)) {
    showFilterMenu.value = false
  }
}

onMounted(() => document.addEventListener('click', handleClickOutside))
onUnmounted(() => document.removeEventListener('click', handleClickOutside))
</script>
