<template>
  <AppLayout>
    <div class="space-y-6 pb-12">
      <div v-if="!opsEnabled" class="card p-6 text-sm text-gray-600 dark:text-gray-300">
        {{ t('admin.ops.requestInspect.disabled') }}
      </div>

      <template v-else>
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
              {{ t('admin.ops.requestInspect.title') }}
            </h1>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.ops.requestInspect.description') }}
            </p>
          </div>
          <button
            type="button"
            class="btn btn-secondary btn-sm flex-shrink-0"
            @click="showAnomalySettings = true"
          >
            {{ t('admin.ops.anomalySettings.title') }}
          </button>
        </div>

        <div class="card flex flex-col gap-4 p-4 lg:flex-row lg:flex-wrap lg:items-end">
          <div class="min-w-[140px]">
            <label class="mb-1 block text-xs font-bold uppercase tracking-wide text-gray-400">
              {{ t('admin.ops.requestInspect.windowLabel') }}
            </label>
            <select v-model="timeRange" class="input w-full">
              <option value="5m">{{ t('admin.ops.timeRange.5m') }}</option>
              <option value="30m">{{ t('admin.ops.timeRange.30m') }}</option>
              <option value="1h">{{ t('admin.ops.timeRange.1h') }}</option>
              <option value="6h">{{ t('admin.ops.timeRange.6h') }}</option>
              <option value="24h">{{ t('admin.ops.timeRange.24h') }}</option>
            </select>
          </div>
          <div class="min-w-[120px]">
            <label class="mb-1 block text-xs font-bold uppercase tracking-wide text-gray-400">
              {{ t('admin.ops.requestInspect.kindFilter') }}
            </label>
            <select v-model="kind" class="input w-full">
              <option value="all">{{ t('admin.ops.requestInspect.kindAll') }}</option>
              <option value="success">{{ t('admin.ops.requestDetails.kind.success') }}</option>
              <option value="error">{{ t('admin.ops.requestDetails.kind.error') }}</option>
            </select>
          </div>
          <!-- User search filter -->
          <div ref="userFilterRef" class="relative min-w-[200px] flex-1">
            <label class="mb-1 block text-xs font-bold uppercase tracking-wide text-gray-400">
              {{ t('admin.ops.requestInspect.userFilter') }}
            </label>
            <div class="relative">
              <input
                v-model="userKeyword"
                type="text"
                class="input w-full pr-8 text-sm"
                :placeholder="t('admin.ops.requestInspect.userPlaceholder')"
                @input="handleUserInput"
                @focus="handleUserFocus"
              />
              <button
                v-if="userKeyword || userId != null"
                type="button"
                class="absolute inset-y-0 right-2 flex items-center text-gray-400 transition-colors hover:text-gray-600 dark:hover:text-gray-200"
                @click="clearUser"
              >
                <Icon name="x" size="xs" />
              </button>
            </div>
            <div
              v-if="showUserDropdown && (userResults.length > 0 || userKeyword.trim())"
              class="absolute z-50 mt-1 max-h-60 w-full overflow-auto rounded-xl border border-gray-200 bg-white py-1 shadow-lg dark:border-dark-700 dark:bg-dark-800"
            >
              <button
                v-for="user in userResults"
                :key="user.id"
                type="button"
                class="flex w-full items-center justify-between gap-3 px-3 py-2 text-left hover:bg-gray-50 dark:hover:bg-dark-700"
                @click="selectUser(user)"
              >
                <span class="truncate text-sm text-gray-800 dark:text-gray-100" :title="user.email">{{ user.email }}</span>
                <span class="flex-shrink-0 font-mono text-xs text-gray-400">#{{ user.id }}</span>
              </button>
              <div
                v-if="userResults.length === 0"
                class="px-3 py-2 text-sm text-gray-400"
              >
                {{ t('common.noOptionsFound') }}
              </div>
            </div>
          </div>
          <!-- Anomaly type multi-select dropdown -->
          <div ref="anomalyFilterRef" class="relative min-w-[200px]">
            <label class="mb-1 block text-xs font-bold uppercase tracking-wide text-gray-400">
              {{ t('admin.ops.requestInspect.anomalyFilter') }}
            </label>
            <button
              type="button"
              class="input flex w-full items-center justify-between gap-2 text-left"
              @click="showAnomalyDropdown = !showAnomalyDropdown"
            >
              <span class="truncate text-sm" :class="anomalyTypes.length > 0 ? 'text-gray-900 dark:text-gray-100' : 'text-gray-400'">
                {{ selectedAnomalyText }}
              </span>
              <Icon name="chevronDown" size="xs" :class="['flex-shrink-0 text-gray-400 transition-transform', showAnomalyDropdown ? 'rotate-180' : '']" />
            </button>
            <div
              v-if="showAnomalyDropdown"
              class="absolute z-50 mt-1 w-full min-w-[200px] rounded-xl border border-gray-200 bg-white p-2 shadow-lg dark:border-dark-700 dark:bg-dark-800"
            >
              <label
                v-for="type in ALL_ANOMALY_TYPES"
                :key="type"
                class="flex cursor-pointer items-center gap-2 rounded-lg px-2 py-2 hover:bg-gray-50 dark:hover:bg-dark-700"
              >
                <input
                  type="checkbox"
                  class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                  :checked="anomalyTypes.includes(type)"
                  @change="toggleAnomalyType(type)"
                />
                <span class="text-sm text-gray-700 dark:text-gray-200">{{ t(`admin.ops.anomaly.types.${type}`) }}</span>
              </label>
              <div v-if="anomalyTypes.length > 0" class="mt-1 border-t border-gray-100 px-2 pt-2 dark:border-dark-700">
                <button
                  type="button"
                  class="text-xs font-semibold text-gray-500 underline-offset-4 hover:text-gray-700 hover:underline dark:text-gray-400 dark:hover:text-gray-200"
                  @click="clearAnomalyTypes"
                >
                  {{ t('admin.ops.anomaly.clearFilter') }}
                </button>
              </div>
            </div>
          </div>
          <div class="min-w-[160px] flex-1">
            <label class="mb-1 block text-xs font-bold uppercase tracking-wide text-gray-400">
              {{ t('admin.ops.requestInspect.platform') }}
            </label>
            <input v-model="platform" type="text" class="input w-full font-mono text-sm" placeholder="anthropic / openai …" />
          </div>
          <div class="min-w-[200px] flex-[2]">
            <label class="mb-1 block text-xs font-bold uppercase tracking-wide text-gray-400">
              {{ t('admin.ops.requestInspect.searchQ') }}
            </label>
            <input
              v-model="q"
              type="text"
              class="input w-full font-mono text-sm"
              :placeholder="t('admin.ops.requestInspect.searchPlaceholder')"
              @keydown.enter.prevent="refresh"
            />
          </div>
          <div class="flex gap-2">
            <button type="button" class="btn btn-primary btn-sm" @click="refresh">
              {{ t('common.search') }}
            </button>
            <button type="button" class="btn btn-secondary btn-sm" @click="fetchData">
              {{ t('common.refresh') }}
            </button>
          </div>
        </div>

        <!-- Fault Owner Filter -->
        <div class="flex items-center gap-1.5 flex-wrap">
          <span class="text-xs font-bold uppercase tracking-wide text-gray-400 mr-1">责任方</span>
          <button
            v-for="opt in [
              { value: 'all',      label: '全部'    },
              { value: 'ok',       label: '正常'    },
              { value: 'client',   label: '客户端'  },
              { value: 'upstream', label: '上游'    },
              { value: 'platform', label: 'sub2api' },
            ]"
            :key="opt.value"
            type="button"
            class="rounded-full px-2.5 py-0.5 text-xs font-medium transition-colors"
            :class="faultOwnerFilter === opt.value
              ? 'bg-gray-900 text-white dark:bg-white dark:text-gray-900'
              : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-400 dark:hover:bg-dark-600'"
            @click="faultOwnerFilter = opt.value as FaultOwner | 'all'"
          >
            {{ opt.label }}
          </button>
        </div>

        <!-- 左右分栏：勿用含逗号的 Tailwind 任意 grid-cols，JIT 易解析失败而退回单列 -->
        <div class="ops-request-inspect-split">
          <div
            class="ops-request-inspect-list flex min-h-0 flex-col overflow-hidden rounded-2xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900"
          >
            <div v-if="loading" class="flex flex-1 items-center justify-center py-16">
              <div class="h-8 w-8 animate-spin rounded-full border-b-2 border-primary-600"></div>
            </div>
            <div v-else-if="items.length === 0" class="flex flex-1 items-center justify-center">
              <div class="px-8 py-12 text-center">
                <div class="text-sm font-medium text-gray-600 dark:text-gray-300">
                  {{ t('admin.ops.requestDetails.empty') }}
                </div>
                <div class="mt-1 text-xs text-gray-400">{{ t('admin.ops.requestDetails.emptyHint') }}</div>
              </div>
            </div>
            <template v-else>
              <div class="min-h-0 flex-1 overflow-auto">
                <table class="w-full border-separate border-spacing-0">
                  <thead class="sticky top-0 z-10 bg-gray-50 dark:bg-dark-800">
                    <tr>
                      <th
                        v-for="col in tableCols"
                        :key="col"
                        class="border-b border-gray-200 px-4 py-2.5 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:border-dark-700 dark:text-dark-400"
                      >
                        {{ col }}
                      </th>
                    </tr>
                  </thead>
                  <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                    <tr
                      v-for="row in filteredItems"
                      :key="rowKey(row)"
                      :class="[
                        'group cursor-pointer transition-colors',
                        anomalyRowClass(row),
                        selectedRow && rowKey(selectedRow) === rowKey(row)
                          ? 'bg-primary-50/70 dark:bg-primary-900/10'
                          : ''
                      ]"
                      @click="selectRow(row)"
                    >
                      <td class="whitespace-nowrap px-4 py-2 font-mono text-xs font-medium text-gray-900 dark:text-gray-200">
                        {{ formatDateTime(row.created_at).split(' ')[1] || '—' }}
                      </td>
                      <td class="whitespace-nowrap px-4 py-2">
                        <span class="rounded-full px-2 py-0.5 text-[10px] font-bold" :class="kindBadgeClass(row.kind)">
                          {{
                            row.kind === 'error'
                              ? t('admin.ops.requestDetails.kind.error')
                              : t('admin.ops.requestDetails.kind.success')
                          }}
                        </span>
                      </td>
                      <!-- 用户 / Key -->
                      <td class="px-4 py-2">
                        <div class="flex flex-col gap-0.5">
                          <span
                            v-if="row.user_name"
                            class="max-w-[120px] truncate text-[11px] font-medium text-gray-800 dark:text-gray-200"
                            :title="row.user_name"
                          >{{ row.user_name }}</span>
                          <span v-else class="text-[11px] text-gray-400">—</span>
                          <span
                            v-if="row.api_key_label"
                            class="max-w-[120px] truncate font-mono text-[10px] text-gray-500 dark:text-gray-400"
                            :title="row.api_key_label"
                          >{{ row.api_key_label }}</span>
                        </div>
                      </td>
                      <!-- 分组 / 账号 -->
                      <td class="px-4 py-2">
                        <div class="flex flex-col gap-0.5">
                          <span
                            v-if="row.group_name"
                            class="max-w-[120px] truncate text-[11px] font-medium text-gray-800 dark:text-gray-200"
                            :title="row.group_name"
                          >{{ row.group_name }}</span>
                          <span v-else class="text-[11px] text-gray-400">—</span>
                          <span
                            v-if="row.account_name"
                            class="max-w-[120px] truncate text-[11px] text-gray-500 dark:text-gray-400"
                            :title="row.account_name"
                          >{{ row.account_name }}</span>
                        </div>
                      </td>
                      <td class="whitespace-nowrap px-4 py-2 text-xs font-medium text-gray-700 dark:text-gray-200">
                        <span
                          class="inline-flex items-center rounded bg-gray-100 px-1.5 py-0.5 text-[10px] font-bold uppercase text-gray-600 dark:bg-dark-700 dark:text-gray-300"
                        >
                          {{ row.platform || '—' }}
                        </span>
                      </td>
                      <td class="px-4 py-2">
                        <div
                          class="max-w-[160px] truncate font-mono text-[11px] text-gray-700 dark:text-gray-300"
                          :title="row.model || ''"
                        >
                          {{ row.model || '—' }}
                        </div>
                      </td>
                      <td class="whitespace-nowrap px-4 py-2 text-xs text-gray-600 dark:text-gray-300">
                        <DurationBadge :ms="row.duration_ms ?? null" />
                      </td>
                      <!-- 认证 -->
                      <td class="whitespace-nowrap px-4 py-2 font-mono text-xs text-gray-500 dark:text-gray-400">
                        {{ row.kind === 'success' && row.auth_latency_ms != null ? `${row.auth_latency_ms} ms` : '—' }}
                      </td>
                      <!-- 路由 -->
                      <td class="whitespace-nowrap px-4 py-2 font-mono text-xs text-gray-500 dark:text-gray-400">
                        {{ row.kind === 'success' && row.routing_latency_ms != null ? `${row.routing_latency_ms} ms` : '—' }}
                      </td>
                      <!-- 上游 -->
                      <td class="whitespace-nowrap px-4 py-2 font-mono text-xs text-gray-500 dark:text-gray-400">
                        {{ row.kind === 'success' && row.upstream_latency_ms != null ? `${row.upstream_latency_ms} ms` : '—' }}
                      </td>
                      <!-- 传输 -->
                      <td class="whitespace-nowrap px-4 py-2 font-mono text-xs text-gray-500 dark:text-gray-400">
                        {{ row.kind === 'success' && row.response_latency_ms != null ? `${row.response_latency_ms} ms` : '—' }}
                      </td>
                      <td class="whitespace-nowrap px-4 py-2 text-xs text-gray-600 dark:text-gray-300">
                        {{ typeof row.request_body_bytes === 'number' ? formatBytes(row.request_body_bytes) : '—' }}
                      </td>
                      <td class="whitespace-nowrap px-4 py-2 text-xs text-gray-600 dark:text-gray-300">
                        {{ displayListStatusCode(row) }}
                      </td>
                      <!-- 责任方标签列 -->
                      <td class="whitespace-nowrap px-4 py-2">
                        <FaultOwnerTag :owner="computeFaultOwner(row)" />
                      </td>
                      <!-- 异常标签列 -->
                      <td class="whitespace-nowrap px-4 py-2">
                        <div v-if="row.anomaly_types && row.anomaly_types.length > 0" class="flex flex-wrap gap-1">
                          <AnomalyBadge
                            v-for="type in row.anomaly_types"
                            :key="type"
                            :type="type"
                            small
                          />
                        </div>
                        <span v-else class="text-xs text-gray-400">—</span>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <Pagination
                v-if="total > 0"
                :total="total"
                :page="page"
                :page-size="pageSize"
                :page-size-options="[10, 20, 50]"
                @update:page="handlePageChange"
                @update:pageSize="handlePageSizeChange"
              />
            </template>
          </div>

          <aside
            class="ops-request-inspect-detail flex min-h-0 flex-col overflow-hidden rounded-2xl border border-gray-200 bg-gray-50/70 dark:border-dark-700 dark:bg-dark-950/40"
          >
            <div class="flex-shrink-0 border-b border-gray-200 px-4 py-3 dark:border-dark-700">
              <h2 class="text-sm font-bold text-gray-900 dark:text-white">
                {{ t('admin.ops.requestInspect.detailTitle') }}
              </h2>
              <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.ops.requestInspect.detailHint') }}
              </p>
            </div>
            <OpsRequestDetailPanel
              class="min-h-0 flex-1 overflow-auto"
              :row="selectedRow"
              :usage-inspect-window="inspectWindow"
              :empty-text="t('admin.ops.requestDetails.detailPaneEmpty')"
            />
          </aside>
        </div>
      </template>
    </div>

    <!-- Anomaly settings modal -->
    <AnomalySettingsModal v-model:show="showAnomalySettings" />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAdminSettingsStore } from '@/stores'
