<template>
  <div class="flex flex-wrap items-center gap-3">
    <SearchInput
      :model-value="searchQuery"
      :placeholder="t('admin.accounts.searchAccounts')"
      class="w-full sm:w-64"
      @update:model-value="$emit('update:searchQuery', $event)"
      @search="$emit('change')"
    />
    <Select :model-value="filters.platform" class="w-40" :options="pOpts" @update:model-value="updatePlatform" @change="$emit('change')" />
    <Select :model-value="filters.type" class="w-40" :options="tOpts" @update:model-value="updateType" @change="$emit('change')" />
    <Select :model-value="filters.status" class="w-40" :options="sOpts" @update:model-value="updateStatus" @change="$emit('change')" />
    <Select :model-value="filters.privacy_mode" class="w-40" :options="privacyOpts" @update:model-value="updatePrivacyMode" @change="$emit('change')" />
    <Select :model-value="filters.group" class="w-40" :options="gOpts" @update:model-value="updateGroup" @change="$emit('change')" />

    <div class="relative w-full sm:w-72" ref="proxyFilterRef">
      <button type="button" class="proxy-filter-trigger" @click="toggleProxyFilter">
        <span class="truncate">{{ selectedProxyLabel }}</span>
        <Icon name="chevronDown" size="sm" :class="['transition-transform duration-200', proxyFilterOpen && 'rotate-180']" />
      </button>

      <Transition name="proxy-filter-dropdown">
        <div v-if="proxyFilterOpen" class="proxy-filter-dropdown">
          <div class="proxy-filter-search">
            <Icon name="search" size="sm" class="text-gray-400" />
            <input
              ref="proxySearchInputRef"
              v-model="proxySearchQuery"
              type="text"
              :placeholder="t('admin.accounts.searchProxyPlaceholder')"
              class="proxy-filter-search-input"
              @click.stop
            />
          </div>

          <div class="proxy-filter-options">
            <button
              type="button"
              class="proxy-filter-option"
              :class="{ 'proxy-filter-option-selected': !filters.proxy_id }"
              @click="updateProxy('')"
            >
              <span class="truncate">{{ t('admin.accounts.allProxies') }}</span>
              <Icon v-if="!filters.proxy_id" name="check" size="sm" class="text-primary-500" />
            </button>

            <button
              v-for="proxy in filteredProxies"
              :key="proxy.id"
              type="button"
              class="proxy-filter-option"
              :class="{ 'proxy-filter-option-selected': String(filters.proxy_id || '') === String(proxy.id) }"
              @click="updateProxy(String(proxy.id))"
            >
              <div class="min-w-0 flex-1 text-left">
                <div class="truncate font-medium">{{ proxy.name }}</div>
                <div class="truncate text-xs text-gray-500 dark:text-gray-400">
                  {{ proxy.host }}:{{ proxy.port }}
                </div>
              </div>
              <Icon
                v-if="String(filters.proxy_id || '') === String(proxy.id)"
                name="check"
                size="sm"
                class="text-primary-500"
              />
            </button>

            <div v-if="filteredProxies.length === 0" class="proxy-filter-empty">
              {{ t('common.noOptionsFound') }}
            </div>
          </div>
        </div>
      </Transition>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import Icon from '@/components/icons/Icon.vue'
import type { AdminGroup, Proxy } from '@/types'

const props = defineProps<{
  searchQuery: string
  filters: Record<string, any>
  groups?: AdminGroup[]
  proxies?: Proxy[]
}>()

const emit = defineEmits(['update:searchQuery', 'update:filters', 'change'])
const { t } = useI18n()

const proxyFilterOpen = ref(false)
const proxySearchQuery = ref('')
const proxyFilterRef = ref<HTMLElement | null>(null)
const proxySearchInputRef = ref<HTMLInputElement | null>(null)

const updatePlatform = (value: string | number | boolean | null) => { emit('update:filters', { ...props.filters, platform: value }) }
const updateType = (value: string | number | boolean | null) => { emit('update:filters', { ...props.filters, type: value }) }
const updateStatus = (value: string | number | boolean | null) => { emit('update:filters', { ...props.filters, status: value }) }
const updatePrivacyMode = (value: string | number | boolean | null) => { emit('update:filters', { ...props.filters, privacy_mode: value }) }
const updateGroup = (value: string | number | boolean | null) => { emit('update:filters', { ...props.filters, group: value }) }

