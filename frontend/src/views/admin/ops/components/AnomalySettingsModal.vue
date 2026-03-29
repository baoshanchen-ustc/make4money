<template>
  <BaseDialog :show="show" :title="t('admin.ops.anomalySettings.title')" width="wide" @close="close">
    <div v-if="loading" class="py-10 text-center text-sm text-gray-500 dark:text-gray-400">
      {{ t('common.loading') }}
    </div>

    <div v-else class="space-y-6">
      <!-- Threshold inputs -->
      <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
        <div class="rounded-2xl bg-gray-50 p-4 dark:bg-dark-700/50">
          <label for="anomaly-slow-request-ms" class="input-label">
            {{ t('admin.ops.anomalySettings.slowRequestMs') }}
          </label>
          <input
            id="anomaly-slow-request-ms"
            v-model.number="form.slow_request_threshold_ms"
            type="number"
            min="0"
            class="input mt-1"
          />
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.ops.anomalySettings.slowRequestMsHint') }}
          </p>
          <p
            v-if="fieldErrors.slow_request_threshold_ms"
            class="mt-2 text-xs font-medium text-red-600 dark:text-red-400"
          >
            {{ fieldErrors.slow_request_threshold_ms }}
          </p>
        </div>

        <div class="rounded-2xl bg-gray-50 p-4 dark:bg-dark-700/50">
          <label for="anomaly-timeout-ms" class="input-label">
            {{ t('admin.ops.anomalySettings.timeoutMs') }}
          </label>
          <input
            id="anomaly-timeout-ms"
            v-model.number="form.timeout_threshold_ms"
            type="number"
            min="0"
            class="input mt-1"
          />
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.ops.anomalySettings.timeoutMsHint') }}
          </p>
          <p
            v-if="fieldErrors.timeout_threshold_ms"
            class="mt-2 text-xs font-medium text-red-600 dark:text-red-400"
          >
            {{ fieldErrors.timeout_threshold_ms }}
          </p>
        </div>
      </div>

      <!-- Toggle settings -->
      <div class="space-y-3">
        <div class="flex items-start justify-between gap-4 rounded-2xl bg-gray-50 p-4 dark:bg-dark-700/50">
          <div>
            <div class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('admin.ops.anomalySettings.detectZeroToken') }}
            </div>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.ops.anomalySettings.detectZeroTokenHint') }}
            </p>
          </div>
          <Toggle v-model="form.detect_zero_token" />
        </div>

        <div class="flex items-start justify-between gap-4 rounded-2xl bg-gray-50 p-4 dark:bg-dark-700/50">
          <div>
            <div class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ t('admin.ops.anomalySettings.saveRawData') }}
            </div>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.ops.anomalySettings.saveRawDataHint') }}
            </p>
          </div>
          <Toggle v-model="form.save_raw_data" />
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" :disabled="saving" @click="close">
          {{ t('common.cancel') }}
        </button>
        <button
          type="button"
          class="btn btn-primary disabled:cursor-not-allowed disabled:opacity-60"
          :disabled="loading || saving || !isFormValid"
          @click="handleSave"
        >
          {{ saving ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { opsAPI, type AnomalySettings } from '@/api/admin/ops'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Toggle from '@/components/common/Toggle.vue'
import { useAppStore } from '@/stores'
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

interface Props {
  show: boolean
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'saved'): void
}>()

const { t } = useI18n()
const appStore = useAppStore()

const DEFAULT_SETTINGS: AnomalySettings = {
  slow_request_threshold_ms: 20000,
  timeout_threshold_ms: 60000,
  detect_zero_token: true,
  save_raw_data: true
}

const loading = ref(false)
const saving = ref(false)
// Set to true when the initial settings fetch fails, so the form is locked
// and a save cannot silently overwrite the real configuration with defaults.
const loadError = ref(false)

const form = reactive<AnomalySettings>({ ...DEFAULT_SETTINGS })

// Monotonic counter to discard stale responses when the dialog is
// closed and re-opened before the previous request finishes.
let requestSeq = 0

type FieldName = 'slow_request_threshold_ms' | 'timeout_threshold_ms'

const fieldErrors = computed<Partial<Record<FieldName, string>>>(() => {
  const errors: Partial<Record<FieldName, string>> = {}

  if (!Number.isFinite(form.slow_request_threshold_ms) || form.slow_request_threshold_ms < 0) {
    errors.slow_request_threshold_ms = t('admin.ops.anomalySettings.validation.nonNegative')
  }
  if (!Number.isFinite(form.timeout_threshold_ms) || form.timeout_threshold_ms < 0) {
    errors.timeout_threshold_ms = t('admin.ops.anomalySettings.validation.nonNegative')
  }
  if (
    !errors.slow_request_threshold_ms &&
    !errors.timeout_threshold_ms &&
    form.timeout_threshold_ms > 0 &&
    form.slow_request_threshold_ms > 0 &&
    form.timeout_threshold_ms < form.slow_request_threshold_ms
  ) {
    errors.timeout_threshold_ms = t('admin.ops.anomalySettings.validation.timeoutGeSlow')
  }

  return errors
})

const isFormValid = computed(() => !loadError.value && Object.keys(fieldErrors.value).length === 0)

function applySettings(settings: AnomalySettings) {
  form.slow_request_threshold_ms = settings.slow_request_threshold_ms
  form.timeout_threshold_ms = settings.timeout_threshold_ms
  form.detect_zero_token = settings.detect_zero_token
  form.save_raw_data = settings.save_raw_data
}

function close() {
  emit('update:show', false)
}

async function loadSettings() {
  const seq = ++requestSeq
  loading.value = true
  loadError.value = false
  try {
    const settings = await opsAPI.getAnomalySettings()
    if (seq !== requestSeq) return
    applySettings(settings)
  } catch (err: unknown) {
    if (seq !== requestSeq) return
    loadError.value = true
    const msg = (err as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    appStore.showError(msg || t('admin.ops.anomalySettings.loadFailed'))
  } finally {
    if (seq === requestSeq) loading.value = false
  }
}

async function handleSave() {
  if (loading.value || saving.value || !isFormValid.value) return
  saving.value = true
  try {
    await opsAPI.updateAnomalySettings({
      slow_request_threshold_ms: form.slow_request_threshold_ms,
      timeout_threshold_ms: form.timeout_threshold_ms,
      detect_zero_token: form.detect_zero_token,
      save_raw_data: form.save_raw_data
    })
    appStore.showSuccess(t('admin.ops.anomalySettings.saveSuccess'))
    emit('saved')
    close()
  } catch (err: unknown) {
    const msg = (err as { response?: { data?: { detail?: string } } })?.response?.data?.detail
    appStore.showError(msg || t('admin.ops.anomalySettings.saveFailed'))
  } finally {
    saving.value = false
  }
}

watch(
  () => props.show,
  (show) => {
    if (!show) {
      requestSeq += 1
      loading.value = false
      saving.value = false
      loadError.value = false
      return
    }
    applySettings(DEFAULT_SETTINGS)
    void loadSettings()
  },
  { immediate: true }
)
</script>