import { opsAPI, computeFaultOwner, type AnomalyType, type FaultOwner, type OpsRequestDetail, type OpsRequestDetailsKind } from '@/api/admin/ops'
import { searchUsers, type SimpleUser } from '@/api/admin/usage'
import { formatDateTime, parseTimeRangeMinutes } from './utils/opsFormatters'
import { formatBytes } from '@/utils/format'
import OpsRequestDetailPanel from './components/OpsRequestDetailPanel.vue'
import DurationBadge from './components/DurationBadge.vue'
import AnomalyBadge from './components/AnomalyBadge.vue'
import AnomalySettingsModal from './components/AnomalySettingsModal.vue'
import FaultOwnerTag from './components/FaultOwnerTag.vue'

const { t } = useI18n()
const adminSettingsStore = useAdminSettingsStore()

const opsEnabled = computed(() => adminSettingsStore.opsMonitoringEnabled)

const timeRange = ref('24h')
const kind = ref<OpsRequestDetailsKind>('all')
const platform = ref('')
const q = ref('')
const faultOwnerFilter = ref<FaultOwner | 'all'>('all')

// User search filter state
const userFilterRef = ref<HTMLElement | null>(null)
const userId = ref<number | undefined>(undefined)
const userKeyword = ref('')
const selectedUser = ref<SimpleUser | null>(null)
const userResults = ref<SimpleUser[]>([])
const showUserDropdown = ref(false)
let userSearchTimeout: ReturnType<typeof setTimeout> | null = null

