<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.rawKeyImportTitle')"
    width="normal"
    close-on-click-outside
    @close="handleClose"
  >
    <form id="raw-key-import-form" class="space-y-4" @submit.prevent="handleImport">
      <div class="text-sm text-gray-600 dark:text-dark-300">
        {{ t('admin.accounts.rawKeyImportHint') }}
      </div>
      <div
        class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-300"
      >
        {{ t('admin.accounts.rawKeyImportFormatHint') }}
      </div>

      <div>
        <label class="input-label">{{ t('admin.accounts.rawKeyImportLabel') }}</label>
        <textarea
          v-model="rawText"
          rows="10"
          class="input font-mono text-sm"
          :placeholder="t('admin.accounts.rawKeyImportPlaceholder')"
        />
      </div>

      <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-dark-300">
        <input v-model="validateAfterImport" type="checkbox" class="rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
        <span>{{ t('admin.accounts.rawKeyImportValidateAfterImport') }}</span>
      </label>

      <div
        v-if="result"
        class="space-y-2 rounded-xl border border-gray-200 p-4 dark:border-dark-700"
      >
        <div class="text-sm font-medium text-gray-900 dark:text-white">
          {{ t('admin.accounts.rawKeyImportResult') }}
        </div>
        <div class="text-sm text-gray-700 dark:text-dark-300">
          {{ t('admin.accounts.rawKeyImportSummary', result) }}
        </div>

        <div v-if="result.results?.length" class="mt-2">
          <div class="text-sm font-medium text-gray-900 dark:text-white">
            {{ t('admin.accounts.rawKeyImportDetails') }}
          </div>
          <div class="mt-2 max-h-56 overflow-auto rounded-lg bg-gray-50 p-3 font-mono text-xs dark:bg-dark-800">
            <div
              v-for="item in result.results"
              :key="`${item.line}-${item.key_preview || item.account_id || item.error}`"
              class="whitespace-pre-wrap"
            >
              {{ formatResultLine(item) }}
            </div>
          </div>
        </div>
      </div>
    </form>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button class="btn btn-secondary" type="button" :disabled="submitting" @click="handleClose">
          {{ t('common.cancel') }}
        </button>
        <button class="btn btn-primary" type="submit" form="raw-key-import-form" :disabled="submitting">
          {{ submitting ? t('admin.accounts.rawKeyImportSubmitting') : t('admin.accounts.rawKeyImportButton') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { adminAPI } from '@/api/admin'
import type { RawAPIKeyImportLineResult, RawAPIKeyImportResult } from '@/api/admin/accounts'
import { useAppStore } from '@/stores/app'

interface Props {
  show: boolean
}

interface Emits {
  (e: 'close'): void
  (e: 'imported'): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const { t } = useI18n()
const appStore = useAppStore()

const rawText = ref('')
const validateAfterImport = ref(true)
const submitting = ref(false)
const result = ref<RawAPIKeyImportResult | null>(null)

watch(
  () => props.show,
  (open) => {
    if (open) {
      rawText.value = ''
      validateAfterImport.value = true
      result.value = null
    }
  }
)

const handleClose = () => {
  if (submitting.value) return
  emit('close')
}

const formatResultLine = (item: RawAPIKeyImportLineResult) => {
  const linePrefix = `#${item.line}`
  const keyPart = item.key_preview || item.account_id || '-'
  if (item.error) {
    return `${linePrefix} ${keyPart} - ${item.error}`
  }
  const parts = [linePrefix, item.platform || '-', String(keyPart)]
  if (item.invalid_disabled) {
    parts.push(t('admin.accounts.rawKeyImportStatusInvalidDisabled'))
  } else if (item.valid) {
    parts.push(t('admin.accounts.rawKeyImportStatusValid'))
  } else if (item.created) {
    parts.push(t('admin.accounts.rawKeyImportStatusCreated'))
  }
  if (item.message) {
    parts.push(item.message)
  }
  return parts.join(' - ')
}

const handleImport = async () => {
  if (!rawText.value.trim()) {
    appStore.showError(t('admin.accounts.rawKeyImportEmpty'))
    return
  }

  submitting.value = true
  try {
    const res = await adminAPI.accounts.importRawAPIKeys({
      raw_text: rawText.value,
      validate_after_import: validateAfterImport.value
    })
    result.value = res

    if (res.failed > 0 || res.invalid_disabled > 0) {
      appStore.showWarning(t('admin.accounts.rawKeyImportFinishedWithIssues', {
        created: res.created,
        invalid_disabled: res.invalid_disabled,
        failed: res.failed
      }))
    } else {
      appStore.showSuccess(t('admin.accounts.rawKeyImportFinished', {
        created: res.created,
        valid: res.valid
      }))
      emit('imported')
    }
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.accounts.rawKeyImportFailed'))
  } finally {
    submitting.value = false
  }
}
</script>
