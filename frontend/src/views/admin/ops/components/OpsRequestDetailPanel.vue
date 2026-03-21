<template>
  <div class="relative h-full">
    <!-- 空状态 -->
    <div
      v-if="!row"
      class="flex min-h-[240px] items-center justify-center px-6 py-10 text-center text-sm text-gray-500 dark:text-gray-400"
    >
      {{ emptyText || t('admin.ops.requestDetails.detailPaneEmpty') }}
    </div>

    <!-- 错误类型：直接内嵌错误详情面板 -->
    <OpsErrorDetailPanel
      v-else-if="row.kind === 'error' && row.error_id"
      :show="true"
      :error-id="row.error_id"
      error-type="request"
      class="h-full"
    />

    <!-- 成功类型 / 无 error_id：展示请求摘要 + usage 入库字段 -->
    <div v-else class="space-y-4 p-6">
      <!-- Request ID -->
      <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-900">
        <div class="text-xs font-bold uppercase tracking-wider text-gray-400">
          {{ t('admin.ops.requestDetails.table.requestId') }}
        </div>
        <div class="mt-1 flex items-center gap-2">
          <span
            class="flex-1 truncate font-mono text-sm font-medium text-gray-900 dark:text-white"
            :title="row.request_id || '—'"
          >
            {{ row.request_id || '—' }}
          </span>
          <button
            v-if="row.request_id"
            class="flex-shrink-0 rounded-md bg-gray-100 px-2 py-1 text-[10px] font-bold text-gray-600 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-300 dark:hover:bg-dark-600"
            @click="handleCopy(row.request_id)"
          >
            {{ t('admin.ops.requestDetails.copy') }}
          </button>
        </div>
      </div>

      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-900">
          <div class="text-xs font-bold uppercase tracking-wider text-gray-400">
            {{ t('admin.ops.requestDetails.table.time') }}
          </div>
          <div class="mt-1 text-sm font-medium text-gray-900 dark:text-white">{{ formatDateTime(row.created_at) }}</div>
        </div>
        <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-900">
          <div class="text-xs font-bold uppercase tracking-wider text-gray-400">
            {{ t('admin.ops.requestDetails.table.kind') }}
          </div>
          <div class="mt-1">
            <span class="rounded-full px-2 py-1 text-[10px] font-bold" :class="kindBadgeClass(row.kind)">
              {{ row.kind === 'error' ? t('admin.ops.requestDetails.kind.error') : t('admin.ops.requestDetails.kind.success') }}
            </span>
          </div>
        </div>
        <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-900">
          <div class="text-xs font-bold uppercase tracking-wider text-gray-400">
            {{ t('admin.ops.requestDetails.table.platform') }}
          </div>
          <div class="mt-1 text-sm font-medium text-gray-900 dark:text-white">{{ (row.platform || '—').toUpperCase() }}</div>
        </div>
        <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-900">
          <div class="text-xs font-bold uppercase tracking-wider text-gray-400">
            {{ t('admin.ops.requestDetails.table.model') }}
          </div>
          <div class="mt-1 truncate text-sm font-medium text-gray-900 dark:text-white" :title="row.model || '—'">
            {{ row.model || '—' }}
          </div>
        </div>
        <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-900">
          <div class="text-xs font-bold uppercase tracking-wider text-gray-400">
            {{ t('admin.ops.requestDetails.table.duration') }}
          </div>
          <div class="mt-1 text-sm font-medium text-gray-900 dark:text-white">
            {{ typeof row.duration_ms === 'number' ? `${row.duration_ms} ms` : '—' }}
          </div>
        </div>
        <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-900">
          <div class="text-xs font-bold uppercase tracking-wider text-gray-400">
            {{ t('admin.ops.requestDetails.table.bodySize') }}
          </div>
          <div class="mt-1 text-sm font-medium text-gray-900 dark:text-white">
            {{ typeof row.request_body_bytes === 'number' ? formatBytes(row.request_body_bytes) : '—' }}
          </div>
        </div>
        <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-900">
          <div class="text-xs font-bold uppercase tracking-wider text-gray-400">
            {{ t('admin.ops.requestDetails.table.status') }}
          </div>
          <div class="mt-1">
            <span
              v-if="displayStatusCode != null"
              :class="['inline-flex items-center rounded-lg px-2 py-1 text-xs font-black ring-1 ring-inset shadow-sm', statusClass]"
            >
              {{ displayStatusCode }}
            </span>
            <span v-else class="text-sm font-medium text-gray-400">—</span>
          </div>
        </div>
      </div>

      <p
        v-if="row.kind === 'success' && usageInspectWindow"
        class="rounded-lg border border-amber-200/80 bg-amber-50/80 px-3 py-2 text-xs text-amber-900 dark:border-amber-800/50 dark:bg-amber-950/30 dark:text-amber-100/90"
      >
        {{ t('admin.ops.requestInspect.usageHint') }}
      </p>

      <div
        v-if="row.kind === 'success' && usageInspectWindow"
        class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800"
      >
        <div class="flex flex-wrap items-center justify-between gap-2">
          <h3 class="text-sm font-black uppercase tracking-wider text-gray-900 dark:text-white">
            {{ t('admin.ops.requestInspect.usageCardTitle') }}
          </h3>
          <div v-if="usageInspectLoading" class="text-xs text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</div>
        </div>
        <div v-if="usageInspectNote && !usageInspectLoading" class="mt-2 text-sm text-gray-600 dark:text-gray-300">
          {{ usageInspectNote }}
        </div>
        <div v-else-if="usageInspect && !usageInspectLoading" class="mt-4 space-y-3 text-sm">
          <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-900">
              <div class="text-[10px] font-bold uppercase text-gray-400">{{ t('admin.ops.requestInspect.fields.clientModel') }}</div>
              <div class="mt-1 font-mono text-xs text-gray-900 dark:text-white">{{ usageInspect.model || '—' }}</div>
            </div>
            <div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-900">
              <div class="text-[10px] font-bold uppercase text-gray-400">{{ t('admin.ops.requestInspect.fields.upstreamModel') }}</div>
              <div class="mt-1 font-mono text-xs text-gray-900 dark:text-white">
                {{ usageUpstreamModelDisplay }}
              </div>
            </div>
            <div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-900">
              <div class="text-[10px] font-bold uppercase text-gray-400">{{ t('admin.ops.requestInspect.fields.inboundEndpoint') }}</div>
              <div class="mt-1 font-mono text-xs text-gray-900 dark:text-white">{{ usageInspect.inbound_endpoint || '—' }}</div>
            </div>
            <div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-900">
              <div class="text-[10px] font-bold uppercase text-gray-400">{{ t('admin.ops.requestInspect.fields.upstreamEndpoint') }}</div>
              <div class="mt-1 font-mono text-xs text-gray-900 dark:text-white">{{ usageInspect.upstream_endpoint || '—' }}</div>
            </div>
            <div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-900">
              <div class="text-[10px] font-bold uppercase text-gray-400">{{ t('admin.ops.requestInspect.fields.tokens') }}</div>
              <div class="mt-1 font-mono text-xs text-gray-900 dark:text-white">
                in {{ usageInspect.input_tokens }} / out {{ usageInspect.output_tokens }}
              </div>
            </div>
            <div class="rounded-lg bg-gray-50 p-3 dark:bg-dark-900">
              <div class="text-[10px] font-bold uppercase text-gray-400">{{ t('admin.ops.requestInspect.fields.stream') }}</div>
              <div class="mt-1 text-xs text-gray-900 dark:text-white">{{ usageInspect.stream ? 'true' : 'false' }}</div>
            </div>
            <div v-if="usageInspect.service_tier" class="rounded-lg bg-gray-50 p-3 dark:bg-dark-900">
              <div class="text-[10px] font-bold uppercase text-gray-400">{{ t('admin.ops.requestInspect.fields.serviceTier') }}</div>
              <div class="mt-1 font-mono text-xs text-gray-900 dark:text-white">{{ usageInspect.service_tier }}</div>
            </div>
            <div v-if="usageInspect.reasoning_effort" class="rounded-lg bg-gray-50 p-3 dark:bg-dark-900">
              <div class="text-[10px] font-bold uppercase text-gray-400">{{ t('admin.ops.requestInspect.fields.reasoningEffort') }}</div>
              <div class="mt-1 font-mono text-xs text-gray-900 dark:text-white">{{ usageInspect.reasoning_effort }}</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useClipboard } from '@/composables/useClipboard'