// Anomaly type multi-select dropdown state
const ALL_ANOMALY_TYPES: AnomalyType[] = ['zero_token', 'slow_request', 'timeout', 'error']
const anomalyFilterRef = ref<HTMLElement | null>(null)
const anomalyTypes = ref<AnomalyType[]>([])
const showAnomalyDropdown = ref(false)
const selectedAnomalyText = computed(() =>
  anomalyTypes.value.length > 0
    ? ALL_ANOMALY_TYPES.filter((at) => anomalyTypes.value.includes(at)).map((at) => t(`admin.ops.anomaly.types.${at}`)).join(' / ')
    : t('admin.ops.requestInspect.anomalyPlaceholder')
)

const showAnomalySettings = ref(false)

const loading = ref(false)
const items = ref<OpsRequestDetail[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const selectedRow = ref<OpsRequestDetail | null>(null)

const filteredItems = computed(() => {
  if (faultOwnerFilter.value === 'all') return items.value
  return items.value.filter(row => computeFaultOwner(row) === faultOwnerFilter.value)
})

const tableCols = computed(() => [
  t('admin.ops.requestDetails.table.time'),
  t('admin.ops.requestDetails.table.kind'),
  t('admin.ops.requestDetails.table.userKey'),
  t('admin.ops.requestDetails.table.groupAccount'),
  t('admin.ops.requestDetails.table.platform'),
  t('admin.ops.requestDetails.table.model'),
  t('admin.ops.requestDetails.table.duration'),
  t('admin.ops.requestDetails.table.authLatency'),
  t('admin.ops.requestDetails.table.routingLatency'),
  t('admin.ops.requestDetails.table.upstreamLatency'),
  t('admin.ops.requestDetails.table.responseLatency'),
  t('admin.ops.requestDetails.table.bodySize'),
  t('admin.ops.requestDetails.table.status'),
  '责任方',
  t('admin.ops.requestDetails.table.anomaly')
])

const inspectWindow = computed(() => {
  const minutes = parseTimeRangeMinutes(timeRange.value)
  const endTime = new Date()
  const startTime = new Date(endTime.getTime() - minutes * 60 * 1000)
  return {
    start_time: startTime.toISOString(),
    end_time: endTime.toISOString()
  }
})

function rowKey(row: OpsRequestDetail) {
  return `${row.kind}|${row.created_at}|${row.request_id || ''}|${row.error_id ?? 0}`
}

function kindBadgeClass(kindVal: string) {
  if (kindVal === 'error') return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
  return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
}

/** Returns a subtle background tint for rows with anomalies */
function anomalyRowClass(row: OpsRequestDetail): string {
  if (!row.anomaly_types || row.anomaly_types.length === 0) {
    return 'hover:bg-gray-50/80 dark:hover:bg-dark-800/50'
  }
  // Prioritize by severity: timeout > error > slow_request > zero_token
  if (row.anomaly_types.includes('timeout')) {
    return 'bg-red-50/40 hover:bg-red-50/70 dark:bg-red-950/10 dark:hover:bg-red-950/20'
  }
  if (row.anomaly_types.includes('error')) {
    return 'bg-rose-50/40 hover:bg-rose-50/70 dark:bg-rose-950/10 dark:hover:bg-rose-950/20'
  }
  if (row.anomaly_types.includes('slow_request')) {
    return 'bg-orange-50/40 hover:bg-orange-50/70 dark:bg-orange-950/10 dark:hover:bg-orange-950/20'
  }
  // zero_token
  return 'bg-amber-50/40 hover:bg-amber-50/70 dark:bg-amber-950/10 dark:hover:bg-amber-950/20'
}

/** 成功行历史上可能无 status_code；与列表 SQL 中 success=200 对齐 */
function displayListStatusCode(row: OpsRequestDetail) {
  if (row.kind === 'success' && (row.status_code === null || row.status_code === undefined)) {
    return 200
  }
  if (row.status_code === null || row.status_code === undefined) return '—'
  return row.status_code
}

function selectRow(row: OpsRequestDetail) {
  selectedRow.value = row
}

function syncSelectedRow() {
  if (selectedRow.value && items.value.some((r) => rowKey(r) === rowKey(selectedRow.value!))) return
  selectedRow.value = items.value.length > 0 ? items.value[0] : null
}

// ---------- User search filter ----------

function debounceUserSearch() {
  if (userSearchTimeout) {
    clearTimeout(userSearchTimeout)
  }
  userSearchTimeout = setTimeout(async () => {
    const keyword = userKeyword.value.trim()
    if (!keyword) {
      userResults.value = []
      return
    }
    try {
      const results = await searchUsers(keyword)
      // Only apply results if keyword hasn't changed during the async call
      if (userKeyword.value.trim() === keyword) {
        userResults.value = results
      }
    } catch {
      if (userKeyword.value.trim() === keyword) {
        userResults.value = []
      }
    }
  }, 300)
}

function handleUserInput() {
  showUserDropdown.value = true
  // If user types something different from selected user, clear the selection and refresh list
  if (selectedUser.value && userKeyword.value.trim() !== selectedUser.value.email) {
    selectedUser.value = null
    userId.value = undefined
    refresh()
  }
  debounceUserSearch()
}

function handleUserFocus() {
  showUserDropdown.value = true
  if (userKeyword.value.trim()) {
    debounceUserSearch()
  }
}

function selectUser(user: SimpleUser) {
  selectedUser.value = user
  userId.value = user.id
  userKeyword.value = user.email
  userResults.value = []
  showUserDropdown.value = false
  refresh()
}

function clearUser() {
  selectedUser.value = null
  userId.value = undefined
  userKeyword.value = ''
  userResults.value = []
  showUserDropdown.value = false
  refresh()
}

// ---------- Anomaly multi-select ----------

function toggleAnomalyType(type: AnomalyType) {
  const next = new Set(anomalyTypes.value)
  if (next.has(type)) {
    next.delete(type)
  } else {
    next.add(type)
  }
  anomalyTypes.value = ALL_ANOMALY_TYPES.filter((item) => next.has(item))
}

function clearAnomalyTypes() {
  anomalyTypes.value = []
}

// ---------- Click outside to close dropdowns ----------

function onDocumentClick(event: MouseEvent) {
  const target = event.target as Node | null
  if (!target) return
  if (userFilterRef.value && !userFilterRef.value.contains(target)) {
    showUserDropdown.value = false
  }
  if (anomalyFilterRef.value && !anomalyFilterRef.value.contains(target)) {
    showAnomalyDropdown.value = false
  }
}

async function fetchData() {
  if (!opsEnabled.value) return
  loading.value = true
  try {
    const w = inspectWindow.value
    const params: Parameters<typeof opsAPI.listRequestDetails>[0] = {
      start_time: w.start_time,
      end_time: w.end_time,
      page: page.value,
      page_size: pageSize.value,
      kind: kind.value,
      sort: 'created_at_desc' as const
    }
    const pf = platform.value.trim()
    if (pf) {
      (params as Record<string, unknown>).platform = pf
    }
    const qq = q.value.trim()
    if (qq) {
      (params as Record<string, unknown>).q = qq
    }
    if (typeof userId.value === 'number') {
      params.user_id = userId.value
    }
    if (anomalyTypes.value.length > 0) {
      params.anomaly_types = anomalyTypes.value
    }
    const res = await opsAPI.listRequestDetails(params)
    items.value = res.items || []
    total.value = res.total || 0
    syncSelectedRow()
  } catch (e) {
    console.error('[OpsRequestInspectView] listRequestDetails failed', e)
    items.value = []
    total.value = 0
    selectedRow.value = null
  } finally {
    loading.value = false
  }
}

function refresh() {
  page.value = 1
  fetchData()
}

function handlePageChange(next: number) {
  page.value = next
  fetchData()
}

function handlePageSizeChange(next: number) {
  pageSize.value = next
  page.value = 1
  fetchData()
}

watch([timeRange, kind, anomalyTypes], () => {
  page.value = 1
  fetchData()
})

onMounted(async () => {
  document.addEventListener('click', onDocumentClick)
  await adminSettingsStore.fetch()
  if (opsEnabled.value) {
    fetchData()
  }
})

onUnmounted(() => {
  document.removeEventListener('click', onDocumentClick)
  if (userSearchTimeout) {
    clearTimeout(userSearchTimeout)
    userSearchTimeout = null
  }
})
</script>

<style scoped>
/* 视口 ≥768px 强制双列；避免 Tailwind 任意值里逗号导致 md:grid-cols-[…] 未产出 */
.ops-request-inspect-split {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  min-height: 70vh;
  width: 100%;
}

@media (min-width: 768px) {
  .ops-request-inspect-split {
    display: grid;
    grid-template-columns: minmax(0, 1.35fr) minmax(280px, 1fr);
    align-items: stretch;
    column-gap: 1rem;
  }
}

.ops-request-inspect-list,
.ops-request-inspect-detail {
  min-width: 0;
}
</style>
