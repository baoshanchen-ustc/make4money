<template>
  <BaseDialog :show="show" :title="t('admin.groups.rateMultipliersTitle')" width="wide" @close="$emit('close')">
    <div v-if="group" class="space-y-4">
      <!-- 分组信息 -->
      <div class="flex flex-wrap items-center gap-3 rounded-lg bg-gray-50 px-4 py-2.5 text-sm dark:bg-dark-700">
        <span class="font-medium text-gray-900 dark:text-white">{{ group.name }}</span>
        <span class="text-gray-400">|</span>
        <span class="text-gray-600 dark:text-gray-400">{{ t('admin.groups.platforms.' + group.platform) }}</span>
        <span class="text-gray-400">|</span>
        <span class="text-gray-600 dark:text-gray-400">
          {{ t('admin.groups.columns.rateMultiplier') }}: {{ group.rate_multiplier }}x
        </span>
      </div>

      <!-- 添加用户 -->
      <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-600">
        <h4 class="mb-2 text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t('admin.groups.addUserRate') }}
        </h4>
        <div class="flex items-end gap-2">
          <div class="relative flex-1">
            <input
              v-model="searchQuery"
              type="text"
              autocomplete="off"
              class="input w-full"
              :placeholder="t('admin.groups.searchUserPlaceholder')"
              @input="handleSearchUsers"
              @focus="showDropdown = true"
            />
            <div
              v-if="showDropdown && searchResults.length > 0"
              class="absolute left-0 right-0 top-full z-10 mt-1 max-h-48 overflow-y-auto rounded-lg border border-gray-200 bg-white shadow-lg dark:border-dark-500 dark:bg-dark-700"
            >
              <button
                v-for="user in searchResults"
                :key="user.id"
                type="button"
                class="flex w-full items-center gap-2 px-3 py-1.5 text-left text-sm hover:bg-gray-50 dark:hover:bg-dark-600"
                @click="selectUser(user)"
              >
                <span class="text-gray-400">#{{ user.id }}</span>
                <span class="text-gray-900 dark:text-white">{{ user.username || user.email }}</span>
                <span v-if="user.username" class="text-xs text-gray-400">{{ user.email }}</span>
              </button>
            </div>
          </div>
          <div class="w-24">
            <input
              v-model.number="newRate"
              type="number"
              step="0.001"
              min="0"
              autocomplete="off"
              class="hide-spinner input w-full"
              placeholder="1.0"
            />
          </div>
          <button
            type="button"
            class="btn btn-primary shrink-0"
            :disabled="!selectedUser || !newRate || addingRate"
            @click="handleAddRate"
          >
            <Icon v-if="addingRate" name="refresh" size="sm" class="mr-1 animate-spin" />
            {{ t('common.add') }}
          </button>
        </div>
      </div>

      <!-- 加载状态 -->
      <div v-if="loading" class="flex justify-center py-6">
        <svg class="h-6 w-6 animate-spin text-primary-500" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
        </svg>
      </div>

      <!-- 已设置的用户列表 -->
      <div v-else>
        <div class="mb-2 flex items-center justify-between">
          <h4 class="text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.groups.rateMultipliers') }} ({{ entries.length }})
          </h4>
          <button
            v-if="entries.length > 0"
            type="button"
            class="btn btn-sm rounded-lg border border-red-200 bg-red-50 px-3 py-1.5 text-sm font-medium text-red-600 transition-colors hover:bg-red-100 dark:border-red-800 dark:bg-red-900/20 dark:text-red-400 dark:hover:bg-red-900/40"
            :disabled="clearing"
            @click="showClearConfirm = true"
          >
            <Icon v-if="clearing" name="refresh" size="sm" class="mr-1 inline animate-spin" />
            {{ t('admin.groups.clearAll') }}
          </button>
        </div>

        <div v-if="entries.length === 0" class="py-6 text-center text-sm text-gray-400 dark:text-gray-500">
          {{ t('admin.groups.noRateMultipliers') }}
        </div>

        <div v-else>
          <!-- 表格：固定表头 + 可滚动内容 -->
          <div class="overflow-hidden rounded-lg border border-gray-200 dark:border-dark-600">
            <div class="max-h-[420px] overflow-y-auto">
              <table class="w-full text-sm">
                <thead class="sticky top-0 z-[1]">
                  <tr class="border-b border-gray-200 bg-gray-50 dark:border-dark-600 dark:bg-dark-700">
                    <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.columns.userEmail') }}</th>
                    <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">ID</th>
                    <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.columns.userName') }}</th>
                    <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.columns.userNotes') }}</th>
                    <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.columns.userStatus') }}</th>
                    <th class="px-3 py-2 text-left text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.groups.columns.rateMultiplier') }}</th>
                    <th class="w-10 px-2 py-2"></th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 dark:divide-dark-600">
                  <tr
                    v-for="entry in paginatedEntries"
                    :key="entry.user_id"
                    class="hover:bg-gray-50 dark:hover:bg-dark-700/50"
                  >
                    <td class="px-3 py-2 text-gray-600 dark:text-gray-400">{{ entry.user_email }}</td>
                    <td class="whitespace-nowrap px-3 py-2 text-gray-400 dark:text-gray-500">{{ entry.user_id }}</td>
                    <td class="whitespace-nowrap px-3 py-2 text-gray-900 dark:text-white">{{ entry.user_name || '-' }}</td>
                    <td class="max-w-[160px] truncate px-3 py-2 text-gray-500 dark:text-gray-400" :title="entry.user_notes">{{ entry.user_notes || '-' }}</td>
                    <td class="whitespace-nowrap px-3 py-2">
                      <span
                        :class="[
                          'inline-flex rounded-full px-2 py-0.5 text-xs font-medium',
                          entry.user_status === 'active'
                            ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                            : 'bg-gray-100 text-gray-600 dark:bg-dark-600 dark:text-gray-400'
                        ]"
                      >
                        {{ entry.user_status }}
                      </span>
                    </td>
                    <td class="whitespace-nowrap px-3 py-2">
                      <input
                        type="number"
                        step="0.001"
                        min="0"
                        autocomplete="off"
                        :value="entry.rate_multiplier"
                        class="hide-spinner w-20 rounded border border-gray-200 bg-white px-2 py-1 text-center text-sm font-medium transition-colors focus:border-primary-500 focus:outline-none focus:ring-1 focus:ring-primary-500/20 dark:border-dark-500 dark:bg-dark-700 dark:focus:border-primary-500"
                        @blur="handleUpdateRate(entry, ($event.target as HTMLInputElement).value)"
                        @keydown.enter="($event.target as HTMLInputElement).blur()"
                      />
                    </td>
                    <td class="px-2 py-2">
                      <button
                        type="button"
                        class="rounded p-1 text-gray-400 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
                        @click="handleDeleteRate(entry)"
                      >
                        <Icon name="trash" size="sm" />
                      </button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <!-- 分页 -->
          <Pagination
            v-if="entries.length > pageSize"
            :total="entries.length"
            :page="currentPage"
            :page-size="pageSize"
            :page-size-options="[20, 50, 100]"
            @update:page="currentPage = $event"
            @update:pageSize="handlePageSizeChange"
          />
        </div>
      </div>
    </div>
  </BaseDialog>

  <!-- 全部清空确认弹框 -->
  <ConfirmDialog
    :show="showClearConfirm"
    :title="t('admin.groups.clearAll')"
    :message="t('admin.groups.confirmClearAll')"
    :confirm-text="t('admin.groups.clearAll')"
    :cancel-text="t('common.cancel')"
    :danger="true"
    @confirm="handleClearAll"
    @cancel="showClearConfirm = false"
  />
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { GroupRateMultiplierEntry } from '@/api/admin/groups'
import type { AdminGroup, AdminUser } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  show: boolean
  group: AdminGroup | null
}>()