import { useAppStore } from '@/stores'
import { opsAPI, type OpsRequestDetail, type OpsUsageInspectDetail } from '@/api/admin/ops'
import { formatDateTime } from '../utils/opsFormatters'
import { formatBytes } from '@/utils/format'
import OpsErrorDetailPanel from './OpsErrorDetailPanel.vue'

interface Props {
  row: OpsRequestDetail | null
  emptyText?: string
  /** When set, successful rows load `/admin/ops/usage-inspect` for recorded routing metadata. */
  usageInspectWindow?: { start_time: string; end_time: string } | null
}

const props = withDefaults(defineProps<Props>(), {
  usageInspectWindow: null
})

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

const usageInspect = ref<OpsUsageInspectDetail | null>(null)
const usageInspectLoading = ref(false)
const usageInspectNote = ref<string | null>(null)

async function handleCopy(text: string) {
  const ok = await copyToClipboard(text, t('admin.ops.requestDetails.requestIdCopied'))
  if (!ok) appStore.showWarning(t('admin.ops.requestDetails.copyFailed'))
}

function kindBadgeClass(kind: string) {
  if (kind === 'error') return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
  return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
}

/** 成功行无 HTTP 状态码时按 200 展示（与 ops 列表 CTE 一致） */
const displayStatusCode = computed(() => {
  const r = props.row
  if (!r) return null
  if (r.kind === 'success' && (r.status_code === null || r.status_code === undefined)) {
    return 200
  }
  return r.status_code ?? null
})

