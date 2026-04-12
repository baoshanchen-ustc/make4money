<!-- frontend/src/views/admin/copilot/CopilotPlatformConfigView.vue -->
<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- Page Header -->
      <div>
        <h1 class="text-2xl font-bold text-gray-900 dark:text-white">
          {{ t('admin.copilot.platformConfig.title') }}
        </h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.copilot.platformConfig.description') }}
        </p>
      </div>

      <!-- Error Banner -->
      <div v-if="loadError" class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-400">
        {{ loadError }}
      </div>

      <!-- Loading Skeleton -->
      <div v-if="loading" class="grid gap-6 lg:grid-cols-2 xl:grid-cols-3">
        <div v-for="n in 5" :key="n" class="h-80 animate-pulse rounded-xl bg-gray-100 dark:bg-dark-800" />
      </div>

      <!-- Config Cards -->
      <div v-else class="grid gap-6 lg:grid-cols-2 xl:grid-cols-3">
        <div
          v-for="entry in localEntries"
          :key="entry.plan_type"
          class="rounded-xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-800"
        >
          <!-- Card Header -->
          <h2 class="mb-4 text-base font-semibold text-gray-900 dark:text-white">
            {{ planLabel(entry.plan_type) }}
          </h2>

          <div class="space-y-4">
            <!-- max_output_tokens -->
            <div>
              <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('admin.copilot.platformConfig.fields.maxOutputTokens') }}
              </label>
              <input
                v-model.number="entry.max_output_tokens"
                type="number"
                min="0"
                class="input w-full"
                :placeholder="t('admin.copilot.platformConfig.fields.maxOutputTokensHint')"
              />
              <p class="mt-1 text-xs text-gray-400">{{ t('admin.copilot.platformConfig.fields.maxOutputTokensHint') }}</p>
            </div>

            <!-- max_body_kb -->
            <div>
              <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('admin.copilot.platformConfig.fields.maxBodyKB') }}
              </label>
              <input
                v-model.number="entry.max_body_kb"
                type="number"
                min="0"
                class="input w-full"
                :placeholder="t('admin.copilot.platformConfig.fields.maxBodyKBHint')"
              />
              <p class="mt-1 text-xs text-gray-400">{{ t('admin.copilot.platformConfig.fields.maxBodyKBHint') }}</p>
            </div>

            <!-- model_mapping (key-value editor) -->
            <div>
              <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('admin.copilot.platformConfig.fields.modelMapping') }}
              </label>
              <div class="space-y-2">
                <div
                  v-for="(row, idx) in mappingRows(entry.plan_type)"
                  :key="idx"
                  class="flex items-center gap-2"
                >
                  <input
                    v-model="row.from"
                    type="text"
                    class="input flex-1 text-sm"
                    placeholder="from"
                    @input="syncMapping(entry.plan_type)"
                  />
                  <span class="text-gray-400">→</span>
                  <input
                    v-model="row.to"
                    type="text"
                    class="input flex-1 text-sm"
                    placeholder="to"
                    @input="syncMapping(entry.plan_type)"
                  />
                  <button
                    type="button"
                    class="text-gray-400 hover:text-red-500"
                    @click="removeMappingRow(entry.plan_type, idx)"
                  >
                    <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                    </svg>
                  </button>
                </div>
              </div>
              <button
                type="button"
                class="mt-2 text-xs text-primary-600 hover:underline dark:text-primary-400"
                @click="addMappingRow(entry.plan_type)"
              >
                + {{ t('admin.accounts.addMapping') }}
              </button>
            </div>

            <!-- model_whitelist -->
            <div>
              <label class="mb-1 block text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('admin.copilot.platformConfig.fields.modelWhitelist') }}
              </label>
              <ModelWhitelistSelector
                v-model="entry.model_whitelist"
                platform="copilot"
              />
              <p class="mt-1 text-xs text-gray-400">{{ t('admin.copilot.platformConfig.fields.modelWhitelistHint') }}</p>
            </div>
          </div>

          <!-- Save Button -->
          <div class="mt-4 flex items-center justify-between">
            <span v-if="saveErrors[entry.plan_type]" class="text-xs text-red-500">{{ saveErrors[entry.plan_type] }}</span>
            <span v-else-if="saveSuccess[entry.plan_type]" class="text-xs text-green-600 dark:text-green-400">
              {{ t('admin.copilot.platformConfig.saveSuccess') }}
            </span>
            <span v-else class="flex-1" />
            <button
              class="btn btn-primary text-sm"
              :disabled="savingMap[entry.plan_type]"
              @click="saveEntry(entry)"
            >
              <span v-if="savingMap[entry.plan_type]">{{ t('admin.copilot.platformConfig.saving') }}</span>
              <span v-else>{{ t('common.save') }}</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import ModelWhitelistSelector from '@/components/account/ModelWhitelistSelector.vue'