const emit = defineEmits<{
  close: []
  success: []
}>()

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const entries = ref<GroupRateMultiplierEntry[]>([])
const searchQuery = ref('')
const searchResults = ref<AdminUser[]>([])
const showDropdown = ref(false)
const selectedUser = ref<AdminUser | null>(null)
const newRate = ref<number | null>(null)
const addingRate = ref(false)
const clearing = ref(false)
const showClearConfirm = ref(false)
const currentPage = ref(1)
const pageSize = ref(20)

let searchTimeout: ReturnType<typeof setTimeout>

const paginatedEntries = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return entries.value.slice(start, start + pageSize.value)
})

const loadEntries = async () => {
  if (!props.group) return
  loading.value = true
  try {
    entries.value = await adminAPI.groups.getGroupRateMultipliers(props.group.id)
    const totalPages = Math.max(1, Math.ceil(entries.value.length / pageSize.value))
    if (currentPage.value > totalPages) {
      currentPage.value = totalPages
    }
  } catch (error) {
    appStore.showError(t('admin.groups.failedToLoad'))
    console.error('Error loading group rate multipliers:', error)
  } finally {
    loading.value = false
  }
}

watch(() => props.show, (val) => {
  if (val && props.group) {
    currentPage.value = 1
    loadEntries()
    searchQuery.value = ''
    searchResults.value = []
    selectedUser.value = null
    newRate.value = null
  }
})

