<template>
  <AppLayout>
    <div class="space-y-6 pb-12">
      <div v-if="!opsEnabled" class="card p-6 text-sm text-gray-600 dark:text-gray-300">
        {{ t('admin.ops.requestInspect.disabled') }}
      </div>

      <template v-else>
        <div>
          <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
            {{ t('admin.ops.requestInspect.title') }}
          </h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.ops.requestInspect.description') }}
          </p>
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
                      v-for="row in items"
                      :key="rowKey(row)"
                      :class="[
                        'group cursor-pointer transition-colors hover:bg-gray-50/80 dark:hover:bg-dark-800/50',
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
                        {{ typeof row.duration_ms === 'number' ? `${row.duration_ms} ms` : '—' }}
                      </td>
                      <td class="whitespace-nowrap px-4 py-2 text-xs text-gray-600 dark:text-gray-300">
                        {{ displayListStatusCode(row) }}
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
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import { useAdminSettingsStore } from '@/stores'
import { opsAPI, type OpsRequestDetail, type OpsRequestDetailsKind } from '@/api/admin/ops'
import { formatDateTime, parseTimeRangeMinutes } from './utils/opsFormatters'
import OpsRequestDetailPanel from './components/OpsRequestDetailPanel.vue'

const { t } = useI18n()
const adminSettingsStore = useAdminSettingsStore()

const opsEnabled = computed(() => adminSettingsStore.opsMonitoringEnabled)

const timeRange = ref('24h')
const kind = ref<OpsRequestDetailsKind>('all')
const platform = ref('')
const q = ref('')

const loading = ref(false)
const items = ref<OpsRequestDetail[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const selectedRow = ref<OpsRequestDetail | null>(null)

const tableCols = computed(() => [
  t('admin.ops.requestDetails.table.time'),
  t('admin.ops.requestDetails.table.kind'),
  t('admin.ops.requestDetails.table.platform'),
  t('admin.ops.requestDetails.table.model'),
  t('admin.ops.requestDetails.table.duration'),
  t('admin.ops.requestDetails.table.status')
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

async function fetchData() {
  if (!opsEnabled.value) return
  loading.value = true
  try {
    const w = inspectWindow.value
    const params = {
      start_time: w.start_time,
      end_time: w.end_time,
      page: page.value,
      page_size: pageSize.value,
      kind: kind.value,
      sort: 'created_at_desc' as const
    }
    const pf = platform.value.trim()
    if (pf) {
      ;(params as Record<string, unknown>).platform = pf
    }
    const qq = q.value.trim()
    if (qq) {
      ;(params as Record<string, unknown>).q = qq
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

watch([timeRange, kind], () => {
  page.value = 1
  fetchData()
})

onMounted(async () => {
  await adminSettingsStore.fetch()
  if (opsEnabled.value) {
    fetchData()
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
