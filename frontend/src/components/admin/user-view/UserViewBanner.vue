<template>
  <div class="mb-6 rounded-lg border border-yellow-200 bg-yellow-50 p-4 dark:border-yellow-700/50 dark:bg-yellow-900/20">
    <div class="flex flex-wrap items-center gap-4">
      <!-- Icon + Label -->
      <div class="flex items-center gap-2 text-yellow-700 dark:text-yellow-400">
        <svg class="h-5 w-5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
          <path stroke-linecap="round" stroke-linejoin="round" d="M2.036 12.322a1.012 1.012 0 010-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.964-7.178z" />
          <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
        </svg>
        <span class="text-sm font-semibold">管理员预览模式</span>
      </div>

      <!-- Selected user info -->
      <div v-if="selectedUser" class="flex items-center gap-2 text-sm text-yellow-700 dark:text-yellow-400">
        <span class="text-yellow-500 dark:text-yellow-500">|</span>
        <span>
          当前查看：<strong>{{ selectedUser.email }}</strong>
          <span class="ml-1 text-yellow-500 dark:text-yellow-600">(ID: {{ selectedUser.id }})</span>
        </span>
      </div>
      <div v-else class="text-sm text-yellow-600 dark:text-yellow-500">
        请选择一个用户以查看其视图
      </div>

      <!-- Spacer -->
      <div class="flex-1"></div>

      <!-- User search combobox -->
      <div class="relative w-full sm:w-80" ref="comboboxRef">
        <div class="relative">
          <svg class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z" />
          </svg>
          <input
            v-model="searchInput"
            type="text"
            placeholder="搜索用户 (邮箱或ID)..."
            class="input w-full pl-9 pr-4 text-sm"
            :class="{ 'rounded-b-none': showDropdown && searchResults.length > 0 }"
            @input="onInput"
            @focus="onFocus"
            @blur="onBlur"
            @keydown.escape="closeDropdown"
            @keydown.down.prevent="highlightNext"
            @keydown.up.prevent="highlightPrev"
            @keydown.enter.prevent="selectHighlighted"
          />
          <div v-if="searching" class="absolute right-3 top-1/2 -translate-y-1/2">
            <svg class="h-4 w-4 animate-spin text-gray-400" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
          </div>
        </div>

        <!-- Dropdown results -->
        <div
          v-if="showDropdown && searchResults.length > 0"
          class="absolute left-0 right-0 top-full z-50 max-h-60 overflow-y-auto rounded-b-lg border border-t-0 border-gray-200 bg-white shadow-lg dark:border-dark-600 dark:bg-dark-800"
        >
          <button
            v-for="(user, index) in searchResults"
            :key="user.id"
            type="button"
            class="flex w-full items-center gap-3 px-4 py-2.5 text-left text-sm transition-colors"
            :class="[
              index === highlightedIndex
                ? 'bg-primary-50 text-primary-700 dark:bg-primary-900/30 dark:text-primary-300'
                : 'text-gray-700 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-dark-700'
            ]"
            @mousedown.prevent="selectUser(user)"
          >
            <div class="flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-full bg-primary-100 dark:bg-primary-900/30">
              <span class="text-xs font-medium text-primary-700 dark:text-primary-300">
                {{ user.email.charAt(0).toUpperCase() }}
              </span>
            </div>
            <div class="min-w-0 flex-1">
              <div class="truncate font-medium">{{ user.email }}</div>
              <div class="text-xs text-gray-400 dark:text-dark-400">ID: {{ user.id }}</div>
            </div>
          </button>
        </div>

        <!-- No results -->
        <div
          v-else-if="showDropdown && searchInput.length >= 1 && !searching && hasSearched"
          class="absolute left-0 right-0 top-full z-50 rounded-b-lg border border-t-0 border-gray-200 bg-white px-4 py-3 text-sm text-gray-500 shadow-lg dark:border-dark-600 dark:bg-dark-800 dark:text-dark-400"
        >
          未找到匹配的用户
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { adminUsageAPI, type SimpleUser } from '@/api/admin/usage'

interface SelectedUser {
  id: number
  email: string
}

const props = defineProps<{
  selectedUser?: SelectedUser | null
}>()

const emit = defineEmits<{
  (e: 'user-selected', user: SelectedUser): void
}>()

const searchInput = ref('')
const searchResults = ref<SimpleUser[]>([])
const searching = ref(false)
const showDropdown = ref(false)
const highlightedIndex = ref(-1)
const hasSearched = ref(false)
const comboboxRef = ref<HTMLElement | null>(null)

let searchTimeout: ReturnType<typeof setTimeout> | null = null

const onInput = () => {
  highlightedIndex.value = -1
  hasSearched.value = false
  if (searchTimeout) {
    clearTimeout(searchTimeout)
  }
  if (searchInput.value.trim().length < 1) {
    searchResults.value = []
    showDropdown.value = false
    return
  }
  searchTimeout = setTimeout(async () => {
    await performSearch()
  }, 300)
}

const performSearch = async () => {
  const query = searchInput.value.trim()
  if (!query) return
  searching.value = true
  try {
    searchResults.value = await adminUsageAPI.searchUsers(query)
    showDropdown.value = true
    hasSearched.value = true
  } catch (error) {
    console.error('Failed to search users:', error)
    searchResults.value = []
  } finally {
    searching.value = false
  }
}

const onFocus = () => {
  if (searchResults.value.length > 0) {
    showDropdown.value = true
  }
}

const onBlur = () => {
  // Delay to allow mousedown on dropdown items to fire first
  setTimeout(() => {
    showDropdown.value = false
  }, 150)
}

const closeDropdown = () => {
  showDropdown.value = false
  highlightedIndex.value = -1
}

const highlightNext = () => {
  if (highlightedIndex.value < searchResults.value.length - 1) {
    highlightedIndex.value++
  }
}

const highlightPrev = () => {
  if (highlightedIndex.value > 0) {
    highlightedIndex.value--
  }
}

const selectHighlighted = () => {
  if (highlightedIndex.value >= 0 && highlightedIndex.value < searchResults.value.length) {
    selectUser(searchResults.value[highlightedIndex.value])
  }
}

const selectUser = (user: SimpleUser) => {
  searchInput.value = user.email
  showDropdown.value = false
  highlightedIndex.value = -1
  emit('user-selected', { id: user.id, email: user.email })
}
</script>