import {
  listCopilotPlatformConfigs,
  updateCopilotPlatformConfig,
  type CopilotPlatformConfigEntry,
  type CopilotPlanType,
  COPILOT_PLAN_TYPES,
} from '@/api/admin/copilotPlatformConfig'
import { useAppStore } from '@/stores'

const { t } = useI18n()
const appStore = useAppStore()

// ─────────────────────────── State ───────────────────────────

const loading = ref(true)
const loadError = ref('')
const localEntries = ref<CopilotPlatformConfigEntry[]>([])

// 每个 plan_type 独立的 saving / error / success 状态
const savingMap = reactive<Record<string, boolean>>({})
const saveErrors = reactive<Record<string, string>>({})
const saveSuccess = reactive<Record<string, boolean>>({})

// model_mapping 多行编辑器中间态（{ from, to }[] per plan_type）
const mappingRowsMap = reactive<Record<string, Array<{ from: string; to: string }>>>({})

// ─────────────────────────── Helpers ───────────────────────────

const PLAN_LABELS: Record<CopilotPlanType, string> = {
  individual_free: 'Free',
  individual_pro: 'Pro',
  individual_pro_plus: 'Pro+',
  business: 'Business',
  enterprise: 'Enterprise',
}

function planLabel(planType: string): string {
  return t(`admin.copilot.platformConfig.planLabels.${planType}`, PLAN_LABELS[planType as CopilotPlanType] ?? planType)
}

function mappingRows(planType: string): Array<{ from: string; to: string }> {
  return mappingRowsMap[planType] ?? []
}

function addMappingRow(planType: string) {
  if (!mappingRowsMap[planType]) mappingRowsMap[planType] = []
  mappingRowsMap[planType].push({ from: '', to: '' })
}

function removeMappingRow(planType: string, idx: number) {
  mappingRowsMap[planType]?.splice(idx, 1)
  syncMapping(planType)
}

function syncMapping(planType: string) {
  const entry = localEntries.value.find(e => e.plan_type === planType)
  if (!entry) return
  const rows = mappingRowsMap[planType] ?? []
  entry.model_mapping = Object.fromEntries(
    rows.filter(r => r.from.trim()).map(r => [r.from.trim(), r.to.trim()])
  )
}

// ─────────────────────────── Load ───────────────────────────

async function loadConfigs() {
  loading.value = true
  loadError.value = ''
  try {
    const entries = await listCopilotPlatformConfigs()
    // 确保 5 个 plan_type 都存在（后端始终返回 5 行，但做防御性检查）
    const byPlanType = Object.fromEntries(entries.map(e => [e.plan_type, e]))
    localEntries.value = COPILOT_PLAN_TYPES.map(pt => {
      const existing = byPlanType[pt]
      if (existing) return { ...existing }
      const entry: CopilotPlatformConfigEntry = {
        plan_type: pt,
        max_output_tokens: null,
        max_body_kb: null,
        model_mapping: {},
        model_whitelist: [],
      }
      return entry
    })
    // 初始化 mappingRowsMap
    for (const entry of localEntries.value) {
      mappingRowsMap[entry.plan_type] = Object.entries(entry.model_mapping ?? {}).map(
        ([from, to]) => ({ from, to })
      )
    }
  } catch (err: unknown) {
    loadError.value = err instanceof Error ? err.message : String(err)
  } finally {
    loading.value = false
  }
}

// ─────────────────────────── Save ───────────────────────────

async function saveEntry(entry: CopilotPlatformConfigEntry) {
  const planType = entry.plan_type as CopilotPlanType
  savingMap[planType] = true
  saveErrors[planType] = ''
  saveSuccess[planType] = false

  // 确保 model_mapping 与 rows 同步
  syncMapping(planType)

  // 注意：v-model.number 在输入框清空时会产生空字符串 ""（而非 null），
  // 必须显式 normalize：非空正整数 → number，空值 → null。
  function toNullableInt(v: unknown): number | null {
    if (v === '' || v === null || v === undefined) return null
    const n = Number(v)
    return Number.isFinite(n) && n > 0 ? n : null
  }

  try {
    await updateCopilotPlatformConfig(planType, {
      max_output_tokens: toNullableInt(entry.max_output_tokens),
      max_body_kb: toNullableInt(entry.max_body_kb),
      model_mapping: entry.model_mapping ?? {},
      model_whitelist: entry.model_whitelist ?? [],
    })
    saveSuccess[planType] = true
    appStore.showSuccess(t('admin.copilot.platformConfig.saveSuccess'))
    setTimeout(() => { saveSuccess[planType] = false }, 3000)
  } catch (err: unknown) {
    saveErrors[planType] = err instanceof Error ? err.message : String(err)
  } finally {
    savingMap[planType] = false
  }
}

onMounted(loadConfigs)
</script>
