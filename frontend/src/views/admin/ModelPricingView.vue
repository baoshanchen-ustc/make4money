<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <div class="flex-1">
            <p class="text-sm text-gray-500 dark:text-dark-400">
              {{ t('admin.modelPricings.description') }}
            </p>
          </div>
          <div class="flex items-center gap-2">
            <button @click="loadEntries" :disabled="loading" class="btn btn-secondary" :title="t('common.refresh')">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button @click="openCreate" class="btn btn-primary">
              <Icon name="plus" size="md" class="mr-1" />
              {{ t('admin.modelPricings.addEntry') }}
            </button>
          </div>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="entries" :loading="loading">
          <!-- Model key + display name -->
          <template #cell-model_key="{ value, row }">
            <div>
              <div class="font-mono text-sm font-medium text-gray-900 dark:text-white">{{ value }}</div>
              <div v-if="row.display_name" class="text-xs text-gray-500 dark:text-dark-400">{{ row.display_name }}</div>
            </div>
          </template>

          <!-- Input price -->
          <template #cell-input_price_per_million="{ value, row }">
            <div class="text-sm">
              <span class="font-medium">${{ formatPrice(value) }}</span>
              <span v-if="row.input_price_per_million_priority > 0" class="ml-2 text-xs text-amber-600 dark:text-amber-400">
                P: ${{ formatPrice(row.input_price_per_million_priority) }}
              </span>
            </div>
          </template>

          <!-- Output price -->
          <template #cell-output_price_per_million="{ value, row }">
            <div class="text-sm">
              <span class="font-medium">${{ formatPrice(value) }}</span>
              <span v-if="row.output_price_per_million_priority > 0" class="ml-2 text-xs text-amber-600 dark:text-amber-400">
                P: ${{ formatPrice(row.output_price_per_million_priority) }}
              </span>
            </div>
          </template>

          <!-- Cache prices -->
          <template #cell-cache_read_price_per_million="{ value, row }">
            <div class="text-xs text-gray-600 dark:text-dark-300">
              <span v-if="value > 0">读 ${{ formatPrice(value) }}</span>
              <span v-if="row.cache_creation_price_per_million > 0" :class="value > 0 ? 'ml-2' : ''">
                建 ${{ formatPrice(row.cache_creation_price_per_million) }}
              </span>
              <span v-if="value === 0 && row.cache_creation_price_per_million === 0" class="text-gray-400">—</span>
            </div>
          </template>

          <!-- Enabled badge -->
          <template #cell-enabled="{ value }">
            <span :class="['badge', value ? 'badge-success' : 'badge-gray']">
              {{ value ? t('common.enabled') : t('common.disabled') }}
            </span>
          </template>

          <!-- Note -->
          <template #cell-note="{ value }">
            <span class="text-xs text-gray-500 dark:text-dark-400">{{ value || '—' }}</span>
          </template>

          <!-- Actions -->
          <template #cell-actions="{ row }">
            <div class="flex items-center gap-2">
              <button @click="openEdit(row)" class="btn btn-xs btn-secondary">
                {{ t('common.edit') }}
              </button>
              <button @click="confirmDelete(row)" class="btn btn-xs btn-danger">
                {{ t('admin.modelPricings.deleteEntry') }}
              </button>
            </div>
          </template>
        </DataTable>
      </template>
    </TablePageLayout>

    <!-- Billing Logic Info Card -->
    <div class="mt-4 rounded-lg border border-blue-200 bg-blue-50 p-4 dark:border-blue-800 dark:bg-blue-900/20">
      <h3 class="mb-2 text-sm font-semibold text-blue-800 dark:text-blue-200">
        {{ t('admin.modelPricings.calculationLogic') }}
      </h3>
      <ol class="list-decimal pl-5 text-xs text-blue-700 dark:text-blue-300 space-y-1">
        <li>
          <strong>LiteLLM 动态价格</strong>（最高优先）— 从 GitHub 远程 JSON 文件实时拉取，支持精确/模糊匹配
        </li>
        <li>
          <strong>数据库配置价格</strong>（本页面）— 按 model_key 精确匹配，可在此页面增删改
        </li>
        <li>
          <strong>内置 Fallback 价格</strong>（兜底）— 硬编码在 <code class="rounded bg-blue-100 px-1 dark:bg-blue-900">billing_service.go</code> 中，支持模糊匹配规则
        </li>
      </ol>
      <p class="mt-2 text-xs text-blue-600 dark:text-blue-400">
        💡 数据库价格按 <strong>model_key 精确匹配</strong>，模型名称需与请求中的模型名（小写后）完全一致。<br />
        🔶 <strong>Priority 价格</strong>：OpenAI 的优先服务层（<code class="rounded bg-blue-100 px-1 dark:bg-blue-900">service_tier: "auto"</code>）会收取约 2× 标准价，可在此单独配置；其他厂商无需填写此字段，留 0 即可。
      </p>
    </div>

    <!-- Create / Edit Dialog -->
    <BaseDialog :show="showDialog" :title="isEditing ? t('admin.modelPricings.editEntry') : t('admin.modelPricings.addEntry')" @close="closeDialog">
      <form @submit.prevent="submitForm" class="space-y-4">
        <!-- model_key + display_name -->
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="label">{{ t('admin.modelPricings.modelKey') }} *</label>
            <input v-model="form.model_key" type="text" class="input font-mono" :placeholder="'e.g. gpt-5.1'" required />
          </div>
          <div>
            <label class="label">{{ t('admin.modelPricings.displayName') }}</label>
            <input v-model="form.display_name" type="text" class="input" :placeholder="'e.g. GPT-5.1'" />
          </div>
        </div>

        <!-- price fields in a grid -->
        <div class="rounded-md border border-gray-200 dark:border-dark-600 p-3">
          <p class="mb-3 text-xs font-medium text-gray-500 dark:text-dark-400 uppercase tracking-wide">
            {{ t('admin.modelPricings.usdPerMillion') }}
          </p>
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="label">{{ t('admin.modelPricings.inputPrice') }}</label>
              <input v-model.number="form.input_price_per_million" type="number" step="any" min="0" class="input" />
            </div>
            <div>
              <label class="label">{{ t('admin.modelPricings.outputPrice') }}</label>
              <input v-model.number="form.output_price_per_million" type="number" step="any" min="0" class="input" />
            </div>
            <!-- Priority fields with explanation -->
            <div class="col-span-2">
              <!-- Priority explanation callout -->
              <div v-if="!showPriorityTip" class="mb-2 flex items-center gap-1.5">
                <span class="text-xs font-medium text-amber-700 dark:text-amber-400 uppercase tracking-wide">
                  Priority 服务层价格
                </span>
                <button type="button" @click="showPriorityTip = true"
                  class="inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs text-amber-600 dark:text-amber-400 hover:bg-amber-100 dark:hover:bg-amber-900/30 transition-colors"
                  :title="t('admin.modelPricings.priorityTipTitle')">
                  <svg class="h-3.5 w-3.5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                    <circle cx="12" cy="12" r="10"/><path d="M12 16v-4M12 8h.01"/>
                  </svg>
                  {{ t('admin.modelPricings.priorityTipTitle') }}
                </button>
              </div>
              <div v-else class="mb-3 rounded-md border border-amber-200 bg-amber-50 p-3 dark:border-amber-800 dark:bg-amber-900/20">
                <div class="flex items-start justify-between gap-2">
                  <div class="flex items-start gap-2">
                    <svg class="mt-0.5 h-4 w-4 flex-shrink-0 text-amber-600 dark:text-amber-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                      <circle cx="12" cy="12" r="10"/><path d="M12 16v-4M12 8h.01"/>
                    </svg>
                    <div>
                      <p class="text-xs font-semibold text-amber-800 dark:text-amber-200 mb-1">
                        {{ t('admin.modelPricings.priorityTipTitle') }}
                      </p>
                      <p class="text-xs text-amber-700 dark:text-amber-300 leading-relaxed">
                        {{ t('admin.modelPricings.priorityTipBody') }}
                      </p>
                    </div>
                  </div>
                  <button type="button" @click="showPriorityTip = false" class="flex-shrink-0 text-amber-500 hover:text-amber-700 dark:text-amber-400 dark:hover:text-amber-200">
                    <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/>
                    </svg>
                  </button>
                </div>
              </div>
            </div>
            <div>
              <label class="label">{{ t('admin.modelPricings.inputPricePriority') }}</label>
              <input v-model.number="form.input_price_per_million_priority" type="number" step="any" min="0" class="input" :placeholder="t('admin.modelPricings.priorityPlaceholder')" />
            </div>
            <div>
              <label class="label">{{ t('admin.modelPricings.outputPricePriority') }}</label>
              <input v-model.number="form.output_price_per_million_priority" type="number" step="any" min="0" class="input" :placeholder="t('admin.modelPricings.priorityPlaceholder')" />
            </div>
            <div>
              <label class="label">{{ t('admin.modelPricings.cacheReadPrice') }}</label>
              <input v-model.number="form.cache_read_price_per_million" type="number" step="any" min="0" class="input" />
            </div>
            <div>
              <label class="label">{{ t('admin.modelPricings.cacheReadPricePriority') }}</label>
              <input v-model.number="form.cache_read_price_per_million_priority" type="number" step="any" min="0" class="input" :placeholder="t('admin.modelPricings.priorityPlaceholder')" />
            </div>
            <div>
              <label class="label">{{ t('admin.modelPricings.cacheCreationPrice') }}</label>
              <input v-model.number="form.cache_creation_price_per_million" type="number" step="any" min="0" class="input" />
            </div>
          </div>
        </div>

        <!-- enabled + note -->
        <div class="flex items-center gap-3">
          <label class="flex items-center gap-2 cursor-pointer">
            <input v-model="form.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />
            <span class="text-sm">{{ t('admin.modelPricings.enabled') }}</span>
          </label>
        </div>
        <div>
          <label class="label">{{ t('admin.modelPricings.note') }}</label>
          <input v-model="form.note" type="text" class="input" :placeholder="t('admin.modelPricings.note')" />
        </div>

        <div class="flex justify-end gap-2 pt-2">
          <button type="button" @click="closeDialog" class="btn btn-secondary">{{ t('common.cancel') }}</button>
          <button type="submit" :disabled="submitting" class="btn btn-primary">
            {{ submitting ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </form>
    </BaseDialog>

    <!-- Confirm delete -->
    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('admin.modelPricings.confirmDelete')"
      :message="t('admin.modelPricings.confirmDelete')"
      :confirm-text="t('common.delete')"
      :danger="true"
      @confirm="doDelete"
      @cancel="showDeleteDialog = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Icon from '@/components/icons/Icon.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import type { Column } from '@/components/common/types'
import { useAppStore } from '@/stores/app'
import modelPricingsAPI, { type ModelPricingEntry, type UpsertModelPricingRequest } from '@/api/admin/modelPricings'

const { t } = useI18n()
const appStore = useAppStore()

// ── Data ──────────────────────────────────────────────────────────────────────
const entries = ref<ModelPricingEntry[]>([])
const loading = ref(false)

const columns: Column[] = [
  { key: 'model_key', label: t('admin.modelPricings.modelKey') },
  { key: 'input_price_per_million', label: t('admin.modelPricings.inputPrice') },
  { key: 'output_price_per_million', label: t('admin.modelPricings.outputPrice') },
  { key: 'cache_read_price_per_million', label: '缓存价格' },
  { key: 'enabled', label: t('admin.modelPricings.enabled') },
  { key: 'note', label: t('admin.modelPricings.note') },
  { key: 'actions', label: t('common.actions') }
]

async function loadEntries() {
  loading.value = true
  try {
    entries.value = await modelPricingsAPI.list()
  } catch (err: any) {
    appStore.showError(err?.response?.data?.message || err?.message || 'Failed to load')
  } finally {
    loading.value = false
  }
}

// ── Create / Edit ─────────────────────────────────────────────────────────────
const showDialog = ref(false)
const isEditing = ref(false)
const showPriorityTip = ref(false)
const editingId = ref<number | null>(null)
const submitting = ref(false)

const emptyForm = (): UpsertModelPricingRequest => ({
  model_key: '',
  display_name: '',
  input_price_per_million: 0,
  output_price_per_million: 0,
  input_price_per_million_priority: 0,
  output_price_per_million_priority: 0,
  cache_read_price_per_million: 0,
  cache_read_price_per_million_priority: 0,
  cache_creation_price_per_million: 0,
  enabled: false, // 默认禁用，防止意外创建 0 成本计费配置
  note: ''
})

const form = ref<UpsertModelPricingRequest>(emptyForm())

function openCreate() {
  isEditing.value = false
  editingId.value = null
  form.value = emptyForm()
  showPriorityTip.value = false
  showDialog.value = true
}

function openEdit(entry: ModelPricingEntry) {
  isEditing.value = true
  editingId.value = entry.id
  form.value = {
    model_key: entry.model_key,
    display_name: entry.display_name,
    input_price_per_million: entry.input_price_per_million,
    output_price_per_million: entry.output_price_per_million,
    input_price_per_million_priority: entry.input_price_per_million_priority,
    output_price_per_million_priority: entry.output_price_per_million_priority,
    cache_read_price_per_million: entry.cache_read_price_per_million,
    cache_read_price_per_million_priority: entry.cache_read_price_per_million_priority,
    cache_creation_price_per_million: entry.cache_creation_price_per_million,
    enabled: entry.enabled,
    note: entry.note
  }
  showPriorityTip.value = false
  showDialog.value = true
}

function closeDialog() {
  showDialog.value = false
}

async function submitForm() {
  submitting.value = true
  try {
    if (isEditing.value && editingId.value !== null) {
      await modelPricingsAPI.update(editingId.value, form.value)
      appStore.showSuccess(t('admin.modelPricings.updateSuccess'))
    } else {
      await modelPricingsAPI.create(form.value)
      appStore.showSuccess(t('admin.modelPricings.createSuccess'))
    }
    closeDialog()
    loadEntries()
  } catch (err: any) {
    appStore.showError(err?.response?.data?.message || err?.message || 'Failed to save')
  } finally {
    submitting.value = false
  }
}

// ── Delete ────────────────────────────────────────────────────────────────────
const showDeleteDialog = ref(false)
const deletingId = ref<number | null>(null)

function confirmDelete(entry: ModelPricingEntry) {
  deletingId.value = entry.id
  showDeleteDialog.value = true
}

async function doDelete() {
  if (deletingId.value === null) return
  try {
    await modelPricingsAPI.remove(deletingId.value)
    appStore.showSuccess(t('admin.modelPricings.deleteSuccess'))
    showDeleteDialog.value = false
    loadEntries()
  } catch (err: any) {
    appStore.showError(err?.response?.data?.message || err?.message || 'Failed to delete')
  }
}

// ── Helpers ───────────────────────────────────────────────────────────────────
function formatPrice(v: number): string {
  if (v === 0) return '0'
  if (v < 0.01) return v.toFixed(4).replace(/\.?0+$/, '')
  return v.toString()
}

onMounted(loadEntries)
</script>