const filteredProxies = computed(() => {
  const proxies = props.proxies || []
  if (!proxySearchQuery.value) return proxies
  const query = proxySearchQuery.value.toLowerCase()
  return proxies.filter((proxy) => {
    const id = String(proxy.id)
    const name = proxy.name.toLowerCase()
    const host = proxy.host.toLowerCase()
    return id.includes(query) || name.includes(query) || host.includes(query)
  })
})

const selectedProxyLabel = computed(() => {
  const selected = (props.proxies || []).find((proxy) => String(proxy.id) === String(props.filters.proxy_id || ''))
  return selected ? `${selected.name} (${selected.host}:${selected.port})` : t('admin.accounts.allProxies')
})

const updateProxy = (value: string) => {
  emit('update:filters', { ...props.filters, proxy_id: value })
  proxyFilterOpen.value = false
  proxySearchQuery.value = ''
  emit('change')
}

const toggleProxyFilter = () => {
  proxyFilterOpen.value = !proxyFilterOpen.value
  if (proxyFilterOpen.value) {
    nextTick(() => proxySearchInputRef.value?.focus())
  } else {
    proxySearchQuery.value = ''
  }
}

const handleClickOutside = (event: MouseEvent) => {
  if (proxyFilterRef.value && !proxyFilterRef.value.contains(event.target as Node)) {
    proxyFilterOpen.value = false
    proxySearchQuery.value = ''
  }
}

const handleEscape = (event: KeyboardEvent) => {
  if (event.key === 'Escape' && proxyFilterOpen.value) {
    proxyFilterOpen.value = false
    proxySearchQuery.value = ''
  }
}

const pOpts = computed(() => [{ value: '', label: t('admin.accounts.allPlatforms') }, { value: 'anthropic', label: 'Anthropic' }, { value: 'openai', label: 'OpenAI' }, { value: 'gemini', label: 'Gemini' }, { value: 'antigravity', label: 'Antigravity' }, { value: 'sora', label: 'Sora' }])
const tOpts = computed(() => [{ value: '', label: t('admin.accounts.allTypes') }, { value: 'oauth', label: t('admin.accounts.oauthType') }, { value: 'setup-token', label: t('admin.accounts.setupToken') }, { value: 'apikey', label: t('admin.accounts.apiKey') }, { value: 'bedrock', label: 'AWS Bedrock' }])
const sOpts = computed(() => [{ value: '', label: t('admin.accounts.allStatus') }, { value: 'active', label: t('admin.accounts.status.active') }, { value: 'inactive', label: t('admin.accounts.status.inactive') }, { value: 'error', label: t('admin.accounts.status.error') }, { value: 'rate_limited', label: t('admin.accounts.status.rateLimited') }, { value: 'temp_unschedulable', label: t('admin.accounts.status.tempUnschedulable') }])
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

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
  document.addEventListener('keydown', handleEscape)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
  document.removeEventListener('keydown', handleEscape)
})
</script>

<style scoped>
.proxy-filter-trigger {
  @apply flex w-full items-center justify-between gap-2 rounded-xl border border-gray-200 bg-white px-4 py-2.5 text-sm text-gray-900 transition-all duration-200 hover:border-gray-300 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-100 dark:hover:border-dark-500;
}

.proxy-filter-dropdown {
  @apply absolute z-[100] mt-2 w-full overflow-hidden rounded-xl border border-gray-200 bg-white shadow-lg shadow-black/10 dark:border-dark-700 dark:bg-dark-800 dark:shadow-black/30;
}

.proxy-filter-search {
  @apply flex items-center gap-2 border-b border-gray-100 px-3 py-2 dark:border-dark-700;
}

.proxy-filter-search-input {
  @apply flex-1 bg-transparent text-sm text-gray-900 placeholder:text-gray-400 focus:outline-none dark:text-gray-100 dark:placeholder:text-dark-400;
}

.proxy-filter-options {
  @apply max-h-60 overflow-y-auto py-1;
}

.proxy-filter-option {
  @apply flex w-full items-center justify-between gap-2 px-4 py-2.5 text-sm text-gray-700 transition-colors duration-150 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-dark-700;
}

.proxy-filter-option-selected {
  @apply bg-primary-50 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300;
}

.proxy-filter-empty {
  @apply px-4 py-8 text-center text-sm text-gray-500 dark:text-dark-400;
}

.proxy-filter-dropdown-enter-active,
.proxy-filter-dropdown-leave-active {
  transition: all 0.2s ease;
}

.proxy-filter-dropdown-enter-from,
.proxy-filter-dropdown-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}
</style>