const handlePageSizeChange = (newSize: number) => {
  pageSize.value = newSize
  currentPage.value = 1
}

const handleSearchUsers = () => {
  clearTimeout(searchTimeout)
  selectedUser.value = null
  if (!searchQuery.value.trim()) {
    searchResults.value = []
    showDropdown.value = false
    return
  }
  searchTimeout = setTimeout(async () => {
    try {
      const res = await adminAPI.users.list(1, 10, { search: searchQuery.value.trim() })
      searchResults.value = res.items
      showDropdown.value = true
    } catch {
      searchResults.value = []
    }
  }, 300)
}

const selectUser = (user: AdminUser) => {
  selectedUser.value = user
  searchQuery.value = user.email
  showDropdown.value = false
  searchResults.value = []
}

const handleAddRate = async () => {
  if (!selectedUser.value || !newRate.value || !props.group) return
  addingRate.value = true
  try {
    await adminAPI.users.update(selectedUser.value.id, {
      group_rates: { [props.group.id]: newRate.value }
    })
    appStore.showSuccess(t('admin.groups.rateAdded'))
    searchQuery.value = ''
    selectedUser.value = null
    newRate.value = null
    await loadEntries()
    emit('success')
  } catch (error) {
    appStore.showError(t('admin.groups.failedToSave'))
    console.error('Error adding rate multiplier:', error)
  } finally {
    addingRate.value = false
  }
}

const handleUpdateRate = async (entry: GroupRateMultiplierEntry, value: string) => {
  if (!props.group) return
  const numValue = parseFloat(value)
  if (isNaN(numValue) || numValue === entry.rate_multiplier) return
  try {
    await adminAPI.users.update(entry.user_id, {
      group_rates: { [props.group.id]: numValue }
    })
    appStore.showSuccess(t('admin.groups.rateUpdated'))
    await loadEntries()
    emit('success')
  } catch (error) {
    appStore.showError(t('admin.groups.failedToSave'))
    console.error('Error updating rate multiplier:', error)
  }
}

const handleDeleteRate = async (entry: GroupRateMultiplierEntry) => {
  if (!props.group) return
  try {
    await adminAPI.users.update(entry.user_id, {
      group_rates: { [props.group.id]: null }
    })
    appStore.showSuccess(t('admin.groups.rateDeleted'))
    await loadEntries()
    emit('success')
  } catch (error) {
    appStore.showError(t('admin.groups.failedToSave'))
    console.error('Error deleting rate multiplier:', error)
  }
}

const handleClearAll = async () => {
  if (!props.group || entries.value.length === 0) return
  showClearConfirm.value = false
  clearing.value = true
  try {
    await adminAPI.groups.clearGroupRateMultipliers(props.group.id)
    appStore.showSuccess(t('admin.groups.rateCleared'))
    await loadEntries()
    emit('success')
  } catch (error) {
    appStore.showError(t('admin.groups.failedToSave'))
    console.error('Error clearing rate multipliers:', error)
  } finally {
    clearing.value = false
  }
}

// 点击外部关闭下拉
const handleClickOutside = () => {
  showDropdown.value = false
}

if (typeof document !== 'undefined') {
  document.addEventListener('click', handleClickOutside)
}
</script>

<style scoped>
.hide-spinner::-webkit-outer-spin-button,
.hide-spinner::-webkit-inner-spin-button {
  -webkit-appearance: none;
  margin: 0;
}
.hide-spinner {
  -moz-appearance: textfield;
}
</style>