/** 计费表仅在 upstream≠client 时写入 upstream_model；与客户端模型相同时回退展示 */
const usageUpstreamModelDisplay = computed(() => {
  const u = usageInspect.value
  if (!u) return '—'
  const up = (u.upstream_model || '').trim()
  if (up) return up
  return (u.model || '').trim() || '—'
})

const statusClass = computed(() => {
  const code = displayStatusCode.value ?? 0
  if (code >= 500) return 'bg-red-50 text-red-700 ring-red-600/20 dark:bg-red-900/30 dark:text-red-400 dark:ring-red-500/30'
  if (code === 429) return 'bg-purple-50 text-purple-700 ring-purple-600/20 dark:bg-purple-900/30 dark:text-purple-400 dark:ring-purple-500/30'
  if (code >= 400) return 'bg-amber-50 text-amber-700 ring-amber-600/20 dark:bg-amber-900/30 dark:text-amber-400 dark:ring-amber-500/30'
  if (code >= 200) return 'bg-green-50 text-green-700 ring-green-600/20 dark:bg-green-900/30 dark:text-green-400 dark:ring-green-500/30'
  return 'bg-gray-50 text-gray-700 ring-gray-600/20 dark:bg-gray-900/30 dark:text-gray-400 dark:ring-gray-500/30'
})

watch(
  () => ({
    kind: props.row?.kind,
    rid: (props.row?.request_id || '').trim(),
    start: props.usageInspectWindow?.start_time || '',
    end: props.usageInspectWindow?.end_time || ''
  }),
  async ({ kind, rid, start, end }) => {
    usageInspect.value = null
    usageInspectNote.value = null
    if (kind !== 'success' || !rid || !start || !end) {
      usageInspectLoading.value = false
      return
    }
    usageInspectLoading.value = true
    try {
      usageInspect.value = await opsAPI.getUsageInspectByRequestId({
        request_id: rid,
        start_time: start,
        end_time: end
      })
    } catch (e: any) {
      const st = e?.response?.status
      if (st === 404) {
        usageInspectNote.value = t('admin.ops.requestInspect.usageNotFound')
      } else {
        usageInspectNote.value = e?.message || t('admin.ops.requestInspect.loadFailed')
      }
    } finally {
      usageInspectLoading.value = false
    }
  },
  { immediate: true }
)
</script>
