<template>
  <div>
    <div v-if="loading" class="py-2 text-xs text-gray-400">Loading...</div>
    <div v-else-if="!requests || requests.items.length === 0" class="py-2 text-xs text-gray-400">
      暂无请求记录
    </div>
    <ul v-else class="space-y-1">
      <li v-for="item in requests.items" :key="item.request_id" class="text-xs">
        <div class="flex items-center gap-2">
          <span
            :class="item.initiator === 'user'
              ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
              : 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'"
            class="rounded px-1.5 py-0.5 font-mono font-semibold uppercase"
          >
            {{ item.initiator }}
          </span>
          <span class="font-medium text-gray-700 dark:text-gray-300">{{ item.model }}</span>
          <span class="text-gray-400">{{ formatTime(item.created_at) }}</span>
          <span v-if="item.duration_ms" class="text-gray-400">{{ item.duration_ms }}ms</span>
          <a
            :href="`/admin/ops/request-inspect?request_id=${item.request_id}`"
            class="ml-auto text-blue-500 hover:text-blue-700"
            target="_blank"
          >↗</a>
        </div>
        <!-- Sub-requests -->
        <ul v-if="item.sub_requests && item.sub_requests.length" class="ml-6 mt-0.5 space-y-0.5">
          <li v-for="sub in item.sub_requests" :key="sub.request_id" class="flex items-center gap-2 text-gray-500 dark:text-gray-400">
            <span class="rounded bg-blue-50 px-1 py-0.5 text-xs font-mono text-blue-600 dark:bg-blue-900/20 dark:text-blue-400">agent</span>
            <span>{{ sub.model }}</span>
            <span>{{ formatTime(sub.created_at) }}</span>
            <span v-if="sub.duration_ms">{{ sub.duration_ms }}ms</span>
          </li>
        </ul>
      </li>
    </ul>
    <!-- Pagination -->
    <div v-if="requests && requests.total > pageSize" class="mt-3 flex items-center justify-between text-xs text-gray-400">
      <span>共 {{ requests.total }} 条</span>
      <div class="flex gap-2">
        <button :disabled="page <= 1" class="disabled:opacity-40" @click="page--; load()">← 上一页</button>
        <span>第 {{ page }} 页</span>
        <button :disabled="page * pageSize >= (requests?.total ?? 0)" class="disabled:opacity-40" @click="page++; load()">下一页 →</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getCopilotUserRequests } from '@/api/admin/copilotAnalytics'
import type { CopilotUserRequestsResult } from '@/api/admin/copilotAnalytics'

const props = defineProps<{ userId: number; date: string }>()

const loading = ref(false)
const requests = ref<CopilotUserRequestsResult | null>(null)
const page = ref(1)
const pageSize = 20

async function load() {
  loading.value = true
  try {
    requests.value = await getCopilotUserRequests(props.userId, {
      date: props.date,
      page: page.value,
      page_size: pageSize,
    })
  } finally {
    loading.value = false
  }
}

function formatTime(iso: string): string {
  return new Date(iso).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

onMounted(load)
</script>
