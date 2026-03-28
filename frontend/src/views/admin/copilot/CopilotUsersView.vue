<template>
  <div class="container mx-auto max-w-7xl px-4 py-6">
    <!-- Header -->
    <div class="mb-6 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
          {{ t('admin.copilot.users.title') }}
        </h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.copilot.users.description') }}
        </p>
      </div>
      <!-- Date picker -->
      <input
        v-model="selectedDate"
        type="date"
        class="rounded-md border border-gray-300 bg-white px-3 py-2 text-sm shadow-sm focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 dark:border-gray-600 dark:bg-gray-700 dark:text-white"
        @change="loadStats"
      />
    </div>

    <!-- Summary cards -->
    <div class="mb-6 grid grid-cols-1 gap-4 sm:grid-cols-3">
      <SummaryCard
        :title="t('admin.copilot.users.premiumRequests')"
        :value="stats?.total_premium_requests ?? 0"
        :loading="loading"
        color="green"
      />
      <SummaryCard
        :title="t('admin.copilot.users.agentRequests')"
        :value="stats?.total_agent_requests ?? 0"
        :loading="loading"
        color="blue"
      />
      <SummaryCard
        :title="t('admin.copilot.users.activeUsers')"
        :value="stats?.active_users ?? 0"
        :loading="loading"
        color="purple"
      />
    </div>

    <!-- Error state -->
    <div v-if="error" class="mb-6 rounded-md bg-red-50 p-4 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-400">
      {{ error }}
    </div>

    <!-- Users table -->
    <div class="rounded-lg border border-gray-200 bg-white shadow-sm dark:border-gray-700 dark:bg-gray-800">
      <div class="flex items-center justify-between border-b border-gray-200 px-4 py-3 dark:border-gray-700">
        <h2 class="text-sm font-semibold text-gray-900 dark:text-white">
          {{ t('admin.copilot.users.userTable') }}
        </h2>
        <input
          v-model="searchQuery"
          type="text"
          :placeholder="t('admin.copilot.users.searchPlaceholder')"
          class="rounded-md border border-gray-300 px-3 py-1.5 text-sm focus:border-blue-500 focus:outline-none dark:border-gray-600 dark:bg-gray-700 dark:text-white"
        />
      </div>
      <div v-if="loading" class="flex h-32 items-center justify-center">
        <LoadingSpinner />
      </div>
      <div v-else-if="filteredUsers.length === 0" class="flex h-32 items-center justify-center text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.copilot.users.noData') }}
      </div>
      <table v-else class="w-full divide-y divide-gray-200 dark:divide-gray-700">
        <thead class="bg-gray-50 dark:bg-gray-900/50">
          <tr>
            <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
              {{ t('admin.copilot.users.username') }}
            </th>
            <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
              Premium
            </th>
            <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
              {{ t('admin.copilot.users.agentCol') }}
            </th>
            <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
              {{ t('admin.copilot.users.models') }}
            </th>
            <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400">
              {{ t('admin.copilot.users.lastRequest') }}
            </th>
            <th class="px-4 py-3" />
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-100 dark:divide-gray-700">
          <template v-for="user in filteredUsers" :key="user.user_id">
            <tr class="hover:bg-gray-50 dark:hover:bg-gray-700/50">
              <td class="px-4 py-3 text-sm font-medium text-gray-900 dark:text-white">
                {{ user.username }}
              </td>
              <td class="px-4 py-3 text-right text-sm">
                <span class="inline-flex items-center rounded-full bg-green-100 px-2 py-0.5 text-xs font-medium text-green-800 dark:bg-green-900/30 dark:text-green-400">
                  {{ user.premium_requests }}
                </span>
              </td>
              <td class="px-4 py-3 text-right text-sm">
                <span class="inline-flex items-center rounded-full bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-800 dark:bg-blue-900/30 dark:text-blue-400">
                  {{ user.agent_requests }}
                </span>
              </td>
              <td class="px-4 py-3 text-xs text-gray-500 dark:text-gray-400">
                {{ user.models?.slice(0, 2).join(', ') }}{{ user.models?.length > 2 ? ` +${user.models.length - 2}` : '' }}
              </td>
              <td class="px-4 py-3 text-xs text-gray-500 dark:text-gray-400">
                {{ user.last_request_at ? formatDateTime(user.last_request_at) : '—' }}
              </td>
              <td class="px-4 py-3 text-right">
                <button
                  class="text-xs text-blue-600 hover:text-blue-800 dark:text-blue-400"
                  @click="toggleExpand(user.user_id)"
                >
                  {{ expandedUsers.has(user.user_id) ? '▲' : '▼' }}
                </button>
              </td>
            </tr>
            <!-- Expanded: request list -->
            <tr v-if="expandedUsers.has(user.user_id)" :key="`exp-${user.user_id}`">
              <td colspan="6" class="bg-gray-50 px-8 py-4 dark:bg-gray-900/30">
                <UserRequestTree :user-id="user.user_id" :date="selectedDate" />
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { getCopilotUserStats } from '@/api/admin/copilotAnalytics'
import type { CopilotUserStatsResult } from '@/api/admin/copilotAnalytics'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import SummaryCard from '@/components/admin/copilot/CopilotSummaryCard.vue'
import UserRequestTree from '@/components/admin/copilot/UserRequestTree.vue'

const { t } = useI18n()

const selectedDate = ref(new Date().toISOString().split('T')[0])
const searchQuery = ref('')
const loading = ref(false)
const error = ref<string | null>(null)
const stats = ref<CopilotUserStatsResult | null>(null)
const expandedUsers = ref(new Set<number>())

const filteredUsers = computed(() => {
  if (!stats.value) return []
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return stats.value.users
  return stats.value.users.filter(u => u.username.toLowerCase().includes(q))
})

async function loadStats() {
  loading.value = true
  error.value = null
  try {
    stats.value = await getCopilotUserStats({ date: selectedDate.value })
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

function toggleExpand(userId: number) {
  if (expandedUsers.value.has(userId)) {
    expandedUsers.value.delete(userId)
  } else {
    expandedUsers.value.add(userId)
  }
}

function formatDateTime(iso: string): string {
  return new Date(iso).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

onMounted(loadStats)
</script>
